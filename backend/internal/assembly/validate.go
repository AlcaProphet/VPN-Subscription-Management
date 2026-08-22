package assembly

import (
	"context"
	"fmt"

	"vpn-sub/internal/node"
)

// 规则类型白名单（Design2 §3.5）。
var validRuleTypes = map[string]bool{
	"DOMAIN": true, "DOMAIN-SUFFIX": true, "DOMAIN-KEYWORD": true,
	"IP-CIDR": true, "IP-CIDR6": true, "PROCESS-NAME": true,
	"PROCESS-NAME-REGEX": true, "USER-AGENT": true,
}

// validate 装配输入校验（渲染前闭环）。
func (s *Service) validate(ctx context.Context, in GenerateInput, ld *loadedData) error {
	switch in.TargetSyntax {
	case ClashYAML, SrSubs, GenericSubs, SrConf:
	default:
		return fmt.Errorf("%w: 非法装配类型", ErrBadRequest)
	}
	// 平台/规则目标存在性与形态匹配
	switch in.TargetSyntax {
	case ClashYAML:
		if ld.platform == nil || ld.platform.ProductType != "yaml" {
			return fmt.Errorf("%w: 目标平台不存在或不是 Clash YAML 产物", ErrBadRequest)
		}
		if !ld.platform.HasSubscription {
			return fmt.Errorf("%w: 请先在订阅管理为该平台创建订阅条目", ErrBadRequest)
		}
	case SrSubs:
		if ld.platform == nil || ld.platform.ProductType != "subs" {
			return fmt.Errorf("%w: 目标平台不存在或不是 SR 节点订阅产物", ErrBadRequest)
		}
		if !ld.platform.HasSubscription {
			return fmt.Errorf("%w: 请先在订阅管理为该平台创建订阅条目", ErrBadRequest)
		}
	case GenericSubs:
		if ld.platform == nil || ld.platform.ProductType != "generic-subs" {
			return fmt.Errorf("%w: 目标平台不存在或不是通用节点订阅产物", ErrBadRequest)
		}
		if !ld.platform.HasSubscription {
			return fmt.Errorf("%w: 请先在订阅管理为该平台创建订阅条目", ErrBadRequest)
		}
	case SrConf:
		if ld.rule == nil {
			return fmt.Errorf("%w: 请选择分流规则实体", ErrBadRequest)
		}
	}
	// 节点可用性（xray 硬校验）
	for _, name := range in.NodeNames {
		nd, ok := ld.nodes[name]
		if !ok {
			return fmt.Errorf("%w: 节点不存在: %s", ErrBadRequest, name)
		}
		if nd.Source == "xray" && (!nd.Enabled || !nd.Allocatable || nd.Missing || !nd.InstanceEnabled) {
			return fmt.Errorf("%w: 节点不可用: %s", ErrBadRequest, name)
		}
	}
	// 代理组勾选与引用校验
	for _, name := range in.GroupNames {
		g, ok := ld.groups[name]
		if !ok {
			return fmt.Errorf("%w: 代理组不存在: %s", ErrBadRequest, name)
		}
		if g.Type == "preset" && !g.Enabled {
			return fmt.Errorf("%w: 预设组已停用，请先启用或移除勾选: %s", ErrBadRequest, name)
		}
		for _, ref := range g.Nodes {
			if !containsString(in.NodeNames, ref) {
				return fmt.Errorf("%w: 组 %s 引用了未勾选或已失效的节点 %s", ErrBadRequest, name, ref)
			}
		}
		for _, ref := range g.Groups {
			// 代理组子组只允许引用 🚀直接连接 / 🌎国外流量；🛟无法归属的流量是 MATCH 兜底终点，不允许作为子组。
			if ref == node.ForceDirect || ref == node.ForceOverseas {
				continue
			}
			if !containsString(in.GroupNames, ref) {
				return fmt.Errorf("%w: 组 %s 引用了未勾选的组 %s", ErrBadRequest, name, ref)
			}
		}
	}
	// 本次装配的代理组节点引用顺序校验：key 必须是已勾选组，value 只能引用该组定义中且已勾选的节点，且不能重复。
	selectedGroupSet := map[string]bool{}
	for _, name := range in.GroupNames {
		selectedGroupSet[name] = true
	}
	selectedNodeSet := map[string]bool{}
	for _, name := range in.NodeNames {
		selectedNodeSet[name] = true
	}
	for gName, order := range in.GroupNodeOrders {
		if !selectedGroupSet[gName] {
			return fmt.Errorf("%w: 代理组节点顺序引用了未勾选的组: %s", ErrBadRequest, gName)
		}
		g, ok := ld.groups[gName]
		if !ok {
			return fmt.Errorf("%w: 代理组节点顺序引用了不存在的组: %s", ErrBadRequest, gName)
		}
		groupNodeSet := map[string]bool{}
		for _, ref := range g.Nodes {
			groupNodeSet[ref] = true
		}
		seen := map[string]bool{}
		for _, ref := range order {
			if !selectedNodeSet[ref] {
				return fmt.Errorf("%w: 组 %s 节点顺序引用了未勾选节点: %s", ErrBadRequest, gName, ref)
			}
			if !groupNodeSet[ref] {
				return fmt.Errorf("%w: 组 %s 节点顺序引用了非本组定义节点: %s", ErrBadRequest, gName, ref)
			}
			if seen[ref] {
				return fmt.Errorf("%w: 组 %s 节点顺序存在重复节点: %s", ErrBadRequest, gName, ref)
			}
			seen[ref] = true
		}
	}
	// 规则目标组必须属于本次输出集合（强制组或已勾选组；sr-conf 允许 PROXY/DIRECT）
	outputGroups := map[string]bool{node.ForceDirect: true, node.ForceOverseas: true, node.ForceFallback: true}
	for _, name := range in.GroupNames {
		outputGroups[name] = true
	}
	checkTarget := func(target string) error {
		if outputGroups[target] {
			return nil
		}
		if in.TargetSyntax == SrConf && (target == "PROXY" || target == "DIRECT") {
			return nil
		}
		return fmt.Errorf("%w: 规则目标组不在本次输出集合: %s", ErrBadRequest, target)
	}
	for _, p := range in.Pools {
		if err := checkTarget(p.Target); err != nil {
			return err
		}
	}
	for _, r := range in.CustomRules {
		if err := checkTarget(r.Target); err != nil {
			return err
		}
		if !validRuleTypes[r.RuleType] {
			return fmt.Errorf("%w: 非法规则类型: %s", ErrBadRequest, r.RuleType)
		}
	}
	// 空产物硬校验
	switch in.TargetSyntax {
	case ClashYAML:
		if len(in.OverseasMembers) == 0 {
			return fmt.Errorf("%w: 『🌎国外流量』组未包含任何节点", ErrBadRequest)
		}
		for _, name := range in.OverseasMembers {
			if !containsString(in.NodeNames, name) {
				return fmt.Errorf("%w: 🌎国外流量成员必须是已勾选节点: %s", ErrBadRequest, name)
			}
			if _, ok := ld.nodes[name]; !ok {
				return fmt.Errorf("%w: 🌎国外流量成员节点不存在: %s", ErrBadRequest, name)
			}
		}
	case SrSubs, GenericSubs:
		if len(in.NodeNames) == 0 || !s.hasLinkableNode(in, ld) {
			return fmt.Errorf("%w: 节点订阅至少需要 1 个可转换链接的节点", ErrBadRequest)
		}
	}
	return nil
}

// hasLinkableNode 判断是否存在当前目标语法可转链接的节点。
func (s *Service) hasLinkableNode(in GenerateInput, ld *loadedData) bool {
	for _, name := range in.NodeNames {
		nd, ok := ld.nodes[name]
		if !ok {
			continue
		}
		p, err := node.GetProtocol(nd.Protocol)
		if err != nil {
			continue
		}
		if in.TargetSyntax == SrSubs && p.LinkMappings.SR {
			return true
		}
		if in.TargetSyntax == GenericSubs && p.LinkMappings.Generic {
			return true
		}
	}
	return false
}
