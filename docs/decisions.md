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
