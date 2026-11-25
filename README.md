# Crispy A · 个人网站脚手架

Go + React + PostgreSQL 的最小可运行骨架。当前只完成「地基」：一页以字母 **A** 为主视觉的首页、一条健康检查、一套 JWT 登录体系，业务模块刻意留空。

## 技术栈

| 层 | 选型 |
| --- | --- |
| 后端 | Go 1.27、chi v5、pgx v5、golang-jwt v5、bcrypt、slog |
| 前端 | React 19、TypeScript 7、Vite 8 |
| 数据库 | PostgreSQL 17 |
| 迁移 | 内嵌 SQL + 轻量迁移器（`schema_migrations` + advisory lock） |

## 目录结构

```
crispy-A/
├─ backend/
│  ├─ cmd/server/main.go              # 入口：配置 → 连接池 → 迁移 → 路由 → 优雅退出
│  ├─ internal/
│  │  ├─ auth/                        # JWT 签发/校验、bcrypt、账号与会话仓储、登录/刷新/登出
│  │  ├─ config/                      # 环境变量加载与启动校验（含 .env BOM 兼容）
│  │  ├─ database/                    # pgx 连接池、内嵌迁移器、migrations/*.sql
│  │  └─ httpapi/                     # 路由、中间件、健康检查、鉴权处理器
│  ├─ .env.example                    # 配置模板（复制为 .env）
│  ├─ go.mod / go.sum
│  └─ bin/                            # 本地构建产物（已忽略）
├─ frontend/
│  ├─ src/
│  │  ├─ App.tsx                      # 首页：字母 A + API 状态
│  │  ├─ api.ts                       # 唯一的接口调用（健康检查）
│  │  ├─ styles.css                   # 深色编辑风视觉与响应式
│  │  └─ main.tsx
│  ├─ index.html
│  ├─ package.json
│  ├─ tsconfig*.json
│  └─ vite.config.ts                  # 开发代理 /api → 127.0.0.1:8080
├─ scripts/
│  ├─ db.ps1                          # 本地 PostgreSQL 启动/停止/状态
│  └─ smoke-test.ps1                  # 后端接口冒烟测试
├─ .gitignore
└─ README.md
```

## 快速开始（Windows / PowerShell）

本机已完成 Go 与 PostgreSQL 的安装，路径见文末「本机环境」。全新环境请先安装 Go 1.27+、Node 20+、PostgreSQL 17。

### 1. 启动数据库

```powershell
powershell -File scripts/db.ps1 start
powershell -File scripts/db.ps1 status
```

`scripts/db.ps1` 默认使用 `D:\crispy\tools\pgsql\bin` 与 `D:\crispy\pgdata`，可通过 `PG_BIN`、`PGDATA` 环境变量覆盖。

### 2. 启动后端

```powershell
$env:PATH = "D:\crispy\tools\go\bin;$env:PATH"
cd backend
go run ./cmd/server
```

首次启动会：

1. 读取 `backend/.env`（不存在则只用真实环境变量）；
2. 自动执行未应用的 SQL 迁移；
3. 在 `users` 表为空时，用 `ADMIN_EMAIL` / `ADMIN_PASSWORD` 创建管理员（已存在则跳过）；
4. 监听 `:8080`。

看到 `http server listening` 后访问 <http://127.0.0.1:8080/api/health>。

### 3. 启动前端

```powershell
cd frontend
npm run dev
```

打开 <http://localhost:5173>。页面底部的状态点会真实请求 `/api/health`：

- 绿色「API 在线」：后端与数据库均正常；
- 橙色「数据库异常」：接口可达但数据库 Ping 失败；
- 红色「API 离线」：后端不可达。

Vite 将 `/api` 代理到 `http://127.0.0.1:8080`；如修改后端端口，同步改 `frontend/vite.config.ts`。

### 4. 首次配置

新克隆的仓库没有 `.env`：

```powershell
cd backend
Copy-Item .env.example .env
```

模板里的 `JWT_SECRET`、`ADMIN_EMAIL`、`ADMIN_PASSWORD` **故意留空**（防止示例凭据被直接用于部署），需要自己填写：

```powershell
# 生成 48 字节随机密钥
$bytes = New-Object byte[] 48
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($bytes); $rng.Dispose()
[Convert]::ToBase64String($bytes)

# 然后编辑 .env：填写 DATABASE_URL、JWT_SECRET、ADMIN_EMAIL、ADMIN_PASSWORD
```

