# 发版手册（Release Runbook）

适用环境：阿里云 Linux 4，项目目录 `/data/crispy-A`，Docker Compose 运行 `db + api + web`，宿主机 Nginx 反代 `127.0.0.1:8081`。

约定：

- 分支 `master`，每次发版打 tag，例如 `v0.1.0`；
- 生产环境文件 `/data/crispy-A/.env.prod`（权限 600，不提交 Git）；
- 数据卷 `crispy-a_pgdata`，数据库迁移只向上、不自动回滚；
- 发布期间旧容器继续服务，镜像构建完成后才重建容器，API 中断通常只有几秒。

---

## 1. 发布前检查（本地）

```bash
# 后端
cd backend
go build ./...
go vet ./...
go test ./...

# 前端
cd ../frontend
npm run build

# Compose 配置
cd ..
docker compose --env-file .env.prod.example --profile web config --quiet
```

确认要发布的提交：

```bash
git status
git log --oneline -5
```

打 tag 并推送：

```bash
git tag -a v0.1.0 -m "release v0.1.0"
git push origin master --tags
```

> 服务器上的回滚点就是上一个 tag 或提交号，发版前记录一下：`git rev-parse HEAD`。

---

## 2. 备份数据库（服务器，必做）

```bash
cd /data/crispy-A
mkdir -p /data/backups

docker compose --env-file .env.prod exec -T db \
  pg_dump -U crispy -d crispy_a -Fc \
  > /data/backups/crispy-$(date +%F-%H%M).dump

ls -lh /data/backups/ | tail -5
```

确认备份文件非空：

```bash
test -s /data/backups/crispy-*.dump && echo "backup ok"
```

建议保留最近 14 天：

```bash
find /data/backups -name 'crispy-*.dump' -mtime +14 -delete
```

> 迁移文件只向上，没有 `down`。没有备份不要发版。

---

## 3. 更新代码

服务器已配置 GitHub 镜像代理时：

```bash
cd /data/crispy-A
git remote -v
git pull --ff-only
git log --oneline -3
```

GitHub 再次被阻断时，用本地 bundle：

```powershell
# 本地 Windows
git bundle create D:\crispy\tools\crispy-a.bundle --all
scp D:\crispy\tools\crispy-a.bundle root@<ECS公网IP>:/tmp/
```

```bash
# 服务器
cd /data/crispy-A
git fetch /tmp/crispy-a.bundle master
git merge --ff-only FETCH_HEAD
git log --oneline -3
```

---

## 4. 发布

```bash
cd /data/crispy-A
chmod +x start.sh stop.sh     # 首次或权限丢失时

./stop.sh --status            # 发布前状态，应 3/3 running
./start.sh --build            # 构建新镜像并重建容器
```

说明：

- `--build` 会先在旧容器运行时构建镜像，构建成功后重建 `api`/`web`，中断时间短；
- 只改了 `.env.prod`（如 CORS、COOKIE_SECURE）不需要 `--build`，用 `./start.sh` 即可重建配置变化的容器；
- 只改了数据库迁移，`--build` 会重新构建并把迁移打进二进制，启动时自动执行。

---

## 5. 发布后验证

```bash
cd /data/crispy-A

./stop.sh --status
# 期望：服务 3/3 running；HTTP 健康检查 OK；结论：运行中且健康

curl -fsS http://127.0.0.1:8081/api/health
# 期望：{"status":"ok","database":"ok",...}

docker compose --env-file .env.prod logs api --tail 50
# 重点看：database migrations applied / admin account created / 无 ERROR

curl -I https://你的域名/
curl -s https://你的域名/api/health
```

浏览器验证（接好 HTTPS 后）：

1. 打开 `https://你的域名/`，页面状态为「API 在线」；
2. 登录接口返回 `Set-Cookie: ... HttpOnly; Secure; SameSite=Lax`；
3. `/api/admin/me` 返回 200。

任何一个不通过：执行第 6 步回滚。

---

## 6. 回滚

### 6.1 代码回滚

```bash
cd /data/crispy-A
git log --oneline -5
git checkout <上一个 tag 或提交号>
./start.sh --build
```

回滚后再验证一次健康检查。确认稳定后，把 `master` 指回该提交或后续修复：

```bash
git checkout master
```

### 6.2 镜像回滚（可选，发布前打旧镜像 tag）

发布前：

```bash
docker tag crispy-a-api:latest crispy-a-api:prev
docker tag crispy-a-web:latest crispy-a-web:prev
```

回滚：

```bash
docker tag crispy-a-api:prev crispy-a-api:latest
docker tag crispy-a-web:prev crispy-a-web:latest
cd /data/crispy-A
docker compose --env-file .env.prod --profile web up -d   # 不带 --build
./stop.sh --status
```

### 6.3 数据库回滚（仅迁移不兼容时）

```bash
cd /data/crispy-A
./stop.sh                                  # 停 api/web，避免连接占用

docker compose --env-file .env.prod up -d db

cat /data/backups/crispy-YYYY-MM-DD-HHMM.dump | \
  docker compose --env-file .env.prod exec -T db \
  pg_restore -U crispy -d crispy_a --clean --if-exists

./start.sh
```

> 恢复会覆盖当前数据，先确认备份文件是本次发布前生成的那一个。

---

## 7. 常见问题

| 现象 | 原因 / 处理 |
| --- | --- |
| `git pull` 失败 | GitHub 被阻断，改用 bundle（第 3 步） |
| 构建卡在拉取基础镜像 | Docker Hub 不可达；检查 `docker info` 的 `registry-mirrors`，必要时本地 `docker save` 后 `docker load` |
| `api` 反复重启 | `docker compose logs api --tail 100`；常见原因：`.env.prod` 校验失败、数据库密码不匹配、迁移报错 |
| 502 Bad Gateway | `web` 容器未运行或端口不对；`./stop.sh --status` 和 `docker compose ps` 确认，Nginx 反代必须是 `127.0.0.1:8081` |
| 登录成功但刷新掉线 | `COOKIE_SECURE=true` 但未走 HTTPS，或 `CORS_ORIGINS` 与域名不一致 |
| 磁盘占用增长 | `docker system df`；`docker builder prune -f` 清构建缓存，**不要** `docker system prune --volumes` |

常用巡检命令：

```bash
cd /data/crispy-A
./stop.sh --status
docker compose --env-file .env.prod ps
docker compose --env-file .env.prod logs --tail 50 api web
df -h /var/lib/docker
docker system df
```

---

## 8. 发版清单

- [ ] 本地 `go test`、`npm run build` 通过
- [ ] 已打 tag 并推送
- [ ] 已备份数据库，且 `test -s` 通过
- [ ] `git pull` 后提交号正确
- [ ] `./start.sh --build` 退出码 0
- [ ] `./stop.sh --status` 显示 3/3 且健康
- [ ] `/api/health` 返回 ok
- [ ] 浏览器页面与登录验证通过
- [ ] 记录本次发版提交号与备份文件名
