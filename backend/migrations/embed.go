// Package migrations 嵌入 SQL 迁移文件，保证单二进制分发。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
