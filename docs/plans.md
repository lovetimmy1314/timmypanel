# 计划

这个文件记三件事：**打算做什么**、**明确不做什么**、以及改完之后**按什么清单验一遍**。
已经做完的事在 git 历史里，这里不重复记账。

---

## 进行中

### 首页悬浮小组件：左上角天气 + 右上角万年历

首页左上角悬浮显示当地天气温度，右上角悬浮显示万年历（带农历和节假日），
两个都能在设置里单独开关，**都只在 PC 端出现，移动端不渲染**，都要有动画。

**分两次开工**：第一次做天气（顺带把两个组件共用的悬浮层地基铺好），第二次做万年历。
第二次开工时先回来读这一节，共同约定不再重复推导。

#### 共同约定（第一次定下，第二次照抄）

- **「不在移动端显示」= 不挂载，不是 `hidden sm:block`。** 新增
  `frontend/src/composables/useIsDesktop.ts`（`matchMedia('(min-width:1024px)')`，
  监听 change），组件用 `v-if="isDesktop && enabled"`。用 CSS 藏起来的话，
  手机上组件照样挂载、照样定时器跑、照样打接口——天气那个是真出站请求，白烧。
- **悬浮定位会撞顶栏，靠让位解决。** 两个卡片都是 `position: fixed` 贴视口角
  （左上 / 右上），而首页顶栏在 `max-w-7xl` 容器里，视口 1280 左右时容器占满宽，
  卡片正好压在 logo 和右侧按钮上。所以：卡片挂到 `<body>` 级的固定层，同时给
  `Home.vue` 的 `<header>` 在 `lg` 以上按开关状态加左右 padding（CSS 变量
  `--tp-float-l` / `--tp-float-r`，由开关状态决定 0 还是卡片宽度）。
  只压缩顶栏那一行，不推整页。
- **样式外壳共用**：`style.css` 里加 `.tp-float-card`（毛玻璃 + 圆角 + 阴影 +
  入场动画），两个组件都用它，明暗色一律走 `--tp-*` 变量，不写 `text-white`（决策 021）。
- **动画统一守规矩**：所有动画包一层 `@media (prefers-reduced-motion: reduce)` 关掉。
  动画一律 CSS（transform/opacity），不引动画库。
- **设置项各自加各自的**，别在第一次就把万年历的字段一起塞进去（`Decode()` 天然
  向前兼容，第二次加不需要迁移）。加字段的四处必须同步：
  `backend/internal/model/settings.go`（结构体 + `DefaultSettings`）、
  `backend/internal/api/setting.go`（`normalizeSettings` 收紧）、
  `frontend/src/api/types.ts`（同构）、`frontend/src/stores/panel.ts`（本地默认值）。
- 新用的 mdi 图标名写完跑一次 `npm run icons`。
- 面向用户的功能 → `README.md` 和 `README.zh-CN.md` **两份都要改**。
- 取舍写进 `docs/decisions.md`：天气那次预留 **033**，万年历那次预留 **034**。

---

#### 第一次开工：左上角天气

**数据从哪儿来**：Open-Meteo（免 key、免注册、有商用友好的免费额度）。
两个上游地址：

- 当前天气：`https://api.open-meteo.com/v1/forecast?latitude=..&longitude=..`
  `&current=temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,is_day,wind_speed_10m`
  `&daily=temperature_2m_max,temperature_2m_min&timezone=auto&forecast_days=1`
- 城市搜索（设置里选城市用）：
  `https://geocoding-api.open-meteo.com/v1/search?name=<关键词>&count=8&language=zh`

**浏览器不直连上游，一律经后端代理**。三个理由，缺一都能单独立住：
CSP 的 `connect-src` 只有 `'self'`（决策 019），直连会被浏览器挡；
直连等于把每个访客的 IP 送给第三方；服务端才能做缓存，不然每开一个标签页就出站一次。

**先说风险**：后端抓取**不走代理**（见下面「已知限制」），所以部署在国内机器上时
`api.open-meteo.com` 有可能直连不通——那样天气卡就一直是 `--`。这不是能在代码里
解决的问题，功能照做，但接口失败必须是「静默显示 `--`」而不是弹窗报错，
并且设置里能整个关掉。真遇上不通，换成部署机能直连的上游是后话。

**后端**：

- `internal/service/weather.go`：新建。**必须复用 `service.Fetcher`**（CLAUDE.md 的硬约束，
  SSRF 防护挂在它的 `DialContext` 上）。Fetcher 现在只有 `Fetch`/`SaveIcon` 这类
  专用方法，需要给它加一个通用的 `GetJSON(rawURL string, max int64, v any) error`
  ——走同一个 transport 和超时，读满 `max`（256KB 足够）就断。
