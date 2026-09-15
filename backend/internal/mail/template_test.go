package mail

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// templateDefinitionForTest 取固定定义；Step 1 的失败优先测试依赖该 API 存在。
func templateDefinitionForTest(t *testing.T, kind TemplateKind) TemplateDefinition {
	t.Helper()
	def, err := Definition(kind)
	if err != nil {
		t.Fatalf("Definition(%q) 失败: %v", kind, err)
	}
	return def
}

// TestTemplateDefinitionsStableOrderAndDefaults 五分支固定顺序、配置键、默认值与变量元数据。
func TestTemplateDefinitionsStableOrderAndDefaults(t *testing.T) {
	defs := Definitions()
	if len(defs) != 5 {
		t.Fatalf("固定模板数应为 5，实际 %d", len(defs))
	}
	want := []struct {
		id               TemplateKind
		configKey        string
		label            string
		scope            string
		subject          string
		body             string
		subjectVars      []string
		bodyVars         []string
		requiredBodyVars []string
	}{
		{
			id:               TemplatePasswordReset,
			configKey:        ConfigKeyPasswordReset,
			label:            "密码重置",
			scope:            ScopePasswordReset,
			subject:          "密码重置",
			body:             "请在 1 小时内使用以下链接重置密码（一次性）：\n{{reset_url}}",
			subjectVars:      []string{},
			bodyVars:         []string{"reset_url"},
			requiredBodyVars: []string{"reset_url"},
		},
		{
			id:               TemplateApprovalApproved,
			configKey:        ConfigKeyApprovalApproved,
			label:            "审批通过",
			scope:            ScopeApprovalNotify,
			subject:          "{{site_name}} 审批通知",
			body:             "您在 {{site_name}} 的账号已通过审批，现在可以登录：\n{{login_url}}",
			subjectVars:      []string{"site_name"},
			bodyVars:         []string{"site_name", "login_url"},
			requiredBodyVars: []string{"login_url"},
		},
		{
			id:               TemplateApprovalRejected,
			configKey:        ConfigKeyApprovalRejected,
			label:            "审批拒绝",
			scope:            ScopeApprovalNotify,
			subject:          "{{site_name}} 审批通知",
			body:             "您在 {{site_name}} 的账号申请未通过审批。",
			subjectVars:      []string{"site_name"},
			bodyVars:         []string{"site_name"},
			requiredBodyVars: []string{},
		},
		{
			id:               TemplateWelcomeLocal,
			configKey:        ConfigKeyWelcomeLocal,
			label:            "本地欢迎",
			scope:            ScopeWelcome,
			subject:          "{{site_name}} 账号已激活",
			body:             "{{site_name}}\n\n您的账号已激活，请使用邮箱与密码登录：{{login_url}}",
			subjectVars:      []string{"site_name"},
			bodyVars:         []string{"site_name", "login_url"},
			requiredBodyVars: []string{"login_url"},
		},
		{
			id:               TemplateWelcomeOIDC,
			configKey:        ConfigKeyWelcomeOIDC,
			label:            "OIDC 欢迎",
			scope:            ScopeWelcome,
			subject:          "{{site_name}} 账号已激活",
			body:             "{{site_name}}\n\n您的账号已激活，请使用单点登录（OIDC）登录：{{login_url}}",
			subjectVars:      []string{"site_name"},
			bodyVars:         []string{"site_name", "login_url"},
			requiredBodyVars: []string{"login_url"},
		},
	}
	for i, w := range want {
		got := defs[i]
		if got.ID != w.id || got.ConfigKey != w.configKey || got.Label != w.label || got.Scope != w.scope {
			t.Fatalf("第 %d 项定义不符: got=%+v want id=%s key=%s label=%s scope=%s", i, got, w.id, w.configKey, w.label, w.scope)
		}
		if got.Default.Subject != w.subject || got.Default.Body != w.body {
			t.Fatalf("%s 默认值不符:\nsubject=%q\nbody=%q", got.ID, got.Default.Subject, got.Default.Body)
		}
		if !equalStringSlices(got.SubjectVariables, w.subjectVars) ||
			!equalStringSlices(got.BodyVariables, w.bodyVars) ||
			!equalStringSlices(got.RequiredBodyVariables, w.requiredBodyVars) {
			t.Fatalf("%s 变量元数据不符: subject=%v body=%v required=%v", got.ID,
				got.SubjectVariables, got.BodyVariables, got.RequiredBodyVariables)
		}
	}
	if _, err := Definition(TemplateKind("missing")); !errors.Is(err, ErrUnknownTemplate) {
		t.Fatalf("未知模板应返回 ErrUnknownTemplate: %v", err)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestValidateTemplateRules 主题/正文长度、控制字符、占位符、必需变量与 CRLF 规范化。
func TestValidateTemplateRules(t *testing.T) {
	valid := func(kind TemplateKind) Template {
		return templateDefinitionForTest(t, kind).Default
	}
	for _, kind := range []TemplateKind{
		TemplatePasswordReset, TemplateApprovalApproved, TemplateApprovalRejected, TemplateWelcomeLocal, TemplateWelcomeOIDC,
	} {
		t.Run(string(kind)+"/默认模板合法", func(t *testing.T) {
			if err := ValidateTemplate(kind, valid(kind)); err != nil {
				t.Fatalf("默认模板应合法: %v", err)
			}
		})
	}

	t.Run("主题长度边界", func(t *testing.T) {
		def := valid(TemplateApprovalApproved)
		for _, tc := range []struct {
			name    string
			subject string
			wantErr bool
		}{
			{"空串", "", true},
			{"1 字符", "x", false},
			{"200 字符", strings.Repeat("x", 200), false},
			{"201 字符", strings.Repeat("x", 201), true},
			{"空白", " \t ", true},
			{"换行", "x\ny", true},
			{"制表符", "x\ty", true},
			{"NUL", "x\x00y", true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tpl := def
				tpl.Subject = tc.subject
				err := ValidateTemplate(TemplateApprovalApproved, tpl)
				if (err != nil) != tc.wantErr {
					t.Fatalf("subject=%q err=%v wantErr=%v", tc.subject, err, tc.wantErr)
				}
			})
		}
	})

	t.Run("正文字符与长度边界", func(t *testing.T) {
		def := valid(TemplatePasswordReset)
		for _, tc := range []struct {
			name    string
			body    string
			wantErr bool
		}{
			{"空串", "", true},
			{"单占位符", "{{reset_url}}", false},
			{"10001 字符", strings.Repeat("a", 10001-len("{{reset_url}}")) + "{{reset_url}}", true},
			{"换行与制表符", "a\n\tb{{reset_url}}", false},
			{"NUL", "a\x00b{{reset_url}}", true},
			{"C1 控制符", "a\u0085b{{reset_url}}", true},
			{"未闭合左", "{{reset_url", true},
			{"未闭合右", "reset_url}}", true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tpl := def
				tpl.Body = tc.body
				err := ValidateTemplate(TemplatePasswordReset, tpl)
				if (err != nil) != tc.wantErr {
					t.Fatalf("body=%q err=%v wantErr=%v", tc.body, err, tc.wantErr)
				}
			})
		}
		// 规范化后恰好 10,000：原始 CRLF 不应按两个字符计数。
		filler := strings.Repeat("a", 10000-len("{{reset_url}}")-2)
		tpl := def
		tpl.Body = filler + "\r\nb" + "{{reset_url}}"
		if err := ValidateTemplate(TemplatePasswordReset, tpl); err != nil {
			t.Fatalf("CRLF 规范化后 10,000 字符应合法: %v", err)
		}
		tpl.Body += "a"
		if err := ValidateTemplate(TemplatePasswordReset, tpl); err == nil {
			t.Fatal("规范化后 10,001 字符应拒绝")
		}
	})

	t.Run("占位符白名单与跨分支", func(t *testing.T) {
		cases := []struct {
			kind TemplateKind
			body string
			ok   bool
		}{
			{TemplatePasswordReset, "{{reset_url}}", true},
			{TemplatePasswordReset, "{{reset_url}}{{reset_url}}", true},
			{TemplatePasswordReset, "{{site_name}}{{reset_url}}", false},
			{TemplatePasswordReset, "{{login_url}}", false},
			{TemplatePasswordReset, "{{foo}}{{reset_url}}", false},
			{TemplatePasswordReset, "{{ reset_url }}{{reset_url}}", false},
			{TemplatePasswordReset, "{{RESET_URL}}{{reset_url}}", false},
			{TemplateApprovalApproved, "{{login_url}}", true},
			{TemplateApprovalApproved, "{{login_url}}{{login_url}}", true},
			{TemplateApprovalApproved, "{{site_name}}{{login_url}}", true},
			{TemplateApprovalApproved, "{{reset_url}}{{login_url}}", false},
			{TemplateApprovalRejected, "{{site_name}}", true},
			{TemplateApprovalRejected, "{{login_url}}{{site_name}}", false},
		}
		for _, tc := range cases {
			def := valid(tc.kind)
			tpl := def
			tpl.Body = tc.body
			err := ValidateTemplate(tc.kind, tpl)
			if (err == nil) != tc.ok {
				t.Fatalf("%s body=%q err=%v wantOK=%v", tc.kind, tc.body, err, tc.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalidTemplate) {
				t.Fatalf("领域校验错误应包装 ErrInvalidTemplate: %v", err)
			}
		}
	})

	t.Run("主题 URL 变量拒绝", func(t *testing.T) {
		for _, kind := range []TemplateKind{TemplateApprovalApproved, TemplateWelcomeLocal, TemplateWelcomeOIDC} {
			def := valid(kind)
			tpl := def
			tpl.Subject = "{{login_url}}"
			if err := ValidateTemplate(kind, tpl); !errors.Is(err, ErrInvalidTemplate) {
				t.Fatalf("%s 主题不应允许 login_url: %v", kind, err)
			}
		}
	})

	t.Run("错误不泄露输入", func(t *testing.T) {
		const marker = "SECRET-MARKER-7f3a"
		tpl := valid(TemplatePasswordReset)
		tpl.Body = marker + "{{unknown}}" + "{{reset_url}}"
		err := ValidateTemplate(TemplatePasswordReset, tpl)
		if err == nil {
			t.Fatal("含未知占位符应失败")
		}
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("错误不得回显输入正文: %v", err)
		}
	})
}

