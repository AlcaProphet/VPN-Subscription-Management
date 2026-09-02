package node

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var defaultCheckTargets = []string{"clash-yaml", "sr-subs", "generic-subs"}

// CheckRenderer 是装配层注入的只读目标适配器检查函数。
type CheckRenderer func(ctx context.Context, target, protocol, renderName, host string, port int, params map[string]any) (CheckRenderResult, error)

// CheckRenderResult 是目标适配器返回的脱敏预览与诊断。
type CheckRenderResult struct {
	Status      string
	Preview     string
	Diagnostics []TargetDiagnostic
}

// CheckRequest 是新建/编辑草稿共用的节点检查入参。
type CheckRequest struct {
	NodeID        int64            `json:"node_id,omitempty"`
	BaseRevision  int64            `json:"base_revision,omitempty"`
	Protocol      string           `json:"protocol"`
	Host          string           `json:"host"`
	Port          int              `json:"port"`
	ProtocolJSON  map[string]any   `json:"protocol_json"`
	CurrentState  *CurrentState    `json:"current_state,omitempty"`
	ResetScopes   []string         `json:"reset_scopes,omitempty"`
	CredentialOps []CredentialOp   `json:"credential_ops,omitempty"`
	ExtensionOps  []ExtensionOp    `json:"extension_ops,omitempty"`
	Extensions    []ExtensionInput `json:"extensions,omitempty"`
	Targets       []string         `json:"targets,omitempty"`
}

// TargetDiagnostic 是单个检查目标的字段级诊断。
type TargetDiagnostic struct {
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	Target    string `json:"target,omitempty"`
	FieldPath string `json:"field_path,omitempty"`
	Message   string `json:"message"`
	Evidence  string `json:"evidence,omitempty"`
}

// TargetCheckResult 是单个目标的检查结果。
type TargetCheckResult struct {
	Status      string             `json:"status"`
	Preview     *string            `json:"preview,omitempty"`
	Diagnostics []TargetDiagnostic `json:"diagnostics"`
}

// CheckResponse 是节点检查响应；服务端不保存该结果。
type CheckResponse struct {
	CheckID      string                       `json:"check_id"`
	CheckVersion int                          `json:"check_version"`
	Targets      map[string]TargetCheckResult `json:"targets"`
}

// Check 对草稿执行目标检查，不写入节点、扩展或其他业务表。
func (s *Service) Check(ctx context.Context, in CheckRequest) (*CheckResponse, error) {
	targets, err := normalizeCheckTargets(in.Targets)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := validateHostPort(in.Host, in.Port); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	proto, err := GetProtocol(in.Protocol)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	params := normalizeProtocolParameters(proto, in.ProtocolJSON)
	var existing *Node
	var extensionRecords []ExtensionRecord
	if in.NodeID > 0 {
		n, err := s.getRaw(ctx, in.NodeID)
		if err != nil {
			return nil, err
		}
		if n.Source != "manual" {
			return nil, fmt.Errorf("%w: 节点信息由实例检测维护", ErrForbidden)
		}
		if in.BaseRevision != n.EditRevision {
			return nil, &revisionConflictError{current: n.EditRevision}
		}
		existing = &n
		normalizedReset, err := normalizeResetScopes(in.ResetScopes)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
		}
		if n.Protocol != in.Protocol && !containsString(normalizedReset, "protocol") {
			normalizedReset = append(normalizedReset, "protocol")
		}
		params = mergeProtocolJSON(n.ProtocolJSON, params, proto, normalizedReset)
		params = normalizeProtocolParameters(proto, params)
		state, stateErr := resolveCurrentState(proto, in.CurrentState, params)
		if stateErr != nil {
			return s.checkValidationResponse(in, targets, stateErr, params), nil
		}
		params, err = s.mergeSensitiveWithOps(ctx, n, proto, params, normalizedReset, in.CredentialOps)
		if err != nil {
			return s.checkValidationResponse(in, targets, err, params), nil
		}
		if err := ValidateCurrentState(proto, state, params); err != nil {
			return s.checkValidationResponse(in, targets, err, params), nil
		}
		extensionRecords, err = s.prepareExtensionOps(ctx, n.extensionRecords, state, normalizedReset, in.ExtensionOps)
		if err != nil {
			return s.checkValidationResponse(in, targets, err, params), nil
		}
	} else {
		if err := validateProtocolFields(proto, params, false); err != nil {
			return s.checkValidationResponse(in, targets, err, params), nil
		}
		state, stateErr := resolveCurrentState(proto, in.CurrentState, params)
		if stateErr != nil {
			return s.checkValidationResponse(in, targets, stateErr, params), nil
		}
		if err := ValidateCurrentState(proto, state, params); err != nil {
			return s.checkValidationResponse(in, targets, err, params), nil
		}
		extensionRecords, err = s.prepareExtensionInputs(ctx, state, in.Extensions)
		if err != nil {
			return s.checkValidationResponse(in, targets, err, params), nil
		}
	}

	state := DeriveCurrentState(proto, params)
	active := ProjectActive(proto, state, params)
	redacted := redactCheckParams(proto, active)
	response := &CheckResponse{CheckID: makeCheckID(in, redacted), CheckVersion: 1, Targets: make(map[string]TargetCheckResult, len(targets))}
	for _, target := range targets {
		result := TargetCheckResult{Diagnostics: nil}
		if validationErr := ValidateCurrentStateForTarget(proto, state, params, target); validationErr != nil {
			result.Status = "error"
			result.Diagnostics = []TargetDiagnostic{{
				Severity: "error", Code: "invalid_node_draft", Target: target,
				FieldPath: checkFieldPath(validationErr), Message: validationErr.Error(), Evidence: "build18-check-v1",
			}}
			response.Targets[target] = result
			continue
		}
		if s.checkRenderer == nil {
			result.Diagnostics = []TargetDiagnostic{{
				Severity: "error", Code: "target_checker_unavailable", Target: target,
				Message: "目标检查适配器未配置", Evidence: "build18-check-v1",
			}}
		} else {
			rendered, renderErr := s.checkRenderer(ctx, target, proto.Protocol, checkRenderName(in, existing), in.Host, in.Port, redacted)
			result.Diagnostics = append(result.Diagnostics, rendered.Diagnostics...)
			if renderErr != nil {
				result.Diagnostics = append(result.Diagnostics, TargetDiagnostic{
					Severity: "error", Code: "core_semantic_unexpressible", Target: target,
					Message: renderErr.Error(), Evidence: targetEvidenceFor(target),
				})
			} else if rendered.Preview != "" {
				preview := rendered.Preview
				result.Preview = &preview
			}
			result.Status = rendered.Status
		}
		result.Diagnostics = append(result.Diagnostics, extensionDiagnostics(target, extensionRecords)...)
		if result.Status == "" {
			result.Status = statusForDiagnostics(result.Diagnostics)
		}
		response.Targets[target] = result
	}
	return response, nil
}