- 缓存：坐标量化到小数点后 1 位（约 11km）当键，内存 map + `sync.RWMutex`，
  TTL 10 分钟，条目数上限（比如 256，超了按最旧淘汰）。地理编码结果按关键词缓存 1 小时。
- 端点（都在 `authed` 组里，跟着 CSRF 组走）：
  - `GET /api/v1/weather?lat=&lon=` → `{city, tempC, feelsLikeC, code, isDay, humidity, windKph, maxC, minC, updatedAt}`
  - `GET /api/v1/weather/geocode?q=` → `{items:[{name, admin1, country, lat, lon}]}`
  - 上游挂了 / 超时：回 502 + 中文文案，**不要**把上游报文透给前端。
- 设置字段 `Settings.Weather`：
  `{enabled, locationMode: "auto"|"manual", city, lat, lon, unit: "c"|"f"}`。
  `normalizeWeatherConf`：mode/unit 走白名单，`city` 截 32 runes，
  lat 夹 [-90,90]、lon 夹 [-180,180] 并**截到 2 位小数**（~1km，够用且少留一点隐私）。
- Go 单测（`conventions.md` 要求纯函数必须有）：`normalizeWeatherConf`、坐标量化、
  上游 JSON → DTO 的解析（喂一段真实响应字面量，含 `current` 缺字段的残缺情况）。

**定位怎么来**：`locationMode`

- `manual`（默认）：设置里搜城市，存 city + lat/lon。没选过城市时卡片显示「去设置里选城市」。
- `auto`：前端 `navigator.geolocation` 拿一次坐标（需 HTTPS + 用户授权），
  结果只存 localStorage，**不入库**；拿不到就回落到 manual 的城市。

**前端**：

- `components/WeatherFloat.vue`：药丸态显示 图标 + 温度 + 城市；hover 展开体感/湿度/风/今日最高最低
  （高度过渡）。10 分钟轮询一次，`visibilitychange` 回到前台且距上次超过 10 分钟才补一次
  ——别做成一切页面切换就打接口。请求失败显示 `--` 并静默，不弹 message 刷屏。
- `components/WeatherIcon.vue`：按 WMO weather_code 分 8 类
  （晴 / 少云 / 阴 / 雾 / 毛毛雨·雨 / 雪 / 雷 / 未知）× 昼夜两态，**内联 SVG + CSS 动画**：
  太阳转 + 光晕呼吸、云横向飘、雨滴下落、雪花飘落、闪电闪。mdi 是静态图标画不了这个，
  所以这里自绘；设置面板里的入口图标仍用 mdi。
- 设置面板：`components/settings/WeatherPanel.vue`，进 `SettingsHub` 的 entries
  （图标 `mdi:weather-partly-cloudy`）。里面有：开关、定位方式、城市搜索（打 geocode 端点）、
  单位 ℃/℉。i18n 两份词典各补一份 key。

**收尾**：`decisions.md` 追加 033（为什么走后端代理、为什么不挂 CSS 隐藏、
Fetcher 加通用 GetJSON 的代价）；README 中英两份各加一句功能说明；
把下面这几条搬进 `plans.md` 末尾的验证清单；提交。

**这次要验的**：

| 检查 | 期望 |
|---|---|
| 浏览器窗口拖窄到 <1024px | 天气卡消失，且网络面板里不再有 `/weather` 请求（不是只藏起来） |
| 关掉设置里的天气开关 | 卡片消失，顶栏 padding 跟着还原，不留一块空白 |
| 网络面板过滤 `open-meteo` | 一条都没有（浏览器不直连上游） |
| 未登录打 `/api/v1/weather` | 401 |
| 同一坐标连打 5 次 `/api/v1/weather` | 只有第一次出站（后面走缓存），响应 `updatedAt` 不变 |
| 断网 / 上游超时 | 接口 502 中文文案，卡片显示 `--`，控制台不刷错 |
| 设置里把 lat 填成 999 | 被夹回 90 |
| 系统开「减弱动态效果」 | 图标和入场动画停掉，内容照常显示 |

---

#### 第二次开工：右上角万年历

**农历算在后端，不在前端。** 理由：农历/节气转换正是 `conventions.md` 里
「解析类纯函数必须有 Go 单测」点名的那类东西，放后端才有测试兜着；节假日表跟着
二进制发版更新，不用重编前端；前端 chunk 一个字节不涨（`lunar-javascript` 那种库
体积不小，不值当为一个角标引进来）。

