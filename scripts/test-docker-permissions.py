#!/usr/bin/env python3
"""用独立 Docker 卷中的 Linux 目录验证镜像权限、初始化和启动失败。"""

import argparse
import json
import subprocess
import time
import urllib.error
import urllib.request
import uuid


parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("image", help="待验证的本地镜像")
args = parser.parse_args()
prefix = "vpn-permissions-" + uuid.uuid4().hex[:10]
containers = []
volumes = []


def docker(*command, check=True):
    result = subprocess.run(["docker", *command], capture_output=True, text=True)
    if check and result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    if command[0] == "logs":
        return (result.stdout + result.stderr).strip()
    return result.stdout.strip()


def new_volume(suffix):
    name = prefix + "-" + suffix
    docker("volume", "create", name)
    volumes.append(name)
    return name


def fixture(script):
    return docker(
        "run", "--rm", "--user", "0", "--mount",
        f"type=volume,source={fixture_volume},target=/fixture",
        "--entrypoint", "/bin/sh", args.image, "-ec", script,
    )


def start(suffix, *, readonly=False, user=None, data_dir="/data", raw=False, env=()):
    name = prefix + "-" + suffix + ("-raw" if raw else "")
    options = []
    if user:
        options += ["--user", user]
    if raw:
        options += ["--entrypoint", "/server"]
    for value in env:
        options += ["-e", value]
    source = fixture_source + "/" + suffix
    mount = f"type=bind,source={source},target={data_dir}"
    if readonly:
        mount += ",readonly"
    docker("run", "-d", "--name", name, "-p", "127.0.0.1::8080",
           "--mount", mount, "-e", "DATA_DIR=" + data_dir, *options, args.image)
    containers.append(name)
    return name


def request(name, path, method="GET"):
    address = docker("port", name, "8080/tcp")
    req = urllib.request.Request("http://" + address + path, method=method)
    with urllib.request.urlopen(req, timeout=2) as response:
        return json.load(response)


def status(name, configured=False, reason=""):
    for _ in range(60):
        try:
            data = request(name, "/api/system/status")["data"]
            break
        except (OSError, ValueError):
            if docker("inspect", "-f", "{{.State.Running}}", name) == "false":
                raise AssertionError(docker("logs", name))
            time.sleep(0.2)
    else:
        raise AssertionError("状态接口不可用\n" + docker("logs", name))
    assert (data["configured"], data["emergency"], data["emergency_reason"]) == (
        configured, bool(reason), reason
    ), data
    try:
        request(name, "/health")
        assert not reason
    except urllib.error.HTTPError as error:
        assert reason and error.code == 503


def identity(name, uid, gid):
    output = docker("exec", name, "cat", "/proc/1/status")
    fields = dict(line.split(":", 1) for line in output.splitlines() if ":" in line)
    assert fields["Uid"].split() == [str(uid)] * 4, output
    assert fields["Gid"].split() == [str(gid)] * 4, output
    assert int(fields["CapEff"].strip(), 16) == 0, output
    assert docker("exec", "--user", f"{uid}:{gid}", name, "readlink", "/proc/1/exe") == "/server"


def failed(name, message):
    for _ in range(60):
        if docker("inspect", "-f", "{{.State.Running}}", name) == "false":
            break
        time.sleep(0.2)
    else:
        raise AssertionError("服务意外保持运行\n" + docker("logs", name))
    logs = docker("logs", name)
    assert docker("inspect", "-f", "{{.State.ExitCode}}", name) == "1", logs
    assert message in logs and "应急恢复模式已触发" not in logs, logs


