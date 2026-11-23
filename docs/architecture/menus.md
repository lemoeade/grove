# 右键菜单和按钮的菜单

[← 架构总览](../architecture.md) · [文件树](files.md)

右键菜单和按钮的菜单大多是 Grove 自己画的（issue #48 起是文件树，#58 起大多数地方）。不用系统菜单：mygo 弹的系统菜单左上角钉在点的地方，窗口靠下时不往上翻，而是折成带滚动箭头的（见 [mygo-pitfalls.md](../mygo-pitfalls.md)）。也不用 mygo 的 `PopoverMenu`：不能按住右键拖到项上松开、子菜单没有延迟和安全三角、灰项说不了原因、点外面那一下会落到底下。

## 哪些地方用

- 文件树：`ui/sidebar.go` 的 `treeMenu`（空白处）/ `rowMenu`（行）、仓库行、钉在顶上的行、压缩目录的每一段，由 `menuPress` 装。
- 文件树以外用 `ctxMenu` / `ctxMenuRow`（右键）、`menuButton` / `openMenuBelow`（按钮）：
  - Git 栏：段头的右键菜单和分支菜单、变更行 / 贮藏里的文件行 / 未跟踪目录行（owner 是 `app.ChangeKey`，已暂存和未暂存的同一个文件是两份；多选照旧按 `MenuChanges` 算）、贮藏、工作树行和工作树操作、推送按钮的右键、⋯ Git 操作、git 队列、提交卡片的“更多”。
  - 标题栏：分支胶囊和段头的分支、工作区按钮。
  - 编辑器和历史：编辑器标签、打开的编辑器、历史的提交 / 改动 / 仓库行、历史面板的标题行和比较行、本地历史、blame、变基每一步的动作、只读条的“换编码…”。
  - 其他：终端标签、底栏「+」的终端配置、搜索结果和摘要行、搜索编辑器的上下文行数、状态栏的语言 / 缩进 / 换行 / 编码和缩放、克隆框的右键。
- 自己收事件的部件（ged 的 Diff）把右键的按下、拖、松开交给 `menuPointer`。
- 还是 mygo 画的（`ContextMenu` / `Menu`）：
  - 设置窗口里的“更多”（设置、快捷键页）：设置是另一个窗口，没有 `gmenuView`，`App.Menu` 也只有主窗口那一份。
  - 标题栏的面包屑（`dirMenu`，两层子菜单、每层最多 `crumbMenuMax` 80 项）：Grove 菜单的 `menuCollect` 连子菜单一起每帧建，最坏几十万项，要先让子菜单按需建。
  - 编辑器的右键菜单（`d.St.ContextMenu`，mygo 编辑器自己开的，Grove 接不过来）。
- `root` 每帧 `c.SetMenuPresentation(wui.PopoverMenus)`：上面这些还是 mygo 的菜单、文本框和可选中文字的编辑菜单都由 mygo 画在窗口里（mygo 的 `PopoverMenu` 的样子，不是 Grove 菜单的）；菜单栏、程序坞菜单还是系统的。

## 打开的位置

- 右键一行时用 `PopMenu.OpenRow` 开：在指针的横坐标、那一行底下展开，下面放不下翻到那一行上面，不盖住它；比 `MenuRowMax`（64）高的（列表空白处）按指针处开。窗口都放不下才滚。
- 按钮的菜单（`openMenuBelow`）也按 `OpenRow` 开、按钮当“那一行”：下面放不下翻到按钮上面，不盖住它（状态栏）。
- 菜单开着时右键别的登记过的地方，关掉旧的、在那里按 `OpenRow` 开新的（见下面“喂事件和画”）。

## 选中的项什么时候执行

- 选中的项在下一帧的开头、建树之前执行（`root` 里的 `gmenuRunChoice`，和系统菜单在下一帧建到那个元素时执行差不多）：换工作区、重搜这类换掉列表状态的动作要是在树建完以后做，这一帧的布局还拿着旧的列表（mygo 的 `layoutList` 空指针）。
- 建菜单函数里“等列表建完再做”的 `do`（搜索结果）传 `menuDo`（直接做）；打开那一帧捕获的 `do` 交出去的动作没人执行。
- 建菜单的函数每帧重跑（`PopMenu.Refresh`），开着时对着的东西可能没了：按打开时的下标取切片的先核对（搜索结果的 `searchRowLive`、变基的那一步、比较段、克隆），不在了就什么也不加；一项都没有时 `Refresh` 关掉菜单，选中的项也不执行。

