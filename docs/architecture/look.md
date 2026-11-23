# 外观、语言和快捷键

[← 架构总览](../architecture.md)

风格、窗口底和颜色、应用图标、界面语言。

## 外观、语言和快捷键

### 窗口底和颜色

#### 两种风格

风格是 `Settings.Look`，代码在 `ui/look.go`：

- 默认 `""` 是 **VS Code**：照 VS Code 1.147 起的 Light 2026 / Dark 2026，不透明的平面。三块颜色是 `vsSide`（标题栏、侧栏、状态栏、终端、右栏）、`vsEditor`（编辑器组）、`vsLine`（线）；`vsActiveLine` 是当前标签和底栏标题的线（浅色黑、深色 #BFBFBF），`vsActivityLine` 是视图切换的线。强调色默认「黑」，「蓝」是它的 #0069CC / #297AA0。
- `grove` 是 **Grove**：窗口底上的圆角卡片，见下面“Grove 风格”。

**怎么切**：取颜色的小函数（`cardBg` / `windowBg` / `headerBg` / `cardLine` / `diffBase`）只拿得到主题、拿不到 `App`，所以每帧 `applyAccent`（主窗口的 `root`、设置窗口、关于窗口）开头先：

- `setLook`：把风格记在包级的 `vsLook`（`vscode()` 读它），把设置里的浮层不透明度记进 `frostPct`，并调 `highlight.SetDocComments`（见下面“代码配色”）。
- `applyLook`（只在 VS Code 风格时）：换这一帧主题的字色、线色、`Surface`（输入框的白，mygo 的输入框、下拉框拿它垫底）、`ScrollbarStyle`，把 `Inverse` / `InverseText` 换成浮层底和正文字色。

#### VS Code 风格：布局

- `card` 不要圆角和阴影；`panelCard`（底栏、右栏）是侧栏色的底、不描边。
- `center` 的缝是 0（拖条还是 `cardGap` 宽，底色就是侧栏色）；根的中间一行顶上、状态栏顶上各一道 `vsLine`。
- 编辑器标签还是 `panelTab`，当前那页顶上一道 `vsActiveLine`，`focusBar` 不画；底栏标签走 `panelTitle`（不分格，当前那个底下一道线）。
- `segTabsOf` 只有图标时（侧栏视图切换）是平的，当前那个底下一道线。
- 标题栏的 `branchButton` 是平的（透明底、400 字重、悬停 `toolFill`）；快速打开照命令中心（`quickOpen`）。
- 对话框的次要按钮走 `vsSecondary`。
- 窗口不透：`cmd/workbench` 的 `vibrancy` 在 VS Code 风格下一律不用系统材质。

#### VS Code 风格：滚动条

`applyLook` 把主题的 `ScrollbarStyle` 换成 `vsScrollbar`：10 宽、方、贴两头、轨道透明，滑块是 25% 的灰（悬停 40%、拖着 55%；比 VS Code Light+ / Dark+ 的 40% 还淡，2026 主题那块 75% 的太实）。ged 的 Diff 概览条 14 宽。

#### VS Code 风格：侧栏和列表

- 侧栏列表左右不留（`sidePadX()`），选中块是方的（`sideRowRadius()`；实心选中时 `newListLook` 换一份圆角 0 的主题）。
- 淡底的三档颜色走 `softFills` 的 VS Code 分支：三档都是中性的灰，活动选中只是深一档的灰。`listLook.rowAt` 不描框（VS Code 的 list.focusOutline 蓝框没照）、不画竖条，建列表的主题 `Radius` 是 0（mygo 按它画悬停块）。
- 文件树的缩进参考线（`drawGuides`）用 `vsGuide`，别的级是它的 40%；半透明的线压在选中、悬停的底上不切缝（`TestVSTreeHoverGuides`）。
- 吸顶栈的底是 `vsSide`。
- `hitText` 的命中只加粗、不换色。
- 侧栏视图标题的字号是 `sideTitle()`（VS Code 11，Grove 同 `sideSub`）。
- 符号的小字（`symbolGlyph`）按 `symbolColor` 上色，照 VS Code symbolIcon.* 的默认：类 #D67E00、函数 #652D90、变量 #007ACC，深色 #EE9D28 / #B180D7 / #75BEFF，别的灰。

#### VS Code 风格：代码配色

- 代码配色走 `vsCodeTheme`（词的颜色是经典的 Light+ / Dark+，其余照 2026），在 `fullCodeTheme` / `softTheme` 里换。
- ged 的 Diff 颜色走 `gedStyle` 里的 VS Code 分支（`PlainNumbers` 让改了的行号不变色）。
- 编辑器的选中、配对括号、查找命中叠 `accentTint`：一般是强调色；深色里强调色是灰的时换成浅灰 #D4D4D4。`vsSelection` 是它的 25%（深色 33%）。
- Python 的 docstring 在 VS Code 风格里是字符串、Grove 风格里是注释：`setLook` 调 `highlight.SetDocComments(Look != "")`，只换词的类、不换分词状态，开着的编辑器下一帧就换。

#### VS Code 风格：浮层、菜单和提示

