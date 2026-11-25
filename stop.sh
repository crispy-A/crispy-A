#!/usr/bin/env bash
# 停止 Crispy A 或查看运行状态。
# 用法：
#   ./stop.sh            停止容器（保留容器、网络与数据卷）
#   ./stop.sh --status   查看项目和容器运行状态
#   ENV_FILE=.env.test ./stop.sh --status   指定环境文件
#
# --status 退出码：0 = 服务齐全且健康；1 = 未运行、部分运行或健康检查失败。
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${ROOT_DIR}/compose.yaml"
ENV_FILE="${ENV_FILE:-${ROOT_DIR}/.env.prod}"

usage() {
    cat <<'EOF'
停止 Crispy A 或查看运行状态。
用法：
  ./stop.sh            停止容器（保留容器、网络与数据卷）
  ./stop.sh --status   查看项目和容器运行状态
  ENV_FILE=.env.test ./stop.sh --status   指定环境文件

--status 退出码：0 = 服务齐全且健康；1 = 未运行、部分运行或健康检查失败。
EOF
    printf '\n可选环境变量：ENV_FILE（默认 %s）\n' "${ROOT_DIR}/.env.prod"
}

MODE="stop"
case "${1:-}" in
    --status) MODE="status" ;;
    "") ;;
    -h|--help) usage; exit 0 ;;
    *)
        printf '未知参数：%s\n' "${1}" >&2
        usage >&2
        exit 2
        ;;
esac
if [ "$#" -gt 1 ]; then
    echo "只接受一个参数。" >&2
    usage >&2
    exit 2
fi

command -v docker >/dev/null 2>&1 || {
    echo "未找到 docker，请先安装 Docker。" >&2
    exit 1
}
if ! docker info >/dev/null 2>&1; then
    echo "Docker 守护进程未运行。" >&2
    exit 1
fi
[ -f "${ENV_FILE}" ] || {
    echo "缺少环境文件：${ENV_FILE}" >&2
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

show_status() {
    local expected running health_state=2
    echo "=== Crispy A 状态 ==="
    echo "项目目录: ${ROOT_DIR}"
    echo "环境文件: ${ENV_FILE}"
    echo "Compose 项目: ${COMPOSE_PROJECT}"
    echo

    if docker info >/dev/null 2>&1; then
        echo "Docker 引擎: running"
    else
        echo "Docker 引擎: stopped"
    fi

    expected="$("${COMPOSE[@]}" config --services 2>/dev/null | sed '/^[[:space:]]*$/d' | wc -l | tr -d ' ')"
    running="$("${COMPOSE[@]}" ps --status running --services 2>/dev/null | sed '/^[[:space:]]*$/d' | wc -l | tr -d ' ')"
    echo "服务: ${running}/${expected} running"
    echo
    echo "容器："
    "${COMPOSE[@]}" ps -a 2>/dev/null || true
    echo

    if command -v curl >/dev/null 2>&1; then
        if curl -fsS --max-time 3 "${HEALTH_URL}" >/dev/null 2>&1; then
            health_state=0
            echo "HTTP 健康检查: OK  ${HEALTH_URL}"
        else
            health_state=1
            echo "HTTP 健康检查: 不可用  ${HEALTH_URL}"
        fi
    else
        echo "HTTP 健康检查: 跳过（未安装 curl）"
    fi

    if docker volume inspect "${COMPOSE_PROJECT}_pgdata" >/dev/null 2>&1; then
        echo "数据卷: ${COMPOSE_PROJECT}_pgdata（存在）"
    else
        echo "数据卷: ${COMPOSE_PROJECT}_pgdata（不存在，首次启动会创建）"
    fi

    if [ "${running}" = "${expected}" ] && [ "${health_state}" -eq 0 ]; then
        echo
        echo "结论: 运行中且健康"
        return 0
    fi
    echo
    echo "结论: 未运行 / 部分运行 / 健康检查失败"
    return 1
}

if [ "${MODE}" = "status" ]; then
    show_status
    exit $?
fi

cd "${ROOT_DIR}"
echo "停止 Crispy A（环境：${ENV_FILE}）"
if [ -z "$("${COMPOSE[@]}" ps -q 2>/dev/null)" ]; then
    echo "没有正在运行的容器，无需停止。"
else
    "${COMPOSE[@]}" stop
    echo "已停止（容器保留；数据卷未受影响）。"
fi
echo
show_status || true