fixture_volume = new_volume("fixture")
fixture_source = docker("volume", "inspect", "-f", "{{.Mountpoint}}", fixture_volume)
try:
    fixture("""
        mkdir /fixture/root /fixture/owner /fixture/explicit /fixture/mismatch
        mkdir /fixture/readonly /fixture/oldfile /fixture/corrupt /fixture/manual
        mkdir /fixture/raw /fixture/custom /fixture/dev /fixture/sidecar /fixture/empty-system
        chown 17001:17002 /fixture/owner /fixture/explicit /fixture/readonly
        chown 17001:17002 /fixture/oldfile /fixture/custom /fixture/dev
        chown 17001:17002 /fixture/sidecar
        chown 100:101 /fixture/empty-system
        chmod 0700 /fixture/owner /fixture/explicit /fixture/readonly
        touch /fixture/oldfile/app-prod.db
        chmod 0600 /fixture/oldfile/app-prod.db
        touch /fixture/sidecar/app-prod.db-wal
        chmod 0600 /fixture/sidecar/app-prod.db-wal
        printf 'invalid SQLite file' > /fixture/corrupt/app-prod.db
        chown app:app /fixture/corrupt/app-prod.db
    """)
    name = start("root")
    status(name)
    identity(name, 10001, 10001)
    assert fixture("stat -c '%u:%g' /fixture/root") == "10001:10001"
    assert request(name, "/api/setup/quickstart", "POST")["data"]["configured"]
    docker("restart", name)
    status(name, configured=True)
    identity(name, 10001, 10001)
    docker("stop", name)
    assert docker("inspect", "-f", "{{.State.ExitCode}}", name) == "0"
    fixture("mkdir /fixture/oldroot; cp -p /fixture/root/app-prod.db* /fixture/oldroot/")
    old_name = start("oldroot")
    status(old_name, configured=True)
    identity(old_name, 10001, 10001)
    fixture("rm /fixture/root/app-prod.db")
    docker("start", name)
    status(name)
    print("PASS root 0755 bind、非 root PID 1、Setup、重启、信号退出、旧库目录、只删主库后初始化")

    name = start("empty-system")
    status(name)
    identity(name, 10001, 10001)
    assert fixture("stat -c '%u:%g' /fixture/empty-system") == "10001:10001"
    print("PASS 100:101 空目录纠正为固定 10001:10001")

    name = start("owner")
    status(name)
    identity(name, 17001, 17002)
    assert fixture("stat -c '%u:%g %a' /fixture/owner") == "17001:17002 700"
    print("PASS 用户 0700 目录自动匹配身份并保留属主和权限")

    name = start("explicit", user="17001:17002")
    status(name)
    identity(name, 17001, 17002)
    failed(start("mismatch", user="17001:17002"), "运行用户没有目录读写权限")
    failed(start("readonly", readonly=True), "无法写入目录")
    failed(start("oldfile"), "已有数据库文件不可读写 app-prod.db")
    failed(start("sidecar"), "已有数据库文件不可读写 app-prod.db-wal")
    failed(start("sidecar", user="17001:17002", raw=True), "请检查数据目录权限、挂载和空间")
    assert fixture("stat -c '%u:%g %a' /fixture/oldfile/app-prod.db") == "0:0 600"
    print("PASS 显式用户、身份不匹配、只读目录、已有 SQLite 权限冲突")

    name = start("corrupt")
    status(name, reason="db_corrupt")
    identity(name, 10001, 10001)
    name = start("manual", env=("RESET_ADMIN_PASSWORD=1",))
    status(name, reason="manual")
    identity(name, 10001, 10001)
    failed(start("raw", user="17001:17002", raw=True), "数据库存储不可访问")
    print("PASS 实际损坏与手动应急、绕过入口时的后端访问错误分类")

    name = start("custom", data_dir="/custom data")
    status(name)
    identity(name, 17001, 17002)
    name = start("dev", env=("APP_MODE=dev",))
    status(name)
    assert fixture("test -f /fixture/dev/app-dev.db; test ! -e /fixture/dev/app-prod.db") == ""
    print("PASS 自定义含空格 DATA_DIR、dev 模式独立数据库")

    volume = new_volume("named")
    name = prefix + "-named"
    docker("run", "-d", "--name", name, "-p", "127.0.0.1::8080", "--mount",
           f"type=volume,source={volume},target=/data", args.image)
    containers.append(name)
    status(name)
    identity(name, 10001, 10001)
    print("PASS 默认命名卷初始化")
finally:
    for name in containers:
        docker("rm", "-f", name, check=False)
    for volume in reversed(volumes):
        docker("volume", "rm", volume, check=False)
