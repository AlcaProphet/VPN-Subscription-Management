// overlay.go：Clash 覆盖层（Merge + Rules/Proxies/Groups Seq），对应 CVR enhance 的扩展装配模型。
package assembly

import (
	"fmt"
	"strings"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// controlPlaneKeys 与 CVR AuthoritativeFields 对齐的顶层控制面键。
var controlPlaneKeys = []string{
	"mode",
	"redir-port",
	"tproxy-port",
	"mixed-port",
	"socks-port",
	"port",
	"allow-lan",
	"log-level",
	"ipv6",
	"external-controller",
	"external-controller-cors",
	"external-controller-unix",
	"external-controller-pipe",
	"secret",
	"tun",
	"unified-delay",
}

// defaultFieldKeys 固定收尾的 Clash 默认字段。
var defaultFieldKeys = []string{"proxies", "proxy-providers", "proxy-groups", "rule-providers", "rules"}

// overlayXrayNamesKey 是渲染期内部键：在生成预览阶段携带 Xray 动态节点渲染名，
// 使悬空清理不会误删尚未注入的占位引用；最终输出前会移除。
const overlayXrayNamesKey = "__xray_render_names"

// parseSeq 把 YAML 文本解析为 SeqMap；空文本返回零值。
func parseSeq(raw string) (*SeqMap, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return &SeqMap{}, nil
	}
	var m SeqMap
	if err := gyaml.UnmarshalWithOptions([]byte(raw), &m, gyaml.UseOrderedMap()); err != nil {
		return nil, fmt.Errorf("覆盖层 YAML 解析失败: %w", err)
	}
	return &m, nil
}

// hasOverlay 判断是否存在需要应用的覆盖层内容。
func hasOverlay(ov OverlayInput) bool {
	return strings.TrimSpace(ov.MergeYAML) != "" ||
		strings.TrimSpace(ov.RulesYAML) != "" ||
		strings.TrimSpace(ov.ProxiesYAML) != "" ||
		strings.TrimSpace(ov.GroupsYAML) != ""
}

// applySeq 实现 CVR use_seq 语义：prepend + (原列表 - delete) + append。
func applySeq(root *gyaml.MapSlice, field string, seq *SeqMap) error {
	if seq == nil {
		return nil
	}
	seqItems := mapSeqList(root, field)
	deleteSet := make(map[string]bool, len(seq.Delete))
	for _, d := range seq.Delete {
		deleteSet[d] = true
	}

	kept := make([]any, 0, len(seqItems))
	for _, item := range seqItems {
		name := goccyNameOf(item)
		if name != "" && deleteSet[name] {
			continue
		}
		kept = append(kept, item)
	}
	out := make([]any, 0, len(seq.Prepend)+len(kept)+len(seq.Append))
	out = append(out, seq.Prepend...)
	out = append(out, kept...)
	out = append(out, seq.Append...)
	mapSet(root, field, out)

	// CVR use_seq 的 proxies 副作用：新增节点插入第一个 selector/select 组最前；
	// 删除节点同步从所有组移除。
	if field == "proxies" {
		added := seqNames(seq.Prepend, seq.Append)
		if len(added) > 0 || len(deleteSet) > 0 {
			applyProxyGroupSideEffects(root, added, deleteSet)
		}
	}
	return nil
}

// mapSeqList 读取顶层列表字段；不存在时返回空列表。
func mapSeqList(root *gyaml.MapSlice, field string) []any {
	value, ok := mapGet(*root, field)
	if !ok {
		return nil
	}
	items, ok := seqOf(value)
	if !ok {
		return nil
	}
	return items
}

// goccyNameOf 提取映射/字符串条目的名称。
func goccyNameOf(item any) string {
	switch v := item.(type) {
	case string:
		return v
	case gyaml.MapSlice:
		return mapString(v, "name")
	case map[string]any:
		m, _ := yamlMap(v)
		return mapString(m, "name")
	default:
		return ""
	}
}