- 悬停提示照 VS Code 是浅的：`applyLook` 换过的 `Inverse` / `InverseText` 让 mygo 的 Tooltip、提示条变成浮层底和正文色，`tipStyle` 再描一道 `vsTipLine`。
- 浮层（Grove 画的菜单、命令面板、对话框、编辑器的悬停提示和补全）的底是 `vsOverlayBg`。磨砂（`ui/frost.go`，照 VS Code 的 frosted glass）两种风格都有：糊开底下的内容、盖一层设置里“浮层不透明度”那么多的底色，见 [menus.md](menus.md#磨砂)。
- Grove 画的菜单（`gmenuPanel`）用 `vsMenuLine` 的框、`vsMenuSep` 的分隔线；高亮项只垫 `vsMenuHot` 的灰色半透明底，不描框、字不换色。不描框是因为 mygo 的 Border 占布局，描了框字会往里挪 1 pt，悬停时字像在跳（`TestGroveMenuHotOnlyFill` 比高亮前后字的像素）。

#### 跟着风格变的尺寸

| | Grove | VS Code |
|---|---|---|
| 侧栏正文字号 `sideFont()` | 14 | 13 |
| 仓库名字号 `repoFont()` | 14.5 | 13 |
| 文件树行高 `treeRowH()` | 24 | 22 |
| 右栏历史行高 `histRowH()` | 30 | 22 |

提交图的节点在行高的正中（`graphNodeY()`）。

#### Grove 风格：窗口底和卡片

窗口底（`Settings.Backdrop`，只在 Grove 风格里用）三种：

- 柔和（默认 `""`）：纯色底上用 `Draw` 画几团很淡的色斑（`paintBackdrop`，Shadow 的模糊；左上那团的颜色从强调色推出来，`blobColor`），不依赖桌面壁纸。
- 纯色（`flat`）。
- 通透（`vibrancy`，再选系统材质 `material`；`WORKBENCH_MATERIAL` 压过设置）。通透在默认壁纸上看起来是平的、HUD 一类材质会把字冲淡，所以不是默认。

各区域是圆角卡片（`ui/widgets.go` 的 `card`），卡片之间 6 pt 的缝（`cardGap`）就是拖条（`resizer`）。标题栏 38 pt 高（`barHeight`），窗口是 `TitleBarHidden`，标题栏里的按钮按红绿灯的位置排（`TrafficLights`）。

#### 强调色和列表选中

强调色在 `ui/accent.go`：

- 可选的（`workspace.AccentKeys`，设置下拉的顺序）：跟随系统 `system`、黑 `black`（默认，`DefaultAccent`，存成 `""`；浅色 #333333、深色 #5C5C61，都配白字）、蓝 `blue`（VS Code 2026 的 #0069CC / #297AA0），还有 `forest` / `steel` / `ink` / `graphite` / `teal` / `indigo`。
- `applyAccent` 每帧在主题上换整套：`Accent`，悬停 / 按下深（深色里亮）8% / 16%，选中的字和焦点环的透明度。跟随系统时不动。

列表选中默认是淡底（`listLook` / `softFills`）：

- 强调色淡淡的一层加左边一道竖条；列表没有键盘时换成中性的灰、不加竖条；悬停更淡。
- 建列表的主题里 Accent 透明，选中块的底由行自己按 `ListFocused` 在 `Draw` 里画。上下相邻的行也选中时那一边不留圆角、竖条通到行边，连着多选的几行接成一整块。
- `SolidSelection` 换成 mygo 原来的实心块；`onAccent` 判断这一行要不要按“在强调色上”画（选中且实心时字、图标换成 `AccentText`）。

尽量少装饰：横幅、悬停这些用很淡的填充（`quietFill` / `bannerFill`），不加边框。

#### 界面缩放

`Settings.Zoom`，按 `ZoomSteps` 一档档走，是每个窗口每帧 `c.SetZoom`，设置窗口、关于窗口也跟着。终端自带的字号键（⌘= ⇧⌘= ⌘- ⌘0）让给界面缩放（`app/terminals.go` 的 `termZoomKeyFor`），终端字号只在设置里改。

### 应用图标

`internal/appicon` 用有向距离场现画（`Draw(size)`），任意大小都抗锯齿；关于窗口的小图标、更新窗口的图标（`Draw(256)`）也是它现画的。`assets/icon.png`（`mygo build` 用的 1024×1024）由它生成：改了画法跑 `go test ./internal/appicon -update`，`TestIconFile` 核对两者一致。

### 语言

所有界面文案都通过 `internal/i18n`：

- `T` / `Tf` / `Tn`（带数，英文按数换单复数）/ `List`（列举的连接词），还有 `Colon` / `Comma` / `Paren` 这类按语言换的标点。
- 原文是代码里的中文；`zh-CN.json` 由 `TestCatalog -update` 从代码里收集（所以文案要写成字面量，不要拼接），`en.json` 是英文。
- 英文的单复数写成 `键|1` 这样的变体；有两个数时是 `|1|`（第一个是 1）、`||1`（第二个是 1）、`|1|1`（都是 1），`PluralKeys` 算该找哪些。
- 设置里选语言（`Settings.Lang`：空是中文，`en` 是英文），`i18n.Set` 切换。mygo 自带的文案（编辑器的查找条、系统菜单、对话框按钮）通过 `SetLanguages(i18n.Locale())` 同步语言：启动时设一次，读完设置后再设。

无障碍：每个图标按钮都要有名字（`Label` / `Tooltip`），`TestAccessLabels` 把主窗口和设置窗口常见的样子走一遍查；新加的区域、浮层、窗口要在它里面加一步画出来（见 [AGENTS.md](../../AGENTS.md)）。

### 快捷键

见 [app.md](app.md) 的“自定义快捷键”。
