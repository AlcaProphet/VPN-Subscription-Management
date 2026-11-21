#!/bin/sh
set -eu

fail() {
    printf '数据目录准备失败 uid=%s gid=%s path=%s reason=%s\n' "$(id -u)" "$(id -g)" "$data_dir" "$1" >&2
    exit 1
}

data_dir=${DATA_DIR:-/data}
app_mode=${APP_MODE:-prod}
case "$app_mode" in
    dev|prod) ;;
    *) fail 'APP_MODE 仅支持 dev|prod' ;;
esac
case "$data_dir" in
    /*) ;;
    *) fail 'DATA_DIR 必须为绝对路径' ;;
esac
case "/$data_dir/" in
    */../*) fail 'DATA_DIR 不允许包含 .. 路径段' ;;
esac
mkdir -p "$data_dir" || fail '无法创建目录，请检查挂载权限'
data_dir=$(cd "$data_dir" && pwd -P) || fail '无法访问目录'
[ "$data_dir" != / ] || fail 'DATA_DIR 不能是根目录'
export DATA_DIR="$data_dir"

if [ "$(id -u)" = 0 ]; then
    # 按实际挂载目录属主选择业务身份，自动修正仅调整目录自身的属主。
    owner_uid=$(stat -c %u "$data_dir")
    owner_gid=$(stat -c %g "$data_dir")
    use_app_identity=false
    if [ "$owner_uid" = 0 ]; then
        use_app_identity=true
    fi
    # v4.1.5 自动分配过 100:101，仅纠正该身份持有的空目录。
    if [ "$owner_uid:$owner_gid" = 100:101 ]; then
        entries=$(find "$data_dir" -mindepth 1 -maxdepth 1 -exec printf x \; -quit) || fail '无法检查目录内容'
        if [ -z "$entries" ]; then
            use_app_identity=true
        fi
    fi
    if [ "$use_app_identity" = true ]; then
        owner_uid=$(id -u app)
        owner_gid=$(id -g app)
        chown "$owner_uid:$owner_gid" "$data_dir" || fail '无法设置目录属主，请检查挂载是否只读或限制 chown'
    fi
    exec su-exec "$owner_uid:$owner_gid" "$0" "$@"
fi

# 显式指定非 root 用户时沿用该身份，以真实写入验证目录及挂载的可用性。
[ -r "$data_dir" ] && [ -w "$data_dir" ] && [ -x "$data_dir" ] || fail '运行用户没有目录读写权限'
probe=$(mktemp "$data_dir/.vpn-write-check.XXXXXX") || fail '无法写入目录，请检查挂载是否只读或空间不足'
rm "$probe" || fail '无法删除目录中的临时文件'
db_name="app-$app_mode.db"
for suffix in '' -wal -shm; do
    db_path="$data_dir/$db_name$suffix"
    if [ -e "$db_path" ] || [ -L "$db_path" ]; then
        [ -f "$db_path" ] && [ -r "$db_path" ] && [ -w "$db_path" ] || fail "已有数据库文件不可读写 $db_name$suffix"
    fi
done
exec "$@"