func normalizeCheckTargets(targets []string) ([]string, error) {
	if len(targets) == 0 {
		return append([]string(nil), defaultCheckTargets...), nil
	}
	seen := make(map[string]bool, len(targets))
	out := make([]string, 0, len(targets))
	for _, raw := range targets {
		target := strings.TrimSpace(raw)
		if target == "" || seen[target] {
			continue
		}
		if target != "clash-yaml" && target != "sr-subs" && target != "generic-subs" {
			return nil, fmt.Errorf("节点检查不支持目标: %s", target)
		}
		seen[target] = true
		out = append(out, target)
	}
	if len(out) == 0 {
		return nil, errors.New("节点检查至少需要一个目标")
	}
	return out, nil
}

func checkRenderName(in CheckRequest, existing *Node) string {
	if existing != nil {
		return RenderName(*existing)
	}
	if name, ok := in.ProtocolJSON["name"].(string); ok && name != "" {
		return name
	}
	return "节点检查"
}

func redactCheckParams(proto Protocol, params map[string]any) map[string]any {
	out := cloneJSONMap(params)
	for _, path := range proto.SensitiveFields {
		if value, ok := GetPath(out, path); ok && hasEffectiveValue(value) {
			SetPath(out, path, "REDACTED")
		}
	}
	return out
}

func extensionDiagnostics(target string, records []ExtensionRecord) []TargetDiagnostic {
	var out []TargetDiagnostic
	for _, record := range records {
		if !containsString(record.Targets, target) {
			continue
		}
		out = append(out, TargetDiagnostic{
			Severity: "warn", Code: "unknown_extension_not_rendered", Target: target,
			FieldPath: "extensions." + record.ID, Message: "未知扩展已配置但当前节点检查适配器不会透传其负载",
			Evidence: "build18-check-v1",
		})
	}
	return out
}

func statusForDiagnostics(diagnostics []TargetDiagnostic) string {
	if len(diagnostics) == 0 {
		return "ok"
	}
	hasError := false
	allSkippable := true
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != "error" {
			continue
		}
		hasError = true
		if diagnostic.Code != "core_semantic_unexpressible" && diagnostic.Code != "target_unsupported" {
			allSkippable = false
		}
	}
	if hasError {
		if allSkippable {
			return "skip"
		}
		return "error"
	}
	return "warn"
}

func (s *Service) checkValidationResponse(in CheckRequest, targets []string, err error, params map[string]any) *CheckResponse {
	diagnostic := TargetDiagnostic{
		Severity: "error", Code: "invalid_node_draft", Message: err.Error(), Evidence: "build18-check-v1",
		FieldPath: checkFieldPath(err),
	}
	redacted := redactCheckParamsForProtocol(in.Protocol, params)
	response := &CheckResponse{CheckID: makeCheckID(in, redacted), CheckVersion: 1, Targets: make(map[string]TargetCheckResult, len(targets))}
	for _, target := range targets {
		diagnostic.Target = target
		response.Targets[target] = TargetCheckResult{Status: "error", Diagnostics: []TargetDiagnostic{diagnostic}}
	}
	return response
}

func redactCheckParamsForProtocol(protocol string, params map[string]any) map[string]any {
	proto, err := GetProtocol(protocol)
	if err != nil {
		return cloneJSONMap(params)
	}
	return redactCheckParams(proto, params)
}

func checkFieldPath(err error) string {
	message := err.Error()
	if strings.HasPrefix(message, "字段 ") {
		message = strings.TrimPrefix(message, "字段 ")
		if index := strings.IndexAny(message, " :"); index >= 0 {
			return message[:index]
		}
	}
	return ""
}

func makeCheckID(in CheckRequest, params map[string]any) string {
	raw, _ := json.Marshal(struct {
		NodeID   int64          `json:"node_id"`
		Revision int64          `json:"revision"`
		Protocol string         `json:"protocol"`
		Params   map[string]any `json:"params"`
	}{in.NodeID, in.BaseRevision, in.Protocol, params})
	hash := sha256.Sum256(raw)
	return fmt.Sprintf("chk-%d-%s", time.Now().UnixNano(), hex.EncodeToString(hash[:])[:12])
}

func targetEvidenceFor(target string) string {
	if target == "clash-yaml" {
		return "mihomo-1.19.29-yaml"
	}
	return "cvr-2.5.2-uri"
}
