package xray

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xtls/xray-core/common/protocol"
	"google.golang.org/protobuf/proto"

	"vpn-sub/internal/tasks"
)

// ReconcileItem 对账结果中的单条记录。
type ReconcileItem struct {
	Email        string `json:"email"`
	Source       string `json:"source"` // user / ext
	UserID       *int64 `json:"user_id,omitempty"`
	ExtAccountID *int64 `json:"ext_account_id,omitempty"`
	InstanceID   int64  `json:"instance_id"`
	InboundTag   string `json:"inbound_tag"`
	NodeID       int64  `json:"node_id"`
	Name         string `json:"name,omitempty"`
	RenderName   string `json:"render_name,omitempty"`
}

// ReconcileResult 对账四分区结果。
type ReconcileResult struct {
	ToPush               []ReconcileItem `json:"to_push"`
	Orphans              []ReconcileItem `json:"orphans"`
	ExtOrphans           []ReconcileItem `json:"ext_orphans"`
	CredentialMismatches []ReconcileItem `json:"credential_mismatches"`
}

// Reconcile 计算指定实例的期望集与实际集并返回四分区。
func (s *SyncService) Reconcile(ctx context.Context, instanceID int64) (*ReconcileResult, error) {
	// 校验实例存在
	var exists int
	if err := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_instances WHERE id = ?`, instanceID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrInstanceNotFound
	}

	desired := map[string]ReconcileItem{}

	// 用户部分：全部 active 用户 ×（组分配 ∪ 公共），经候选集与可用性过滤，再与本实例交集。
	userIDs, err := s.activeUserIDs(ctx)
	if err != nil {
		return nil, err
	}
	for _, uid := range userIDs {
		targets, err := s.Targets(ctx, uid)
		if err != nil {
			return nil, err
		}
		for _, t := range targets {
			if t.InstanceID != instanceID {
				continue
			}
			key := UserEmail(uid) + "|" + t.Tag
			desired[key] = ReconcileItem{
				Email: UserEmail(uid), Source: "user", UserID: &uid,
				InstanceID: instanceID, InboundTag: t.Tag, NodeID: t.NodeID,
				Name: t.Name, RenderName: t.RenderName,
			}
		}
	}

	// 独立账号部分：xray_ext_users 推送目标，仅可用性过滤、不经候选集。
	if s.ext != nil {
		rows, err := s.store.DB().QueryContext(ctx,
			`SELECT xu.ext_account_id, xu.instance_id, xu.inbound_tag, xu.node_id, n.name,
			        COALESCE(NULLIF(n.display_name,''), n.name), n.enabled, n.allocatable, n.missing, i.enabled
			 FROM xray_ext_users xu
			 JOIN nodes n ON n.id = xu.node_id
			 JOIN xray_instances i ON i.id = xu.instance_id
			 WHERE xu.instance_id = ?`, instanceID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var extID, instID, nodeID int64
			var tag, name, renderName string
			var nEnabled, allocatable, missing, iEnabled int
			if err := rows.Scan(&extID, &instID, &tag, &nodeID, &name, &renderName, &nEnabled, &allocatable, &missing, &iEnabled); err != nil {
				_ = rows.Close()
				return nil, err
			}
			_ = instID
			if nEnabled != 1 || allocatable != 1 || missing != 0 || iEnabled != 1 {
				continue
			}
			email := ExtEmail(extID)
			key := email + "|" + tag
			desired[key] = ReconcileItem{
				Email: email, Source: "ext", ExtAccountID: &extID,
				InstanceID: instanceID, InboundTag: tag, NodeID: nodeID,
				Name: name, RenderName: renderName,
			}
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}

	// 实际集：逐个 inbound 读取用户。
	client, err := s.instances.ClientFor(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	tags, err := s.instanceTags(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	actual := map[string]ReconcileItem{}
	actualUsers := map[string]*protocol.User{}
	for _, tag := range tags {
		resp, err := client.GetInboundUsers(ctx, tag, "")
		if err != nil {
			return nil, fmt.Errorf("读取实例入站用户失败: %w", err)
		}
		for _, u := range resp.GetUsers() {
			email := u.GetEmail()
			if email == "" {
				continue
			}
			key := email + "|" + tag
			actual[key] = ReconcileItem{Email: email, InstanceID: instanceID, InboundTag: tag}
			actualUsers[key] = u
		}
	}

	res := &ReconcileResult{ToPush: []ReconcileItem{}, Orphans: []ReconcileItem{}, ExtOrphans: []ReconcileItem{}, CredentialMismatches: []ReconcileItem{}}

	// 待补推 / 凭据不一致
	for key, exp := range desired {
		_, ok := actual[key]
		if !ok {
			res.ToPush = append(res.ToPush, exp)
			continue
		}
		mismatch, err := s.accountMismatch(ctx, exp, actualUsers[key])
		if err != nil {
			// 无法构造期望账号时不阻断，按不匹配处理便于管理员介入。
			mismatch = true
		}
		if mismatch {
			res.CredentialMismatches = append(res.CredentialMismatches, exp)
		}
	}

	// 无头用户 / 疑似独立账号残留
	for key, act := range actual {
		if _, ok := desired[key]; ok {
			continue
		}
		email := act.Email
		switch {
		case strings.HasPrefix(email, "user-"):
			res.Orphans = append(res.Orphans, act)
		case strings.HasPrefix(email, "ext-"):
			res.ExtOrphans = append(res.ExtOrphans, act)
		default:
			// 无法匹配前缀同样归入疑似独立账号残留，前端默认不勾选。
			res.ExtOrphans = append(res.ExtOrphans, act)
		}
	}
	return res, nil
}

// PushOne 单条补推（同步，120s 由接入层超时控制）。
func (s *SyncService) PushOne(ctx context.Context, item ReconcileItem) error {
	if item.Source == "ext" {
		if s.ext == nil || item.ExtAccountID == nil {
			return errors.New("独立账号服务未注入")
		}
		return s.ext.pushOne(ctx, *item.ExtAccountID, ExtPushTarget{
			InstanceID: item.InstanceID, InboundTag: item.InboundTag, NodeID: item.NodeID,
		}, false)
	}
	if item.UserID == nil {
		return errors.New("缺少用户 ID")
	}
	target := Target{NodeID: item.NodeID, InstanceID: item.InstanceID, Tag: item.InboundTag}
	return s.pushUserTarget(ctx, *item.UserID, target)
}

// CredentialsOne 单条凭据修复：先 Remove 再 Add。
func (s *SyncService) CredentialsOne(ctx context.Context, item ReconcileItem) error {
	if item.Source == "ext" {
		if s.ext == nil || item.ExtAccountID == nil {
			return errors.New("独立账号服务未注入")
		}
		t := ExtPushTarget{InstanceID: item.InstanceID, InboundTag: item.InboundTag, NodeID: item.NodeID}
		_ = s.ext.removeOne(ctx, *item.ExtAccountID, t)
		return s.ext.pushOne(ctx, *item.ExtAccountID, t, false)
	}
	if item.UserID == nil {
		return errors.New("缺少用户 ID")
	}
	target := Target{NodeID: item.NodeID, InstanceID: item.InstanceID, Tag: item.InboundTag}
	_, _, _ = s.RemoveUserFromTargets(ctx, *item.UserID, []Target{target})
	return s.pushUserTarget(ctx, *item.UserID, target)
}

// RepairPushAsync 异步执行待补推。
func (s *SyncService) RepairPushAsync(ctx context.Context, instanceID int64) (string, error) {
	taskID := s.registry.Register(tasks.KindReconcileExec)
	go func() {
		res, err := s.Reconcile(ctx, instanceID)
		if err != nil {
			s.registry.Fail(taskID, err.Error())
			return
		}
		success, failed := 0, 0
		for _, item := range res.ToPush {
			if err := s.PushOne(ctx, item); err != nil {
				failed++
			} else {
				success++
			}
		}
		s.registry.Succeed(taskID, map[string]any{"pushed": success, "failed": failed})
	}()
	return taskID, nil
}

// CleanOrphansAsync 异步清理显式勾选的孤儿账号。
func (s *SyncService) CleanOrphansAsync(ctx context.Context, instanceID int64, emails []string) (string, error) {
	taskID := s.registry.Register(tasks.KindReconcileExec)
	go func() {
		success, failed := 0, 0
		for _, email := range emails {
			tags, err := s.instanceTags(ctx, instanceID)
			if err != nil {
				s.registry.Fail(taskID, err.Error())
				return
			}
			client, err := s.instances.ClientFor(ctx, instanceID)
			if err != nil {
				s.registry.Fail(taskID, err.Error())
				return
			}
			for _, tag := range tags {
				if err := client.RemoveUser(ctx, tag, email); err != nil && !IsNotFound(err) {
					failed++
				} else {
					success++
				}
			}
		}
		s.registry.Succeed(taskID, map[string]any{"removed": success, "failed": failed})
	}()
	return taskID, nil
}

// RepairCredentialsAsync 异步修复勾选项凭据。
func (s *SyncService) RepairCredentialsAsync(ctx context.Context, instanceID int64, items []ReconcileItem) (string, error) {
	taskID := s.registry.Register(tasks.KindReconcileExec)
	go func() {
		success, failed := 0, 0
		for _, item := range items {
			if err := s.CredentialsOne(ctx, item); err != nil {
				failed++
			} else {
				success++
			}
		}
		s.registry.Succeed(taskID, map[string]any{"repaired": success, "failed": failed})
	}()
	return taskID, nil
}

// pushUserTarget 推送单个用户到单个目标。
func (s *SyncService) pushUserTarget(ctx context.Context, userID int64, t Target) error {
	client, err := s.apiFor(ctx, t.InstanceID)
	if err != nil {
		s.markFailed(ctx, userID, t, err)
		return err
	}
	if err := s.writePending(ctx, userID, t); err != nil {
		if errors.Is(err, ErrAdvancedOff) {
			return nil
		}
		s.markFailed(ctx, userID, t, err)
		return err
	}
	uuid, secret, err := s.creds.Credentials(ctx, userID)
	if err != nil {
		s.markFailed(ctx, userID, t, err)
		return err
	}
	nv, err := s.nodeViewForTarget(ctx, t.NodeID)
	if err != nil {
		s.markFailed(ctx, userID, t, err)
		return err
	}
	nv.Tag = t.Tag
	u, err := BuildUser(userID, uuid, secret, nv)
	if err != nil {
		s.markFailed(ctx, userID, t, err)
		return err
	}
	if err := client.AddUser(ctx, t.Tag, u); err != nil {
		s.markFailed(ctx, userID, t, err)
		return err
	}
	s.markSynced(ctx, userID, t)
	return nil
}

func (s *SyncService) instanceTags(ctx context.Context, instanceID int64) ([]string, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT tag FROM nodes WHERE instance_id = ? AND source = 'xray' ORDER BY tag`, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		out = append(out, tag)
	}
	return out, rows.Err()
}

