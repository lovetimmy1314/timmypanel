# 决策记录

只记「为什么这么选」和「代价是什么」——代码能回答的不写在这里。
新增往后追加，编号递增。推翻旧决策**不要删**：把状态改成「已废弃（被 00X 取代）」，再补新的一条。

## 029 跨组拖拽关掉 vue-draggable-plus 的默认深拷贝

状态：生效。相关：`frontend/src/components/BoardGrid.vue`、`frontend/src/stores/panel.ts`、
`frontend/src/views/Home.vue`。

`vue-draggable-plus@0.6.1` 的 `clone` 选项默认值是 `JSON.parse(JSON.stringify(x))`。同组内排序
走 `onUpdate`，只是数组内换位置，不经过它；**跨组**走 `onStart` + `onAdd`，onStart 会先按这个
函数把被拖的元素拷一份挂到 DOM 节点上，onAdd 再把**这份副本**插进目标分组的数组。

于是跨组拖完之后，`boards` 里落在新分组的那张卡片不再是 `panel.sites` 里的那个对象。
`persistSiteOrder` 原来是就地改 `s.groupId`/`s.sort`，改的是副本，store 里那条记录的 `groupId`
还是旧分组。请求本身是对的（带的是 id），后端也写进去了，但只要有任何一次
`panel.grouped` 重算（拖第二张时必然发生：这次挪动会改到真实对象的 `sort`），
`boards` 就被从 store 重建一遍，第一次跨组拖的结果被打回原处。

表现为「第一张拖过去看着好了，拖第二张时两张一起弹回原分组」，且不报任何错——
接口全是 200，刷新之后顺序反而是对的，所以很容易误判成后端问题。

选择：`BoardGrid` 上显式传 `:clone="keepRef"`（恒等函数），让跨组插进去的就是原对象，
恢复「boards 里的 Site 是 store 里的同一批对象引用」这个全项目都在依赖的前提。
同时 `persistSiteOrder` 改成按 id 在 `sites.value` 里找对象再回写，不再靠引用相等。

代价：

- `clone` 恒等之后就不能再用 Sortable 的 `pull: 'clone'`（拖出去留一份），
  真要做「复制到另一组」得自己在 `onAdd` 里造对象。目前没这个需求。
- 两道防护有重叠：`keepRef` 保住引用，按 id 回写又不依赖引用。留着重叠是故意的——
  升级拖拽库时 `clone` 的默认行为可能再变，按 id 回写是兜底的那一道。
- 传给拖拽组件的 `clone` **必须是稳定引用**。写成模板内联箭头的话，每次渲染都是新函数，
  库里那个 `deep: true` 的 options watcher 会跟着反复触发。

## 030 update.sh 只管换镜像重启，不备份也不回滚

状态：生效。相关：`deploy/update.sh`、`docker-compose.yml`、`README.md`、`README.zh-CN.md`。

升级动作原本散在 README 里：compose 那条路是两条命令还好，`docker run` 那条路要
「pull → 删容器 → 把那条带 5 个参数的 run 原样再敲一遍」，而漏掉 `TP_SECURE` 或
`TP_TRUSTED_PROXIES` 都是**静默失效**（Cookie 少个 Secure 标记、限流能被伪造的
`X-Forwarded-For` 绕过），手敲重放迟早会漏。所以收成一个脚本。

三个刻意的取舍：

- **不备份卷、不探活、不自动回滚。** 数据在命名卷里，换镜像根本不碰它；真要备份，
  README 有那条 tar 命令，应用自己也每天留快照。加上备份+回滚会把脚本推到 120 行，
  而它多出来的那些失败模式（备份盘满、tag 回滚把 `latest` 指歪）自己也要人来收拾。
  代价：升级后新版本起不来时，要人自己 `TP_TAG=旧版本 ./update.sh` 回去。
- **重建容器用 `docker stop -t 30` + `docker rm`，不用 `docker rm -f`。** `rm -f` 直接发
  SIGKILL，`main.go` 的 Shutdown 那 10 秒 drain 和 sqlite 的收尾都跑不到。README 里原来
  写的就是 `rm -f`，一并改掉。30 这个值与 compose 的 `stop_grace_period` 对齐。
- **清理旧镜像限定本仓库**（`docker images $REPO --filter dangling=true`），不用
  `docker image prune -f`——宿主机上通常不止跑这一个东西，一个升级脚本没资格把别人的
  悬空镜像一起删了。代价：多层构建留下的中间层清不掉，那是 `prune` 的活，让人自己决定。

