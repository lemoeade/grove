# Markdown 渲染

[← 架构总览](../architecture.md)

Markdown 预览（internal/gmd）、HTML 块的样式（internal/css）、源码整理（internal/mdprep、internal/emoji）、Mermaid 和公式（internal/render），以及并排预览和同步滚动。

## Markdown 渲染

### 总览：一篇 Markdown 怎么画出来

| 一步 | 代码 | 做什么 |
|---|---|---|
| 取源码 | `ui/mdpreview.go` 的 `markdownPreview` | 每帧拿源码（并排预览拿源文件的缓冲区） |
| 整理 | `internal/mdprep`（`mdprep.Prepare`） | front matter、跨块的 HTML 容器换成围栏代码块；最后经 `internal/emoji` 换 emoji 短代码 |
| 画 | `internal/gmd`（`ui/gmdview.go` 的 `mdShow` → `gmd.View`） | 解析、排版、选择复制；`WORKBENCH_GMD=off` 退回 mygo 的 `markdown.View` |
| 带样式的 HTML 块 | `internal/css` + `gmd/html.go`、`gmd/htmlrender.go` | 算层叠，按 grid / flex / 盒子画 |
| Grove 补的内容 | `ui/mdpreview.go` 的 `mdEnv.setup` 挂的钩子 | Mermaid 图和 TeX 公式（经 `internal/render` 的 WebView）、带图片的表格（`ui/mdtable.go`）、front matter 卡片、折叠块、徽章一排…… |

两个视图吃同一份钩子：`gmd.Options` 的字段和 mygo 的 `markdown.Options` 一一对应（`CodeBlock`、`Code`、`Block`、`HTML`、`Math`、`InlineMath`、`LinkMenu`、`OnLink`、`CodeLink`、`Preview`、`LoadImage`……），`gmd.Options.Mygo()` 每次把字段拷成 mygo 的那份（代码块的钩子包一层，mygo 的 `CodeBlockView` 换成 gmd 的）。gmd 另外多了 `CSS`、`Image` 和 `Strings`（视图里的几句文案，`ui.mdStrings` 给译好的）。

### internal/gmd：Grove 自己画的 Markdown 视图

做法照 `internal/ged`：解析还用 mygo 的 `ui/markdown/parser`，排版和画只用 mygo 公开的界面接口（Column / Row / Grid、RichText、Painter）。不认 Grove 的主题和 `internal/ui`：样子由 `markdown.Theme` 给（`gmd/theme.go`），文案由 `Options.Strings` 给。段落、列表、表格、代码块的画法改自 mygo 的 `ui/markdown`，去掉了流式显示、提及 / 引用和链接图标这些 Grove 不用的。

主要文件：

- `view.go`：`View(c, src, o)` 画一篇，状态（解析结果、段落的字、代码块的高亮、表格列宽、HTML 块排好的样式）留在返回的元素上（`viewState`），src 没变的块下一帧不重算。按块的种类分派（段落、标题、分隔线、代码块、HTML 块、`$$` 公式、引用和 GitHub 的提示块、列表、表格），还有脚注的编号和列表。
- `options.go`：`Options`（见上）和 `Mygo()`。
- `spans.go`：段落是带样式的字（RichText），在链接处切开，链接是里面的 `ui.Link`；记着每段字在源码里的位置，复制时用。
- `code.go`：`CodeBlockView`、`DefaultCodeBlock`（不接手时的样子，带复制按钮），只对改了的行重新高亮。
- `table.go`：表格的自动布局（见下面“表格”）。
- `image.go`：图片。先问 `Options.Image`（调用方管缓存），没给就第一次用时在后台 goroutine 里调 `LoadImage`，结果缓存在 `Options` 里（`sync.Map`）。
- `html.go`、`htmlrender.go`：HTML 块自己的 HTML + CSS 排版（见下面“带样式的 HTML”）。
- `copy.go`：选择和复制。按文字复制时段落之间空一行、代码块和紧凑列表一行一行、表格单元格用 Tab 隔开；“复制为 Markdown”（右键菜单，或焦点在视图里时 ⇧⌘C）复制选区两端之间的源码，被切断的强调、代码、链接语法补齐。
- `alert_icons.go`：GitHub 提示块（Note、Tip……）的 Octicons 图标。