约束：`JWT_SECRET` 至少 32 字符；`ADMIN_PASSWORD` 至少 10 字符；`ADMIN_EMAIL` 与 `ADMIN_PASSWORD` 必须同时设置或同时留空。启动时会拒绝 `change-me` / `replace-with` 这类模板占位值。

`ADMIN_*` 只在 `users` 表为空时生效；账号创建后修改环境变量不会更新密码（见「已知限制」）。

## 配置项

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_ENV` | `development` | `production` 时日志转 JSON、cookie 默认 Secure |
| `HTTP_ADDR` | `:8080` | 监听地址 |
| `DATABASE_URL` | 必填 | PostgreSQL 连接串 |
| `JWT_SECRET` | 必填，≥32 字符 | HS256 签名密钥 |
| `ACCESS_TOKEN_TTL` | `15m` | access token 有效期 |
| `REFRESH_TOKEN_TTL` | `168h` | refresh 会话有效期 |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | 空 | 仅在用户表为空时用于初始化管理员，之后修改无效 |
| `CORS_ORIGINS` | `http://localhost:5173` | 允许跨域的前端地址，逗号分隔 |
| `COOKIE_SECURE` | `false` | HTTPS 部署必须设为 `true` |

## API

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/health` | 否 | 健康检查，数据库不可达返回 503 |
| POST | `/api/auth/login` | 否 | 邮箱密码登录；返回 access token，refresh 写入 HttpOnly cookie |
| POST | `/api/auth/refresh` | refresh cookie | 轮换 refresh token，返回新 access token |
| POST | `/api/auth/logout` | refresh cookie | 撤销当前会话并清除 cookie，返回 204 |
| GET | `/api/admin/me` | Bearer | 当前管理员信息 |

错误统一为：

```json
{ "error": { "code": "invalid_credentials", "message": "邮箱或密码不正确" } }
```

### 调用示例（PowerShell）

```powershell
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$body = @{ email = $env:ADMIN_EMAIL; password = $env:ADMIN_PASSWORD } | ConvertTo-Json