脚本必须**自包含**：VPS 上没有仓库（README 的快速开始只 `curl` 了一个 `docker-compose.yml`），
所以它不能引用 `deploy/` 下的其他文件，`docker run` 那条路的参数只能在脚本里再写一份——
改 README 的 run 命令时记得同步改脚本，反之亦然。

## 031 鉴权之前的成本和耗时，一律钉死

状态：生效。相关：`backend/internal/api/auth.go`、`backend/internal/api/ingest.go`、
`backend/internal/api/backup.go`。

三处问题同一个形状：**在确认调用方是谁之前，服务器就已经付出了可观测的代价**——
要么是时间（能被拿来问问题），要么是内存（能被拿来打垮进程）。

**登录耗时对齐。** `handleLogin` 原来写的是
`if err != nil || bcrypt.CompareHashAndPassword(...) != nil`。Go 的 `||` 短路，
用户查不到时 bcrypt 根本不跑。实测：`admin` + 错密码 0.20s，`nosuchuser` + 错密码
0.0007s，差 200 倍。错误文案统一成「用户名或密码错误」这件事因此完全白做——
不用看 body，掐表就知道哪些用户名存在。改成先算 `passwordOK` 再判断，查不到用户时
拿 `dummyPasswordHash`（进程启动时用当前 `BcryptCost` 现算的一串随机密码哈希）走一遍。
修完实测两条路径都是 0.199s。

- 代价一：**每次登录尝试都必然烧一次 bcrypt**，包括纯粹瞎猜用户名的。cost 12 约 200ms
  CPU，这让登录接口成了更贵的 DoS 面。靠现有的 `LoginLimiter`（5 次/15 分钟，IP 和
  用户名各限一路）压住，没有再加东西。
- 代价二：`dummyPasswordHash` 在包初始化时算，二进制启动多花约 200ms。选现算而不是
  写死一串常量，是为了 `BcryptCost` 以后调了假比对能跟着走——写死就会重新裂开。
- **没有解决的**：换 IP 慢速枚举依然可行（每个 IP 5 次）。真堵死要上验证码或全局限流，
  一个自用导航站不值当。这一条只是把「零成本、无限次」的旁路降回和暴力破解同一档。

**ingest 端点的请求体上限。** `POST /api/v1/ingest` 是全站唯一不走会话的写端点
（决策 026），而 `c.PostForm("token")` 那一下就会触发
`ParseMultipartForm(MaxMultipartMemory)`——`main.go` 里设的是 16MB。也就是说
**令牌校验之前** body 已经被读进内存了，无令牌的请求照样能让每条连接吃掉 16MB。
现在在限流之后、碰表单之前套 `http.MaxBytesReader(2MB)`，并显式
`ParseMultipartForm` 一次把错误捞出来（gin 的 `c.PostForm` 会把它吞掉）。

- 2MB = 图标上限 1MB 的一倍，余量给表单字段和 multipart 分隔符。
- 必须放在限流**之后**：锁定中的 IP 连解析都不该走到。
- 书签有两种发法，urlencoded（只带 iconUrl）会让 `ParseMultipartForm` 回
  `ErrNotMultipart`，但表单在那之前已经由 `ParseForm` 解析好了，所以这个错误要放行。
  两条路径都实测过：200；5MB 的 body 回 413。
- 代价：图标超过 1MB 的站点，书签这条路会直接 413 而不是像以前那样静默丢图标。
  文案里写清了 1MB 这个数。

**备份 zip 的累计解压上限。** `readBackupZip` 原来只卡单张 8MB 和总张数 500，
但 assets 是全攒在内存里等着还原的，500 × 8MB = 4GB——一个还没到 64MB 上传上限的
高压缩比 zip 就能把内存打爆。加 `maxBackupAssetTotal`(64MB) 按**实际读到的字节数**
累计，超了直接报错。按实际读到的记，不按 `UncompressedSize64`：那是 zip 自己声明的，
可以随便写。

- 代价：图片总量超过 64MB 的备份包导不进来，得先删点图。这个量级的个人导航站不存在。

## 032 多搜索框只是「一份共用配置 + 若干个默认源」

相关：`internal/model/settings.go`、`internal/api/setting.go`、
`frontend/src/views/Home.vue`、`frontend/src/components/SearchBar.vue`、
`frontend/src/components/settings/SearchPanel.vue`

