// Command errgate 是生产 Go 代码错误忽略静态门禁。
//
// 它使用 go/packages 加载真实类型信息，只报告真正把 error 类型结果赋给空白标识符、
// 或把返回 error 的调用作为表达式语句丢弃的位置。测试文件、comma-ok bool、显式
// // errgate:allow <reason> 和 defer/go 清理语句不报告。默认从 errgate_baseline.json
// 读取已确认基线；当基线无对应项、存在新违规或基线条目已过期时以非零退出。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const baselineVersion = 1

// Violation 是一条被丢弃 error 的可审计定位。基线按 File/Func/Kind/Expr 匹配；
// Kind 只使用 assign/expr，避免把 defer/go 明确清理语义纳入本门禁。
type Violation struct {
	File string `json:"file"`
	Func string `json:"func"`
	Kind string `json:"kind"`
	Expr string `json:"expr"`
}

// BaselineEntry 是错误门禁基线条目；Reason 只用于审计，不参与匹配。
type BaselineEntry struct {
	File   string `json:"file"`
	Func   string `json:"func"`
	Kind   string `json:"kind"`
	Expr   string `json:"expr"`
	Reason string `json:"reason,omitempty"`
}

type baselineFile struct {
	Version int             `json:"version"`
	Allowed []BaselineEntry `json:"allowed"`
}

func (v Violation) key() string {
	return strings.Join([]string{v.File, v.Func, v.Kind, v.Expr}, "\x00")
}

func (e BaselineEntry) key() string {
	return strings.Join([]string{e.File, e.Func, e.Kind, e.Expr}, "\x00")
}

type funcRange struct {
	start, end token.Pos
	name       string
}

func main() {
	baselinePath := flag.String("baseline", "errgate_baseline.json", "基线 JSON 文件路径；传入空字符串等价于空基线")
	list := flag.Bool("list", false, "列出当前违规 JSON 后退出（不比较基线）")
	writeBaseline := flag.Bool("write-baseline", false, "把当前违规写入 -baseline 指定文件（仅用于 Step 1 建立基线；条目使用待分类原因）")
	flag.Parse()

	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	violations, err := scan(patterns)
	if err != nil {
		fmt.Fprintln(os.Stderr, "errgate:", err)
		os.Exit(2)
	}
	if *writeBaseline {
		if err := writeBaselineFile(*baselinePath, violations); err != nil {
			fmt.Fprintln(os.Stderr, "errgate:", err)
			os.Exit(2)
		}
		fmt.Printf("errgate: 已写入基线 %s（%d 项待分类审计）\n", *baselinePath, len(violations))
		return
	}
	if *list {
		data, err := json.MarshalIndent(violations, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "errgate:", err)
			os.Exit(2)
		}
		fmt.Println(string(data))
		return
	}

	allowed, err := loadBaseline(*baselinePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "errgate:", err)
		os.Exit(2)
	}
	if unexpected, stale := compare(violations, allowed); len(unexpected) > 0 || len(stale) > 0 {
		printReport(unexpected, stale)
		os.Exit(1)
	}
	fmt.Printf("errgate: OK (%d ignored errors matched baseline; baseline entries %d; 0 unexpected)\n", len(violations), len(allowed))
}

func scan(patterns []string) ([]Violation, error) {
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("加载包时发现编译错误")
	}
	base, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("获取当前工作目录失败: %w", err)
	}
	var out []Violation
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			absPath := pkg.Fset.Position(file.Pos()).Filename
			if isTestFile(absPath) {
				continue
			}
			if pkg.TypesInfo == nil {
				return nil, fmt.Errorf("包 %s 缺少类型信息", pkg.PkgPath)
			}
			filePath := relativePath(base, absPath)
			errorType := types.Universe.Lookup("error").Type()
			allowLines := fileAllows(pkg.Fset, file)
			ranges := collectFuncRanges(pkg.Fset, file)
			for _, fn := range ranges {
				body := funcBody(file, fn)
				if body == nil {
					continue
				}
				ast.Inspect(body, func(n ast.Node) bool {
					if n == nil {
						return true
					}
					if _, ok := n.(*ast.FuncLit); ok {
						return false // 嵌套函数由自己的范围单独检查，避免重复
					}
					stmt, ok := n.(ast.Stmt)
					if !ok {
						return true
					}
					for _, v := range violationsForStmt(pkg.Fset, allowLines, filePath, fn.name, stmt, pkg.TypesInfo, errorType) {
						out = append(out, v)
					}
					return true
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Func != out[j].Func {
			return out[i].Func < out[j].Func
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Expr < out[j].Expr
	})
	return out, nil
}

func isTestFile(path string) bool {
	return strings.HasSuffix(filepath.ToSlash(path), "_test.go")
}

func collectFuncRanges(fset *token.FileSet, file *ast.File) []funcRange {
	var ranges []funcRange
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			name := x.Name.Name
			if x.Recv != nil && len(x.Recv.List) > 0 {
				name = "(" + formatNode(fset, x.Recv.List[0].Type) + ")." + name
			}
			ranges = append(ranges, funcRange{start: x.Pos(), end: x.End(), name: name})
		case *ast.FuncLit:
			ranges = append(ranges, funcRange{start: x.Pos(), end: x.End(), name: "<funcLit>"})
		}
		return true
	})
	return ranges
}