`ShareSelection`：视图嵌在另一个可选择的容器里（`<details>` 的正文、大文件的一段、表格单元格），和外面共用选区，自己不再 Selectable、不接复制。

### internal/css：HTML 块的样式表

不碰界面的 CSS 解析和层叠，元素由调用方实现 `css.Element` 接口（`Tag` / `Attr` / `Parent` / `Prev` / `Next`）给进来；gmd 的 `hnode`（`gmd/html.go`，用 `x/net/html` 解析出来的节点）就是它。

- `parse.go`：`ParseSheet`（`<style>` 的内容 → `Sheet`）、`ParseDecls`（`style=` → `[]Decl`），去注释、认 `!important`。
- `selector.go`：选择器（复合选择器加 `' '`、`>`、`+`、`~` 组合符）和优先级。
- `cascade.go`：`Compute(author, e, pseudo, inline, parent, rootPx)` 算一个元素（或 `::before` / `::after`）的 `Style`。层叠顺序：UA → 表现属性 → 文档的 `<style>` → `style=`；`!important` 反过来。同一层按优先级、再按先后。展开简写、继承、`var()`；`font-size` 总换成 px、`font-weight` 换成数、`line-height` 的 em / % / rem 换成 px，`currentcolor` 换成元素的 `color`。
- `ua.go`：`UA`（照 HTML 标准 Rendering 一节、删掉画不出来的浏览器自带样式）、`Presentational`（`align`、`valign`、`width` / `height`、`bgcolor`、`<font>`、`<table>` 的 `border` / `cellpadding` / `cellspacing`、`nowrap` 折成的声明）。
- `color.go`、`length.go`：`ParseColor`（十六进制、`rgb()` / `hsl()`、颜色名）、`ParseLength`（px、em、rem、%、fr、vw / vh；pt 等换成 px）、`ParseTracks`（grid 的轨道）。

只认预览里画得出来的。认不出的规则、选择器、值一律跳过，不报错也不 panic。

### 带样式的 HTML

1. `gmd.DocCSS` 只从 HTML 块里收 `<style>`（围栏代码里的不算）。`ui` 按整理结果缓存（`mdView.styles`），放进 `mdEnv.css`，再给 `Options.CSS`：分段画的大文件、嵌在 `<details>` 里的段落都按全文的样式表排。
2. `ui/mdhtml.go` 的 HTML 钩子先问 `gmd.WantsCSS`：块里有 `<style>`；或者文档里有样式表、块里写了 `class=` / `id=`；或者 `style=` 里有底色、边框、`display`、内外边距、grid、flex、定位、阴影、宽度这类属性。是就交给 `Options.HTMLView`（按 `Options.CSS` 加块里自己的 `<style>` 排）；不是就照旧走下面“预览的钩子”里那条 README 式的路（居中、`<details>`、徽章不受影响）。`WORKBENCH_GMD=off` 时一律走 README 式的路。
3. `gmd/htmlrender.go` 把算好样式的节点搭成 mygo 元素：
   - 块级流（外边距折叠）、flex（Row / Column、换行、对齐）、grid（`grid-template-columns` 的 fr / px / `repeat` / `minmax`，`grid-column: span`）、表格（列按内容、`colspan`）、绝对定位；
   - 背景色和线性渐变、边框（四边一样的用原生边框，不同的画梯形）、圆角、阴影、透明度、`overflow: hidden`；
   - 行内的连成一段富文本（链接、装饰、行内底色、空的 inline-block 小圆点当对象画）。
4. 字色（`textColor`）：作者给了底色、没给字色时，字换成那块底上看得清的颜色；作者写的字色和底色几乎分不出时（比如暗色主题里写死的深灰字）也换成看得清的（`readable`）。
5. 只有 `<style>`（和注释、`<script>` 这类）的 HTML 块不占地方。

### 表格和带图片的表格

预览铺满标签的宽（照 VS Code，不限一栏的宽）。

表格照浏览器的自动表格布局（`gmd/table.go`）：

