# AI短链平台

一个轻量、现代、单体部署的短链/活码平台。后台使用 Go，管理端为内置原生 HTML/CSS/JavaScript，无 Node 构建链、无 ORM。数据库支持两种模式：

- **内置嵌入式 SQLite**：默认推荐，首次启动即可安装，适合轻量部署、小团队、私有化场景。
- **MySQL / MariaDB**：适合正式生产、多人协作和更高并发场景。

## 核心能力

### 短链

- 长链接转换为短链：`/s/{code}`。
- 支持根路径短链兼容：`/{code}`。
- 支持自定义短码或自动生成短码。
- 支持 301 / 302 / 307 / 308 跳转。
- 支持启用/停用、开始时间、过期时间、访问上限、失效备用链接。
- 每条短链可长期复用，访问次数持续统计。
- 自动生成短链二维码：`/qr/short/{code}.png`。
- **审核机制**：新建或修改短链后默认为待审，只有管理员审核通过后才会实际跳转。

### 活码

- 固定入口：`/q/{code}`，入口二维码不变，后台可维护多套二维码。
- 单弹窗 Tab 配置：基础信息、二维码组、发布确认一次性完成。
- 后端事务保存活码基础信息、二维码新增、编辑和删除，避免部分保存成功造成配置不一致。
- 支持上传二维码图片，或填写已有图片 URL。
- 每张二维码支持启用/停用、开始时间、过期时间、展示上限、排序和权重。
- 自动轮替策略：轮询、按权重随机、最少展示优先。
- 扫码后打开活码页，展示当前命中的二维码和“长按识别二维码”指引。
- 支持无法识别时的备用目标链接按钮。
- 自动生成活码入口二维码：`/qr/live/{code}.png`。
- **审核机制**：活码入口和每张二维码都需要审核通过；未通过时公开访问不会展示二维码。

### 统计

- 记录短链访问、活码展示、长按/右键识别意图事件。
- 统计近 30 日访问量、独立 IP、按日期、设备、浏览器分布。
- 最近访问明细包含时间、事件、状态、设备、浏览器、IP。

> 微信/浏览器无法可靠回传用户是否真的完成了“识别图中二维码”。本项目会统计活码页展示，并通过 `touchstart/contextmenu/sendBeacon` 记录“长按识别意图”。如果二维码图片本身指向本平台短链，后续跳转也可以继续被统计。

## 管理后台

- 专业品牌图标，已替换后台 Logo 与 favicon；侧栏 Logo 已做紧凑化处理，避免挤压内容区。
- 后台布局已做桌面、平板、手机端响应式适配：移动端顶部导航、卡片式数据表、底部抽屉式弹窗、紧凑安装向导。
- 白天/黑夜模式，右下/侧栏使用太阳/月亮图标切换。
- 国际化支持：中文/英文，默认可自动匹配浏览器语言，也可在系统设置里指定。
- 首页只展示轻量账户恢复入口；恢复 Key 通过小弹窗查看和复制，不再全站冗余展示。
- 系统设置页集中维护站点名称、公网域名、语言策略、登录模式、SMTP 参数、管理员邮箱。

## 登录与安装

### 首次进入

首次访问会进入 `/setup` 安装向导，依次配置：

1. 数据库：内置 SQLite 或 MySQL/MariaDB。
2. 站点：站点名称、公网域名、语言策略。
3. 管理员：管理员邮箱和名称。
4. SMTP：可选配置，用于 Magic Link 登录。

安装完成后会自动绑定当前浏览器，并显示恢复 Key。请立即复制保存到密码管理器。

### 登录方式

后台支持三种登录模式：

- **Magic Link + 浏览器一键登录**：默认推荐。公网环境优先使用邮箱 Magic Link，已绑定浏览器仍可一键进入。
- **仅 Magic Link**：更适合公网严格访问控制。
- **仅浏览器一键登录**：适合本地、内网或极简个人部署。