// accountMismatch 判断期望账号与实际账号凭据是否不一致。
func (s *SyncService) accountMismatch(ctx context.Context, item ReconcileItem, actual *protocol.User) (bool, error) {
	var expected *protocol.User
	if item.Source == "user" && item.UserID != nil {
		uuid, secret, err := s.creds.Credentials(ctx, *item.UserID)
		if err != nil {
			return false, err
		}
		nv, err := s.nodeViewForTarget(ctx, item.NodeID)
		if err != nil {
			return false, err
		}
		nv.Tag = item.InboundTag
		expected, err = BuildUser(*item.UserID, uuid, secret, nv)
		if err != nil {
			return false, err
		}
	}
	if item.Source == "ext" && item.ExtAccountID != nil && s.ext != nil {
		creds, err := s.ext.GetExtCredentials(ctx, *item.ExtAccountID)
		if err != nil {
			return false, err
		}
		nv, err := s.nodeViewForTarget(ctx, item.NodeID)
		if err != nil {
			return false, err
		}
		nv.Tag = item.InboundTag
		expected, err = BuildExtUser(*item.ExtAccountID, creds.UUID, creds.ProxySecret, nv)
		if err != nil {
			return false, err
		}
	}
	if expected == nil {
		return false, nil
	}
	if actual == nil {
		return true, nil
	}
	return !proto.Equal(expected.GetAccount(), actual.GetAccount()), nil
}
