# Timmy 标志文件包

网页聚合站标志，主标为九宫格 T：亮起的五格构成字母 T，暗格代表尚未收录的站点，右中一格为点缀色，可作为"新增收藏"的高亮位。

## 文件清单

| 文件 | 用途 |
| --- | --- |
| `mark.svg` | 主标，内嵌 `prefers-color-scheme`，浅/深色自动切换 |
| `mark-light.svg` | 固定浅色模式，用于不支持媒体查询的场景（如邮件、部分 CDN 图床） |
| `mark-dark.svg` | 固定深色模式 |
| `mark-mono.svg` | 单色版，`fill="currentColor"`，颜色跟随父元素文字色 |
| `mark-mono-solid.svg` | 单色实心镂空版，用于印刷、盖章、单色丝印 |
| `favicon.svg` | 16/32 px 专用，去掉暗格、圆角收紧 |
| `lockup.svg` | 横向组合标（图标 + 字标） |
| `concept-b-bookmark.svg` | 备选方案 B，书签负形 |
| `concept-c-dots.svg` | 备选方案 C，圆点阵列 |
| `preview.html` | 本地预览页，双击直接用浏览器打开 |

## 色值

| 名称 | 浅色模式 | 深色模式 |
| --- | --- | --- |
| 主色 | `#534AB7` | `#7F77DD` |
| 点缀色 | `#1D9E75` | `#9FE1CB` |
| 图标内亮格 | `#FFFFFF` | `#FFFFFF` |
| 图标内暗格 | 白色 22% | 白色 26% |

CSS 变量写法：

```css
:root {
  --brand: #534AB7;
  --brand-accent: #1D9E75;
}
@media (prefers-color-scheme: dark) {
  :root {
    --brand: #7F77DD;
    --brand-accent: #9FE1CB;
  }
}
```

## 在 Timmypanel 里的接入点

这个目录是**设计源，不参与构建**——界面上的标志是内联画的，不引用这里的文件（原因见
`docs/decisions.md` 决策 018）。改标志时要同步改下面这几处：

| 位置 | 用的是什么 |
| --- | --- |
| `frontend/src/components/BrandMark.vue` | 主标内联版，`variant="mono"` 对应 `mark-mono.svg` |
| `frontend/index.html` | favicon，`favicon.svg` 压成 data URI（`#` 要写成 `%23`） |
| `frontend/src/stores/site.ts` | `DEFAULT_FAVICON`，和上面那份必须一致 |
| `frontend/src/style.css` | `--tp-brand-*` 色值变量、`.tp-hero-bg` 登录页渐变 |
| `frontend/src/App.vue` | Naive UI `themeOverrides` 的主色，按 `data-theme` 取深浅两档 |
| `frontend/public/icons/` | 手机收藏 / 主屏位图，`gen_touch_icons.py` 生成（决策 025） |
| `backend/internal/web/dist/index.html` | 「前端尚未构建」占位页里的标志（独立一份，手写） |

## 接入方式

**favicon**（现代浏览器优先取 SVG，旧浏览器回落到 ico）：

```html
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="icon" href="/favicon.ico" sizes="32x32">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
```

**React 中作为组件使用**：把 `mark-mono.svg` 的内容直接内联成 JSX，颜色交给 `className` 控制，就能跟着主题走：

```jsx
// 单色版依赖 currentColor，父级的 text 颜色即图标颜色
<TimmyMark className="text-violet-600 dark:text-violet-400" size={32} />
```

`<img src="mark.svg">` 引入时，SVG 内部的 `<style>` 仍然生效，深浅色会自动切换；但 `currentColor` **不会**生效——外部引用的 SVG 拿不到宿主页面的文字色，所以单色版必须内联使用。

## 生成位图

需要 PNG（微信分享封面、apple-touch-icon、Windows 磁贴）时：

```bash
# 首选：纯标准库脚本，任何装了 Python 的机器都能跑，产物直接写进 frontend/public/icons/
python brand/gen_touch_icons.py

# 备选：rsvg-convert，比 ImageMagick 的 SVG 渲染质量好
sudo apt install librsvg2-bin
rsvg-convert -w 180 -h 180 mark-light.svg -o apple-touch-icon.png
rsvg-convert -w 512 -h 512 mark-light.svg -o icon-512.png

# 生成 ico（含 16/32/48 三个尺寸）
rsvg-convert -w 48 -h 48 favicon.svg -o /tmp/48.png
rsvg-convert -w 32 -h 32 favicon.svg -o /tmp/32.png
rsvg-convert -w 16 -h 16 favicon.svg -o /tmp/16.png
convert /tmp/16.png /tmp/32.png /tmp/48.png favicon.ico
```

注意用 `mark-light.svg` 而不是 `mark.svg`——命令行渲染器不会走深色模式分支，但显式指定更保险。

## 两点提醒

1. `lockup.svg` 里的 "timmy" 是 `<text>` 元素，依赖系统字体。正式发布前建议在 Figma 或 Inkscape 里执行"文本转路径"（Inkscape 快捷键 `Shift+Ctrl+C`），否则不同设备上字形宽度会有出入。
2. 所有 SVG 都没有用滤镜和渐变，压缩后单文件在 1 KB 以内。上生产前可以过一遍 `svgo`：`npx svgo -f . --multipass`。
