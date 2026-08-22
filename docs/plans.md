# 计划

这个文件记三件事：**打算做什么**、**明确不做什么**、以及改完之后**按什么清单验一遍**。
已经做完的事在 git 历史里，这里不重复记账。

---

## 进行中

### 2026-08-22 全量代码审查的修复

一轮通读后攒下的问题，按「安全 → 正确性 → 健壮性 → 优化」四批做，每批一个提交。
带「实测」的三条是起了实例复现过的，改完按同样的方式回验。

**第一批 · 安全**

- [ ] **登录耗时泄露账号是否存在**（`api/auth.go`）。`err != nil || bcrypt.Compare(...)`
      短路，用户不存在时 bcrypt 根本不跑。**实测**：`admin` + 错密码 0.20s，
      `nosuchuser` + 错密码 0.0007s，差 200 倍。文案统一了但时间没有。
      改：查不到用户时也对一个固定假哈希跑一次 bcrypt。
- [ ] **`/api/v1/ingest` 在鉴权前解析整个 multipart**（`api/ingest.go`）。
      `c.PostForm("token")` 触发 `ParseMultipartForm(16MB)`，而这是全站唯一
      不需要会话的写端点，body 没有任何上限。改：这条路由套 `http.MaxBytesReader`。
- [ ] **备份 zip 最坏 4GB 常驻内存**（`api/backup.go`）。`maxBackupAssets`(500) ×
      `maxBackupAssetBytes`(8MB) 全攒在内存里。改：加**累计**字节预算。

**第二批 · 正确性**

- [ ] **`/assets/` 吐出构建产物清单**（`web/embed.go`）。`sub.Open("assets")` 对目录
      也成功，于是落进 `http.FileServer` 生成目录列表。**实测**：`/assets/` 返回
      404 状态 + 完整 `<a href=...>` 列表（状态码是 gin NoRoute 盖的）。
      改：`Stat()` 出来是目录就按 404 走，和「assets 下找不到就是真 404」对齐。
- [ ] **按字节截断把中文切碎**（`api/site.go` 的 `sanitizeIconValue`、
      `service/bookmark.go` 的 `sanitizeFolder`）。**实测**：30 个「文」截到 64 字节，
      末尾 `e6 96 87 e6`，`utf8.ValidString` = false，序列化后是 `�`。
      同一个坑 `sanitizeFooterHTML` 和 `normalizeSiteConfig` 都躲过了，这两处漏了。
      改：按 rune 截，各补一条单测。
- [ ] **`before-import` 快照永不清理**（`api/backup.go` 的 `pruneBackups`）。
      只认文件名里的 `-auto-`。导入是普通用户能触发的，磁盘会无限涨。

**第三批 · 健壮性**

- [ ] **后台 goroutine 里 panic 整个进程挂掉**（`service/fetcher.go` 的
      `ForEachLimited`、`api/site.go` 的 `go s.backfillIcons`）。`gin.Recovery()`
      覆盖不到请求 goroutine 之外。改：worker 自己 recover 并记日志。
- [ ] **`TP_SECURE` 写错值静默关掉 Secure Cookie**（`config/config.go`）。
      `strconv.ParseBool` 的 error 被丢掉，`TP_SECURE=yes` → `false`。
      这是公网部署明确要求打开的一项，配错却毫无提示。改：解析失败保留原值 + `slog.Warn`。

**第四批 · 优化**

- [ ] **管理员用户列表是 N+1**（`api/admin.go`）。每个用户两次 `Count`，
      改成两条 `GROUP BY user_id`。
- [ ] **`sniffIconExt` 的文档注释错位到了 `SniffStoredImage` 头上**（`service/fetcher.go`）。
      中间少一个空行，godoc 里显示的是错的。
- [ ] **`normalizeSettings` 不限制搜索引擎条数和字段长度**（`api/setting.go`）。
      还有 `SearchEngine.Icon` 从头到尾没消毒也没被前端用到（`SearchBar` 只渲染星标）。
- [ ] **上传失败留孤儿文件**（`api/upload.go`）。`io.Copy` 或 `db.Create` 失败时，
      `os.Create` 出来的文件不会被删。

**这次不做，及理由**

- `ingestQueues.list` 只有测试在调用。它是 `set`/`popCanonical` 之外唯一的只读入口，
  删了测试就得去摸 map 内部，那更糟。留着。
- 登录限流仍是「5 次/15 分钟/IP」。上面第一条只堵住耗时旁路，换 IP 慢速枚举依旧可行；
  真要堵死得上验证码或全局限流，不值当，见决策 031。

## 待办

- **可选：TOTP 两步验证**（`pquerna/otp`，约半天）。公网部署，值得做。
  `config.auth.secret` 已经在生成和落盘了，就是留给它的——当前没有任何代码读它。