- 每列要它最宽的单元格排成一行的宽（max-content），最窄让到折不开的最长一截（min-content，`minContent` 按空格、连字符、中日韩字切开量）。
- 放得下时整张表按内容宽；放不下时表就是这一栏宽，每个单元格 `Width(max).Shrink(1).MinWidth(min)` 按要的宽成比例让。
- 只有 min-content 加起来都放不下才包 `ScrollHorizontal` 横着滚（看上一帧这一栏的宽，`Draw` 里发现变了再要一帧）。

带图片的表格在 `ui/mdtable.go`：`Block` 钩子碰到单元格里有图片的表格（`tableHasImage`）自己搭——列等宽铺满一栏，全空的表头行不画，每个单元格是一个 `ShareSelection` 的小预览视图（`mdShow`；选项存在表格元素的 `ui.Local` 上，图片缓存才留得住）。`cellSource` 给开头像块标记的字加反斜杠，免得单独解析成标题、列表。别的表格照旧由预览自己排。

### internal/mdprep 和 internal/emoji：交给解析之前先整理

`mdprep.Prepare` 处理 mygo 的解析器不认、README 里又常见的写法：

- 文件开头的 front matter（`---` YAML `---` 或 `+++` TOML `+++`）；
- 跨了几个块的 HTML 容器（`<details>`、`<div align>`、`<p align>`、`<center>`……开、闭标签之间隔着空行写 Markdown 的）。

做法是把这样的一段换成一个围栏代码块，信息串写成 `grove-region N`，第 N 段的内容记在 `Doc.Regions` 里，预览在 `CodeBlock` 钩子里认出来自己画。`Doc` 还带着链接引用定义（`Defs`，分块画时接在每块后面）和有没有脚注（`Footnotes`，有就不分块）。

`Prepare` 最后经 `internal/emoji` 的 `emoji.Replace` 把 `:tada:` 这样的短代码换成 emoji：

- 表 `emoji/gemoji.txt` 是 GitHub gemoji 的别名（一行“别名 TAB emoji”，embed 进二进制，第一次用时读成 map；`Lookup` 查一个）；
- 围栏代码块、行内代码里的不换，冒号两边挨着字母、数字的（`10:100:20`）不算；整理出来的围栏块里的也不换（容器里的 Markdown 另外整理时再换）。

两步都只换一行里面的字、不增删换行：源码的第 k 行在整理后还是第 k 行，大纲跳转、同步滚动按行对得上。

### internal/render：Mermaid 和公式

每个窗口一个 `Renderer`（`app/render.go` 的 `a.Renderer()` 懒建；设置 `NoDiagrams` 关掉，只显示代码块和 TeX 原文）。

- 预览每帧来取（`Get`）：命中缓存直接返回；否则排进队列，先拿这张图在主题 / 缩放变化前的版本顶着，没有旧版本时显示代码块或 TeX 原文。
- 一个后台 goroutine 依次渲染，完成后通知界面重绘。
- 缓存最多 512 张 / 32 MB；队列 128 条，满了丢最早的请求（新请求优先）。
- 空闲 60 秒后销毁后端，下次需要时重建。源码本身的错（`SourceError`）只是这一张画不了；别的错当后端坏了，关掉重建。

后端（`web.go`）是一个看不见的 `mygo.WebView`，跑 `page.html`（`wbRender`）。资源 embed 在二进制里，脚本存成 `.js.gz`，经自定义协议 `wbrender://` 给 WebView 时解开（`assetHandler`：有原样的给原样的，没有就找 `.gz`）。页面按需加载脚本。

#### Mermaid

mermaid.js 12.3 画所有的图，外观由 `page.html` 的 `groveConfig` 定：

