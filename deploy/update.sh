#!/usr/bin/env bash
# 拉最新镜像并更新 Timmypanel 容器。两种部署方式都认：
#
#   - 当前目录有 docker-compose.yml → 走 compose（pull + up -d）
#   - 没有                          → 按 README 那条 docker run 重建同名容器
#
# 用法（compose 那条路要在放 docker-compose.yml 的目录里跑）：
#
#   curl -fsSLO https://raw.githubusercontent.com/lovetimmy1314/timmypanel/main/deploy/update.sh
#   chmod +x update.sh
#   ./update.sh
#
# 可用环境变量，与 docker-compose.yml 里同名：
#   TP_TAG              镜像 tag，默认 latest。注意**不带 v**（git tag v1.2.3 出的镜像是 1.2.3）
#   TP_PORT             宿主机端口，默认 8080。无论如何都只发布到 127.0.0.1
#   TP_NAME             容器名，默认 timmypanel
#   TP_VOLUME           数据卷名，默认 timmypanel-data（只有 docker run 那条路用得上）
#   TP_TRUSTED_PROXIES  反代源地址，默认现场探测 bridge 网关（只有 docker run 那条路用得上）
#
# 这个脚本只管「换镜像重启」，**不备份卷**。数据在命名卷里，升级不碰它；
# 要整卷备份见 README 里那条 tar 命令，应用自己也每天留快照（决策 030）。

set -euo pipefail

REPO="ghcr.io/lovetimmy1314/timmypanel"
IMAGE="${REPO}:${TP_TAG:-latest}"
NAME="${TP_NAME:-timmypanel}"

command -v docker >/dev/null 2>&1 || {
  echo "找不到 docker，先把 docker 装上再跑这个脚本" >&2
  exit 1
}

# compose v2 是 docker 的子命令，v1 是独立的 docker-compose 可执行文件，两个都认
COMPOSE=""
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE="docker-compose"
fi

HAS_COMPOSE_FILE=""
for f in docker-compose.yml docker-compose.yaml compose.yml compose.yaml; do
  [ -f "$f" ] && HAS_COMPOSE_FILE="$f" && break
done

if [ -n "$COMPOSE" ] && [ -n "$HAS_COMPOSE_FILE" ]; then
  echo "==> compose 模式（$COMPOSE，$HAS_COMPOSE_FILE）"
  $COMPOSE pull
  $COMPOSE up -d
else
  echo "==> docker run 模式（当前目录没有 compose 文件）"

  # 先把镜像拉好，容器下线的窗口才够短
  docker pull "$IMAGE"

  # 反代过来的源地址。留空的话登录限流会被伪造的 X-Forwarded-For 绕过，
  # 所以宁可现场探测也不写死——README 里那个 172.17.0.1 只是常见值，不是保证。
  PROXIES="${TP_TRUSTED_PROXIES:-}"
  if [ -z "$PROXIES" ]; then
    PROXIES="$(docker network inspect bridge -f '{{(index .IPAM.Config 0).Gateway}}')"
    echo "    探测到 bridge 网关：$PROXIES"
  fi

  # 用 stop -t 30 而不是 rm -f：rm -f 直接发 SIGKILL，后端 Shutdown 那 10 秒 drain
  # 和 sqlite 的收尾都跑不到。30 这个值与 compose 的 stop_grace_period 对齐。
  if docker inspect "$NAME" >/dev/null 2>&1; then
    docker stop -t 30 "$NAME" >/dev/null
    docker rm "$NAME" >/dev/null
  fi

  # 下面这几项与 README 的 docker run 段逐项一致，改一处记得改另一处。
  # -p 前面那个 127.0.0.1 不能删：docker 发布端口是直接写 iptables 的，会绕过 ufw，
  # 写成 8080:8080 等于把面板裸奔到公网。TP_SECURE 缺了会话 Cookie 就不带 Secure。
  docker run -d --name "$NAME" --restart unless-stopped \
    -p "127.0.0.1:${TP_PORT:-8080}:8080" \
    -v "${TP_VOLUME:-timmypanel-data}:/data" \
    -e TP_SECURE=true \
    -e "TP_TRUSTED_PROXIES=$PROXIES" \
    --stop-timeout 30 \
    "$IMAGE" >/dev/null
fi

# 只清本仓库换下来的悬空镜像。不用 docker image prune -f——那会把宿主机上
# 别的项目的悬空镜像一起删掉，这个脚本没资格替它们做主。
DANGLING="$(docker images "$REPO" --filter dangling=true -q)"
if [ -n "$DANGLING" ]; then
  echo "$DANGLING" | xargs docker rmi >/dev/null 2>&1 || true
fi

echo
docker ps --filter "name=^/${NAME}$" --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}'