func funcBody(file *ast.File, fn funcRange) *ast.BlockStmt {
	var body *ast.BlockStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if body != nil {
			return false
		}
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Pos() == fn.start {
				body = x.Body
				return false
			}
		case *ast.FuncLit:
			if x.Pos() == fn.start {
				body = x.Body
				return false
			}
		}
		return true
	})
	return body
}

func violationsForStmt(fset *token.FileSet, allowLines map[int]string, file string, fn string, stmt ast.Stmt, info *types.Info, errorType types.Type) []Violation {
	if _, ok := allowLines[fset.Position(stmt.Pos()).Line]; ok {
		return nil
	}
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		call, ok := s.X.(*ast.CallExpr)
		if !ok || skipDeliberateDiscard(info, call) || !callTypeContainsError(info, call, errorType) {
			return nil
		}
		return []Violation{{File: file, Func: fn, Kind: "expr", Expr: types.ExprString(call)}}
	case *ast.AssignStmt:
		var out []Violation
		if len(s.Rhs) == 0 {
			return nil
		}
		if len(s.Rhs) == 1 {
			call, ok := s.Rhs[0].(*ast.CallExpr)
			if !ok || skipDeliberateDiscard(info, call) {
				return nil
			}
			t := info.TypeOf(call)
			if tuple, ok := t.(*types.Tuple); ok {
				for i := 0; i < len(s.Lhs) && i < tuple.Len(); i++ {
					if isBlankIdent(s.Lhs[i]) && isErrorType(tuple.At(i).Type(), errorType) {
						out = append(out, violation(file, fn, call))
						break
					}
				}
				return out
			}
			if len(s.Lhs) == 1 && isBlankIdent(s.Lhs[0]) && isErrorType(t, errorType) {
				out = append(out, violation(file, fn, call))
			}
			return out
		}
		for i := 0; i < len(s.Lhs) && i < len(s.Rhs); i++ {
			if !isBlankIdent(s.Lhs[i]) {
				continue
			}
			call, ok := s.Rhs[i].(*ast.CallExpr)
			if !ok || skipDeliberateDiscard(info, call) || !isErrorType(info.TypeOf(call), errorType) {
				continue
			}
			out = append(out, violation(file, fn, call))
		}
		return out
	default:
		return nil
	}
}

// skipDeliberateDiscard 识别用户已确认的明确清理/诊断输出/内存写语义：
// Close/Rollback 清理、标准库 fmt 输出、os.Remove 失败清理、strings.Builder/bytes.Buffer
// 这类不会返回业务错误的写操作。它们不进入 error 门禁基线，避免把清理语义误报为缺陷。
func skipDeliberateDiscard(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "Close", "Rollback":
		return true
	case "Remove":
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "os" {
			return true
		}
	}
	if id, ok := sel.X.(*ast.Ident); ok {
		switch id.Name {
		case "fmt":
			name := sel.Sel.Name
			return strings.HasPrefix(name, "Fprint") || strings.HasPrefix(name, "Print")
		case "bytes", "strings":
			name := sel.Sel.Name
			return strings.HasPrefix(name, "Write") || strings.HasPrefix(name, "WriteRune")
		}
	}
	if recv := info.TypeOf(sel.X); recv != nil {
		base := recv
		if ptr, ok := base.(*types.Pointer); ok {
			base = ptr.Elem()
		}
		if named, ok := base.(*types.Named); ok && named.Obj() != nil && named.Obj().Pkg() != nil {
			pkgPath := named.Obj().Pkg().Path()
			name := named.Obj().Name()
			if (pkgPath == "strings" && name == "Builder") || (pkgPath == "bytes" && name == "Buffer") {
				return strings.HasPrefix(sel.Sel.Name, "Write")
			}
		}
	}
	return false
}

