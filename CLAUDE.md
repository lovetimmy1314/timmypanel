# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目

Timmypanel 是自托管的网址导航站：登录后把常用网站以卡片形式聚合到首页。Go 后端 + Vue 3 前端，
前端资源 `embed` 进二进制，部署产物是**一个可执行文件 + 一个 sqlite 文件**。目标部署环境是
公网 Linux VPS（systemd + Caddy 反代），因此安全约束比纯内网项目严格，见下文「不变量」。

项目以 **AGPL-3.0 开源**，对外文档是**中英两份**：`README.md`（英文）和 `README.zh-CN.md`（中文）。
改了面向用户的功能、配置项或部署方式，**两份都要改**，别只改一份。

## 落盘文件：开工前先看

`docs/` 下**只有三个文件**，是这个项目的书面记忆，代码里看不出来的东西都在那儿。
刻意保持三个，别再拆目录。

| 文件 | 答什么 | 什么时候读写 |
|---|---|---|
| `docs/conventions.md` | 按什么规矩写代码 | 动手前读；定下新约定时追加 |
| `docs/decisions.md` | 当初为什么这么定 | 要改的代码在某条的「相关」里出现过就先读 |
| `docs/plans.md` | 打算做什么、明确不做什么、改完按什么清单验 | 跨会话的活，收尾前更新 |

（本文件 CLAUDE.md 答「怎么做」，是命令式的，每次会话自动读。别把内容互相抄。）

规矩：

- **想删掉一段看起来多余的代码之前**，先在 `docs/decisions.md` 里搜一下那个文件名。
  好几处防御性写法是踩过坑才加的（SPA 回落不用 `http.FileServer`、上传路由的 CSP 头），
  删掉编译照过、测试照过，线上才出问题。
- **做了有取舍的技术决定**：在 `decisions.md` 末尾追加一条，编号递增，
  写清**为什么**和**代价**。推翻旧决策不要删，把状态改成「已废弃（被 00X 取代）」再补新条目。
- **跨会话的活**：在 `plans.md` 的「进行中」维护，会话结束前更新进展——
  写清卡在哪比写做完了什么更有用。**做完就把那条删掉**，别在 `plans.md` 里记流水账：
  做过什么 git 历史里有，这个文件只答「还要做什么」。
- 改了鉴权、隔离、上传、抓取相关代码，按 `plans.md` 末尾的验证清单跑一遍。

## 提交：改完就提交，不用问

**这个仓库的既定要求是每次执行完后，就进行`git commit` 一次，用中文写 message**，不需要每次征求同意
（这一条覆盖「只在用户要求时才提交」的默认行为）。提交前先跑通检查，
格式和粒度见 `docs/conventions.md` 的 Git 一节。直接提交到 `main`，不开分支，
不 push 到远端除非人明确要求。

## 常用命令

开发（两个终端）：

```bash
cd backend && go run . -config ../data/config.yaml -debug
```

```bash
cd frontend && npm run dev
```

前端 dev server 在 5173，`/api` 和 `/uploads` 代理到 8080，所以会话 Cookie 是同源的。
首次启动后端会生成 `data/config.yaml` 并在日志里打印初始管理员密码；
用 `TP_ADMIN_PASSWORD=xxx` 可指定。

后端检查：

```bash
cd backend && gofmt -l . && go vet ./... && go test ./...
```

跑单个测试：

```bash
cd backend && go test ./internal/service -run TestParseBookmarks -v
```

前端类型检查（`npm run build` **不做**类型检查，改完 TS 要单独跑）：

```bash
cd frontend && npm run typecheck
```

出构建产物到 `dist/`：

```powershell
.\build.ps1 -Target all
```

不想开 PowerShell 就双击 `build.bat`（等价于 `-Target windows`），
也能带参数：`build.bat linux` / `build.bat all` / `build.bat -SkipFrontend`。
它只是 `build.ps1` 的入口，构建步骤别在里面抄第二份。

`-Target windows|linux|all`，`-SkipFrontend` 只重编 Go。sqlite 驱动是纯 Go 的
（`modernc.org/sqlite`，经 `github.com/glebarez/sqlite` 接 GORM），所以 `CGO_ENABLED=0`
就能在 Windows 上交叉编译 Linux 二进制，不需要 gcc。

## 架构要点

### 构建耦合：前端产物是 Go 的编译期依赖

`frontend/vite.config.ts` 的 `outDir` 指向 `backend/internal/web/dist`，
`backend/internal/web/embed.go` 用 `//go:embed all:dist` 把它打进二进制。因此：

