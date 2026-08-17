# 编码约定

只记「这个项目跟别处不一样」的部分。通用最佳实践不写在这里。

## 语言

注释、日志、**后端**错误文案一律**中文**；标识符和文件名一律 **ASCII**。

**界面文案走 i18n**（决策 020）：新增的每一句都要在 `frontend/src/i18n/zh.ts`
和 `en.ts` 各写一条，组件里用 `t('key')`。`zh.ts` 是基准词典，`en.ts` 漏一条
`vue-tsc` 就报错；反过来 `zh.ts` 漏了没人管，界面上会显示 key 本身。
`t()` 要在**渲染期**调用才有响应性——`<script setup>` 顶层存进普通常量的话，
切语言不会更新，包 `computed` 或直接写在模板里。
后端返回的错误文案仍是中文（没做服务端 i18n）。
提交信息用中文，格式见下面的 Git 一节。
注释解释**为什么**，不复述代码。防御性代码必须写清防的是什么——不写清楚，
下一个读到它的人会当成冗余删掉，而这类代码删了往往编译和测试都照过。

## 后端（Go）

**错误响应**走 `internal/api/server.go` 的助手，不要自己拼 `c.JSON`：
`badRequest` / `notFound` / `serverError` / `fail(c, code, msg)` / `ok(c, data)`。

**响应形状**：列表 `{"items": [...]}`｜单条直接返回对象｜纯操作 `{"ok": true}`｜
出错 `{"error": "中文说明"}`｜批量 `{"created": n, "skipped": n, "invalid": n}`。

**用户数据查询**一律从 `s.scoped(c)` 起手。接受外部传入的 `groupId` / 资源 ID 时，
先确认它属于当前用户（参考 `applySiteReq`、`handleSortSites`）。

**对外暴露**用单独的 DTO（如 `toUserDTO`），不要直接返回 model——
靠 `json:"-"` 挡敏感字段太脆，新增字段容易漏。

**批量写入**放进 `db.Transaction`，别循环单条提交（排序、批量导入、备份导入都是这么做的）。

**数据库变更**靠 `AutoMigrate`，不写迁移脚本：只加字段、不改字段语义；
删字段前确认老数据和备份文件的兼容（备份有 `BackupVersion` 可以卡）。

**出站 HTTP** 只能通过 `service.Fetcher`，不要新起 `http.Client`——SSRF 防护挂在它上面。

**并发**：SQLite 开了 WAL + busy_timeout，后台 goroutine 直接用同一个 `*gorm.DB`，
不要另开连接池。

## 前端（Vue 3 + TS）

- 组件一律 `<script setup lang="ts">`。
- **所有请求走 `src/api/http.ts`**。绕过它直接 `fetch` 会因缺 `X-Requested-With` 被挡成 403。
- 类型集中在 `src/api/types.ts`，必须与 Go 侧的 JSON tag 同构，改一边记得改另一边。
- 用到新的 `n-xxx` 组件要加进 `src/main.ts` 的注册清单，否则运行时才报错。
- 跨组件共享的状态才放 `stores/`；只有一个组件用的别为了「统一」提上去。
- **首页上的文字不要写 `text-white`**，用 `.tp-text` / `.tp-text-soft` /
  `.tp-text-dim` / `.tp-text-faint`（浅色模式下白字看不见，决策 021）。
  登录页和账号管理页是例外——那两页背景恒为深色渐变，`text-white` 是对的。
- **图标只能用 `mdi:` 前缀**（决策 019）。新用一个图标名之后跑一次
  `npm run icons` 更新 `src/icons/ui-icons.ts`（`npm run dev` / `build` 会自动跑）。

## 测试

- **纯函数必须有 Go 单测**：解析、归一化、安全判定这三类。当前已覆盖
  `normalizeURL`、`canonicalURL`、`sanitizeIconValue`、`sanitizeIconBg`、`ParseBookmarks`、`isBlockedIP`、
  `ValidUsername`、`ValidPassword`、`safeFileSegment`、`safeCSSColor`、`normalizeSettings`（logoText 截断）、
  `sanitizeFooterHTML`、`isOwnUploadPath`、`normalizeSiteConfig`、`uploadKind`、
  `uploadDiskPath`、`serveUploadRel`、`zipUploadRel`、`remapOwnUploadPath`、
   `(*Setting).Decode`、`safeDial` 的选址判定、`ResetPassword`、
   `newIngestToken`/`ingestQueues`、`SaveIconData`。这类逻辑出错最隐蔽、改动最频繁。
  新写一个这样的函数就顺手补一条用例，别攒着。
- HTTP 层没有自动化测试，靠手工回归，清单在 `plans.md` 末尾。
- 造含中文的测试数据**不要**用 `curl -d '{"name":"中文"}'`——Git Bash 会弄坏参数，
  存进库就是乱码。用 heredoc 写文件再 `-F`，或在浏览器里 `fetch`。

## Git

**每完成一个逻辑改动就提交一次，不要攒着。**
一个逻辑改动 = 一件能用一句话说清、且此刻代码是自洽可跑的事。
「加一个接口 + 它的前端调用 + 它的测试」是一个提交；
「顺手改了另一个模块的 bug」拆成另一个提交。

提交前**必须**先跑完下面的检查，别把跑不过的代码提交进去：

```bash
cd backend && gofmt -l . && go vet ./... && go test ./...
```

```bash
cd frontend && npm run typecheck
```

`npm run build` 不做类型检查，别拿它当验证。

**提交信息**用中文，只有 `type` 前缀保持 ASCII。

```
<type>: <一句话说清做了什么>

<可选正文：为什么这么改，取舍是什么>

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
```

`type` 取 `feat` / `fix` / `refactor` / `docs` / `chore` / `test` / `build`。
写**做了什么**，不写「update code」这种等于没说的话。改动背后的取舍如果值得留存，
正文里指一下 `docs/decisions.md` 的条目号，别把整段理由塞进提交信息。

维护者日常直接提交到 `main`，不开分支（除非在试一个可能推倒重来的方案）。
外部贡献走分支 + PR，PR 前把上面那两条检查跑过。

会被忽略的：`data/`（含配置里的密钥和 sqlite）、根目录 `dist/`（二进制）、
`node_modules/`、`backend/internal/web/dist/assets/`、
`backend/internal/web/dist/icons/` 和 `dist/manifest.json`
（`frontend/public/` 的构建拷贝，决策 025）。

`backend/internal/web/dist/index.html` **是跟踪的**（embed 要求它常在），
但**版本库里那一份必须是占位页，不是构建产物**——`npm run build` 会把它改成
带哈希的真页面，提交前用 `git checkout -- backend/internal/web/dist/index.html`
还原回去。理由见决策 014：`assets/` 不进版本库，提交了真页面就等于提交了一份
指向不存在文件的 HTML。`git status` 里看到这个文件是脏的，**默认答案是还原而不是提交**。