- **可选：图标集不止 mdi**。当前只打包了 mdi（决策 019），卡片图标填别的前缀会回落成文字图标。

## 明确不做

- **iframe 内嵌网页**：主流站点都发 `X-Frame-Options: DENY`，能嵌的是少数，做了也大半是空框。
- **站点在线状态监控**：定时探测会引出后台任务、超时和误报一堆问题，一个导航页不值当。

## 已知限制

**后端抓取不走代理。** `NewFetcher` 自建 `http.Transport` 时没设 `Proxy` 字段
（Go 语义：nil = 不用代理），所以 `HTTP_PROXY` 这类环境变量对它无效。
后果是「浏览器能打开、后端抓不到」——被墙的站点在国内机器上部署时就是这样。
按「部署在能直连目标站点的机器上」来解决；真要加代理支持，注意它和决策 003 的冲突
（SSRF 防护是挂在 `DialContext` 上的，走了代理就等于把选址权交给代理，见决策 016 的「代价」）。

---

## 验证清单

改动涉及鉴权、用户隔离、上传、抓取时，按这份跑一遍。

```bash
cd backend && gofmt -l . && go vet ./... && go test ./...
```

```bash
cd frontend && npm run typecheck
```

打包后跑二进制，逐条确认：

| 检查 | 期望 |
|---|---|
| 未登录访问 `/api/v1/sites` | 401 |
| 写请求缺 `X-Requested-With` | 403 |
| 跨账号读写他人卡片 / 上传文件 | 404 |
| 连续 5 次密码错误 | 429，锁 15 分钟 |
| `/sites/parse` 传 `127.0.0.1`、`192.168.x.x`、`169.254.169.254` | 全部被拒 |
| 导出 JSON → 覆盖导入 → 再合并导入 | 数据一致，第二次全部跳过 |
| `curl -o /dev/null -w "%{http_code}" .../admin` | 200（SPA 深链接） |
| 批量导入传别人的 `groupId` | 400「分组不存在」 |
| 建号用 `../../evil` 之类的用户名 | 400 |
| 设置里把 `background.value` 填成 `url(https://…)` | 被回落成默认渐变 |
| 拿别人的上传 ID 打 `DELETE /uploads/:id` | 404，文件还在 |
| 非管理员打 `GET/PUT /admin/site` | 403 |
| 未配置图标时访问 `/auth/site-asset/icon` | 404；配置后未登录也能取到，且响应头带 `sandbox` CSP |
| `siteIcon` 填外链或 `data:` | 被清空（只收 `/uploads/{uid}/{bg\|icons}/…`） |
| 浏览器控制台有没有 CSP 报错、图标是否都在 | 无报错、图标正常（图标集已内置，报错说明生成物没跟上） |
| 网络面板过滤 `iconify\|simplesvg\|unisvg` | **一条请求都不该有**（决策 019） |
| 设置里把语言切成 English 再刷新 | 界面仍是英文（存在服务端，不只是 localStorage） |
| 顶栏点明暗开关再刷新 | 主题保持，且首页文字/卡片在浅色下读得清 |
| 给指向 `127.0.0.1` 的卡片打 `/sites/backfill` | 逐条报「拒绝访问内网地址」，其余卡片照常补 |
| 拿别人的卡片 ID 打 `/sites/backfill` | 该条报「卡片不存在」，不改动任何数据 |
| 登录填错密码 | 401「用户名或密码错误」，**不**跳转、不丢草稿 |
| 改密填错原密码 | 401「原密码不正确」，仍留在账号对话框 |
| 未勾「记住我」登录后改密 | Set-Cookie 无 Max-Age（仍是会话 Cookie） |
| `iconBg` 填 `url(https://…)` 或带分号 | 入库为空，卡片用标题哈希色 |
| 覆盖导入时把 `data/backups` 弄成只读 | 500「导入前备份失败」，库里数据还在 |
| 导出 ZIP → 导进另一个账号 | 图片落到新 uid 目录，卡片图标能打开 |
| `POST /api/v1/ingest` 不带 `X-Requested-With` | 正常受理（它在 CSRF 组外，决策 026）；其余写接口仍 403 |
| ingest 用错令牌 / 已吊销令牌 | 401，同 IP 连续 10 次后 429 |
| 同 URL 二次 ingest（协议/www/斜杠不同） | 走更新不新建，只补空标题和图标 |
| `PUT /ingest/queue` 后逐个 ingest | 响应 `next` 依次给出队列里下一个，`remaining` 递减 |

提交前额外确认一条（不限于上面那几类改动）：

```bash
git diff --stat backend/internal/web/dist/index.html
```

有输出就说明提交里混进了构建产物，`git checkout --` 还原掉（决策 014）。
