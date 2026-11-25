#!/usr/bin/env bash
# 启动 Crispy A：PostgreSQL + Go API + Nginx 前端。
# 用法：
#   ./start.sh            启动（使用已有镜像）
#   ./start.sh --build    重新构建镜像后启动
#   ENV_FILE=.env.test ./start.sh   指定环境文件
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${ROOT_DIR}/compose.yaml"
ENV_FILE="${ENV_FILE:-${ROOT_DIR}/.env.prod}"

usage() {
    cat <<'EOF'
启动 Crispy A：PostgreSQL + Go API + Nginx 前端。
用法：
  ./start.sh            启动（使用已有镜像）
  ./start.sh --build    重新构建镜像后启动
  ENV_FILE=.env.test ./start.sh   指定环境文件
EOF
    printf '\n可选环境变量：ENV_FILE（默认 %s）\n' "${ROOT_DIR}/.env.prod"
}

BUILD=0
for arg in "$@"; do
    case "${arg}" in
        --build) BUILD=1 ;;
        -h|--help) usage; exit 0 ;;
        *)
            printf '未知参数：%s\n' "${arg}" >&2
            usage >&2
            exit 2
            ;;
    esac
done

command -v docker >/dev/null 2>&1 || {
    echo "未找到 docker，请先安装 Docker。" >&2
    exit 1
}
docker info >/dev/null 2>&1 || {
    echo "Docker 守护进程未运行，请先执行：systemctl start docker" >&2
    exit 1
}
[ -f "${ENV_FILE}" ] || {
    echo "缺少环境文件：${ENV_FILE}" >&2
    echo "请先执行：cp .env.prod.example .env.prod 并填写配置。" >&2
    exit 1
}
[ -f "${COMPOSE_FILE}" ] || {
    echo "缺少 compose 文件：${COMPOSE_FILE}" >&2
    exit 1
}

read_env() {
    local key="$1" fallback="$2" value
    value="$(grep -E "^${key}=" "${ENV_FILE}" | tail -n 1 | cut -d= -f2- |
        sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//")"
    printf '%s' "${value:-${fallback}}"
}

WEB_PORT="$(read_env WEB_PORT 8081)"
COMPOSE_PROJECT="$(awk '/^name:[[:space:]]*/{print $2; exit}' "${COMPOSE_FILE}" || true)"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-$(basename "${ROOT_DIR}")}"

COMPOSE=(docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" --profile web)
HEALTH_URL="http://127.0.0.1:${WEB_PORT}/api/health"

echo "启动 Crispy A（环境：${ENV_FILE}）"
cd "${ROOT_DIR}"

if [ "${BUILD}" -eq 1 ]; then
    "${COMPOSE[@]}" up -d --build
else
    "${COMPOSE[@]}" up -d
fi

echo
echo "等待服务就绪（最多 120 秒）..."
ready=0
for _ in $(seq 1 60); do
    if curl -fsS --max-time 3 "${HEALTH_URL}" >/dev/null 2>&1; then
        ready=1
        break
    fi
    sleep 2
done

echo
"${COMPOSE[@]}" ps

if [ "${ready}" -ne 1 ]; then
    echo
    echo "健康检查超时：${HEALTH_URL}" >&2
    echo "最近 api 日志：" >&2
    "${COMPOSE[@]}" logs --tail 40 api >&2 || true
    exit 1
fi

echo
echo "启动完成：${HEALTH_URL}"
echo "查看状态：./stop.sh --status"
