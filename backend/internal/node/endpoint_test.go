package node

import (
	"context"
	"strings"
	"testing"
)

func TestDefaultEndpointPolicyKeepsLegacyRequiredSemantics(t *testing.T) {
	proto := selectorTestProtocol()
	if err := ValidateEndpoint(proto, CurrentState{}, "", 0); err == nil || !strings.Contains(err.Error(), "服务器地址不能为空") {
		t.Fatalf("普通协议空 endpoint 应失败: %v", err)
	}
	if err := ValidateEndpoint(proto, CurrentState{}, "example.com", 0); err == nil || !strings.Contains(err.Error(), "端口须在 1-65535") {
		t.Fatalf("普通协议非法端口应失败: %v", err)
	}
	host, port, policy, err := NormalizeEndpoint(proto, CurrentState{}, " example.com ", 443)
	if err != nil {
		t.Fatal(err)
	}
	if host != " example.com " || port != 443 || !policy.EmitHost || !policy.EmitPort {
		t.Fatalf("默认 endpoint policy 规范化异常: host=%q port=%d policy=%+v", host, port, policy)
	}
}

func TestHiddenEndpointPolicyNormalizesAndDoesNotEmit(t *testing.T) {
	proto := selectorTestProtocol()
	proto.EndpointPolicies = []EndpointPolicy{{
		HostMode: "hidden", PortMode: "hidden", EmitHost: false, EmitPort: false,
	}}
	host, port, policy, err := NormalizeEndpoint(proto, CurrentState{}, "example.com", 443)
	if err != nil {
		t.Fatal(err)
	}
	if host != "" || port != 0 || policy.EmitHost || policy.EmitPort {
		t.Fatalf("hidden endpoint 未规范化: host=%q port=%d policy=%+v", host, port, policy)
	}
}

func TestEndpointPolicySelectorCondition(t *testing.T) {
	proto := selectorTestProtocol()
	proto.EndpointPolicies = []EndpointPolicy{
		{When: &ConditionRule{Selectors: map[string][]string{"mode": {"a"}}}, HostMode: "required", PortMode: "hidden", EmitHost: true, EmitPort: false},
		{When: &ConditionRule{Selectors: map[string][]string{"mode": {"b"}}}, HostMode: "required", PortMode: "required", EmitHost: true, EmitPort: true},
	}
	hostA, portA, policyA, err := NormalizeEndpoint(proto, CurrentState{Selectors: map[string]string{"mode": "a"}}, "a.example.com", 443)
	if err != nil {
		t.Fatal(err)
	}
	if hostA != "a.example.com" || portA != 0 || policyA.PortMode != "hidden" {
		t.Fatalf("selector=a endpoint policy 错误: host=%q port=%d policy=%+v", hostA, portA, policyA)
	}
	if _, portB, _, err := NormalizeEndpoint(proto, CurrentState{Selectors: map[string]string{"mode": "b"}}, "b.example.com", 443); err != nil || portB != 443 {
		t.Fatalf("selector=b endpoint policy 错误: port=%d err=%v", portB, err)
	}
}

func TestEndpointPolicyRegistrationAndMatchErrors(t *testing.T) {
	proto := selectorTestProtocol()
	proto.EndpointPolicies = []EndpointPolicy{{
		HostMode: "hidden", PortMode: "hidden", EmitHost: true, EmitPort: false,
	}}
	if err := validateProtocolEndpointPolicies(proto); err == nil {
		t.Fatal("hidden host 输出应注册失败")
	}
	proto.EndpointPolicies = []EndpointPolicy{{}, {}}
	if _, err := MatchEndpointPolicy(proto, CurrentState{}); err == nil || !strings.Contains(err.Error(), "必须且只能命中一条") {
		t.Fatalf("多 policy 命中应失败: %v", err)
	}
}

func TestStatusForDiagnosticsIgnoresInfoEvidence(t *testing.T) {
	if got := statusForDiagnostics([]TargetDiagnostic{{Severity: "info", Code: "legacy_adapter_pending"}}); got != "ok" {
		t.Fatalf("info 证据不应改变目标状态: %s", got)
	}
}

func TestHiddenEndpointServiceNormalizesHostAndPort(t *testing.T) {
	proto := selectorTestProtocol()
	proto.Protocol = "endpoint-hidden-api"
	proto.Label = "Hidden Endpoint Test"
	proto.EndpointPolicies = []EndpointPolicy{{
		HostMode: "hidden", PortMode: "hidden", EmitHost: false, EmitPort: false,
	}}
	protocolIndex[proto.Protocol] = proto
	defer delete(protocolIndex, proto.Protocol)

	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), CreateManualInput{
		Name: "hidden-endpoint", Protocol: proto.Protocol, Host: "attacker.example", Port: 443,
		ProtocolJSON: map[string]any{"branch": "a"},
	})
	if err != nil {
		t.Fatalf("创建 hidden endpoint 节点失败: %v", err)
	}
	if created.Host != "" || created.Port != 0 {
		t.Fatalf("hidden endpoint 应规范化为空值: host=%q port=%d", created.Host, created.Port)
	}
}