- **改了前端必须先 `npm run build` 再 `go build`**，否则二进制里还是旧页面。
- `dist/index.html` 必须始终存在（embed 不接受空目录），未构建时它是一个占位页。
  `.gitignore` 只忽略 `dist/assets/`。
- **版本库里那份 index.html 必须保持占位页**：构建会把它改成带哈希的真页面，
  而 `assets/` 不进版本库，提交了就等于提交一份指向不存在文件的 HTML——
  新克隆跑起来是**静默白屏**，不报「前端资源缺失」。构建完 `git status` 见它是脏的，
  用 `git checkout -- backend/internal/web/dist/index.html` 还原（决策 014，已踩过一次）。

### 会话与鉴权

`internal/session` 是独立包，被 `internal/middleware` 和 `internal/api` 共用（这样不产生循环依赖）。
Cookie 里放 32 字节随机 token 原文，数据库 `sessions` 表只存它的 SHA-256。
勾「记住我」→ 30 天 + 滑动续期；不勾 → 会话 Cookie（服务端仍有 12 小时上限）。
改密码会 `DestroyAllForUser` 再给当前浏览器重签。

### 用户数据隔离

`(*Server).scoped(c)`（`internal/api/group.go`）是所有用户数据查询的**唯一正确入口**，
它把 `user_id = 当前用户` 固定进查询。新增涉及 Group/Site/Setting/Upload 的 handler 时必须走它，
或显式带上 `user_id` 条件。写操作若接受外部传入的 `groupId`，一律用
`(*Server).ownsGroup(uid, groupID)` 校验归属（`group.go`，已被 `applySiteReq`、
`handleSortSites`、`handleBatchCreateSites` 共用）——批量导入当初就是漏了这一步。

### 设置是一整块 JSON

`model.Setting` 只有 `UserID` + `Data`（JSON 字符串）。新增设置项只需改 `model.Settings` 结构体，
不用改表；`(*Setting).Decode()` 负责给老数据补默认值。所有来自前端的设置都要过
`normalizeSettings`（`internal/api/setting.go`）——它把数值夹回区间、过滤搜索引擎条目，
并对 `background.value` **按类型分别收紧**：image 只放行 `/uploads/` 和 http(s)，
color/gradient 走 `safeCSSColor`（只允许颜色和渐变函数用得到的字符，挡掉 `url(...)`
这种会让背景去外站发请求的写法）。这个字段是直通 CSS 的，三种类型都不能漏。
前端 `src/api/types.ts` 的 `Settings` 必须与 Go 侧保持同构，**只有 `engineSeed` 是例外**
（服务端独占的记账字段，决策 013）。注意 TS 类型不删运行时数据：前端照样会把 GET 到的
`engineSeed` 原样 PUT 回来，挡住陈旧值的是 `handlePutSettings` 里那行盖章，别删它。

### 备份用「分组名」而不是分组 ID

`internal/api/backup.go` 导出的 JSON 里，卡片记录的是 `groupName`。导入时通过
`resolveGroup(tx, uid, name)`（`group.go`，与批量导入共用）按名查找或新建分组。
这样备份能导进任意账号。改动备份结构时记得 bump `BackupVersion`。

### SSRF 防护在建连层，不在参数校验层

`internal/service/fetcher.go` 的真正防线是 `http.Transport.DialContext` 里对**实际建连 IP**
的检查——这样 DNS rebinding 也躲不过。`checkHostAllowed` 的预检查只是为了给出友好错误信息，
不能当作唯一防护。任何新增的「后端按用户给的 URL 发出站请求」的功能都必须复用 `Fetcher`。

### 上传文件的三重约束

`/uploads/*` 走鉴权中间件，且路径首段必须等于当前用户 ID（`handleServeUpload`）。
上传按**文件实际内容**（`http.DetectContentType`）判类型，只收 PNG/JPG/WebP/GIF。
抓取来的 favicon 允许 SVG，靠响应头 `Content-Security-Policy: ...; sandbox` 兜底——
所以别把这个头从 `handleServeUpload` 里删掉。

### 实例级配置与登录页的图

`model.SiteConfig` 是**整站一行**（`id=1`）的 JSON 配置（站点标题 / 图标 / 登录页背景），
和每人一份的 `model.Setting` 不是一回事，也不进用户备份。只有管理员能改
（`GET/PUT /admin/site`），归一化在 `internal/api/siteconfig.go` 的 `normalizeSiteConfig`。