// seqNames 收集 prepend/append 中新增节点名。
func seqNames(lists ...[]any) []string {
	var out []string
	seen := map[string]bool{}
	for _, list := range lists {
		for _, item := range list {
			name := goccyNameOf(item)
			if name != "" && !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return out
}

// applyProxyGroupSideEffects 处理 proxies seq 对 proxy-groups 的联动。
func applyProxyGroupSideEffects(root *gyaml.MapSlice, added []string, deleteSet map[string]bool) {
	groupsValue, ok := mapGet(*root, "proxy-groups")
	if !ok {
		return
	}
	groups, ok := seqOf(groupsValue)
	if !ok {
		return
	}
	updated := make([]any, 0, len(groups))
	appendedToSelector := false
	for _, raw := range groups {
		group, ok := yamlMap(raw)
		if !ok {
			updated = append(updated, raw)
			continue
		}
		groupProxies, hasProxies := mapGet(group, "proxies")
		var proxies []any
		var proxiesOK bool
		if hasProxies {
			proxies, proxiesOK = seqOf(groupProxies)
			if !proxiesOK {
				// 非列表 proxies 原样保留，不参与删除/插入。
				updated = append(updated, raw)
				continue
			}
		}

		// 删除命中的节点/组名。
		if hasProxies {
			filtered := make([]any, 0, len(proxies))
			for _, p := range proxies {
				if name, ok := p.(string); ok && deleteSet[name] {
					continue
				}
				filtered = append(filtered, p)
			}
			proxies = filtered
		}

		// 新增节点插入第一个普通 selector/select 组最前；系统强制组成员由装配字段独立控制，覆盖层不得隐式改写。
		inserted := false
		if !appendedToSelector && len(added) > 0 && isSelectorGroup(group) && !isForceGroupName(mapString(group, "name")) {
			merged := make([]any, 0, len(added)+len(proxies))
			seen := map[string]bool{}
			for _, name := range added {
				if !seen[name] {
					seen[name] = true
					merged = append(merged, name)
				}
			}
			for _, p := range proxies {
				if name, ok := p.(string); ok {
					if seen[name] {
						continue
					}
					seen[name] = true
				}
				merged = append(merged, p)
			}
			proxies = merged
			appendedToSelector = true
			inserted = true
		}

		if hasProxies || inserted {
			mapSet(&group, "proxies", proxies)
		}
		updated = append(updated, gyaml.MapSlice(group))
	}
	mapSet(root, "proxy-groups", updated)
}

func isSelectorGroup(group gyaml.MapSlice) bool {
	t := strings.ToLower(mapString(group, "type"))
	return t == "select" || t == "selector"
}

func isForceGroupName(name string) bool {
	return name == node.ForceDirect || name == node.ForceOverseas || name == node.ForceFallback
}

// deepMerge 实现 CVR use_merge 语义：MapSlice 递归合并，其余以 patch 覆盖。
func deepMerge(base, patch *gyaml.MapSlice) {
	if patch == nil {
		return
	}
	for _, item := range *patch {
		key, ok := item.Key.(string)
		if !ok {
			continue
		}
		if existing, exists := mapGet(*base, key); exists {
			baseMap, baseOK := yamlMap(existing)
			patchMap, patchOK := yamlMap(item.Value)
			if baseOK && patchOK {
				deepMerge(&baseMap, &patchMap)
				mapSet(base, key, gyaml.MapSlice(baseMap))
				continue
			}
		}
		mapSet(base, key, item.Value)
	}
}

// lowercaseTopLevelKeys 将 merge 顶层 key 小写化，与 CVR use_merge 一致。
func lowercaseTopLevelKeys(root *gyaml.MapSlice) {
	for i := range *root {
		if key, ok := (*root)[i].Key.(string); ok {
			(*root)[i].Key = strings.ToLower(key)
		}
	}
}

// snapshotControlPlane 保存当前存在的控制面键值。
func snapshotControlPlane(root *gyaml.MapSlice) map[string]any {
	out := map[string]any{}
	for _, key := range controlPlaneKeys {
		if value, ok := mapGet(*root, key); ok {
			out[key] = value
		}
	}
	return out
}

// enforceControlPlane 恢复控制面快照；快照缺失的键从最终配置删除。
func enforceControlPlane(root *gyaml.MapSlice, snapshot map[string]any) {
	for _, key := range controlPlaneKeys {
		if value, ok := snapshot[key]; ok {
			mapSet(root, key, value)
		} else {
			mapDelete(root, key)
		}
	}
}

// snapshotDNSIPv6 单独保存 dns.ipv6。
func snapshotDNSIPv6(root *gyaml.MapSlice) (any, bool) {
	dnsRaw, ok := mapGet(*root, "dns")
	if !ok {
		return nil, false
	}
	dns, ok := yamlMap(dnsRaw)
	if !ok {
		return nil, false
	}
	ipv6, ok := mapGet(dns, "ipv6")
	if !ok {
		return nil, false
	}
	return ipv6, true
}

// enforceDNSIPv6 仅在最终文档仍有 dns 映射时恢复 dns.ipv6。
func enforceDNSIPv6(root *gyaml.MapSlice, ipv6 any, has bool) {
	if !has {
		return
	}
	dnsRaw, ok := mapGet(*root, "dns")
	if !ok {
		return
	}
	dns, ok := yamlMap(dnsRaw)
	if !ok {
		return
	}
	mapSet(&dns, "ipv6", ipv6)
	mapSet(root, "dns", gyaml.MapSlice(dns))
}

// cleanupProxyGroups 与 CVR cleanup_proxy_groups 同口径，并保留 COMPATIBLE 内建策略。
func cleanupProxyGroups(root *gyaml.MapSlice) {
	allowed := collectAllowedNames(root)
	allowed[node.ReservedCompatible] = true

	groupsValue, ok := mapGet(*root, "proxy-groups")
	if !ok {
		return
	}
	groups, ok := seqOf(groupsValue)
	if !ok {
		return
	}
	providerNames := collectProviderNames(root)
	updated := make([]any, 0, len(groups))
	for _, raw := range groups {
		group, ok := yamlMap(raw)
		if !ok {
			updated = append(updated, raw)
			continue
		}
		hasValidProvider := false
		if usesRaw, ok := mapGet(group, "use"); ok {
			if uses, ok := seqOf(usesRaw); ok {
				kept := make([]any, 0, len(uses))
				for _, u := range uses {
					name, ok := u.(string)
					if !ok {
						continue
					}
					if providerNames[name] {
						hasValidProvider = true
						kept = append(kept, name)
					}
				}
				mapSet(&group, "use", kept)
			}
		}
		if proxiesRaw, ok := mapGet(group, "proxies"); ok {
			if proxies, ok := seqOf(proxiesRaw); ok {
				kept := make([]any, 0, len(proxies))
				for _, p := range proxies {
					name, isString := p.(string)
					if isString && !allowed[name] && !hasValidProvider {
						continue
					}
					kept = append(kept, p)
				}
				mapSet(&group, "proxies", kept)
			}
		}
		updated = append(updated, gyaml.MapSlice(group))
	}
	mapSet(root, "proxy-groups", updated)
}

func collectAllowedNames(root *gyaml.MapSlice) map[string]bool {
	out := map[string]bool{
		node.ReservedDirect:     true,
		node.ReservedReject:     true,
		node.ReservedRejectDrop: true,
		node.ReservedPass:       true,
		node.ReservedCompatible: true,
	}
	if proxies, ok := mapGet(*root, "proxies"); ok {
		if items, ok := seqOf(proxies); ok {
			for _, item := range items {
				if name := goccyNameOf(item); name != "" {
					out[name] = true
				}
			}
		}
	}
	if groups, ok := mapGet(*root, "proxy-groups"); ok {
		if items, ok := seqOf(groups); ok {
			for _, item := range items {
				if name := goccyNameOf(item); name != "" {
					out[name] = true
				}
			}
		}
	}
	if namesRaw, ok := mapGet(*root, overlayXrayNamesKey); ok {
		if items, ok := seqOf(namesRaw); ok {
			for _, item := range items {
				if name, ok := item.(string); ok && name != "" {
					out[name] = true
				}
			}
		}
	}
	for name := range collectProviderNames(root) {
		out[name] = true
	}
	return out
}

func collectProviderNames(root *gyaml.MapSlice) map[string]bool {
	out := map[string]bool{}
	raw, ok := mapGet(*root, "proxy-providers")
	if !ok {
		return out
	}
	switch v := raw.(type) {
	case gyaml.MapSlice:
		for _, item := range v {
			if key, ok := item.Key.(string); ok && key != "" {
				out[key] = true
			}
		}
	case map[string]any:
		for name := range v {
			out[name] = true
		}
	}
	return out
}

// sortTopLevel 对齐 CVR use_sort：控制面键 → 其他键 → 固定默认字段收尾。
func sortTopLevel(root *gyaml.MapSlice) {
	control := map[string]bool{}
	for _, key := range controlPlaneKeys {
		control[key] = true
	}
	defaults := map[string]bool{}
	for _, key := range defaultFieldKeys {
		defaults[key] = true
	}

	sorted := make(gyaml.MapSlice, 0, len(*root))
	seen := map[string]bool{}
	seenTo := func(m *gyaml.MapSlice, key string) {
		if seen[key] {
			return
		}
		if value, ok := mapGet(*root, key); ok {
			*m = append(*m, gyaml.MapItem{Key: key, Value: value})
			seen[key] = true
		}
	}
	for _, key := range controlPlaneKeys {
		seenTo(&sorted, key)
	}
	for _, item := range *root {
		key, ok := item.Key.(string)
		if !ok || seen[key] || control[key] || defaults[key] {
			continue
		}
		sorted = append(sorted, item)
		seen[key] = true
	}
	for _, key := range defaultFieldKeys {
		seenTo(&sorted, key)
	}
	*root = sorted
}

// applyClashOverlay 单入口：seq → merge → 控制面恢复 → 清理 → 排序。
func applyClashOverlay(root *gyaml.MapSlice, ov OverlayInput) error {
	rules, err := parseSeq(ov.RulesYAML)
	if err != nil {
		return err
	}
	proxies, err := parseSeq(ov.ProxiesYAML)
	if err != nil {
		return err
	}
	groups, err := parseSeq(ov.GroupsYAML)
	if err != nil {
		return err
	}
	if err := applySeq(root, "rules", rules); err != nil {
		return err
	}
	if err := applySeq(root, "proxies", proxies); err != nil {
		return err
	}
	if err := applySeq(root, "proxy-groups", groups); err != nil {
		return err
	}

	control := snapshotControlPlane(root)
	dnsIPv6, hasDNSIPv6 := snapshotDNSIPv6(root)
	if strings.TrimSpace(ov.MergeYAML) != "" {
		var mergeRoot gyaml.MapSlice
		if err := gyaml.UnmarshalWithOptions([]byte(ov.MergeYAML), &mergeRoot, gyaml.UseOrderedMap()); err != nil {
			return fmt.Errorf("Merge YAML 解析失败: %w", err)
		}
		lowercaseTopLevelKeys(&mergeRoot)
		deepMerge(root, &mergeRoot)
	}
	enforceControlPlane(root, control)
	enforceDNSIPv6(root, dnsIPv6, hasDNSIPv6)
	cleanupProxyGroups(root)
	sortTopLevel(root)
	return nil
}

// mapSet 设置顶层/嵌套 MapSlice 字段：已存在则替换值，不存在则追加。
func mapSet(m *gyaml.MapSlice, key string, value any) {
	for i := range *m {
		if k, ok := (*m)[i].Key.(string); ok && k == key {
			(*m)[i].Value = value
			return
		}
	}
	*m = append(*m, gyaml.MapItem{Key: key, Value: value})
}

// mapDelete 删除 MapSlice 字段。
func mapDelete(m *gyaml.MapSlice, key string) {
	out := make(gyaml.MapSlice, 0, len(*m))
	for _, item := range *m {
		if k, ok := item.Key.(string); ok && k == key {
			continue
		}
		out = append(out, item)
	}
	*m = out
}
