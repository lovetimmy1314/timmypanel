# Timmypanel

**自托管的导航起始页：登录后，常用的网站都在一屏之内。**

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8)
![Vue](https://img.shields.io/badge/Vue-3-42b883)

English: [README.md](README.md)

Timmypanel 是给少数几个信得过的人用的私人导航站——你自己、家人、家里那台 NAS 的使用者。
它**不是**公开的网址目录：每个页面都要求登录，每个账号只看得到自己的分组、卡片、壁纸和设置。

整个项目的部署产物是**一个可执行文件 + 一个 SQLite 文件**。Vue 前端用 `embed.FS` 编译进 Go
二进制，所以不需要 Node 运行时、不需要另起 web 服务器、不需要装数据库。把二进制拷到 VPS 上，
前面挂个反代就能用——或者直接跑现成的 Docker 镜像，连这步都省了。

<!-- 截图放这里。 -->

## 功能

**把链接收进来**

- **批量导入**：粘贴文本、上传浏览器导出的书签 HTML、粘贴 JSON，三种入口都先给预览再落库。
  书签的目录结构会还原成分组。
- **单条添加**：填个网址点「自动抓取」，标题、描述、图标自动补齐。
- **批量补全**：给已经收录的卡片成批抓图标和描述，可按分组筛选，带进度条、能中途停。
- **收藏书签**（bookmarklet）：服务器抓不动的站点，把书签拖到书签栏，在目标页面点一下，
  由**你的浏览器**把图标和标题传回面板。

**用起来**

- **卡片首页**：按分组展示，可折叠，平铺和页签两种布局。编辑模式下可拖拽排序、跨组移动。
- **搜索**：站内模糊搜索 + 可配置的外部引擎（Google / Bing / 百度……）聚合，`/` 或 `Ctrl+K` 聚焦。
- **内外网双地址**：给 NAS、软路由这类卡片同时填公网和内网地址，顶部一键切换整站模式。
- **观感**：明暗模式；图片 / 纯色 / 渐变壁纸，附模糊度和暗色遮罩滑块；上传过的图片集中在图库里管理。
- **中英双语**：界面语言按账号存。
- **适配手机**：响应式布局，支持「添加到主屏幕」，有分组跳转和回到顶部两个悬浮按钮。

**数据是你自己的**

- **多账号**，彼此完全隔离。
- **备份**：导出 JSON（只有数据）或 ZIP（额外带上传的图片）；服务器还会每天自动留一份快照。
  导入前会先自动存一份当前数据，覆盖导入误操作也能救回来。
- **会话**：勾「记住我」有效期 30 天；可以查看已登录设备并远程踢下线。
- **全局设置**（管理员）：站点标题、浏览器图标、登录页背景，整站一份。
- **运行时不发第三方请求**：图标集是打包进产物的，不去 CDN 取；页面的 `connect-src` 只有 `'self'`。

## 快速开始（Docker）

镜像由 CI 构建并推到 GHCR，`amd64` 和 `arm64` 都有，树莓派、甲骨文的 Ampere、群晖那些 arm 机器
直接能跑。镜像约 58 MB——alpine 加一个静态二进制，跑在 uid 10001 下。

```bash
curl -fsSLO https://raw.githubusercontent.com/lovetimmy1314/timmypanel/main/docker-compose.yml
docker compose up -d
docker compose logs timmypanel     # 生成的初始管理员密码在这里
```

面板现在在 `127.0.0.1:8080` 上。**端口是刻意只发布到回环口的**——对外要在前面挂反代做 TLS，
见下一节。用日志里那串密码登录，然后立刻改掉。

升级不影响数据（数据在命名卷 `timmypanel_timmypanel-data` 里）：

```bash
docker compose pull && docker compose up -d
```

几个要知道的点：

- `latest` 跟的是 `main` 的最新提交，不是最新的 release。想钉死版本：
  `TP_TAG=1.2.3 docker compose up -d`。**镜像 tag 不带 `v`**——git tag 打的是 `v1.2.3`，
  出来的镜像是 `1.2.3` 和 `1.2`。
- 宿主机 8080 被占了就 `TP_PORT=18080 docker compose up -d`。
- **别用 `TP_ADMIN_PASSWORD` 设初始密码**：它会被明文写进卷里的 `config.yaml`。
  从日志里读随机密码就行。
- 备份整卷：

  ```bash
  docker run --rm -v timmypanel_timmypanel-data:/data -v "$PWD":/out alpine \
    tar czf /out/timmypanel-backup.tar.gz -C /data .
  ```

不想放 compose 文件、想一条命令跑起来的话，下面这条等价：

```bash
docker run -d --name timmypanel --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v timmypanel-data:/data \
  -e TP_SECURE=true \
  -e TP_TRUSTED_PROXIES=172.17.0.1 \
  --stop-timeout 30 \
  ghcr.io/lovetimmy1314/timmypanel:latest

docker logs timmypanel     # 生成的初始管理员密码在这里
```

前面挂了 HTTPS 反代时，这两个环境变量一个都不能少：没有 `TP_SECURE`，会话 Cookie 不带
`Secure` 标记；没有 `TP_TRUSTED_PROXIES`，伪造的 `X-Forwarded-For` 能绕过登录限流。
`172.17.0.1` 是默认 bridge 网关的常见值，但别照抄，确认一下——这也是 compose 和
`docker run` **唯一不通用**的一项（compose 锁了自己的子网 `172.28.0.1`）：

```bash
docker network inspect bridge -f '{{(index .IPAM.Config 0).Gateway}}'
```

和 compose 那条路还有两点不同：卷名就是 `timmypanel-data`，没有项目名前缀（上面那条备份命令
要相应改名）；升级是「重建容器」，数据在卷里所以不会丢：

```bash
docker pull ghcr.io/lovetimmy1314/timmypanel:latest
docker rm -f timmypanel
# 然后重新执行上面那条 docker run
```

完整的环境变量清单见 [`deploy/Dockerfile`](deploy/Dockerfile) 顶部。

## 不用 Docker 部署（systemd + Caddy）

```bash
# 1. 建用户和目录
sudo useradd -r -s /usr/sbin/nologin timmypanel
sudo mkdir -p /opt/timmypanel/data
sudo chown -R timmypanel:timmypanel /opt/timmypanel
```

```bash
# 2. 上传二进制（在本地执行）
scp dist/timmypanel-linux-amd64 root@你的服务器:/opt/timmypanel/timmypanel
```

```bash
# 3. 装 systemd 服务
sudo cp deploy/timmypanel.service /etc/systemd/system/
sudo chmod +x /opt/timmypanel/timmypanel
sudo systemctl enable --now timmypanel
sudo journalctl -u timmypanel -n 30    # 初始管理员密码在这里
```

```bash
# 4. 配反代（自动 HTTPS，先把 Caddyfile 里的域名换成你的）
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

然后改 `/opt/timmypanel/data/config.yaml`：`server.secure` 设成 `true`（否则会话 Cookie 不带
`Secure` 标记），`server.trusted_proxies` 填 `["127.0.0.1"]`（否则登录限流会被伪造的
`X-Forwarded-For` 绕过），重启服务。

## 配置说明

`data/config.yaml` 在首次启动时生成，包含随机密钥和随机的初始管理员密码。

```yaml
server:
  listen: 127.0.0.1:8080     # 只监听本地，由反代对外
  secure: true               # HTTPS 部署必须开，否则会话 Cookie 不带 Secure
  trusted_proxies:           # 留空 = 不信任任何代理头
    - 127.0.0.1
data:
  dir: ./data
auth:
  secret: ...                # 首次启动自动生成
  remember_days: 30          # 「记住我」的有效期
  initial_admin:
    username: admin
    password: ...            # 只在库里一个用户都没有时才用得上
  allow_register: false      # 公网建议保持 false，账号由管理员在后台创建
fetch:
  allow_private: false       # 是否允许抓取内网地址的图标（默认关，防 SSRF）
  timeout_sec: 8
backup:
  auto_daily: true
  keep: 7
```

和部署有关的项都能用环境变量覆盖：`TP_LISTEN`、`TP_SECURE`、`TP_DATA_DIR`、
`TP_TRUSTED_PROXIES`（逗号分隔）、`TP_SECRET`、`TP_ADMIN_USER`、`TP_ADMIN_PASSWORD`、
`TP_ALLOW_PRIVATE_FETCH`。

命令行参数：`-config <路径>`、`-debug`、`-version`、`-reset-password <用户名>`。

## 忘记管理员密码

> `config.yaml` 里那行 `initial_admin.password` 在**你改过密码之后就是废的**。
> 它只在库里一个用户都没有时才会被读，文件里留着的那个旧值登录会 401。别去翻这份文件。

还有另一个管理员账号的话，用它登录，在「账号管理」里给忘记密码的账号设新密码。
否则让二进制自己重置——不用停服务、不用碰 sqlite：

```bash
docker exec timmypanel /app/timmypanel -config /data/config.yaml -reset-password admin
```

```bash
sudo -u timmypanel /opt/timmypanel/timmypanel \
  -config /opt/timmypanel/data/config.yaml -reset-password admin
```

标准输出会打印一串新密码，该账号已有的会话会被全部踢掉。
若刚连续输错触发了限流，等过 15 分钟锁定期，或重启一次。

## 从源码构建

需要 Go 1.25+ 和 Node 20+。

```powershell
.\build.ps1 -Target all
```

产物在 `dist/`：`timmypanel.exe` 和 `timmypanel-linux-amd64`。因为 sqlite 驱动是纯 Go 的
（`modernc.org/sqlite`），`CGO_ENABLED=0` 就能在 Windows 上直接交叉编译 Linux 二进制，
不需要装 gcc。`-Target windows|linux|all` 选平台，`-SkipFrontend` 只重编 Go 那一半。
Windows 上也可以双击 `build.bat`，它只是同一个脚本的入口。

手动构建是**有先后顺序的两步**，因为前端产物是 Go 编译期的依赖：

```bash
cd frontend && npm ci && npm run build     # 产物写进 backend/internal/web/dist
cd ../backend && go build -o timmypanel .  # 把它 embed 进去
```

Docker 镜像正常由 CI 出（`.github/workflows/docker.yml`，推 `main` 和 `v*` tag 时触发，
用自带的 `GITHUB_TOKEN`，不用配 secret）。本地要自己构建：

```bash
docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
```

## 本地开发

两个终端：

```bash
cd backend && go run . -config ../data/config.yaml -debug
```

```bash
cd frontend && npm install && npm run dev
```

浏览器打开 <http://localhost:5173>。前端 dev server 把 `/api` 和 `/uploads` 代理到 8080，
所以会话 Cookie 是同源的。后端首次启动会在日志里打印生成的管理员账号和密码；
想自己指定：`TP_ADMIN_PASSWORD=xxx go run .`。

提交前要跑的检查：

```bash
cd backend && gofmt -l . && go vet ./... && go test ./...
```

```bash
cd frontend && npm run typecheck
```

`npm run build` **不做**类型检查，所以 `npm run typecheck` 不是可选项。

```
Timmypanel/
├── backend/
│   ├── main.go
│   └── internal/
│       ├── config/     配置加载（yaml + TP_ 环境变量）
│       ├── model/      数据模型与迁移
│       ├── session/    服务端会话（Cookie 只存随机 token，库里存它的哈希）
│       ├── middleware/ 鉴权、CSRF、登录限流
│       ├── service/    元信息抓取（SSRF 防护）、书签解析
│       ├── api/        HTTP 接口
│       └── web/        embed 进来的前端产物
├── frontend/           Vue 3 + TypeScript + Vite + Naive UI + Tailwind
├── brand/              标志源文件（SVG 各变体 + 预览页）
├── deploy/             systemd unit、Caddyfile、Dockerfile
├── docs/               编码约定、决策记录、计划
└── data/               运行时生成：config.yaml、sqlite、uploads、backups
```

[`docs/`](docs/) 下的三个文件装的是代码里看不出来的东西：

| 文件 | 答什么 |
|---|---|
| [`conventions.md`](docs/conventions.md) | 这个项目按什么规矩写代码 |
| [`decisions.md`](docs/decisions.md) | 当初为什么这么定，代价是什么 |
| [`plans.md`](docs/plans.md) | 计划做什么、明确不做什么，以及手工验证清单 |

**想删掉一段看起来多余的代码之前先读 `decisions.md`。** 有好几处防御性写法——手写的 SPA 回落、
上传响应上的 CSP 头、拖拽那条看着冗余的分支——删掉编译照过、测试照过，跑起来才出问题。

## 安全设计

Timmypanel 是按「挂在公网、前面只有一道登录」来设计的。

- **会话在服务端**。Cookie 是 `HttpOnly` 的，里面是 32 字节随机 token，库里只存它的 SHA-256。
  脚本读不到，且任何一个会话都能随时吊销。不用 `localStorage` 里的 JWT。
- **登录失败按 IP 和用户名分别限流**——5 次锁 15 分钟。「用户不存在」和「密码错误」返回同一句话，
  避免枚举账号。
- **抓取图标是「登录用户可控的任意出站请求」**，所以默认拒绝内网地址。校验发生在**真正建连的
  IP** 上，DNS rebinding 也躲不过；重定向次数、超时和响应体大小都有上限。
- **上传按文件实际内容判类型**，不看扩展名，只收 PNG/JPG/WebP/GIF，落盘时重命名为随机名。
- **`/uploads/*` 需要登录**，且只能读自己的目录。响应带 `Content-Security-Policy: sandbox`，
  抓来的 SVG 图标即使被直接导航打开也执行不了脚本。
- **CSRF** 靠强制所有写请求带 `X-Requested-With` 头，配合 `SameSite=Lax` Cookie。

发现安全问题：请开一个标注了性质的 issue，或者私下联系维护者，别直接贴可用的利用代码。

## 已知取舍

- **没做 iframe 内嵌网页**：主流站点基本都发 `X-Frame-Options: DENY`，能嵌的是少数。
- **没做在线状态监控**：定时探测会引出后台任务、超时和误报一堆问题，一个导航页不值当。
- **少数站点仍然抓不动**：抓取用的是浏览器 UA、图标会依次试多个候选，大部分站点够用了；
  但 Cloudflare 高防那一档还看 TLS 指纹和 IP 信誉，仍会返回 403，只能手动传图标。
- **抓取不走代理**：图标是**服务器**去下载的，不是你的浏览器。所以「浏览器能打开但服务器访问
  不了」的站点抓不到图标——部署在能直连目标站点的机器上即可。

## 参与开发

欢迎提 issue 和 PR。开 PR 之前：

- 读一遍 [`docs/conventions.md`](docs/conventions.md)——它只写「这个项目和别处不一样」的部分，很短；
- 跑一遍[本地开发](#本地开发)那节列的检查；
- 改动涉及鉴权、用户隔离、上传、抓取的话，按 [`docs/plans.md`](docs/plans.md) 末尾的手工清单走一遍；
- **保持 `backend/internal/web/dist/index.html` 是占位页**。`npm run build` 会把它改成带哈希的
  真页面，而 `assets/` 不进版本库，提交了真页面等于提交一份指向不存在文件的 HTML——新克隆跑
  起来是静默白屏。提交前 `git checkout -- backend/internal/web/dist/index.html` 还原。

注释、日志、后端错误文案一律中文，标识符和文件名一律 ASCII。界面文案走 `frontend/src/i18n/`，
中英各要写一条——漏一条英文类型检查就不过。

## 许可证

[GNU AGPL v3](LICENSE)。你可以自由使用、修改和再分发本项目，但如果你把修改过的版本作为网络服务
提供给别人使用，就必须向这些用户提供你那份修改后的源码。