## 状态机：`app.PopMenu`

`app/popmenu.go`，每个窗口一个 `App.Menu`，不碰界面。

- 项由建菜单的函数经 `app.MenuSink` 收集（每帧重收，灰不灰跟着状态）。选中一项后再跑一遍建菜单的函数，选中那项的 `Chosen()` 报真（和 mygo 的菜单一样）。
- 位置、命中、子菜单延迟（`MenuSubDelay` 0.2 s）、安全三角、灰项提示（`MenuTipDelay` 0.5 s）、按住拖松开（挪出 4 pt 才算动过，没动过就松开菜单留着）、零滚动不关（`dy` 小于 0.01 不算滚）、键盘都在里面。
- `popmenu_test.go` 直接喂事件测（Tester 按不住右键）。
- 项很多的菜单带筛选框（`PopMenu.Filterable`，界面是 `filterMenuButton`，现在只有状态栏的语言）：
  - 框钉在面板顶上（`MenuLevel.Head`，`gmenuFilter` 画，它不拿键盘，字由接收层交给 `PopMenu.Key`），面板最多露 `MenuFilterRows`（12）项。
  - 打的字用 `internal/fuzzy` 筛第 0 层（`shown`：匹配得好的在前，没有的是一行灰字），第一项高亮、回车选它；退格删字，Esc 先清空再关。
  - 面板的位置和宽按全部项算，筛出来的少时从底下收短，框不挪地方。
- 窗口放不下、要滚的菜单一开就把打勾的项滚进来（`revealChecked`），键盘挪到面板外面的项时跟着滚（`reveal`）；在不用滚的菜单上滚轮不再把它关掉。
- 菜单项在无障碍树里是能按的（`gmenuItem` 那一行的 `Clicked()`）：辅助技术按了走 `PopMenu.Press`，和键盘回车一样选它（有子菜单的开子菜单）。鼠标的按下松开不走这条，还是接收层交给 `PopMenu` 的那一套。

## 喂事件和画：`ui/gmenu.go`

- `menuPress` 给树里的元素挂 `HandleInput`（文件树以外的用 `ctxMenu`，按钮的菜单用 `openMenuBelow` 在按钮下面开）：右键按下（⌃点当右键）打开菜单，按住时的移动和松开（归按下的那个元素）转给菜单。
- `gmenuRunChoice`（`root` 开头）执行选中的项；`gmenuView`（`root` 里、`pending` 之前）画各层面板，开着时在 Overlay 里铺一层全窗口的接收层：点外面关菜单并吞掉这一下，滚动、Esc、失去焦点关，键盘也是它接。
- 接收层盖着整个窗口，事件到不了底下的元素，所以 `menuPress` 每帧把挂了菜单的元素（位置、`menuZ` 算的叠放次序、建菜单的函数）登记给 `PopMenu.AddTarget`。开着时在别处右键（`Down` 的 button 1）按登记的找到那一块（文件树列表的空白处兼作裁剪；文件树以外的 Z 从 `MenuZFree` 起，不受裁剪），关掉旧的、开新的；左键和别的键只关、吞掉。
- 键盘开菜单（⇧F10 / 菜单键）：
  - 文件树有键盘时由列表的 `HandleInput` 接（`openTreeMenuByKey`，用 `TreeSpans` 里光标行的位置），`PopMenu.OpenByKey` 开、第一项高亮；没选就关了，`App.MenuByKey` 让键盘回到文件树。
  - 文件树以外：`ctxMenu` 的元素有键盘、或 `ctxMenuRow` 的行是有键盘的列表的选中行时，每帧登记成 `App.MenuKeyAt`，`gmenuView` 只在有它时认 ⇧F10 / 菜单键（`c.Shortcut`），在元素左下角下面 `OpenByKey`。
