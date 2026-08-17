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

