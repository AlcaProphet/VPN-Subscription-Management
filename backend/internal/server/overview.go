// Package server 的管理概览端点：聚合只读状态、资源计数、首次发布清单与动态摘要。
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/approval"
	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/node"
	"vpn-sub/internal/platform"
	"vpn-sub/internal/pool"
	"vpn-sub/internal/proxygroup"
	"vpn-sub/internal/rule"
	"vpn-sub/internal/share"
	"vpn-sub/internal/subscription"
	"vpn-sub/internal/user"
	"vpn-sub/internal/xray"
)

// OverviewHandler 管理概览接入层依赖；所有服务仅用于只读聚合。
type OverviewHandler struct {
	cfg           *config.Service
	mode          string
	platformSvc   *platform.Service
	subSvc        *subscription.Service
	nodeSvc       *node.Service
	ruleSvc       *rule.Service
	shareSvc      *share.Service
	poolSvc       *pool.Service
	proxyGroupSvc *proxygroup.Service
	approvalSvc   *approval.Service
	adminUserSvc  *user.AdminService
	accessSvc     *log.AccessService
	xraySvc       *xray.InstanceService
	extSvc        *xray.ExtService
}

type overviewStatus struct {
	AppMode      string `json:"app_mode"`
	AdvancedMode bool   `json:"advanced_mode"`
	Emergency    bool   `json:"emergency"`
}

type overviewCounts struct {
	Platforms     int `json:"platforms"`
	Subscriptions int `json:"subscriptions"`
	Nodes         int `json:"nodes"`
	UsableNodes   int `json:"usable_nodes"`
	ManualNodes   int `json:"manual_nodes"`
	XrayNodes     int `json:"xray_nodes"`
	Rules         int `json:"rules"`
	Shares        int `json:"shares"`
	Users         int `json:"users"`
	PendingUsers  int `json:"pending_users"`
	Pools         int `json:"pools"`
	ProxyGroups   int `json:"proxy_groups"`
	XrayInstances int `json:"xray_instances"`
	ExtAccounts   int `json:"ext_accounts"`
}

type overviewChecklistItem struct {
	Key         string `json:"key"`
	Done        bool   `json:"done"`
	Manual      bool   `json:"manual,omitempty"`
	Label       string `json:"label"`
	ActionPath  string `json:"action_path"`
	ActionLabel string `json:"action_label"`
}

type overviewRecent struct {
	PendingUsers []approval.PendingUser `json:"pending_users"`
	AccessLogs   []log.AccessLog        `json:"access_logs"`
}

// RegisterOverviewRoutes 注册管理概览端点，叠加会话与管理员双重校验。
func RegisterOverviewRoutes(engine *gin.Engine, h *OverviewHandler, sessionMW, adminMW gin.HandlerFunc) {
	engine.GET("/api/admin/overview", sessionMW, adminMW, h.get)
}

func (h *OverviewHandler) get(c *gin.Context) {
	ctx := c.Request.Context()
	advancedMode, err := h.cfg.GetBoolStrict(ctx, config.KeyAdvancedMode, false)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	platforms, err := h.platformSvc.List(ctx)
	if err != nil {
		handleOverviewError(c, err)
		return
	}
	subscriptions, err := h.subSvc.List(ctx)
	if err != nil {
		handleOverviewError(c, err)
		return
	}
	nodes, err := h.nodeSvc.List(ctx, "")
	if err != nil {
		handleOverviewError(c, err)
		return
	}
	rules, err := h.ruleSvc.List(ctx)
	if err != nil {
		handleOverviewError(c, err)
		return
	}
	shares, err := h.shareSvc.List(ctx)
	if err != nil {
		handleOverviewError(c, err)
		return
	}
	pools, err := h.poolSvc.List(ctx)
	if err != nil {
		handleOverviewError(c, err)
		return
	}
	proxyGroups, err := h.proxyGroupSvc.List(ctx)
	if err != nil {
		handleOverviewError(c, err)
		return
	}
	_, userTotal, err := h.adminUserSvc.List(ctx, user.ListQuery{Page: 1, Size: 1})
	if err != nil {
		handleOverviewError(c, err)
		return
	}
	_, pendingTotal, err := h.approvalSvc.List(ctx, 1, 1)
	if err != nil {
		handleOverviewError(c, err)
		return
	}
	recentPending, err := h.approvalSvc.RecentPending(ctx, 5)
	if err != nil {
		handleOverviewError(c, err)
		return
	}
	recentAccess, _, err := h.accessSvc.Query(ctx, "", "", 1, 5)
	if err != nil {
		handleOverviewError(c, err)
		return
	}

	xrayInstances := 0
	extAccounts := 0
	if advancedMode {
		instances, err := h.xraySvc.List(ctx)
		if err != nil {
			handleOverviewError(c, err)
			return
		}
		extList, err := h.extSvc.ListExt(ctx)
		if err != nil {
			handleOverviewError(c, err)
			return
		}
		xrayInstances = len(instances)
		extAccounts = len(extList)
	}

	counts := overviewCounts{
		Platforms:     len(platforms),
		Subscriptions: len(subscriptions),
		Nodes:         len(nodes),
		Rules:         len(rules),
		Shares:        len(shares),
		Users:         int(userTotal),
		PendingUsers:  int(pendingTotal),
		Pools:         len(pools),
		ProxyGroups:   len(proxyGroups),
		XrayInstances: xrayInstances,
		ExtAccounts:   extAccounts,
	}
	for _, n := range nodes {
		if n.Source == "manual" {
			counts.ManualNodes++
		}
		if n.Source == "xray" {
			counts.XrayNodes++
		}
		if n.Enabled && !n.Missing && (n.Source == "manual" || n.Allocatable) {
			counts.UsableNodes++
		}
	}

	versionActive := false
	for _, sub := range subscriptions {
		if sub.CurrentVersion > 0 {
			versionActive = true
			break
		}
	}
	OK(c, gin.H{
		"status": overviewStatus{
			AppMode:      h.mode,
			AdvancedMode: advancedMode,
			Emergency:    false,
		},
		"counts": counts,
		"checklist": []overviewChecklistItem{
			{Key: "platforms", Done: len(platforms) > 0, Label: "创建至少一个平台", ActionPath: "/admin/platforms", ActionLabel: "创建平台"},
			{Key: "subscriptions", Done: len(subscriptions) > 0, Label: "为平台创建订阅条目", ActionPath: "/admin/subscriptions", ActionLabel: "新建订阅"},
			{Key: "nodes", Done: counts.UsableNodes > 0, Label: "添加至少一个可用节点", ActionPath: "/admin/nodes", ActionLabel: "新建节点"},
			{Key: "version_active", Done: versionActive, Label: "生成并激活首个版本", ActionPath: "/admin/assembly", ActionLabel: "前往装配"},
			{Key: "member_check", Done: false, Manual: true, Label: "以普通用户身份检查", ActionPath: "/", ActionLabel: "查看用户首页"},
		},
		"recent": overviewRecent{PendingUsers: recentPending, AccessLogs: recentAccess},
	})
}

func handleOverviewError(c *gin.Context, err error) {
	Fail(c, http.StatusInternalServerError, err.Error())
}