- 接收层第一帧不拿键盘、第二帧才 `Focus()`：mygo 在 Overlay 出现那一帧（建完以后）记下当时有键盘的元素，接收层消失时还回去（`restoreFocus`），所以关了菜单（选了、Esc、点外面）键盘回到打开前的元素。
- 样子：字号 13、项高 24、面板圆角 8、内边距 4、项圆角 5，最小宽 180；勾选位只在这一层有勾选项时留（`MenuLevel.TextL`），没有时字左边留 10；高亮项是字色淡填充（浅色 8%、深色 10%）、字色不变（`gmenuHot`）。面板是 `RoleMenu`、项是 `RoleMenuItem`，测试的 `shownMenu` / `chooseMenu` / `closeMenu` 照旧认。

## 磨砂

- 菜单面板是磨砂的（`gmenuPanel`）：`Material(frostMaterial{gmenuTint(t)})`（`ui/frost.go`）把窗口画在它底下的内容糊开（mygo 的 `plugins/glass` 的 `glass.Blur`，和 mygo 同一个模块，模糊半径 `frostBlur` 12），再按圆角盖一层半透明的底色。
- `gmenuTint`：Grove 风格是 `gmenuBg` 的颜色，浅色 44%、深色 60% 不透明；VS Code 风格是 `vsOverlayBg` 盖 `frostAlpha()`。Grove 风格的两个不透明度是能透到的底：底下是主题自己的底色时字（面板上的、高亮项上的）够 4.5:1，底下是最坏的颜色（浅色主题底下全黑、深色底下全白）时也够 3:1，再透就不够了（`TestGroveMenuTintContrast`）。
- 系统的“减少透明度”/“增强对比度”开着时（`gmenuFrosted`）不糊，直接用不透明的底色（`TestGroveMenuFrosted`）。
- 别的浮层也照 VS Code 的 frosted glass 磨砂：`frostFill(c, el, bg)` 在 `canFrost`（没开减少透明度、增强对比度，不透明度不是 100）时给 `el` 上 `frostMaterial`（`glass.Blur{Radius: frostBlur}` 糊开底下，再 `FillRadii` 盖 `bg` 的 `frostAlpha()`，一个 Material 画完，不另铺子元素——铺满的子元素会占 `Gap`），不能时是纯色 `bg`。
  - 用在命令面板（`paletteBg`）、`dialogFrame`（`dialogBg`）、编辑器的悬停提示 `edTipCard` 和补全列表 `edCompletionList`（`widgetBg`）。
  - VS Code 风格下底色是 VS Code 2026 的 menu / quickInput / editorWidget 底（`vsOverlayBg`，`vsQuickBg` / `vsWidgetBg` 都是它，#FAFAFD / #202122）。
  - `TestFrostFill` 在黑白交界上量糊没糊、退没退回纯色。
- 盖多少是设置的“浮层不透明度”（`Settings.Frost`，json `frostOpacity`，50–100，0 是 `DefaultFrost` 80，照 VS Code 默认；以前的 65 让底下糊开的代码一团团带颜色透上来）。取颜色的函数拿不到 App，每帧 `setLook` 把它记进包级的 `frostPct`；100 时 `canFrost` 报假、直接纯色。默认的不透明度下 VS Code 的字色在最坏的底上也有 2.5:1（`TestFrostContrast`）。

## 建菜单的函数：`ui/menus.go`

- 都收 `menu` 接口（`app.MenuSink`），系统菜单（`ContextMenu` / `Menu`）经 `nativeMenu(m)` 适配，所以两边共用。
- 灰项说原因用 `whyItem` / `whySub`：系统菜单括在标签后面（`withWhy`），Grove 画的用悬停提示。
- 破坏性的项（废纸篓、丢弃、删除、强推、中止、硬重置、从列表移除）链上 `Danger()`：Grove 画的用 `t.Danger`，悬停填充也是；系统菜单画不了，什么也不做。
- 文件和目录的菜单顶层 14 项以内（`fileMenu`），不常用的进“更多”（`fileMoreItems`）；组序：打开查看 → 暂存 → 访达和路径 → 复制粘贴 → 重命名、废纸篓 → 更多，`changeMenu` 跟这个顺序。