首页可以放不止一个搜索框（想同时挂着「站内 + Google + 磁力」这种），
数据结构上**没有**把搜索框做成一等对象：`SearchConf` 里加的是
`Bars []SearchBar`，而 `SearchBar` 只有 `enabled` 和 `default` 两个字段。
引擎清单（`engines`）和配色（`style`）仍然整站一份，所有搜索框共用。
主搜索框继续用 `SearchConf.Enabled` / `SearchConf.Default` 表达，不进 `bars`
——老数据（没有 `bars` 键）因此天然就是「一个搜索框」，不需要迁移。

- 代价一：**没法给某个搜索框单独配色或单独裁引擎清单**。真要做，得把 style 和
  engines 也搬进 `SearchBar`，那时候 `SearchConf` 顶层那几个字段就成了历史包袱。
  按当前需求（多开几个框、各自默认一个源）不值当。
- 代价二：主搜索框和附加搜索框是两套字段，凡是要「遍历所有搜索框」的地方都得手工拼
  （`Home.vue` 的 `searchBars` computed 就是干这个的，主搜索框记 `barIndex = -1`）。
  点星星设默认引擎时按这个下标回写，`-1` 写 `search.default`，其余写 `bars[i].default`。

**全局快捷键只归第一个搜索框。** `/` 和 `Ctrl+K` 原来由 `SearchBar` 自己
`window.addEventListener` 认领，多开几个之后每个都会 focus 自己，最后谁赢全看注册顺序
（表现为按 `/` 光标跳到最下面那个框）。所以加了 `hotkeys` prop，只有 `searchBars`
里的第一个拿到 true；占位文案也跟着分两条，没有快捷键的框不提示「按 / 聚焦」。

**站内搜索的关键词仍然只有一份。** 所有框的 `update:query` 都写进 `Home.vue` 的同一个
`query`，也就是「最后敲字的那个框」决定卡片过滤。`SearchBar` 的两个 watch 都不是
immediate，没人碰的框不会发事件，所以不会互相清空。多个框同时选「站内搜索」时，
下面的卡片列表跟着最后动的那个走——这是刻意的，卡片列表只有一个，没有第二种解释。


## 033 天气组件：数据走后端代理，组件在移动端不挂载

状态：生效。相关：`backend/internal/service/weather.go`、`backend/internal/api/weather.go`、
`backend/internal/service/fetcher.go`（`GetJSON`）、`frontend/src/components/WeatherFloat.vue`、
`frontend/src/composables/useIsDesktop.ts`、`frontend/src/style.css`（`.tp-float*`）。

首页左上角悬浮天气。四个有取舍的选择：

**上游是 Open-Meteo，但浏览器不直连，一律经 `GET /api/v1/weather` 代理。**
三个理由各自都能立住：CSP 的 `connect-src` 只有 `'self'`（决策 019），直连当场被浏览器
挡掉；直连等于把每个访客的 IP 送给第三方；只有服务端这一层才谈得上缓存——否则每开一个
标签页、每次刷新都是一次出站。缓存按坐标量化做键，TTL 10 分钟，真正发给上游的也是量化后的坐标。
量化精度最初是 0.1°（约 11km），后由 035 改为 0.01°（约 1km）。

- 代价一：**后端抓取不走代理**（`plans.md` 的「已知限制」），所以部署在连不上
  `api.open-meteo.com` 的机器上时，天气卡永远是 `--`。因此接口失败必须静默：
  卡片显示 `--`，不弹 message——10 分钟一轮的轮询弹起来就没完了。
- 代价二：`Fetcher` 多了个通用的 `GetJSON`，语义从「抓站点元信息」扩成了「受管控的
  出站 HTTP 客户端」。这是刻意的：CLAUDE.md 要求所有出站都复用它，另起
  `http.Client` 就把 SSRF 那道 `DialContext` 绕过去了。
- 代价三：地点搜索的关键词是用户随手敲的，等于一个「登录后可用的出站放大器」。
  加了每人每小时 60 次的**出站**配额（命中缓存不计），没有再上更重的限流。

**「移动端不显示」= 不挂载（`useIsDesktop` + `v-if`），不是 `hidden lg:block`。**
CSS 藏起来的话组件照样挂载、定时器照样跑、接口照样打，手机上纯属白烧。
代价：拖动窗口跨过 1024px 断点会把组件销毁重建，重建时会再打一次 `/weather`
——服务端缓存挡着，不会真的出站。