- `theme: "base"`、`look: "classic"`；`layout` 不写，由 12 自己挑（流程图、状态图、类图、ER 图用 ELK，线走直角、拐角带圆；全局写 `elk` 的话思维导图画不出来）。
- `themeVariables` 从配色混出来，是 VS Code 里 Markdown 预览常见的黑白文档风：节点白底（深色是比底亮一档的灰）、深色细边，连线是字色，边上的字垫浅灰底，节点带 Mermaid 自己那种往右下偏的淡影子（`themeCSS` 只给形状加，不给字）。
- 饼图、思维导图、git 图、甘特图用一套低饱和的六色（浅色是淡的、深色是暗的，字都是正文色；VS Code 2026 的 `charts.*` 和 Mermaid 自己的 default 主题都太艳）。
- `flowchart` / `state` 的 `minNodeWidth` 关掉（12 默认 120，短的节点会被撑宽）。
- 画完 `polish` 只做一件事：把思维导图圆形根节点的字挪回正中。
- 用户自己的 front matter / `%%{init}%%` 照样盖得过这些（比如 `layout: dagre` 退回老排法）。
- `htmlLabels: false`，标签画成 SVG 文字，canvas 才取得到。图都画到 canvas 上取 PNG。
- Mermaid 12 要 Safari 17.4 起的 WebKit：`mermaidSVG` 先查一下，老的抛 `render.OldWebKit`，预览里换成一句“要更新系统”。

配色：`ui/mdrender.go` 的 `mdPalette` 从主题算（底色、字色、线色、强调色、节点底、边框），JSON 放在 `Request.Palette`，在缓存的键里；换主题先拿旧的顶着再重画。底色取 mygo 浅色 / 深色主题的窗口底，不取当前元素的 `Background`（排版和画的时候不一样会画两遍）。

#### 公式

MathJax 4（`tex-svg.js`，New Computer Modern 字体；字体分片和 mhchem / bbm / dsfont 的字体扩展在 `mathjax/fonts/`，`loader.paths.fonts` 指过去，不上网）。

- SVG 里只有路径（`fontCache: "none"`，字形是 `currentColor`）时返回 SVG 文本，`render` 用 `ui.ParseSVG` 解成 `Result.Vector`，mygo 按画的时候的字色和像素画。
- 有 `<text>`（字体里没有的字，比如 `\text{}` 里的中文）等 mygo 画不了的才画成 PNG。
- 矢量的结果不分主题、缩放、字色：`Get` 拿同一张（`near`）直接给、不排队。

#### 接到预览上

`ui/mdrender.go` 的 `mdRender` 把渲染器接到 `gmd.Options` 上：挂 `Code["mermaid"]`、`Math`、`InlineMath`，`$…$` 行内公式靠 `ExtDollarMath`。结果的键里带主题、配色，大小按界面缩放乘 `ZoomFactor`；返回 `Gen()`，预览拿它当那一栏的 key（有了新的行内公式结果，段落重排）。渲染器没有（设置里关了、测试里没给）时这几个钩子摘掉，`$…$` 也不认，照旧是字。

### 预览的钩子

预览（`ui/mdpreview.go` 的 `markdownPreview`）每帧拿源码，先过 `mdprep.Prepare`，再给 `mdShow`。选项是 `app/markdown.go` 的 `MarkdownOptions`（存在 `Doc.MD`，第一次预览时建，改名后、换了网上图片的设置后重建），钩子都在 `mdEnv.setup` 里挂：

- 块：`CodeBlock`（`ui/mdblocks.go` 的 `codeBlock`）认出 `grove-region` 围栏：front matter 画成能收起的键值卡（`mdFrontMatter`），容器画成折叠块 / 对齐的一栏，里面的 Markdown 用嵌套的 `ShareSelection` 视图（`mdEnv.nested`，选项、整理结果存在元素的 `ui.Local` 上）。`Block` 钩子一直挂着：放跳转记号和同步用的探针（下面），接手只有图片的段落（一排徽章）和带图片的表格。
- HTML：`HTML` 钩子（`ui/mdhtml.go` 的 `mdHTML`）带样式的交给 `Options.HTMLView`（见上）；别的用 `x/net/html` 解析，块级标签各自成块，连着的行内节点转成 Markdown 排成一段，只有图片的画成一排按 `width` / `height` 的图片，`<picture>` 按主题挑 `<source media>`。不认识的标签只显示字，`<script>`、`<style>`、注释不显示。
- 图片有两条路：
  - gmd 自己画的图片（Markdown 的 `![]()`）走 `app/mdimages.go` 的 `ImageLoader`（就是 `Options.LoadImage`）：本地的相对文档目录读，缓存在 gmd 的 `Options` 里。
  - Grove 自己搭的图片（一排徽章、HTML 里的 `<img>`，`ui/mdblocks.go` 的 `mdImage`）走 `PreviewImage`：进程级的缓存（几个窗口、几个预览共用，按像素字节算，48 MB），后台读好了叫界面重画；带文字的 SVG（shields.io 那种徽章）交给窗口的渲染器（WebKit）画成位图，渲染器没有时才退回 mygo 画（字画不出来，只剩色块）。
  - 两条路都认本地文件、`data:` 地址和网上的地址；网上的经 `FetchImage`（只 GET、不带 Cookie、8 MB 上限，同时最多取 6 张，测试换成桩）在后台取。`PreviewImage` 那条路不读 `http:` 的（`imageKey` 返回 `errInsecure`），`ImageLoader` 没有这道检查。设置 `NoRemoteImages` 关掉网上的，只显示说明文字。`ForgetImages` 清空进程级的缓存（测试用）。
