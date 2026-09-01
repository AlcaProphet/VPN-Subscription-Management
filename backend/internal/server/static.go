package server

import (
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"vpn-sub/web"
)

// securityHeaders 全局安全响应头；当前仅增加 nosniff，不扩大行为面。
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	}
}

// registerStatic 静态资源分级 + SPA 回退（Design1 §5.6，缓存策略见 Design1 表格）。
// 需在 health/status 之后注册（NoRoute 兜底不影响已注册路由）。
func registerStatic(engine *gin.Engine, dataDir string) error {
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("提取嵌入前端产物失败: %w", err)
	}

	// /assets/*：前端产物（文件名含哈希）→ immutable 长期缓存。
	// 注：不用 http.FileServer 的 FileFromFS——其对 *filepath 路由参数处理不可靠（返回 404），
	// 改为直接读取嵌入文件内容返回。
	engine.GET("/assets/*filepath", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		name := "assets" + c.Param("filepath")
		data, err := fs.ReadFile(distFS, name)
		if err != nil {
			Fail(c, http.StatusNotFound, "资源不存在")
			return
		}
		ctype := mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		c.Data(http.StatusOK, ctype, data)
	})

	// /public/*：数据卷内可缓存资源（安装包/站点 ICON）→ public + max-age；
	// 路径穿越防护：Clean 后校验仍在 dataDir/public 内（禁止 .. 与绝对路径逃逸）
	publicRoot := filepath.Join(dataDir, "public")
	engine.GET("/public/*filepath", func(c *gin.Context) {
		rel := filepath.Clean(strings.TrimPrefix(c.Param("filepath"), "/"))
		if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			Fail(c, http.StatusNotFound, "资源不存在")
			return
		}
		full := filepath.Join(publicRoot, rel)
		if !strings.HasPrefix(full, publicRoot+string(os.PathSeparator)) {
			Fail(c, http.StatusNotFound, "资源不存在")
			return
		}
		c.Header("Cache-Control", "public, max-age=86400")
		// 安装包统一以附件下载，避免同源新标签页执行 HTML/SVG 等造成持久型 XSS。
		if strings.HasPrefix(rel, "installers/") {
			filename := filepath.Base(full)
			c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
		}
		clearWriteDeadline(c)
		c.File(full) // 文件不存在时由 gin 返回 404
	})

	// 其余非 API GET 路径：SPA 回退到 index.html（不缓存，保证新版本即时生效）。
	// 注：不用 http.FileServer 的 FileFromFS——其对以 /index.html 结尾的路径返回 301 重定向（标准库行为），
	// 会导致 SPA 回退无限重定向；改为直接读取嵌入文件内容返回。
	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return fmt.Errorf("读取嵌入 index.html 失败: %w", err)
	}
	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet || strings.HasPrefix(c.Request.URL.Path, "/api/") {
			// 未匹配的 /api/* GET 也返回 404 JSON，避免 SPA 回退吞掉 API 错误（R14-01 用户决策）
			Fail(c, http.StatusNotFound, "接口不存在")
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})
	return nil
}