保留浏览器一键登录体验：首次安装时自动绑定当前浏览器，之后同一浏览器可直接进入。更换浏览器、清理 Cookie 或令牌丢失时，可使用恢复 Key 或 Magic Link 重新绑定。

Magic Link 需要在安装向导或系统设置中配置 SMTP。系统会发送 15 分钟有效的一次性登录链接。

为了降低登录邮件进入垃圾箱的概率，建议：

- 使用和 SMTP 账号同域或同账号的发信邮箱，例如 SMTP 账号是 `no-reply@example.com` 时，发信邮箱也使用 `no-reply@example.com`。
- 为发信域名配置 SPF、DKIM、DMARC。使用企业邮箱、Resend、SendGrid、阿里云邮件推送等服务时，按服务商给出的 DNS 记录配置。
- 公网域名使用 HTTPS，避免邮件正文里的登录链接长期指向 IP、`localhost` 或临时域名。
- 自建 SMTP 时同时检查服务器反向解析（PTR/rDNS）和出站 IP 信誉。

## 快速启动

### 默认轻量启动（内置 SQLite）

```bash
cp .env.example .env
docker compose up -d --build
```

访问：`http://localhost:8080`，按安装向导完成配置。

### Render 一键部署

仓库根目录包含 `render.yaml`（Docker 部署蓝图），可直接用于 Render 一键导入。

1. 将仓库推送到 GitHub。
2. 在 Render 创建 Web Service，选择该仓库。
3. Render 会自动识别 `render.yaml`，按 `Dockerfile` 构建并部署。
4. 使用分配域名访问 `https://<你的服务名>.onrender.com/setup`，完成安装。

注意事项：

- 当前默认使用嵌入式 SQLite，运行时数据会存储在 `/app/data`。
- Render 免费实例建议为服务挂载持久化存储（Disk）到 `/app/data`，否则容器重建会丢失数据与管理员安装配置。
- HTTPS 下请在系统设置中开启反代安全参数（`TRUST_PROXY=true`、`COOKIE_SECURE=true`）。

### 使用 MySQL/MariaDB

启动应用和 MariaDB：

```bash
cp .env.example .env
docker compose --profile mysql up -d --build
```

安装向导里选择 `MySQL / MariaDB`，DSN 可填写：

```text
shortlink:shortlink@tcp(db:3306)/ai_shortlink?charset=utf8mb4&parseTime=true&loc=Local
```

### 原生二进制部署

生产部署目录统一为 `/opt/shortlink`。发布包包含一个内置静态资源和迁移 SQL 的单文件二进制、启动配置文件样例和 systemd 服务文件；Linux amd64 发布包默认静态链接，便于直接部署：

```bash
make release
sudo mkdir -p /opt/shortlink
sudo cp dist/ai-shortlink-linux-amd64/ai-shortlink dist/ai-shortlink-linux-amd64/shortlink.env.example dist/ai-shortlink-linux-amd64/ai-shortlink.service /opt/shortlink/
cd /opt/shortlink
sudo cp shortlink.env.example shortlink.env
sudo chmod +x ./ai-shortlink
./ai-shortlink
```

程序启动时会自动读取当前目录的 `shortlink.env`，也可以通过 `SHORTLINK_CONFIG=/opt/shortlink/shortlink.env` 指定配置文件。系统环境变量优先于配置文件，方便 systemd、Docker 或面板覆盖。首次启动后访问 `/setup`，安装向导会把数据库、站点、SMTP、管理员等运行时配置写入 `DATA_DIR/app-config.json`。

如使用 systemd，发布包内的 `ai-shortlink.service` 已固定工作目录为 `/opt/shortlink`：

```bash
sudo cp /opt/shortlink/ai-shortlink.service /etc/systemd/system/ai-shortlink.service
sudo systemctl daemon-reload
sudo systemctl enable --now ai-shortlink
```

### 本地编译运行

系统需要安装 SQLite3 开发库，因为内置数据库模式使用 cgo 绑定 SQLite。

```bash
make build
./bin/ai-shortlink
```

