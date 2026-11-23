package providertraffic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func newTestTraffic(t *testing.T) (*store.Store, *config.Service, *Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移测试库失败: %v", err)
	}
	lg := log.New("error", "console")
	cfg := config.NewService(st, lg)
	return st, cfg, NewService(st, cfg, lg)
}

func TestDefaultSettings(t *testing.T) {
	st, _, svc := newTestTraffic(t)
	ctx := context.Background()

	settings, err := svc.GetSettings(ctx)
	if err != nil {
		t.Fatalf("读取默认设置失败: %v", err)
	}
	if settings.Enabled {
		t.Fatal("无记录时总开关应为关闭")
	}
	assertProviderOrder(t, settings)
	for _, provider := range settings.Providers {
		if provider.Enabled || provider.InstanceID != "" || provider.Revision != "" || provider.CredentialConfigured {
			t.Fatalf("无记录时 %s 应为未绑定且关闭: %+v", provider.Provider, provider)
		}
	}

	// 仅保存总开关也应创建默认行。
	settings, err = svc.SetEnabled(ctx, false)
	if err != nil {
		t.Fatalf("保存总开关失败: %v", err)
	}
	if settings.Enabled {
		t.Fatal("保存 false 后总开关应为关闭")
	}
	var rows int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_traffic_settings`).Scan(&rows); err != nil {
		t.Fatalf("查询功能表失败: %v", err)
	}
	if rows != 1 {
		t.Fatalf("仅保存总开关后应有 1 行，实际 %d", rows)
	}

	settings, err = svc.SetEnabled(ctx, true)
	if err != nil {
		t.Fatalf("开启总开关失败: %v", err)
	}
	if !settings.Enabled {
		t.Fatal("保存 true 后总开关应为开启")
	}
	for _, provider := range settings.Providers {
		if provider.Enabled || provider.InstanceID != "" {
			t.Fatalf("总开关不应改变两家绑定: %+v", provider)
		}
	}
}

func TestSaveBindingEncryptsAndKeepsProvidersIndependent(t *testing.T) {
	st, cfg, svc := newTestTraffic(t)
	ctx := context.Background()
	const (
		kiwivmCredential = "synthetic-kiwivm-key"
		vpsCredential    = "synthetic-vps-refresh-token"
	)

	settings, err := svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderKiwiVM,
		Enabled:    true,
		InstanceID: "1001",
		Credential: CredentialInput{Value: kiwivmCredential},
	})
	if err != nil {
		t.Fatalf("保存 KiwiVM 绑定失败: %v", err)
	}
	if settings.Enabled {
		t.Fatal("保存单家绑定不应改变总开关")
	}
	assertProviderOrder(t, settings)
	kiwivm := settings.Providers[0]
	if !kiwivm.Enabled || kiwivm.InstanceID != "1001" || !kiwivm.CredentialConfigured || kiwivm.Revision == "" {
		t.Fatalf("KiwiVM 绑定状态异常: %+v", kiwivm)
	}
	if vps := settings.Providers[1]; vps.Enabled || vps.InstanceID != "" || vps.Revision != "" || vps.CredentialConfigured {
		t.Fatalf("保存 KiwiVM 不应改变 V.PS: %+v", vps)
	}

	enabled, instanceID, ciphertext, revision := rawBinding(t, st, ProviderKiwiVM)
	if enabled != 1 || instanceID != "1001" {
		t.Fatalf("KiwiVM 原始行异常: enabled=%d instance=%q", enabled, instanceID)
	}
	if ciphertext == "" || ciphertext == kiwivmCredential || strings.Contains(ciphertext, kiwivmCredential) {
		t.Fatalf("凭据应以密文入库: %q", ciphertext)
	}
	signingKey, err := cfg.GetSigningKey(ctx)
	if err != nil {
		t.Fatalf("读取签名密钥失败: %v", err)
	}
	plain, err := config.Decrypt(ciphertext, signingKey)
	if err != nil {
		t.Fatalf("解密入库密文失败: %v", err)
	}
	if string(plain) != kiwivmCredential {
		t.Fatalf("入库密文解密结果不一致: got %q", string(plain))
	}
	loadedInstance, loadedRevision, loadedCredential, err := svc.LoadCredential(ctx, ProviderKiwiVM)
	if err != nil {
		t.Fatalf("读取并解密凭据失败: %v", err)
	}
	if loadedInstance != "1001" || loadedRevision != revision || loadedCredential != kiwivmCredential {
		t.Fatalf("LoadCredential 结果异常: instance=%q revision=%q credential=%q", loadedInstance, loadedRevision, loadedCredential)
	}

	settings, err = svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderVPS,
		Enabled:    false,
		InstanceID: "2002",
		Credential: CredentialInput{Value: vpsCredential},
	})
	if err != nil {
		t.Fatalf("保存 V.PS 绑定失败: %v", err)
	}
	kiwivmAfter := settings.Providers[0]
	if kiwivmAfter.InstanceID != "1001" || kiwivmAfter.Revision != revision || !kiwivmAfter.Enabled {
		t.Fatalf("保存 V.PS 不应改变 KiwiVM: %+v", kiwivmAfter)
	}
	vps := settings.Providers[1]
	if vps.Enabled || vps.InstanceID != "2002" || !vps.CredentialConfigured || vps.Revision == "" {
		t.Fatalf("V.PS 绑定状态异常: %+v", vps)
	}
	vpsInstance, vpsRevision, vpsLoadedCredential, err := svc.LoadCredential(ctx, ProviderVPS)
	if err != nil {
		t.Fatalf("读取 V.PS 凭据失败: %v", err)
	}
	if vpsInstance != "2002" || vpsRevision != vps.Revision || vpsLoadedCredential != vpsCredential {
		t.Fatalf("V.PS 凭据读取异常: instance=%q revision=%q credential=%q", vpsInstance, vpsRevision, vpsLoadedCredential)
	}
	_, _, ciphertextAfter, revisionAfter := rawBinding(t, st, ProviderKiwiVM)
	if ciphertextAfter != ciphertext || revisionAfter != revision {
		t.Fatal("保存 V.PS 不应改变 KiwiVM 的密文或修订")
	}

	settings, err = svc.SetEnabled(ctx, true)
	if err != nil {
		t.Fatalf("开启总开关失败: %v", err)
	}
	if !settings.Enabled || settings.Providers[0].InstanceID != "1001" || settings.Providers[1].InstanceID != "2002" {
		t.Fatalf("总开关不应清除两家绑定: %+v", settings)
	}
}

func TestSameInstanceOmittedCredentialKeepsRevisionAndCiphertext(t *testing.T) {
	st, _, svc := newTestTraffic(t)
	ctx := context.Background()

	settings, err := svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderKiwiVM,
		Enabled:    true,
		InstanceID: "1001",
		Credential: CredentialInput{Value: "secret-1"},
	})
	if err != nil {
		t.Fatalf("首次绑定失败: %v", err)
	}
	revision := settings.Providers[0].Revision
	_, _, ciphertext, _ := rawBinding(t, st, ProviderKiwiVM)

	// 同一实例省略凭据：保留密文和修订，只切换单家开关。
	settings, err = svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderKiwiVM,
		Enabled:    false,
		InstanceID: "1001",
		Revision:   strPtr(revision),
		Credential: CredentialInput{Omitted: true},
	})
	if err != nil {
		t.Fatalf("同一实例省略凭据应允许保留: %v", err)
	}
	after := settings.Providers[0]
	if after.Enabled || after.Revision != revision {
		t.Fatalf("省略凭据应保留修订并只切换开关: %+v", after)
	}
	_, _, ciphertextAfter, _ := rawBinding(t, st, ProviderKiwiVM)
	if ciphertextAfter != ciphertext {
		t.Fatal("省略凭据不应改变密文")
	}

	// 提交新凭据：生成新修订并替换密文。
	settings, err = svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderKiwiVM,
		Enabled:    true,
		InstanceID: "1001",
		Revision:   strPtr(revision),
		Credential: CredentialInput{Value: "secret-2"},
	})
	if err != nil {
		t.Fatalf("换证失败: %v", err)
	}
	newRevision := settings.Providers[0].Revision
	if newRevision == "" || newRevision == revision {
		t.Fatalf("换证应生成新修订: old=%q new=%q", revision, newRevision)
	}
	_, _, newCiphertext, _ := rawBinding(t, st, ProviderKiwiVM)
	if newCiphertext == ciphertext {
		t.Fatal("换证应替换密文")
	}
	_, _, credential, err := svc.LoadCredential(ctx, ProviderKiwiVM)
	if err != nil {
		t.Fatalf("读取新凭据失败: %v", err)
	}
	if credential != "secret-2" {
		t.Fatalf("换证后凭据不一致: %q", credential)
	}
}

func TestCredentialInputSemantics(t *testing.T) {
	st, _, svc := newTestTraffic(t)
	ctx := context.Background()

	settings, err := svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderKiwiVM,
		Enabled:    true,
		InstanceID: "1001",
		Credential: CredentialInput{Value: "secret-1"},
	})
	if err != nil {
		t.Fatalf("首次绑定失败: %v", err)
	}
	revision := settings.Providers[0].Revision
	_, _, beforeCiphertext, _ := rawBinding(t, st, ProviderKiwiVM)

	for name, input := range map[string]CredentialInput{
		"explicit empty": {Value: ""},
		"whitespace":     {Value: " \t "},
		"explicit null":  {Null: true},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.SaveBinding(ctx, SaveBindingInput{
				Provider:   ProviderKiwiVM,
				Enabled:    true,
				InstanceID: "1001",
				Revision:   strPtr(revision),
				Credential: input,
			})
			if !errors.Is(err, ErrInvalidCredential) {
				t.Fatalf("显式无效凭据应返回 ErrInvalidCredential，实际: %v", err)
			}
			_, _, ciphertext, currentRevision := rawBinding(t, st, ProviderKiwiVM)
			if ciphertext != beforeCiphertext || currentRevision != revision {
				t.Fatal("无效凭据不得改变绑定")
			}
		})
	}

	// 省略与显式空串语义不同：省略允许保留。
	settings, err = svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderKiwiVM,
		Enabled:    true,
		InstanceID: "1001",
		Revision:   strPtr(revision),
		Credential: CredentialInput{Omitted: true},
	})
	if err != nil {
		t.Fatalf("省略凭据应允许: %v", err)
	}
	if settings.Providers[0].Revision != revision {
		t.Fatal("省略凭据应保留修订")
	}

	// 首次绑定省略凭据应拒绝。
	if _, err := svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderVPS,
		Enabled:    true,
		InstanceID: "2002",
		Credential: CredentialInput{Omitted: true},
	}); !errors.Is(err, ErrCredentialRequired) {
		t.Fatalf("首次绑定省略凭据应返回 ErrCredentialRequired，实际: %v", err)
	}
}

func TestBindingValidation(t *testing.T) {
	_, _, svc := newTestTraffic(t)
	ctx := context.Background()

	for _, instanceID := range []string{"", "0", "01", "-1", "1a", " 1", "1 ", "12.3", "１２３"} {
		if _, err := svc.SaveBinding(ctx, SaveBindingInput{
			Provider:   ProviderKiwiVM,
			Enabled:    true,
			InstanceID: instanceID,
			Credential: CredentialInput{Value: "secret"},
		}); !errors.Is(err, ErrInvalidInstanceID) {
			t.Fatalf("实例 ID %q 应返回 ErrInvalidInstanceID，实际: %v", instanceID, err)
		}
	}

	if _, err := svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderKiwiVM,
		Enabled:    true,
		InstanceID: "1001",
		Revision:   strPtr("not-null"),
		Credential: CredentialInput{Value: "secret"},
	}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("首次绑定非 null 修订应返回 ErrRevisionConflict，实际: %v", err)
	}

	settings, err := svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderKiwiVM,
		Enabled:    true,
		InstanceID: "1001",
		Credential: CredentialInput{Value: "secret"},
	})
	if err != nil {
		t.Fatalf("首次绑定失败: %v", err)
	}
	revision := settings.Providers[0].Revision

	for name, input := range map[string]SaveBindingInput{
		"nil revision": {
			Provider: ProviderKiwiVM, Enabled: true, InstanceID: "1001",
			Credential: CredentialInput{Omitted: true},
		},
		"empty revision": {
			Provider: ProviderKiwiVM, Enabled: true, InstanceID: "1001",
			Revision: strPtr(""), Credential: CredentialInput{Omitted: true},
		},
		"stale revision": {
			Provider: ProviderKiwiVM, Enabled: true, InstanceID: "1001",
			Revision: strPtr("stale"), Credential: CredentialInput{Omitted: true},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.SaveBinding(ctx, input); !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("应返回 ErrRevisionConflict，实际: %v", err)
			}
		})
	}

	// 更换实例必须提交新凭据。
	if _, err := svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderKiwiVM,
		Enabled:    true,
		InstanceID: "1002",
		Revision:   strPtr(revision),
		Credential: CredentialInput{Omitted: true},
	}); !errors.Is(err, ErrCredentialRequired) {
		t.Fatalf("更换实例省略凭据应返回 ErrCredentialRequired，实际: %v", err)
	}

	settings, err = svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderKiwiVM,
		Enabled:    true,
		InstanceID: "1002",
		Revision:   strPtr(revision),
		Credential: CredentialInput{Value: "secret-new-instance"},
	})
	if err != nil {
		t.Fatalf("更换实例并提交凭据失败: %v", err)
	}
	if settings.Providers[0].InstanceID != "1002" || settings.Providers[0].Revision == revision {
		t.Fatalf("更换实例应更新实例 ID 和修订: %+v", settings.Providers[0])
	}
}

func TestUpdateCredentialByRevision(t *testing.T) {
	st, _, svc := newTestTraffic(t)
	ctx := context.Background()

	settings, err := svc.SaveBinding(ctx, SaveBindingInput{
		Provider:   ProviderKiwiVM,
		Enabled:    true,
		InstanceID: "1001",
		Credential: CredentialInput{Value: "secret-1"},
	})
	if err != nil {
		t.Fatalf("首次绑定失败: %v", err)
	}
	revision := settings.Providers[0].Revision
	enabledBefore, instanceBefore, ciphertextBefore, _ := rawBinding(t, st, ProviderKiwiVM)

	updated, err := svc.UpdateCredentialByRevision(ctx, ProviderKiwiVM, revision, "rotated-secret")
	if err != nil {
		t.Fatalf("按修订更新凭据失败: %v", err)
	}
	if !updated {
		t.Fatal("匹配修订应更新成功")
	}
	enabledAfter, instanceAfter, ciphertextAfter, revisionAfter := rawBinding(t, st, ProviderKiwiVM)
	if enabledAfter != enabledBefore || instanceAfter != instanceBefore || revisionAfter != revision {
		t.Fatal("自动续期不得改变开关、实例或修订")
	}
	if ciphertextAfter == ciphertextBefore {
		t.Fatal("自动续期应替换密文")
	}
	_, _, credential, err := svc.LoadCredential(ctx, ProviderKiwiVM)
	if err != nil {
		t.Fatalf("读取轮换后凭据失败: %v", err)
	}
	if credential != "rotated-secret" {
		t.Fatalf("轮换后凭据不一致: %q", credential)
	}

	// 旧修订或不存在修订：影响零行并直接丢弃。
	updated, err = svc.UpdateCredentialByRevision(ctx, ProviderKiwiVM, "stale-revision", "ignored")
	if err != nil {
		t.Fatalf("旧修订应无错误丢弃: %v", err)
	}
	if updated {
		t.Fatal("旧修订不应更新")
	}
	_, _, ciphertextUnchanged, revisionUnchanged := rawBinding(t, st, ProviderKiwiVM)
	if ciphertextUnchanged != ciphertextAfter || revisionUnchanged != revision {
		t.Fatal("旧修订不得改变绑定")
	}

	updated, err = svc.UpdateCredentialByRevision(ctx, ProviderKiwiVM, "", "ignored")
	if err != nil || updated {
		t.Fatalf("空修订应返回 false, nil，实际 updated=%v err=%v", updated, err)
	}

	if _, err := svc.UpdateCredentialByRevision(ctx, ProviderKiwiVM, revision, "   "); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("空凭据应返回 ErrInvalidCredential，实际: %v", err)
	}

	updated, err = svc.UpdateCredentialByRevision(ctx, ProviderVPS, revision, "ignored")
	if err != nil || updated {
		t.Fatalf("未绑定服务商应返回 false, nil，实际 updated=%v err=%v", updated, err)
	}
}

func TestClearBinding(t *testing.T) {
	_, _, svc := newTestTraffic(t)
	ctx := context.Background()

	settings, err := svc.SaveBinding(ctx, SaveBindingInput{
		Provider: ProviderKiwiVM, Enabled: true, InstanceID: "1001",
		Credential: CredentialInput{Value: "kiwivm-secret"},
	})
	if err != nil {
		t.Fatalf("保存 KiwiVM 失败: %v", err)
	}
	kiwivmRevision := settings.Providers[0].Revision
	settings, err = svc.SaveBinding(ctx, SaveBindingInput{
		Provider: ProviderVPS, Enabled: true, InstanceID: "2002",
		Credential: CredentialInput{Value: "vps-secret"},
	})
	if err != nil {
		t.Fatalf("保存 V.PS 失败: %v", err)
	}
	vpsRevision := settings.Providers[1].Revision
	if _, err := svc.SetEnabled(ctx, true); err != nil {
		t.Fatalf("开启总开关失败: %v", err)
	}

	if _, err := svc.ClearBinding(ctx, ProviderKiwiVM, "stale-revision"); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("旧修订解绑应返回 ErrRevisionConflict，实际: %v", err)
	}
	settings, err = svc.GetSettings(ctx)
	if err != nil {
		t.Fatalf("读取设置失败: %v", err)
	}
	if settings.Providers[0].InstanceID != "1001" || settings.Providers[1].InstanceID != "2002" {
		t.Fatal("旧修订解绑不得改变任何绑定")
	}

	settings, err = svc.ClearBinding(ctx, ProviderKiwiVM, kiwivmRevision)
	if err != nil {
		t.Fatalf("解除 KiwiVM 绑定失败: %v", err)
	}
	if !settings.Enabled {
		t.Fatal("解除单家绑定不应关闭总开关")
	}
	if settings.Providers[0].InstanceID != "" || settings.Providers[0].Revision != "" || settings.Providers[0].Enabled || settings.Providers[0].CredentialConfigured {
		t.Fatalf("KiwiVM 解绑后应完全清空: %+v", settings.Providers[0])
	}
	if settings.Providers[1].InstanceID != "2002" || settings.Providers[1].Revision != vpsRevision {
		t.Fatalf("解除 KiwiVM 不应改变 V.PS: %+v", settings.Providers[1])
	}

	settings, err = svc.ClearBinding(ctx, ProviderVPS, vpsRevision)
	if err != nil {
		t.Fatalf("解除 V.PS 绑定失败: %v", err)
	}
	if !settings.Enabled {
		t.Fatal("解除单家绑定不应关闭总开关")
	}
	if settings.Providers[1].InstanceID != "" || settings.Providers[1].Revision != "" || settings.Providers[1].Enabled || settings.Providers[1].CredentialConfigured {
		t.Fatalf("V.PS 解绑后应完全清空: %+v", settings.Providers[1])
	}
	if _, err := svc.ClearBinding(ctx, ProviderVPS, vpsRevision); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("重复解绑应返回 ErrRevisionConflict，实际: %v", err)
	}
}

func TestLoadCredentialUnbound(t *testing.T) {
	_, _, svc := newTestTraffic(t)
	if _, _, _, err := svc.LoadCredential(context.Background(), ProviderKiwiVM); !errors.Is(err, ErrNotBound) {
		t.Fatalf("未绑定应返回 ErrNotBound，实际: %v", err)
	}
}

func TestUnknownProvider(t *testing.T) {
	_, _, svc := newTestTraffic(t)
	ctx := context.Background()
	const unknown Provider = "unknown"
	if _, err := svc.SaveBinding(ctx, SaveBindingInput{
		Provider: unknown, Enabled: true, InstanceID: "1001",
		Credential: CredentialInput{Value: "secret"},
	}); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("未知服务商保存应返回 ErrUnknownProvider，实际: %v", err)
	}
	if _, err := svc.ClearBinding(ctx, unknown, "revision"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("未知服务商解绑应返回 ErrUnknownProvider，实际: %v", err)
	}
	if _, _, _, err := svc.LoadCredential(ctx, unknown); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("未知服务商读取应返回 ErrUnknownProvider，实际: %v", err)
	}
	if _, err := svc.UpdateCredentialByRevision(ctx, unknown, "revision", "secret"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("未知服务商轮换应返回 ErrUnknownProvider，实际: %v", err)
	}
}

func assertProviderOrder(t *testing.T, settings Settings) {
	t.Helper()
	if len(settings.Providers) != 2 {
		t.Fatalf("服务商数量 = %d，期望 2", len(settings.Providers))
	}
	if settings.Providers[0].Provider != ProviderKiwiVM || settings.Providers[1].Provider != ProviderVPS {
		t.Fatalf("服务商顺序错误: %s, %s", settings.Providers[0].Provider, settings.Providers[1].Provider)
	}
}

func rawBinding(t *testing.T, st *store.Store, provider Provider) (enabled int, instanceID, ciphertext, revision string) {
	t.Helper()
	cols, err := columnsFor(provider)
	if err != nil {
		t.Fatalf("解析服务商列失败: %v", err)
	}
	query := fmt.Sprintf(
		`SELECT %s, %s, %s, %s FROM provider_traffic_settings WHERE id = 1`,
		cols.enabled, cols.instanceID, cols.credential, cols.revision)
	var instance, credential, currentRevision sql.NullString
	if err := st.DB().QueryRow(query).Scan(&enabled, &instance, &credential, &currentRevision); err != nil {
		t.Fatalf("读取 %s 原始绑定失败: %v", provider, err)
	}
	return enabled, instance.String, credential.String, currentRevision.String
}

func strPtr(value string) *string {
	return &value
}