**悬浮卡压在顶栏上，靠顶栏让位而不是挪卡片。** 首页顶栏在 `max-w-7xl` 容器里，
视口 1280 左右容器就占满宽了，左上角正好是 logo 的位置。所以卡片固定贴视口角，
顶栏那一行在 `lg` 以上加 `padding-left: 148px`（`.tp-header-gap-l`）。
代价：顶栏可用宽度少了一截；换成「等视口够宽才显示」的话，1440 这类常见分辨率
就看不到这个功能了，更亏。

**天气图标自绘 SVG，不用 mdi。** mdi 是一条静态路径，转不起来也下不了雨，
而这个组件的动画是需求本身。按 WMO code 收成 8 类，昼夜两态。
代价：多一份手写 SVG 要维护，且它不在 `npm run icons` 的生成物里——加天气类型时
要自己改这个组件。设置面板里的入口图标仍然是 mdi。

**默认开着但没有城市。** 这时卡片显示一个「选择城市」的入口，不发请求，也不弹
浏览器定位授权。默认关掉的话，这个功能等于藏起来了，没人会知道它在。


## 034 万年历：农历算在后端，历法专名不进 i18n

状态：生效。相关：`backend/internal/service/lunar.go`、`backend/internal/service/calendar.go`、
`backend/internal/service/holidays.go`、`backend/internal/api/calendar.go`、
`frontend/src/components/CalendarFloat.vue`、`frontend/src/style.css`（`.tp-cal*`）。

首页右上角悬浮万年历，桌面端限定（不挂载的做法同决策 033）。四个有取舍的选择：

**农历、节气、节日、调休一律算在后端。** 前端只排版。三个理由：这类转换正是
`conventions.md` 点名「必须有 Go 单测」的纯函数，放后端才有测试兜着；节假日表
跟着二进制发版更新，不用重编前端；`lunar-javascript` 那类库体积不小，为一个角标
把它打进 chunk 不值当（现在前端一个字节没涨）。

- 代价：多了一个接口。它是纯计算，不出站、不碰用户数据，但仍挂在 authed 组下
  ——这是登录后才有的界面功能，没理由白送给未登录的人算力。

**年月由前端按本地时区算好再传，服务端不读 `time.Now()`。** 服务器 TZ 通常是 UTC，
「今天是几号」在东八区必错一天。整个 `service` 侧的日期运算都用 `time.UTC` 做**民用日历
算术**（只有年月日参与），节气则显式换算到东八区定日——农历是中国历法，
用服务器本地时区或 UTC 判归属都会在跨零点时错一天。

**节气用太阳视黄经的低精度公式（Meeus 第 25 章）算，不用逐年偏移量表。**
误差约 0.01°，折算成时间约一刻钟，只有节气正好落在午夜前后 15 分钟内才可能判错日期；
通行的「每年一个固定分钟偏移」那套线性近似在世纪两端会整天地偏。

- 代价：多了 ΔT 分段多项式和一段迭代求解，比查表难读。所以配了对照测试：
  几个公开可查的节气定点，加上「每年 24 个节气各占一天、按序、落在预期月份」的整体校验。
- 农历本身仍是 1900–2100 的压缩表（通行数据），不自己算朔望月。表覆盖之外的日期
  **只回公历、农历字段留空**，不外推。

**法定调休按年写死，没有该年数据就一个班/休角标都不显示。** 放假和补班安排每年由
国务院办公厅单独发通知，没有规律可推（同一个节日哪年调休、调哪个周末全看当年通知）。
当前录到 2026 年。

- 代价：每年 11 月通知发布后要手工补一段，补之前那一年只有节日名、没有班/休。
  这是刻意的——拿去年的表猜今年会给出确凿的错误信息，比不显示更糟。
- 抄错了没人拦得住，所以 `TestHolidayTableWellFormed` 替 `expandSpan` 盯着：
  它对写坏的条目是静默跳过的（编译期常量，不值得 panic），测试则要求每个条目都
  展得开、放假和补班不撞在同一天、全年放假天数落在合理区间。

**农历、节气、节日名一律中文，不进 i18n。** 它们是中文历法专名，没有通行英译，
硬翻出来的英文没人看得懂（"Beginning of Autumn"？）。所以后端直接返回拼好的中文，
前端原样显示；英文界面下框架文案（Previous month / Back to today / Off / Work）走 i18n，
农历那一行仍是中文。