登录页未登录就要显示图标和背景，而 `/uploads/*` 是强制登录的，于是有
`GET /api/v1/auth/site-asset/:kind`（`icon` | `login-bg`）这个免登录端点：
它**不接受路径参数**，只吐当前配置里指向的那一个文件，所以不等于放开整个 uploads
（决策 011）。`siteIcon` 因此只收 `/uploads/{uid}/{bg|icons}/{name}` 形状的值
（`isOwnUploadPath`），外链和 `data:` 一律清空。这个端点同样带 sandbox CSP，别删。

### SPA 回落不能用 http.FileServer 服务 index.html

`http.FileServer` 和 gin 的 `FileFromFS` 会把 `/index.html` 301 重定向到目录根，
导致 `/admin` 这类深链接丢掉前端路由。`embed.go` 因此把 index.html 读进内存后用
`c.Data` 直接写出。`assets/` 下找不到的文件返回 404 而不是回落 HTML。

### CSRF 靠自定义请求头

所有非 GET/HEAD/OPTIONS 请求必须带 `X-Requested-With`，由 `middleware.CSRF()` 强制。
前端在 `src/api/http.ts` 统一加上，**绕过这层封装直接 `fetch` 写接口会 403**。

唯一的例外是 `POST /api/v1/ingest`（决策 026）：它注册在 CSRF 组**之外**，
因为 bookmarklet 在别人站点的上下文里发跨站 simple request，设不了自定义头。
它靠表单里的 ingest 令牌（库内只存 SHA-256）鉴权，不用会话 Cookie，
所以 CSRF 防护对它没有意义；响应带 `Access-Control-Allow-Origin: *` 是为了
让书签 JS 读到 `next` 做队列跳转。新增「不走会话」的端点要照这个模式想清楚再放行。

前端配套：`/ingest-bridge` 是唯一**免登录公开路由**（决策 027，路由不能加
`meta.auth`）——目标站 CSP 拦直连时书签 `window.open` 它，postMessage 交接后
由它在面板源下同源提交；它收任意源的消息，但回传结果的 targetOrigin 锁来消息
那一帧的 `event.origin`。

### 前端拖拽的数据流

`stores/panel.ts` 的 `grouped` 是只读 computed，拖拽组件写不回去。`views/Home.vue` 因此
维护一份可变副本 `boards`（元素仍是 store 里的同一批 Site 对象引用），
`persistSiteOrder(boards)` 顺带就地更新这些对象，避免列表重建时闪回旧顺序。

**交给 `VueDraggable` 的 `v-model` 必须是 `boards` 里那个数组本身**。`visibleBoards` 一旦
`.filter()` 就会产出新数组，拖拽结果只写进这个临时数组，而 `onDragEnd` 提交的是 `boards`——
表现为拖完看着对了、一刷新就打回原形，且不报任何错。所以 `visibleBoards` 在 `canDrag`
（任一编辑态——全局或分组级——且无搜索词）时**直接返回 `boards.value`**，逐组的
`boardCanDrag` 才是拖拽的 `:disabled` 依据。这个分支不是冗余，别把它化简回一律 `.filter()`。
编辑分两级：全局编辑下各组 `VueDraggable` 共用拖拽组名 `sites`（可跨组）；分组级编辑
（分组名旁的铅笔）下 `dragGroup` 按组隔离成 `sites-{id}`，拖拽天然锁在组内（决策 028）。

**`BoardGrid` 上那个 `:clone="keepRef"` 不能删**。vue-draggable-plus 的 `clone` 默认是
`JSON.parse(JSON.stringify(x))`，跨组拖动时插进目标数组的会是**副本**，于是 `boards` 里那张卡片
不再是 store 里的对象，`persistSiteOrder` 就地改的 groupId 写进了副本，下一次 `grouped`
重算就把它弹回原分组——现象是「拖第二张时第一张一起还原」，全程 200 无报错（决策 029）。
`persistSiteOrder` 也因此改成按 id 在 `sites.value` 里找对象回写，别化简回直接改传进来的那个。

### 图标全部离线，分两层供货

`@iconify/vue` 默认是运行时去 `api.iconify.design` 取图形数据的，这条路现在**被关掉了**
（`src/icons/index.ts` 里 `addAPIProvider('', { resources: [] })`）。图标改成本地两层：

