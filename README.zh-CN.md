# Timmypanel

**自己搭的网址导航页：登录后，常用网站都在首页，点一下就能打开。**

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8)
![Vue](https://img.shields.io/badge/Vue-3-42b883)

English: [README.md](README.md)

Timmypanel 把你常用的网站收成一张卡片墙。它是私人的：每个页面都要登录，每个账号只能看到自己的分组、卡片、壁纸和设置。适合自己、家人或小团队，不是公开的网址大全。

部署只需要 **一个程序 + 一份数据**。网页已经打进程序里，不用再装 Node、网站服务器或数据库。最省事的办法是直接跑现成的 Docker 镜像。

![截图1](screenshot/1.jpg)
![截图2](screenshot/2.jpg)
![截图3](screenshot/3.jpg)

## 功能

- 批量导入浏览器书签，也能单个添加并自动抓标题、描述和图标
- 服务器抓不到的站点，可用收藏书签从浏览器回传
- 卡片按分组展示，支持拖拽排序、跨组移动
- 站内搜索，并可加上 Google / Bing / 百度等外部引擎
- 一张卡片可同时填公网和内网地址，顶部一键切换
- 明暗主题，图片 / 纯色 / 渐变壁纸
- 中英双语，适配手机，可添加到主屏幕
- 多账号彼此隔离；支持备份导出 / 导入，服务器每天自动留快照
- 运行时不访问外部 CDN，图标都打在程序里

## 部署

推荐用 Docker：复制下面的命令就能跑起来。不会 Docker、只想跑一个文件，看后面的「不用 Docker」。

程序默认只在本机 `127.0.0.1:8080` 提供服务，不会直接暴露到公网。先按下面步骤确认能登录，再配域名和 HTTPS。

### 用 Docker 部署（推荐）

先在机器上装好 [Docker](https://docs.docker.com/get-docker/)（现在一般会连 Compose 一起装上）。树莓派、群晖、普通 Linux VPS 都可以，镜像约 58 MB。

**1. 建一个目录，下载配置并启动**

```bash
mkdir timmypanel && cd timmypanel
curl -fsSLO https://raw.githubusercontent.com/lovetimmy1314/timmypanel/main/docker-compose.yml
docker compose up -d
```

**2. 查看初始密码并登录**

```bash
docker compose logs timmypanel
```

日志里会打印管理员用户名和随机密码。用浏览器打开 <http://127.0.0.1:8080>，登录后立刻改掉这个密码。

**3. 以后升级**

数据在 Docker 卷里，升级不会丢：

```bash
docker compose pull && docker compose up -d
```

也可以用现成脚本（自动判断你是 compose 还是 `docker run`）：

```bash
curl -fsSLO https://raw.githubusercontent.com/lovetimmy1314/timmypanel/main/deploy/update.sh
chmod +x update.sh
./update.sh
```

**常见调整**

- 8080 端口已被占用：`TP_PORT=18080 docker compose up -d`，然后访问 <http://127.0.0.1:18080>。
- 想钉死版本、不要跟着最新代码变：`TP_TAG=1.2.3 docker compose up -d`。镜像 tag **不带 `v`**（git 的 tag 是 `v1.2.3`，镜像是 `1.2.3`）。
- **不要**用 `TP_ADMIN_PASSWORD` 指定初始密码，它会明文写进数据卷。从日志里读随机密码即可。
- 备份全部数据：

  ```bash
  docker run --rm -v timmypanel_timmypanel-data:/data -v "$PWD":/out alpine \
    tar czf /out/timmypanel-backup.tar.gz -C /data .
  ```

  上面这条卷名，只有你按第 1 步把目录命名为 `timmypanel` 时才对得上。目录名不同的话，用 `docker volume ls` 找带 `timmypanel-data` 的那一个。

### 让外网能访问

面板只监听本机，前面需要 Nginx 或 Caddy 做 HTTPS。以 Caddy 为例，把域名换成你的：

```
nav.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

更完整的配置见 [`deploy/Caddyfile`](deploy/Caddyfile)。

用上面的 compose 文件时，公网部署需要的两项已经写好了。**不要**把端口改成 `8080:8080`（不带 `127.0.0.1`），那样等于把面板直接裸奔到公网，防火墙也拦不住。

### 不用 compose，一条命令启动

```bash
docker run -d --name timmypanel --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v timmypanel-data:/data \
  -e TP_SECURE=true \
  -e TP_TRUSTED_PROXIES=172.17.0.1 \
  --stop-timeout 30 \
  ghcr.io/lovetimmy1314/timmypanel:latest

docker logs timmypanel     # 初始管理员密码在这里
```

前面挂了 HTTPS 反代时，`TP_SECURE` 和 `TP_TRUSTED_PROXIES` 两个都不能少：前者让登录 Cookie 只能走 HTTPS，后者让登录限流认反代传来的真实 IP。`172.17.0.1` 是 Docker 默认网桥网关的常见值，用下面命令确认一下（compose 那条路不用管这个，它锁了自己的网段）：

```bash
docker network inspect bridge -f '{{(index .IPAM.Config 0).Gateway}}'
```

这条路的数据卷名是 `timmypanel-data`（没有项目名前缀）。升级用上面的 `update.sh`，或手工三步——注意用 `stop`，不要 `rm -f`，否则数据库来不及收尾：

```bash
docker pull ghcr.io/lovetimmy1314/timmypanel:latest
docker stop -t 30 timmypanel && docker rm timmypanel
# 然后重新执行上面那条 docker run
```

完整环境变量清单见 [`deploy/Dockerfile`](deploy/Dockerfile) 顶部。

### 不用 Docker（systemd + Caddy）

这一路需要先有 Linux 可执行文件。仓库发布的是 Docker 镜像；二进制请按后面「从源码构建」编出来，再上传到服务器。

```bash
# 1. 建用户和目录
sudo useradd -r -s /usr/sbin/nologin timmypanel
sudo mkdir -p /opt/timmypanel/data
sudo chown -R timmypanel:timmypanel /opt/timmypanel
```

```bash
# 2. 上传二进制（在你自己的电脑上执行）
scp dist/timmypanel-linux-amd64 root@你的服务器:/opt/timmypanel/timmypanel
```

```bash
# 3. 装成系统服务
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

然后改 `/opt/timmypanel/data/config.yaml`：把 `server.secure` 设成 `true`，把 `server.trusted_proxies` 设成 `["127.0.0.1"]`，再重启服务。不改这两项，登录 Cookie 和登录限流在公网上会出问题。

## 配置说明

`data/config.yaml` 在第一次启动时自动生成，里面有随机密钥和随机的初始管理员密码。

```yaml
server:
  listen: 127.0.0.1:8080     # 只监听本机，由反代对外
  secure: true               # HTTPS 部署必须开
  trusted_proxies:           # 留空 = 不信任任何代理头
    - 127.0.0.1
data:
  dir: ./data
auth:
  secret: ...                # 首次启动自动生成
  remember_days: 30          # 「记住我」的有效期
  initial_admin:
    username: admin
    password: ...            # 只在库里还一个用户都没有时才用得上
  allow_register: false      # 预留，注册尚未实现；账号由管理员在后台创建
fetch:
  allow_private: false       # 是否允许抓取内网地址的图标（默认关）
  timeout_sec: 8
backup:
  auto_daily: true
  keep: 7
```

和部署有关的项也能用环境变量覆盖：`TP_LISTEN`、`TP_SECURE`、`TP_DATA_DIR`、`TP_TRUSTED_PROXIES`（逗号分隔）、`TP_SECRET`、`TP_ADMIN_USER`、`TP_ADMIN_PASSWORD`、`TP_ALLOW_PRIVATE_FETCH`。

其中两个布尔项（`TP_SECURE`、`TP_ALLOW_PRIVATE_FETCH`）**只认** `true` / `false` / `1` / `0`。写成 `yes`、`on` 之类会被忽略并在日志里告警，配置文件里的值原样保留——不会因为拼错就把 `TP_SECURE` 静默关掉。

命令行参数：`-config <路径>`、`-debug`、`-version`、`-reset-password <用户名>`。

## 忘记管理员密码

> `config.yaml` 里那行 `initial_admin.password` 在**你改过密码之后就没用了**。
> 它只在数据库里一个用户都没有时才会被读。别去翻这份文件。

还有另一个管理员账号的话，用它登录，在「账号管理」里给忘记密码的账号设新密码。否则让程序自己重置——不用停服务、不用改数据库：

```bash
docker exec timmypanel /app/timmypanel -config /data/config.yaml -reset-password admin
```

```bash
sudo -u timmypanel /opt/timmypanel/timmypanel \
  -config /opt/timmypanel/data/config.yaml -reset-password admin
```

屏幕上会打印一串新密码，该账号已有的登录会被全部踢掉。
若刚连续输错被锁，等 15 分钟，或重启一次。

## 从源码构建

需要 Go 1.25+ 和 Node 20+。

```powershell
.\build.ps1 -Target all
```

产物在 `dist/`：`timmypanel.exe` 和 `timmypanel-linux-amd64`。sqlite 驱动是纯 Go 的，所以在 Windows 上也能直接编出 Linux 程序，不用装 gcc。`-Target windows|linux|all` 选平台，`-SkipFrontend` 只重编程序、不重编网页。Windows 上也可以双击 `build.bat`。

手动构建是**有先后顺序的两步**，网页必须先编好，再编进程序里：

```bash
cd frontend && npm ci && npm run build     # 产物写进 backend/internal/web/dist
cd ../backend && go build -o timmypanel .  # 把它打进二进制
```

Docker 镜像一般由 GitHub Actions 自动构建。要在本地自己打镜像：

```bash
docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
```

## 本地开发

两个终端分别启动：

```bash
cd backend && go run . -config ../data/config.yaml -debug
```

```bash
cd frontend && npm install && npm run dev
```

浏览器打开 <http://localhost:5173>。前端会把 `/api` 和 `/uploads` 转到 8080，所以登录状态是通的。后端第一次启动会在日志里打印管理员账号和密码；想自己指定：`TP_ADMIN_PASSWORD=xxx go run .`。

提交前要跑的检查：

```bash
cd backend && gofmt -l . && go vet ./... && go test ./...
```

```bash
cd frontend && npm run typecheck
```

`npm run build` **不会**做类型检查，所以 `npm run typecheck` 不能省。

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
│       └── web/        打进程序里的前端产物
├── frontend/           Vue 3 + TypeScript + Vite + Naive UI + Tailwind
├── brand/              标志源文件
├── deploy/             systemd、Caddyfile、Dockerfile、update.sh
├── docs/               编码约定、决策记录、计划
└── data/               运行时生成：config.yaml、sqlite、uploads、backups
```

[`docs/`](docs/) 下三个文件装的是代码里看不出来的东西：

| 文件 | 答什么 |
|---|---|
| [`conventions.md`](docs/conventions.md) | 这个项目按什么规矩写代码 |
| [`decisions.md`](docs/decisions.md) | 当初为什么这么定，代价是什么 |
| [`plans.md`](docs/plans.md) | 计划做什么、明确不做什么，以及手工验证清单 |

想删掉一段看起来多余的代码之前，先读 `decisions.md`。有好几处是踩过坑才加上的，删掉能编译、能过测试，跑起来才出问题。

## 安全设计

Timmypanel 按「挂在公网、前面只有一道登录」来设计。

- **会话在服务端**。Cookie 读不了脚本，里面是随机 token，库里只存哈希。任何一个登录都能随时踢掉。不用浏览器本地存的 JWT。
- **登录失败会限流**：同一 IP 或同一用户名连续 5 次错误，锁 15 分钟。「用户不存在」和「密码错误」返回同一句话，避免被人试出有哪些账号。
- **抓取图标默认拒绝内网地址**，并且检查的是真正连上的 IP，避免被骗去打内网。超时、跳转次数、下载大小都有上限。
- **上传按文件实际内容判类型**，不看后缀，只收常见图片，保存时换成随机文件名。
- **上传的文件必须登录才能读**，而且只能读自己的。响应带沙箱头，抓来的图标即使直接打开也执行不了脚本。
- **写操作必须带自定义请求头**，配合浏览器的 Cookie 策略，防止跨站伪造请求。

发现安全问题：请开一个标明性质的 issue，或私下联系维护者，不要直接贴可利用的细节。

## 已知取舍

- **没有 iframe 内嵌网页**：主流网站大多禁止被嵌进别人的页面，做了也大半是空框。
- **没有在线状态监控**：定时探测会引出一堆误报和后台任务，一个导航页不值当。
- **少数网站仍然抓不到图标**：大部分站点没问题；遇到高防或只认特定浏览器的，请手动上传图标。
- **抓取不走代理**：图标是**服务器**去下载的，不是你的浏览器。浏览器能打开、服务器访问不了的站点，就抓不到——把面板部署在能直接访问那些网站的机器上即可。

## 参与开发

欢迎提 issue 和 PR。开 PR 之前：

- 读一遍 [`docs/conventions.md`](docs/conventions.md)——很短，只写这个项目和别处不一样的地方；
- 跑一遍[本地开发](#本地开发)那节的检查；
- 改动涉及登录、用户隔离、上传、抓取的话，按 [`docs/plans.md`](docs/plans.md) 末尾的清单走一遍；
- **不要提交构建出来的** `backend/internal/web/dist/index.html`。构建会改写它，但配套的静态文件不进仓库，提交了新克隆打开会白屏。提交前执行 `git checkout -- backend/internal/web/dist/index.html`。

注释、日志、后端错误文案用中文，代码标识符和文件名用英文。界面文案走 `frontend/src/i18n/`，中英各写一条——漏一条英文，类型检查就过不了。

## 许可证

[GNU AGPL v3](LICENSE)。你可以自由使用、修改和再分发本项目；但如果你把修改过的版本作为网络服务提供给别人使用，就必须向这些用户提供你那份修改后的源码。
