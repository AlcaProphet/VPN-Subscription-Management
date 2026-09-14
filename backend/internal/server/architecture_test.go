package server

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// architectureAllowlist 记录当前已确认、尚待后续 Step 拆除的接入层直访存储位置。
// 键为仓库 backend 目录下的相对路径；value 为拆除 Step 与业务原因。
// Step 4～7 每完成一个服务化迁移，必须删除对应条目；本 map 最终必须为空。
var architectureAllowlist = map[string]string{}

type architectureViolation struct {
	File string
	Line int
	Expr string
}

// scanArchitectureNoDirectStore 扫描 server 包非测试生产文件中的 DB()/TxImmediate() 调用。
// 测试文件不参与扫描；调用形态按 AST 判断，避免把注释或普通字符串误判为违规。
func scanArchitectureNoDirectStore(dir string) ([]architectureViolation, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var out []architectureViolation
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("解析 %s 失败: %w", entry.Name(), err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "DB" && sel.Sel.Name != "TxImmediate") {
				return true
			}
			pos := fset.Position(call.Pos())
			out = append(out, architectureViolation{
				File: entry.Name(),
				Line: pos.Line,
				Expr: fmt.Sprintf("%s.%s(...)", exprString(sel.X), sel.Sel.Name),
			})
			return true
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}

// exprString 仅用于错误信息中的稳定展示，不参与扫描判定。
func exprString(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	default:
		return "<expr>"
	}
}

func TestArchitectureNoDirectStore(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位 architecture_test.go 路径")
	}
	dir := filepath.Dir(currentFile)
	violations, err := scanArchitectureNoDirectStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	var unexpected []architectureViolation
	seen := map[string]bool{}
	for _, v := range violations {
		seen[v.File] = true
		if _, ok := architectureAllowlist[v.File]; !ok {
			unexpected = append(unexpected, v)
		}
	}
	if len(unexpected) > 0 {
		var lines []string
		for _, v := range unexpected {
			lines = append(lines, fmt.Sprintf("%s:%d %s", v.File, v.Line, v.Expr))
		}
		t.Fatalf("internal/server 生产文件仍直接访问 DB()/TxImmediate()，且不在当前允许清单中:\n%s", strings.Join(lines, "\n"))
	}
	for file := range architectureAllowlist {
		if !seen[file] {
			t.Fatalf("架构门禁允许清单包含已不存在的违规项: %s（迁移完成后必须删除该条目）", file)
		}
	}
}
