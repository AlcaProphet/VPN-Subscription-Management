// diagnose.go：装配输出阶段的节点级目标诊断（Build20 Step 4）。
package assembly

import (
	"context"
	"fmt"

	"vpn-sub/internal/node"
)

// NodeDiagnostic 是输出阶段附加到节点的目标诊断。
type NodeDiagnostic struct {
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	Target    string `json:"target"`
	FieldPath string `json:"field_path,omitempty"`
	Message   string `json:"message"`
}

// diagnoseNodeForTarget 使用实际目标检查器获取节点诊断，不保存任何状态。
func (s *Service) diagnoseNodeForTarget(target string, nd *nodeData) []NodeDiagnostic {
	res, err := s.CheckNodeTarget(context.Background(), target, nd.Protocol, nd.RenderName, nd.Host, nd.Port, activeProtocolJSON(nd))
	if err != nil {
		return []NodeDiagnostic{{
			Severity: "error",
			Code:     "core_semantic_unexpressible",
			Target:   target,
			Message:  err.Error(),
		}}
	}
	return toNodeDiagnostics(res.Diagnostics)
}

func toNodeDiagnostics(diags []node.TargetDiagnostic) []NodeDiagnostic {
	out := make([]NodeDiagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, NodeDiagnostic{
			Severity:  d.Severity,
			Code:      d.Code,
			Target:    d.Target,
			FieldPath: d.FieldPath,
			Message:   d.Message,
		})
	}
	return out
}

func hasBlockingNodeDiagnostic(diags []NodeDiagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

func hasCoreBlockingNodeDiagnostic(diags []NodeDiagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" && (d.Code == "core_semantic_unexpressible" || d.Code == "target_unsupported") {
			return true
		}
	}
	return false
}

func firstNodeDiagnosticMessage(diags []NodeDiagnostic) string {
	for _, d := range diags {
		if d.Severity == "error" {
			if d.FieldPath != "" {
				return fmt.Sprintf("%s (%s)", d.Message, d.FieldPath)
			}
			return d.Message
		}
	}
	if len(diags) > 0 {
		return diags[0].Message
	}
	return ""
}