首次访问 `/setup` 完成安装。运行时配置会保存到 `DATA_DIR/app-config.json`。

## 环境变量

现在只保留少量启动级配置，绝大多数系统参数进入后台设置页维护。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_ADDR` | `:8080`（如未设置且存在 `PORT`，会自动使用 `PORT`） | HTTP 监听地址 |
| `DATA_DIR` | `./data` | 数据库文件、上传图片、运行时配置存储目录 |
| `APP_SECRET` | 自动生成并保存 | 用于 Cookie 签名、恢复 Key 加密、哈希。生产环境首次部署后不要随意更换 |
| `TRUST_PROXY` | `false` | 反向代理后是否信任 `X-Forwarded-For` / `X-Real-IP` |
| `COOKIE_SECURE` | `false` | HTTPS 部署时设为 `true` |
| `SESSION_TTL_HOURS` | `87600` | 浏览器登录令牌有效期，默认约 10 年 |
| `UPLOAD_MAX_MB` | `8` | 单张二维码图片上传大小上限 |

以下配置也仍兼容环境变量，但推荐通过安装向导/系统设置维护：

- `DATABASE_MODE=embedded|mysql`
- `SQLITE_PATH=/path/to/ai-shortlink.db`
- `DATABASE_DSN=...`
- `APP_NAME=...`
- `APP_BASE_URL=https://s.example.com`

## 反向代理建议

Nginx 示例：

```nginx
server {
    listen 443 ssl http2;
    server_name s.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

反代和 HTTPS 部署时建议：

```env
TRUST_PROXY=true
COOKIE_SECURE=true
```

并在后台“系统设置”中把公网域名设置为 `https://s.example.com`。

## 路由说明

| 路由 | 说明 |
| --- | --- |
| `/setup` | 首次安装向导 |
| `/admin` | 管理后台 |
| `/login` | 登录页，支持 Magic Link / 浏览器一键 / 恢复 Key |
| `/s/{code}` | 短链跳转 |
| `/{code}` | 根路径短链兼容跳转 |
| `/q/{code}` | 活码展示页 |
| `/qr/short/{code}.png` | 短链二维码图 |
| `/qr/live/{code}.png` | 活码入口二维码图 |
| `/uploads/...` | 上传后的二维码图片 |
| `/api/admin/...` | 管理后台 JSON API |

## 项目结构

```text
cmd/server/                    程序入口
internal/auth/                 Cookie、浏览器标识、恢复 Key、加密/哈希
internal/config/               启动配置与运行时配置
internal/dbutil/               数据库连接与自动迁移
internal/dbutil/migrations/    内置 MySQL/SQLite 迁移
internal/model/                数据模型
internal/mysqlmini/            轻量 MySQL/MariaDB text protocol driver
internal/sqlitecgo/            轻量 SQLite database/sql driver
internal/qrcode/               轻量 QR SVG 编码器
internal/server/               HTTP 路由、API、安装、登录、公开访问页
internal/store/                SQL 数据访问层
internal/util/                 短码、IP、UA、时间等工具
migrations/                    可读 SQL 迁移文件
web/static/                    管理后台静态资源
web/templates/                 HTML 模板
```

## 数据库表

核心表：

- `system_settings`：安装状态、域名、语言、登录模式、SMTP 等系统设置。
- `short_links`：短链配置与审核状态。
- `live_qrs`：活码入口配置与审核状态。
- `live_qr_items`：活码下的多套二维码及审核状态。
- `visit_logs`：访问、展示、长按识别意图事件。
- `admin_accounts`：后台账户、邮箱、恢复 Key。
- `admin_devices`：已绑定后台浏览器。
- `magic_login_tokens`：Magic Link 一次性登录令牌。
- `audit_logs`：后台操作审计。

## 质量检查

```bash
go test ./...
go vet ./...
go build -o bin/ai-shortlink ./cmd/server
node --check web/static/app.js
```

当前交付版本已在构建环境通过以上检查，并已构建 Linux amd64 可执行文件：`bin/ai-shortlink`。