- 端点：`GET /api/v1/calendar/month?y=2026&m=8`（authed），返回整月每天一条：
  `{date, lunarDay, lunarMonth, festival, solarTerm, dayType: ""|"off"|"work"}`。
  **年月由前端按本地时区算好再传**，后端只做纯计算，不读 `time.Now()` 的时区
  ——服务器 TZ 通常是 UTC，靠服务端判断「今天」必错一天。
- `internal/service/lunar.go`：1900–2100 的农历压缩表（每年一个 int32，闰月 + 大小月），
  公历 ↔ 农历转换、24 节气（用通行的近似算法 + 世纪修正）、干支/生肖。
  Go 单测必须覆盖：几个已知对照日（2026 春节、含闰月的年份、月末跨月）、
  节气日期与公开数据比对、边界年 1900/2100 不 panic。
- 节日分三类：农历节日（春节/除夕/元宵/端午/七夕/中秋/重阳/腊八/小年）、
  公历节日（元旦/妇女/劳动/青年/儿童/建党/建军/教师/国庆/圣诞…）、24 节气。
- **法定调休（哪天休、哪天补班）单独一张年度表**（`holidays.go` 里按年写死，
  数据来自国务院当年通知）。**没有该年数据时不显示班/休角标，只显示节日名**
  ——宁可少显示，也不能拿去年的表猜今年。这条要写进「已知限制」。
- **农历、节气、节日名一律中文，不进 i18n。** 它们是中文历法专名，没有通行英译，
  硬翻出来的英文没人看得懂。英文界面下卡片的框架文案（「今天」「上个月」）走 i18n，
  农历那行保持中文。这个取舍写进决策 034。

**前端**：

- `components/CalendarFloat.vue`：药丸态显示 `8月24日 周一 · 七月初二`（有节日就显示节日名）；
  点击展开当月面板——今天高亮、每格公历大字 + 农历小字、节日/节气标色、
  休/班角标、上下月切换、「回到今天」。展开用 scale+fade 过渡，翻月用左右滑动过渡。
- 数据缓存在组件里按 `y-m` 存，翻过的月份不重复请求；跨零点自动刷新当天。
- 设置字段 `Settings.Calendar`：`{enabled, weekStart: "mon"|"sun"}`，
  面板 `components/settings/CalendarPanel.vue`（图标 `mdi:calendar-month-outline`）。

**收尾**：`decisions.md` 追加 034；README 两份；验证清单；提交。

**这次要验的**：

| 检查 | 期望 |
|---|---|
| 窗口 <1024px | 万年历卡消失，且不再打 `/calendar/month` |
| 卡片显示的农历 | 与手机系统日历同一天一致（春节、闰月年各抽查一天） |
| 把系统时区改成 UTC+0 再看 | 「今天」仍是本地日期（月份是前端算好传上去的） |
| 翻到没有调休数据的年份 | 节日名照常显示，班/休角标一个不出现（不是显示错的） |
| 界面切成 English | 框架文案是英文，农历那行仍是中文 |
| 连点上/下月十几次 | 每个月只打一次接口 |
| 同时开天气和万年历 | 顶栏左右各让出一块，logo 和右侧按钮都没被盖住 |

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
| 登录时给存在和不存在的用户名各发几次错密码，掐 `%{time_total}` | 两者耗时同一量级（都是一次 bcrypt，约 0.2s）。差两个数量级说明短路又回来了，决策 031 |
| 给 `/api/v1/ingest` 发 5MB body | 413，且**不**消耗令牌限流次数；正常的 urlencoded 和 multipart 两种发法仍 200 |
| `curl /assets/`、`/assets`、`/icons/` | 全 404，body 只有 gin 默认那句。出现 `<a href=...>` 列表就是目录又被当资源供货了 |
| 同上之后再访问 `/admin`、`/`、带哈希的真实资源 | 依次 200 / 200 / 200，深链接和长缓存没被误伤 |
| 文字图标填 30 个中文、书签目录名填超长中文 | 存回来是合法 UTF-8，没有 `U+FFFD` |
| 连着覆盖导入 N 次（N > `backup.keep`） | `data/backups` 里 `-before-import-` 只剩 keep 份，且 `-auto-` 那批一份没少 |
| `TP_SECURE=yes` 启动 | 日志有一条 WARN，配置文件里的值原样保留（**不是**静默变 false） |
| 管理页用户列表 | 卡片/分组数正确，且一个记录都没有的新账号显示 0 而不是漏行 |

提交前额外确认一条（不限于上面那几类改动）：

```bash
git diff --stat backend/internal/web/dist/index.html
```

有输出就说明提交里混进了构建产物，`git checkout --` 还原掉（决策 014）。