$login = Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/auth/login `
  -ContentType 'application/json' -Body $body -WebSession $session

Invoke-RestMethod -Uri http://127.0.0.1:8080/api/admin/me `
  -Headers @{ Authorization = "Bearer $($login.accessToken)" }

Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/auth/refresh -WebSession $session
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/auth/logout -WebSession $session
```

> PowerShell 调用 `curl.exe` 时双引号容易被吞，建议用 `Invoke-RestMethod` 或把 JSON 写入文件后 `--data-binary '@file'`，参考 `scripts/smoke-test.ps1`。

## 鉴权设计

- **access token**：HS256 JWT，15 分钟，放在 `Authorization: Bearer` 头；前端应只存内存，避免 localStorage 被 XSS 读取。
- **refresh token**：32 字节随机数，仅存 SHA-256 哈希；HttpOnly + SameSite=Lax + `Path=/api/auth` cookie，7 天。
- **轮换**：每次刷新都会原子地作废旧会话、写入新会话；旧 token 立即失效。
- **登出**：撤销当前会话并清除 cookie。
- **密码**：bcrypt cost 12；账号不存在时也执行一次等价耗时校验，降低账号枚举风险。
- **初始化**：`users` 表为空时创建单管理员；后续不会覆盖。

## 数据库

三张表：

- `users`：管理员账号（`email` 唯一索引基于 `lower(email)`，登录不区分大小写）；
- `sessions`：refresh 会话（`token_hash` 唯一，记录过期与撤销时间）；
- `schema_migrations`：已执行迁移记录。

迁移器按文件名顺序执行 `backend/internal/database/migrations/*.sql`，每个文件一个事务，并使用 PostgreSQL advisory lock 防止多实例并发迁移。

新增迁移：

```sql
-- 0002_posts.sql
CREATE TABLE posts (...);
```

把文件放入 `migrations/` 后重启服务即可，无需额外命令。

**回滚**：迁移只支持向上，不提供 `down`。每个迁移文件头部写明回滚 SQL；开发环境整体重置：

```sql
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
```

然后重启服务重新迁移。该操作会删除全部数据，仅限本地。

**备份**：当前便携版 PostgreSQL 只带 `initdb` / `pg_ctl` / `postgres`，不含 `psql` / `pg_dump`。需要备份时请安装完整 PostgreSQL 客户端，或改用 Docker / 完整安装版。

## 测试与检查

```powershell
# 后端：编译、静态检查、单元测试
$env:PATH = "D:\crispy\tools\go\bin;$env:PATH"
cd backend
go build ./...
go vet ./...
go test ./...

# 集成测试（真实 PostgreSQL，会建临时用户并在结束时删除）
$env:TEST_DATABASE_URL = 'postgres://crispy:<密码>@127.0.0.1:5432/crispy_a?sslmode=disable'
go test ./... -v

# 前端：类型检查 + 生产构建
cd ../frontend
npm run build
```

未配置 `TEST_DATABASE_URL` 时，集成测试会跳过而不是失败。

接口冒烟（需后端已启动，并导出 `ADMIN_EMAIL` / `ADMIN_PASSWORD`）：

```powershell
powershell -File scripts/smoke-test.ps1
```

## 生产部署提示

1. 前端 `npm run build` 生成 `frontend/dist`，由 Nginx/Caddy 托管；
2. 后端 `go build -o bin/server ./cmd/server`，用 systemd / Windows 服务托管；
3. 反向代理把 `/api` 转发到后端，保证前后端同源（refresh cookie 的 `Path=/api/auth` 依赖同源）；
4. 必须设置：`APP_ENV=production`、`COOKIE_SECURE=true`、强随机 `JWT_SECRET`、正式的 `CORS_ORIGINS`。

Nginx 参考：

```nginx
server {
    listen 443 ssl;
    server_name example.com;
    root /var/www/crispy-a/frontend/dist;

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location / {
        try_files $uri /index.html;
    }
}
```

## Docker 部署

仓库自带 `compose.yaml`、前后端 Dockerfile 与宿主机 Nginx 示例。默认只启动 `db + api`，前端 `dist` 交给宿主机 Nginx；需要容器托管前端时加 `--profile web`。

```bash
cp .env.prod.example .env.prod   # 填 POSTGRES_PASSWORD / JWT_SECRET / CORS_ORIGINS / ADMIN_*
chmod 600 .env.prod
docker compose --env-file .env.prod up -d --build db api
```

完整步骤、两种拓扑、验证、备份与回滚见 [deploy/README.md](deploy/README.md)，宿主机 Nginx 配置见 [deploy/nginx-host.conf](deploy/nginx-host.conf)。国内拉取 Docker Hub 镜像失败时，先按部署文档配置 `registry-mirrors`。

### 启停脚本

项目根目录提供 Linux 脚本，默认读取 `.env.prod`（可用 `ENV_FILE` 覆盖），启动 `db + api + web`：

```bash
chmod +x start.sh stop.sh    # 首次拉取后如无执行权限
./start.sh                   # 启动（使用已有镜像）
./start.sh --build           # 重新构建镜像后启动
./stop.sh                    # 停止（保留容器、网络与数据卷）
./stop.sh --status           # 查看状态；退出码 0=全部运行且健康，1=异常
```

`--status` 会输出 Docker 引擎、各容器状态、`running/expected` 服务数、HTTP 健康检查和数据卷情况，适合直接用于巡检或监控脚本。

## 已知限制

- 登录接口没有限流 / 验证码，暴力破解防护需在网关或后续中间件补齐；
- 没有改密、找回密码、多用户与角色管理，只有单管理员；管理员密码创建后只能直接更新 `users.password_hash`；
- refresh token 只做轮换与撤销，未实现「重放即全量撤销」；refresh 失败不会清除 cookie，前端接入时应对 refresh 做单飞（single-flight）并限制重试；
- 前端只接了健康检查，尚未做登录页与后台 UI；
- 迁移只向上，不提供自动回滚；
- 没有 CI、Dockerfile 与生产监控埋点。

## 下一步建议

1. 新增 `posts` / `projects` 表与后台 CRUD，作为第一个业务模块；
2. 加登录限流（按 IP + 账号）与审计日志；
3. 配置 CI（`go vet` + `go test` + `npm run build`）；
4. 需要单二进制部署时，用 `embed` 把前端产物打进 Go 服务。

## 本机环境（当前机器）

| 组件 | 位置 / 版本 |
| --- | --- |
| Go | `D:\crispy\tools\go\bin`，go1.27.4 |
| PostgreSQL | `D:\crispy\tools\pgsql\bin`，17.9（便携实例，未注册 Windows 服务） |
| 数据目录 | `D:\crispy\pgdata` |
| Node / npm | v24.21.0 / 12.2.2 |
| 数据库 | 库 `crispy_a`，用户 `crispy`，密码见 `backend/.env` |

`go env -w GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct` 已设置，便于国内拉取依赖。
