// Package web 嵌入前端构建产物占位目录；容器构建时被真实 dist 覆盖。
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