- 主题：`ui/mdtheme.go` 的 `mdTheme` 在 mygo 默认主题上调字号、行距、标题和块的间距，代码用编辑器的字体；VS Code 风格照它的 `markdown.preview.fontSize` 14 / `lineHeight` 1.6；按（界面主题、字体、风格）缓存。
- emoji 短代码：见上面 `internal/emoji`。
- 链接：
  - `LinkMenu`（`ui/mdlinks.go` 的 `mdLinkMenu`）：打开链接、复制链接，本地文件再加复制路径、在访达中显示，后面是选中文字的编辑项。
  - `Preview`（`mdLinkPreview`）：停在本地文件的链接上时显示它的仓库内路径（仓库外的写成 `~/…`），找不到的说一声。
  - `CodeLink`（`ui/mdblocks.go` 的 `codeLink`）把行内代码里像路径、而且在文档目录或仓库根下 `os.Stat` 得到的变成链接（结果按代码缓存在 `mdEnv.links`）。
  - 这几处在主线程只 stat、不读内容。

点链接（`app/markdown.go` 的 `MarkdownLink`）：`#标题` 滚到那个标题；http、https、mailto 交给系统；别的当文件路径（相对文档所在目录，可带 `:行[:列]`）在编辑器里打开，是 Markdown 就直接显示预览（带 `#片段` 的滚到那个标题），目录在文件树里显示；别的协议不开。

### 跳转

点 `#标题` 的链接设 `d.Anchor`；侧栏大纲点一项、旁边的编辑器滚了设 `d.AnchorLine`（从 1 起）。

- `markdownPreview` 只认这一帧开始时就有的值（这一帧里点链接设的下一帧才算）。
- `Block` 钩子碰到那个标题 / 盖住那一行的顶层块时放一个看不见、比视图还高的记号（`mdMarker`）并 `ScrollIntoView`，那一块就滚到最上面。
- 找不到就提示（`#标题`）或作罢（行号）。

### 大文件

整理后超过 32 KB（`mdChunkMin`）且没有脚注时按顶层块分段（`mdSplit`：一段到 512 字节后遇到标题、到 4 KB 后遇到哪块都另起），每段一个 `mdShow` 视图，放在 `ui.List` 里只排看得见的。

- 跳到还没排出来的地方时先 `ListState.ScrollTo` 那一段（`seekChunk`），下一帧那段排出来了再放记号，最多试几帧。
- 每段最后一块的行号范围到下一段第一行为止。
- `mdView`（存在 `Doc.MDView`，按文档留着）记着整理结果、分段和列表位置：在源码和预览之间切换不重新整理；打字时源码变了等停下 150 ms 再重排（`mdSettle`），刚切过来（500 ms 没画过，`mdIdle`）就马上排。

### 并排预览

`app/mdside.go`。入口：⌥⇧⌘V（`view.markdownSidePreview`），或 Markdown 标签条上的“在旁边打开预览”按钮（照 VS Code，标签条上不放就地预览的眼睛，就地预览 ⇧⌘V 走快捷键和菜单）。