func violation(file, fn string, call *ast.CallExpr) Violation {
	return Violation{File: file, Func: fn, Kind: "assign", Expr: types.ExprString(call)}
}

func isBlankIdent(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == "_"
}

func callTypeContainsError(info *types.Info, call *ast.CallExpr, errorType types.Type) bool {
	t := info.TypeOf(call)
	if tuple, ok := t.(*types.Tuple); ok {
		for i := 0; i < tuple.Len(); i++ {
			if isErrorType(tuple.At(i).Type(), errorType) {
				return true
			}
		}
		return false
	}
	return isErrorType(t, errorType)
}

func isErrorType(t types.Type, errorType types.Type) bool {
	if t == nil {
		return false
	}
	if types.Identical(t, errorType) {
		return true
	}
	iface, ok := errorType.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	return types.Implements(t, iface)
}

func relativePath(base, abs string) string {
	rel, err := filepath.Rel(base, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

// fileAllows 返回某文件被 allow 注释覆盖的行号集合。注释写在违规语句同一行，
// 或违规语句上一行（紧邻）时生效；理由必须非空。
func fileAllows(fset *token.FileSet, file *ast.File) map[int]string {
	out := map[int]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		cg, ok := n.(*ast.CommentGroup)
		if !ok || !strings.Contains(cg.Text(), "errgate:allow") {
			return true
		}
		reason := strings.TrimSpace(strings.TrimPrefix(cg.Text(), "// errgate:allow"))
		reason = strings.TrimSpace(strings.TrimPrefix(reason, "errgate:allow"))
		if reason == "" {
			return true
		}
		start := fset.Position(cg.Pos()).Line
		end := fset.Position(cg.End()).Line
		// 同一行（start）或紧邻上一行（end == line-1）生效；把触发行直接映射到理由。
		out[start] = reason
		out[end] = reason
		out[end+1] = reason
		return true
	})
	return out
}

func formatNode(fset *token.FileSet, node any) string {
	var b strings.Builder
	if n, ok := node.(ast.Node); ok {
		if err := format.Node(&b, fset, n); err == nil {
			return b.String()
		}
	}
	return "<unknown>"
}

func writeBaselineFile(path string, violations []Violation) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("baseline 路径不能为空")
	}
	allowed := make([]BaselineEntry, 0, len(violations))
	for _, v := range violations {
		allowed = append(allowed, BaselineEntry{
			File: v.File, Func: v.Func, Kind: v.Kind, Expr: v.Expr,
			Reason: "Build26 Step 1 当前已确认忽略点；待 R28-07C3 分类修复/允许",
		})
	}
	data, err := json.MarshalIndent(baselineFile{Version: baselineVersion, Allowed: allowed}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func loadBaseline(path string) ([]BaselineEntry, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var b baselineFile
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("解析基线 %s 失败: %w", path, err)
	}
	if b.Version != baselineVersion {
		return nil, fmt.Errorf("基线版本 %d 不受支持", b.Version)
	}
	return b.Allowed, nil
}

func compare(violations []Violation, allowed []BaselineEntry) (unexpected []Violation, stale []BaselineEntry) {
	remaining := map[string]int{}
	for _, a := range allowed {
		remaining[a.key()]++
	}
	for _, v := range violations {
		if remaining[v.key()] > 0 {
			remaining[v.key()]--
			continue
		}
		unexpected = append(unexpected, v)
	}
	for _, a := range allowed {
		if remaining[a.key()] > 0 {
			remaining[a.key()]--
			stale = append(stale, a)
		}
	}
	return unexpected, stale
}

func printReport(unexpected []Violation, stale []BaselineEntry) {
	if len(unexpected) > 0 {
		fmt.Fprintln(os.Stderr, "errgate: 发现未在基线中的被丢弃 error:")
		for _, v := range unexpected {
			fmt.Fprintf(os.Stderr, "  %s %s [%s] %s\n", v.File, v.Func, v.Kind, v.Expr)
		}
	}
	if len(stale) > 0 {
		fmt.Fprintln(os.Stderr, "errgate: 基线中存在已不存在的条目（请删除）:")
		for _, e := range stale {
			fmt.Fprintf(os.Stderr, "  %s %s [%s] %s\n", e.File, e.Func, e.Kind, e.Expr)
		}
	}
}
