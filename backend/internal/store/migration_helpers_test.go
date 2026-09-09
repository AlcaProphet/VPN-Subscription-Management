package store

import (
	"io/fs"
	"testing/fstest"

	"vpn-sub/migrations"
)

// migrationsThrough 返回真实迁移文件中版本号 <= target 的 MapFS。
// 禁止把 1016/1017/未来迁移混入 1015 FS；按解析版本过滤而非文件名 glob。
func migrationsThrough(target int) fstest.MapFS {
	out := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		v, err := parseVersion(e.Name())
		if err != nil {
			continue
		}
		if v > target {
			continue
		}
		data, err := fs.ReadFile(migrations.FS, e.Name())
		if err != nil {
			panic(err)
		}
		out[e.Name()] = &fstest.MapFile{Data: data}
	}
	return out
}
