# Docker 部署说明

> 已在 Windows 11 + Docker Desktop（Docker 29.9.3 / Compose v5.7.1 / WSL 3.0.5）本地验证：`db + api + web` 全栈构建、迁移、管理员初始化、登录、静态资源与 `/api` 反代全部通过。Linux 服务器只需替换路径、证书与域名。

## 拓扑

```
浏览器
  │ HTTPS 443
  ▼
宿主机 Nginx（TLS、静态文件、/api 反代）
  ├─ /            → /var/www/crispy-a/current（frontend/dist）
  └─ /api/*       → 127.0.0.1:8080（compose 的 api 容器）
                        │
                        ▼
                     db:5432（compose 内网，不映射宿主机端口）
```

Compose 默认只启动 `db + api`，适合「宿主机已有 Nginx」的部署；`web` 服务放在 `web` profile 下，需要容器内 Nginx 托管前端时再启用。

## 前置

- Linux 服务器，安装 Docker Engine 24+ 与 Compose v2；
- 宿主机 Nginx 已配置 443；
- 仓库已拉到服务器，例如 `/srv/crispy-a`。

## 首次部署（方式 A：宿主机 Nginx 托管静态文件）

```bash
cd /srv/crispy-a

# 1. 生产环境变量
cp .env.prod.example .env.prod
chmod 600 .env.prod
# 编辑 .env.prod：POSTGRES_PASSWORD、JWT_SECRET、CORS_ORIGINS、ADMIN_* 等

# 2. 启动数据库 + 后端（api 启动时自动跑迁移、首次创建管理员）
docker compose --env-file .env.prod up -d --build db api

# 3. 确认状态
docker compose --env-file .env.prod ps
curl -fsS http://127.0.0.1:8080/api/health

# 4. 构建前端并发布静态文件
#    有 Node 时：cd frontend && npm ci && npm run build && cd ..
#    没有 Node 时用 Docker 构建并导出 dist：
#      docker build -t crispy-a-web:build ./frontend
#      cid=$(docker create crispy-a-web:build)
#      docker cp "$cid":/usr/share/nginx/html ./frontend-dist
#      docker rm "$cid"
#    然后发布（有 Node 时把 frontend-dist 换成 frontend/dist）：
TS=$(date +%Y%m%d-%H%M%S)
mkdir -p "/var/www/crispy-a/releases/$TS"
rsync -a --delete frontend-dist/ "/var/www/crispy-a/releases/$TS/"
ln -sfn "/var/www/crispy-a/releases/$TS" /var/www/crispy-a/current
```

宿主机 Nginx 参考 [deploy/nginx-host.conf](nginx-host.conf)，把它合并进现有 server 块后 `nginx -t && systemctl reload nginx`。

> 也可以直接用仓库根目录的 `./start.sh` / `./stop.sh --status`（默认读取 `.env.prod`，会同时启动 `db + api + web`）。

> 也可以不用 rsync：直接把 `frontend/dist` 交给 CI 发布，或启用 `web` profile 用容器托管。

## 方式 B：容器托管前端

```bash
docker compose --env-file .env.prod --profile web up -d --build
```

此时前端在 `127.0.0.1:8081`，宿主机 Nginx 只需全量反代到它（见 `nginx-host.conf` 中方式 B 注释），`/api` 由容器内 Nginx 转发给 `api`。

## 验证清单

```bash
curl -fsS https://你的域名/api/health
# 期望 200，body 含 "database":"ok"

# 登录后检查 cookie 属性
curl -i -c /tmp/cookies.txt -H 'Content-Type: application/json' \
  -d '{"email":"ADMIN_EMAIL","password":"ADMIN_PASSWORD"}' \
  https://你的域名/api/auth/login
# 期望 Set-Cookie 含 HttpOnly; Secure; SameSite=Lax

# 重启后数据仍在
docker compose --env-file .env.prod restart
docker compose --env-file .env.prod exec db psql -U crispy -d crispy_a -c 'select count(*) from users;'
```

## 更新与回滚

```bash
git pull
docker compose --env-file .env.prod up -d --build api
```

- 前端：构建到新的 `releases/<时间戳>` 再切 `current` symlink，失败时切回旧目录即可；
- 后端：镜像重建即回滚到上一版需要提前打 tag，例如 `docker tag crispy-a-api:latest crispy-a-api:backup-20261125`，回滚时改回旧镜像；
- 数据库迁移只向上，发布前先备份：

```bash
docker compose --env-file .env.prod exec -T db \
  pg_dump -U crispy -d crispy_a -Fc > /backup/crispy-$(date +%F-%H%M).dump
```

恢复：

```bash
docker compose --env-file .env.prod exec -T db \
  pg_restore -U crispy -d crispy_a --clean --if-exists < /backup/crispy-xxxx.dump
```

## 本地验证（Windows / 不冲突端口）

本机已有 8080/5432 的开发服务时，用独立端口和临时 env 验证整栈。`.env.test` 已被 `.gitignore` 忽略：

```powershell
# .env.test 额外设置：API_PORT=18080  WEB_PORT=18081  COOKIE_SECURE=false  CORS_ORIGINS=http://127.0.0.1:18081
docker compose --env-file .env.test --profile web up -d --build
curl.exe -fsS http://127.0.0.1:18080/api/health
curl.exe -I http://127.0.0.1:18081/
docker compose --env-file .env.test --profile web down
```

`down` 会保留 `crispy-a_pgdata` 数据卷；只有 `down -v` 才会删除数据库数据。

## 国内网络

Docker Hub 直连可能超时，报错类似 `failed to resolve reference "docker.io/library/..."`。需要在 Docker 配置里加镜像加速：

- Docker Desktop：编辑 `%USERPROFILE%\.docker\daemon.json`，加入
  `"registry-mirrors": ["https://docker.m.daocloud.io"]`，然后重启 Docker Desktop；
- Linux 服务器：编辑 `/etc/docker/daemon.json` 后 `systemctl restart docker`。

镜像源可用性会变化，构建前先用 `docker pull alpine:3.22` 验证。

## 注意事项

- `API_PORT` 只绑定 `127.0.0.1`，不要暴露 8080 到公网；数据库也不要映射端口。
- `.env.prod` 不要提交 Git；`backend/.env` 也不会进入镜像（`.dockerignore` 已排除）。
- `COOKIE_SECURE=true` 必须搭配 HTTPS，否则浏览器不会回传 refresh cookie。
- 1 GB 内存的机器构建前端可能 OOM，建议在 CI 构建镜像或加 swap。
- `api` 健康检查在数据库不可达时返回 503，不要把它当 liveness 用于自动重启。