- **界面自己用的**（源码里写死的 `mdi:xxx`，当前 45 个）→ `src/icons/ui-icons.ts`，
  由 `scripts/gen-icons.mjs` 扫源码生成，**是生成物但进版本库**，`main.ts` 一进来就注册。
  加了新图标名之后跑 `npm run icons`（`npm run dev` / `npm run build` 会自动跑）。
  脚本对拼错的图标名直接抛异常，别把它绕过去。
- **用户给卡片挑的**（可能是 mdi 里的任意一个）→ `loadIconSet()` 按需加载整份 mdi，
  独立 chunk，3MB。只有存在「图标库」类型的卡片时才会下载。

因此：CSP 的 `connect-src` 现在只有 `'self'`，**图标不显示不要把 iconify 那三个域加回去**，
那说明生成物没跟上或图标集没加载。只支持 `mdi:` 前缀，别的前缀会回落成文字图标。
`SiteCard` 里 `<Icon>` 上那个 `&& iconSetReady` 也不能删（原因见决策 019 的「代价」）。

### 界面文案走 i18n，语言存在设置里

`src/i18n/` 是手写的（没引 vue-i18n）：`zh.ts` 是基准词典，`en.ts` 声明成
`Record<MessageKey, string>`，**漏译一条 `npm run typecheck` 就报错**——这是这套东西
唯一的编译期保障，别把它改成 `Partial`。组件里用 `t('key', { vars })`，
且必须在**渲染期**调用才有响应性（`<script setup>` 顶层存成普通常量的话切语言不更新）。

语言存 `Settings.language`（服务端，和 theme 一路），同时镜像一份到 localStorage 供
登录页和首帧使用；`panel.loadAll` / `saveSettings` 拿到服务端值后会再盖一次。详见决策 020。

### 明暗模式：首页颜色一律走 CSS 变量

`App.vue` 把最终生效的明暗写进 `<html data-theme>`，`style.css` 据此切
`--tp-fg*` / `--tp-surface*` / `--tp-scrim`，tailwind 的 `dark:` 变体也跟着它
（`darkMode: ['selector', '[data-theme="dark"]']`，不是默认的跟系统偏好）。

首页压在用户自选的壁纸上，所以浅色模式靠**把遮罩换成白色并保底 0.62 不透明度**
来保证可读（`Home.vue` 的 `maskStyle`）。首页新写的文字用 `.tp-text*` 那几个类，
不要 `text-white`；登录页和账号管理页背景恒为深色渐变，那两页的 `text-white` 是对的——
它们用的 `.tp-glass` 和首页的 `.tp-surface-glass` 是两个类，别合并（决策 021）。

### Naive UI 组件需手动注册

`src/main.ts` 用 `create({ components: [...] })` 只注册用到的组件（全量安装会让首屏 chunk 翻倍）。
**用了新的 `n-xxx` 组件必须同步加进这份清单**，否则运行时才会报组件未找到。

## 环境相关的坑

- `build.ps1` 存的是 **UTF-8 with BOM**。Windows PowerShell 5.1 按 ANSI 读 `.ps1`，
  没 BOM 会把中文注释读成乱码并导致解析失败。用编辑工具改这个文件后要确认 BOM 还在。
- `build.bat`（`build.ps1` 的双击入口）**只能用 ASCII**，一个中文字都不能有。
  `cmd.exe` 按**控制台代码页**读 `.bat`：PowerShell 里是 65001，普通 cmd 窗口是 936。
  同一份中文在两种代码页下解出的字符数不同，cmd 又按字节偏移往下读，于是从注释里
  读出半截命令来执行——`2026-08-16` 实测过，存成 GBK 后在 65001 的终端里跑，
  满屏 `'xxx' is not recognized`。**在第一行写 `chcp` 救不了**，那时文件已经在被读了。
- 在 Git Bash 里用 `curl -d '{"name":"中文"}'` 测接口，中文参数会在传给 curl.exe 时被弄坏，
  存进库的就是乱码。这不是应用的 bug——要造含中文的测试数据，用文件（heredoc 写文件再 `-F`）
  或在浏览器里 `fetch`。

## 上公网前必须改的配置

`data/config.yaml` 里 `server.secure: true`（否则会话 Cookie 不带 Secure）、
`server.trusted_proxies: ["127.0.0.1"]`（否则登录限流会被伪造的 `X-Forwarded-For` 绕过）、
`auth.allow_register` 保持 `false`。`server.listen` 保持 `127.0.0.1:8080`，由 Caddy 对外。