- 代价：`en.ts` 里查不到这些词条是**正常**的，别有人「补全翻译」时把它们翻出来。
- 干支和生肖**逐日给**，不是整月一份：农历年在正月初一换，跨春节的那个月里前后
  两截分属两个干支年，月级字段无论取月初还是月中都会有一半是错的。


## 035 天气地点精确到区：admin2 + 0.01°，不换上游

状态：生效。相关：`backend/internal/service/weather.go`、
`frontend/src/components/settings/WeatherPanel.vue`、决策 033。

天气组件要能选到「海淀」而不是只到「北京」。两条路里选了改显示、不换数据源：

**地理编码默认仍走 Open-Meteo，结果多透 admin2。** 033 已经把免 key 的上游钉成这两个域名，
换 Nominatim 等于再开一条出站、再写一份 User-Agent / 节流。Open-Meteo 对中国区县本来
就不完整（「海淀」有，「天河区」经常没有）。要精确到区，走可选的和风天气，见决策 036。

显示名按 `name · admin2 · admin1` 去重拼接：`广州` 和 `广州市` 算同一层，
否则卡片上叠两遍。列表右侧 hint 用剩下的层级 + 国家。入库的仍是一个 `city`
字符串，设置 JSON 形状不变。

**出站坐标从 0.1° 收到 0.01°（约 1km），和入库的 `roundCoord` 对齐。**
原先海淀 (39.99, 116.29) 和朝阳 (39.92, 116.44) 量化后都可能落到邻近的 0.1° 格；
库里存两位小数、出站却砍到一位，等于白存。上游国内格点仍约 9–15km，
再细没有新信息，所以停在两位——缓存键空间大约变成 100 倍，但条目上限还是 256，
自用导航站不会有那么多不同坐标。

代价：相邻区的天气读数经常仍是同一份；Open-Meteo 搜不到的区，界面上就是没结果，
不会回落成所属市再搜一次（那会让「天河」变成「广州」，看起来像精确到了区其实没有）。
要覆盖这些区，在 `config.yaml` 配和风（决策 036）。

## 036 天气上游默认可选：Open-Meteo 免 key，和风按需

状态：生效。相关：`backend/internal/config/config.go`、`backend/internal/service/weather.go`、
`backend/internal/service/qweather.go`、`backend/internal/service/fetcher.go`（`GetJSONHeader`）、
决策 033、决策 035。

天气要精确到中国的区，Open-Meteo 地理编码经常没有（决策 035 已经认了这个锅）。
和风对中国区县全，但要开发者账号、独立 API Host 和 Key。所以默认不动，做成实例级可选项：

**默认 Open-Meteo，host+key 都配齐才切和风。** 配置在 `config.yaml` 的 `weather` 段
（或 `TP_QWEATHER_HOST` / `TP_QWEATHER_KEY`），不进用户设置、不进备份——Key 是实例机密，
跟 `auth.secret` 一路，不能跟主题配色一起导出。`provider: open-meteo` 可在配了 Key 时强制
回落到免 key 上游。缺一项就当没配：半开会让接口全 502，卡片永远 `--`。

- API Host 只收 `*.qweatherapi.com` 和三个旧公共域名。这个值带着 Key 出站，写错成别人的
  域名等于把密钥送出去。随手贴的 `https://` 会剥掉，带路径或端口的直接清空。
- 鉴权用 API Key（`X-QW-Api-Key`），不用 JWT。JWT 要 Ed25519 私钥，对自托管导航站过重，
  而且私钥更不该出现在 yaml 里。和风仍支持 Key，只是 SDK 5 和 2027 年起会限流。
- 现象码在服务端收成前端已经认识的 8 档 WMO 粗分类。图标组件不认和风码，未知码回落阴，
  和 Open-Meteo 未知码同一条路。风速 m/s 转 km/h，湿度 [0,1] 转百分数。
- 实况和日预报拆成两次请求：日预报失败不影响温度，只是没有今日高低和昼夜。
- 缓存键带上游前缀。切到和风之后不能把 Open-Meteo 那格的旧数据当新的用。

代价：国内机器连 Open-Meteo 常被墙（`plans.md` 已知限制），配了和风才稳；Key 配错或欠费
时接口 502，卡片仍静默显示 `--`。浏览器照样不直连任何天气上游。