// TestParseTemplateJSONStrict 持久化严格 JSON。
func TestParseTemplateJSONStrict(t *testing.T) {
	validJSON, err := json.Marshal(Template{Subject: "主题", Body: "{{reset_url}}"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseTemplateJSON(string(validJSON))
	if err != nil || got.Subject != "主题" || got.Body != "{{reset_url}}" {
		t.Fatalf("合法 JSON 解析失败: got=%+v err=%v", got, err)
	}
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"空串", ""},
		{"非对象", `"x"`},
		{"缺 subject", `{"body":"x"}`},
		{"缺 body", `{"subject":"x"}`},
		{"未知字段", `{"subject":"x","body":"y","extra":1}`},
		{"非字符串", `{"subject":1,"body":"y"}`},
		{"null", `{"subject":null,"body":"y"}`},
		{"尾随值", `{"subject":"x","body":"y"} {}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseTemplateJSON(tc.raw); !errors.Is(err, ErrInvalidTemplate) {
				t.Fatalf("应返回 ErrInvalidTemplate: %v", err)
			}
		})
	}
}

// TestValidateTemplateOverrides 导入前只校验五个已知键，未知 mail_template_ 前缀不变。
func TestValidateTemplateOverrides(t *testing.T) {
	validOverrides := map[string]string{}
	for _, def := range Definitions() {
		raw, err := json.Marshal(def.Default)
		if err != nil {
			t.Fatal(err)
		}
		validOverrides[def.ConfigKey] = string(raw)
	}
	if err := ValidateTemplateOverrides(validOverrides); err != nil {
		t.Fatalf("五个合法覆盖不应失败: %v", err)
	}
	if err := ValidateTemplateOverrides(nil); err != nil {
		t.Fatalf("nil map 不应失败: %v", err)
	}
	unknown := map[string]string{
		ConfigKeyPasswordReset:        `{"subject":"密码重置","body":"{{reset_url}}"}`,
		"mail_template_future_branch": `{not-json`,
		"unrelated_key":               "anything",
	}
	if err := ValidateTemplateOverrides(unknown); err != nil {
		t.Fatalf("未知 mail_template_ 前缀键不应由邮件回调拒绝: %v", err)
	}
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"非法 JSON", `{`},
		{"领域非法", `{"subject":"x","body":"没有必需变量"}`},
		{"未知字段", `{"subject":"x","body":"{{reset_url}}","extra":1}`},
	} {
		cfg := map[string]string{ConfigKeyPasswordReset: tc.raw}
		if err := ValidateTemplateOverrides(cfg); !errors.Is(err, ErrInvalidTemplate) {
			t.Fatalf("%s 应返回 ErrInvalidTemplate: %v", tc.name, err)
		}
	}
}

// TestTemplateStorageStatesAndIsolation 三态、单键保存/恢复、数据库错误与损坏回退。
func TestTemplateStorageStatesAndIsolation(t *testing.T) {
	st, svc := newTestMail(t)
	ctx := context.Background()

	// 缺键 → default + 内置默认值。
	def := templateDefinitionForTest(t, TemplatePasswordReset)
	tpl, state, err := svc.LoadTemplate(ctx, TemplatePasswordReset)
	if err != nil || state != TemplateStateDefault || tpl != def.Default {
		t.Fatalf("缺键应返回 default 状态: tpl=%+v state=%s err=%v", tpl, state, err)
	}

	// 合法覆盖 → customized；保存只写目标键。
	custom := Template{Subject: "自定义主题", Body: "自定义正文{{reset_url}}"}
	view, err := svc.SaveTemplate(ctx, TemplatePasswordReset, custom)
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if view.State != TemplateStateCustomized || view.Subject != custom.Subject || view.Body != custom.Body {
		t.Fatalf("保存返回视图异常: %+v", view)
	}
	rawOther := "smtp-secret-sentinel"
	if err := svc.cfg.Set(ctx, KeyPassword, rawOther); err != nil {
		t.Fatal(err)
	}
	if err := svc.cfg.Set(ctx, KeyScopes, `["welcome"]`); err != nil {
		t.Fatal(err)
	}
	got, state, err := svc.LoadTemplate(ctx, TemplatePasswordReset)
	if err != nil || state != TemplateStateCustomized || got != custom {
		t.Fatalf("合法覆盖读取异常: got=%+v state=%s err=%v", got, state, err)
	}
	// 另一分支仍为默认。
	if _, state, err := svc.LoadTemplate(ctx, TemplateWelcomeLocal); err != nil || state != TemplateStateDefault {
		t.Fatalf("保存单分支不应影响其他分支: state=%s err=%v", state, err)
	}
	// SMTP 密码键不应受影响。
	if v, err := svc.cfg.Get(ctx, KeyPassword); err != nil || v != rawOther {
		t.Fatalf("模板保存不应触碰 SMTP 密码: %q err=%v", v, err)
	}
	// scope 不应受影响。
	if v := svc.cfg.GetOr(ctx, KeyScopes); v != `["welcome"]` {
		t.Fatalf("模板保存不应触碰 scope: %q", v)
	}

	// 损坏 JSON → damaged + 默认值，不回显坏值。
	secretRaw := `{"subject":"坏值-SECRET","body":`
	if err := svc.cfg.Set(ctx, ConfigKeyPasswordReset, secretRaw); err != nil {
		t.Fatal(err)
	}
	got, state, err = svc.LoadTemplate(ctx, TemplatePasswordReset)
	if err != nil || state != TemplateStateDamaged || got != def.Default {
		t.Fatalf("损坏 JSON 应回退默认: got=%+v state=%s err=%v", got, state, err)
	}
	if strings.Contains(statusSafeString(got), "坏值-SECRET") {
		t.Fatal("损坏值不得回显")
	}

	// 恢复默认 → 缺键 + default，重复恢复幂等。
	view, err = svc.RestoreTemplate(ctx, TemplatePasswordReset)
	if err != nil || view.State != TemplateStateDefault || view.Subject != def.Default.Subject {
		t.Fatalf("恢复默认异常: view=%+v err=%v", view, err)
	}
	if _, err := svc.cfg.Get(ctx, ConfigKeyPasswordReset); err != nil {
		t.Fatalf("恢复后读取错误: %v", err)
	}
	if _, err := svc.RestoreTemplate(ctx, TemplatePasswordReset); err != nil {
		t.Fatalf("重复恢复应幂等: %v", err)
	}

	// 键存在但值为空 → damaged，不能被误判为缺键 default。
	oidcDef := templateDefinitionForTest(t, TemplateWelcomeOIDC)
	if err := svc.cfg.Set(ctx, ConfigKeyWelcomeOIDC, ""); err != nil {
		t.Fatal(err)
	}
	got, state, err = svc.LoadTemplate(ctx, TemplateWelcomeOIDC)
	if err != nil || state != TemplateStateDamaged || got != oidcDef.Default {
		t.Fatalf("空值键应为 damaged+默认值: got=%+v state=%s err=%v", got, state, err)
	}

	// 未知 kind。
	if _, err := svc.SaveTemplate(ctx, TemplateKind("missing"), custom); !errors.Is(err, ErrUnknownTemplate) {
		t.Fatalf("未知模板保存应为 ErrUnknownTemplate: %v", err)
	}
	if _, err := svc.RestoreTemplate(ctx, TemplateKind("missing")); !errors.Is(err, ErrUnknownTemplate) {
		t.Fatalf("未知模板恢复应为 ErrUnknownTemplate: %v", err)
	}

	// 数据库读取错误 → 默认值 + damaged + 非 nil 安全错误，业务发送可回退。
	if err := st.Close(); err != nil {
		t.Fatalf("关闭测试库失败: %v", err)
	}
	got, state, err = svc.LoadTemplate(ctx, TemplatePasswordReset)
	if err == nil || state != TemplateStateDamaged || got != def.Default {
		t.Fatalf("数据库错误应返回默认+damaged+error: got=%+v state=%s err=%v", got, state, err)
	}
}

func statusSafeString(t Template) string {
	return t.Subject + "|" + t.Body
}

// TestSaveTemplateRejectsInvalidAndKeepsOldValue 非法保存不得覆盖旧值。
func TestSaveTemplateRejectsInvalidAndKeepsOldValue(t *testing.T) {
	st, svc := newTestMail(t)
	ctx := context.Background()
	def := templateDefinitionForTest(t, TemplatePasswordReset)
	first := Template{Subject: "第一版", Body: "第一版{{reset_url}}"}
	if _, err := svc.SaveTemplate(ctx, TemplatePasswordReset, first); err != nil {
		t.Fatal(err)
	}
	before, err := svc.cfg.Get(ctx, def.ConfigKey)
	if err != nil {
		t.Fatal(err)
	}
	bad := first
	bad.Body = "缺少必需变量"
	if _, err := svc.SaveTemplate(ctx, TemplatePasswordReset, bad); !errors.Is(err, ErrInvalidTemplate) {
		t.Fatalf("非法保存应失败: %v", err)
	}
	after, err := svc.cfg.Get(ctx, def.ConfigKey)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("非法保存不应覆盖旧值: before=%s after=%s", before, after)
	}
	_ = st
}