- 它是一个 `docs.MDPreview` 标签：`MDSource` 指向源文件的 Doc，Key 是 `(MDPreview, Root, Rel)`（不在仓库里时用绝对路径）。
- 放在旁边一组（已经有旁边的组就挪过去，只有一组时 `splitOff` 向右分出一组），固定标签，一开就滚到编辑器正看着的那一节，键盘还给源文件。
- 它不读盘，每帧拿 `MDSource` 的缓冲区。
- 源文件的编辑器看不见时（预览被拖进同一组、源文件在显示就地预览，`App.EditorShown`）预览不跟它：看不见的编辑器不排版，`TopLine` 停在最后画的那一帧。
- 这种标签不进会话（state.json 不变）。

#### 和源码的对应（`ui/mdsync.go`）

照 VS Code 的 markdown 预览：

- 探针：每个顶层块里一个看不见、铺满它的探针（`mdEnv.topBlock`），把这块的行号范围和上一帧的框记进 `mdView.blocks`；`lineAt(y)` 按块里的位置比例换算出带小数的行号。
- 按屏幕行摊：照 VS Code，一块的顶到下一块的顶（连着中间的空当）对着这块的第一行到下一块的第一行。中间按源文件编辑器的屏幕行摊（`Doc.RowOf` / `LineAtRow`，ged 的折行和折叠都算上；mygo 的编辑器一行一个屏幕行），不按源码行数：按行数的话，折成三行的一段和它后面一个空行各算一行，编辑器匀速滚时预览一帧快一帧慢。
- 放跳转记号时这块和下一块的框都取上一帧的 `mdView.prev`（同一次排版量的，滚着时差才不带一帧的滚动）。
- `RowOf` / `LineAtRow` 用的是 ged 只按估的折行数算的 `etops`（`TopLine` 还用真排出来的 `tops`）：滚动时新排出来的行和估的不一样，`tops` 会变、块里比例的分母跟着变，预览就一跳一跳；`etops` 只在行、宽、字体变了时重算，滚着时不动。
- 分块的预览（`seekChunk`）要放记号的那块上一帧已经排着时不先 `ScrollTo` 到块首，直接放记号，不然一帧块首一帧原处地来回闪。

#### 两边同步

- 编辑器 → 预览：`Doc.TopLine()`（ged 是 `Editor.TopLine`，带小数：顶上那一行滚过了几分之几）变了就设 `AnchorLine`，小数记在 `mdView.frac`，放记号时按比例往块里挪。对应 VS Code 的 `scrollPreviewWithEditor`；设置 `NoPreviewSync`（state.json 里是 `noPreviewScrollSync`）关掉。
- 预览 → 编辑器：指针在预览上、预览顶上对着的行变了，`Doc.ScrollToFrac`（ged 的 `ScrollToLineFrac` 按小数停在那一行中间）滚编辑器。对应 VS Code 的 `scrollEditorWithPreview`；设置 `NoEditorSync`（`noEditorScrollSync`）关掉。
- 一边拖着另一边走之后 `mdSyncQuiet`（200 ms）里另一边的变化只更新基准、不往回传，免得两边来回拉。

#### 别的对应

- 源文件光标所在的块左边一道竖条（VS Code 的 `markEditorSelection`，在 `topBlock` 里画）。
- 双击预览（滚动容器的 `HandleInput`，`Clicks == 2`）下一帧 `RevealSource` 回到那一行（VS Code 的 `doubleClickToSwitchToEditor`）。
- 标题左边悬停出 🔗（`headingLink`），点了复制 `仓库内路径#slug` 并滚到它。

#### 跟着当前标签

- 键盘换到另一组里的另一个 Markdown 文件（它自己没开并排预览）时 `FollowSidePreview` 把这个预览改指向它，滚到那个编辑器正看着的地方。
- 源文件的标签没了（预览标签被 `addDoc` 顶掉不经 `RemoveDoc`）的预览也跟，所以先跟再看源文件在不在。
- `view.markdownLockPreview`（标签右键菜单里也有“锁定预览”）锁定了就不跟，标题写成 `[预览] x`（没锁的是 `预览 x`）。
- 源文件关了（`RemoveDoc` → `closeSidePreviews`）改跟同一个文件的另一份，没有就一起关。
