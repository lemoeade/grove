# Diff 和暂存

[← 架构总览](../architecture.md) · [Git](git.md)

块级、行级的暂存 / 取消暂存 / 丢弃，补丁怎么拼；Diff 标签（默认是 Grove 自己画的 `internal/ged`）和它上面的就地操作、就地编辑；
编辑器改动条上的就地卡。文件标签里的 ged 单栏编辑器见 [editor.md](editor.md#编辑器部件internalged)。

## Diff 的两条代码路径

- **默认：ged**。Diff 标签、比较标签、替换预览都用 Grove 自己画的 `internal/ged`（`useGed`，`ui/gedview.go`），并排、统一、就地编辑都是它画的。
  文件标签的编辑器默认也是 ged（`useGedEditor`，`ui/gedfile.go`，见 [editor.md](editor.md#编辑器部件internalged)）。
- **退回 mygo**：`WORKBENCH_GED=off`（`gedOn`）让 Diff 退回 mygo 的 `editor.DiffView`；`WORKBENCH_GED_EDITOR=off`（`gedEditorOn`）让文件标签退回 mygo 的 `editor.View`。
  测 mygo 那条路的测试用 `mygoDiff(t)` / `gedEditor(t, false)` 钉住。
- 两条路共用：`docs.Doc.DiffSt`（mygo 的 `editor.DiffState`：两边的缓冲区、`SelectedLines`、`Layout`、`Editable`）、块上的按钮 `hunkActions`、
  选行后的 `linesBar`、`app/hunks.go` / `app/lines.go` 的暂存逻辑。`Doc.UsingGed()`（建过 `Doc.Ged`）为真时，上 / 下一处改动和行级暂存的配对按 ged 的 `Hunks` 走。

## 暂存、取消暂存和丢弃

### 块级（`app/hunks.go`）

Diff 标签里每块改动右上角有暂存 / 取消暂存 / 丢弃按钮（ged 里也在右键菜单里）。

- 能点什么由 `HunkOps` 定：未暂存的改了的文件（`Code == 'M'`）能暂存、丢弃；已暂存的改了 / 改名的（`M` / `R`）能取消暂存。
  未跟踪、有冲突、提交里的，以及新加 / 删掉 / 改了类型的整个文件没有块按钮（在 Git 栏按整个文件做）；比较标签、替换预览也没有。
- `diffContent`（`ui/tabs.go`）据此设 `DiffState.HunkActions`；就地编辑时不放（见下面“就地编辑”）。
- 补丁（`hunkPatch`）按这个 Diff 标签读到的两边原样内容（`Doc.Raw[0]` / `Raw[1]`）算：先用 `git.ReplaceLines` 把那一块套到一边上得到另一份全文，
  再 `git.HunkPatch` 比出只有这一块的补丁：
  - 暂存（`StageHunk`）：旧的一边套上这一块，`git apply --cached -`。
  - 取消暂存（`UnstageHunk`）：新的一边去掉这一块，`git apply --cached -R -`。
  - 丢弃（`DiscardHunk`）：同样去掉这一块，在工作区上 `git apply -R -`；先 `a.AskDanger`。
- 行号对不上（diff 刚重读过）或补丁为空时报“这一块已经和磁盘上的对不上了”。git apply 要上下文对得上才打，磁盘上的文件读完 diff 后又改过时它会失败，不会乱改。

### 行级（`app/lines.go`）

选几行再暂存是行级：点行号选行（⇧ 点扩大到那一行），只暂存 / 取消暂存 / 丢弃选中的那几行。能做的和块上的一样（`LinesOps`），
命令是 `diff.stageLines` / `diff.unstageLines` / `diff.discardLines`，Diff 底下的 `linesBar` 也有按钮。

- `LinesEdits` 把选中的行换成一串 `git.LineEdit`：一块改动里旧的第 i 行和新的第 i 行配成一对（i 小于两边行数少的那个）。
  - 暂存（改旧的一边）：选中新的一行，配了对的换掉对面那行，没配对的插在这一块旧的末尾；选中旧的一行，删掉它。
  - 取消暂存、丢弃（改新的一边）：选中新的一行，配了对的换回对面那行，没配对的删掉；选中旧的一行，配了对的插回它对面那行前面，没配对的插在这一块新的末尾。
  - 选中的行里没改的上下文什么也不做；都是没改的行时 toast “选中的行里没有改动”。
- `git.LinesPatch` 生成只含这些行的补丁，**总是正着打**（不用 `-R`），不用处理反向补丁的上下文问题，也不用再算一份“去掉选中的行”的全文：
  - 暂存：套在旧的一边（索引）上，`git apply --cached`。
  - 取消暂存：套在新的一边（已暂存 Diff 的新的一边就是索引）上，`git apply --cached`，工作区不动。
  - 丢弃：套在新的一边（工作区）上，`git apply`；先问。
- 配对用的块是 `diffHunks`：`UsingGed` 时按 ged 的 `Hunks`，不然是 mygo 的 `DiffState.Hunks()`（两边对齐得不一样时，补丁配的行要和看到的一样）。
- 做完取消选择（`ClearSelectedLines`），Esc 也取消。

### 补丁怎么拼（`git/hunk.go`）

- `HunkPatch(path, from, to)`：`from` 和 `to` 只差连续的一段，去掉相同的头尾，中间就是那一块，交给 `LinesPatch`。
- `LinesPatch(path, old, edits)`：git diff 格式，每处改动一块，上下各带 `HunkContext`（3）行；隔得近（中间没改的不超过 2×`HunkContext` 行）的并成一块。
  `edits` 要按 `Start` 排好、互不重叠，超出范围时返回 ""。没换行的最后一行后面插了东西时，把它也算进改动、补上换行；文件名有引号、反斜杠、控制字符时像 git 一样转义（`patchName`）。
- 行号：Diff 视图按 `"\n"` 分行（CRLF 当一行），`git.Lines` 按 git 的算法分行（每行带自己的 `"\n"`）；以换行结尾的文件视图比 git 多一行空的，那一行不算。

## Diff 标签

### 上 / 下一处改动、上 / 下一个文件

- 上 / 下一处改动（J / K，`diff.nextHunk` / `diff.prevHunk`；标签条按钮、底栏的“第 i / n 处改动”）是 `app/diffnav.go` 的 `HunkPos` / `GoToHunk`：
  `UsingGed` 时问 `Diff.Current` / `Diff.GoToHunk`，不然问 mygo 的 `CurrentHunk` / `GoToHunk`。
- ⌥J / ⌥K 跳下一个 / 上一个文件（`diff.nextFile` / `diff.prevFile` → `StepFile`，`app/hunks.go`）：未提交的改动按 Git 栏的顺序走所有仓库，
  提交里的走那次提交的文件；打开的是预览标签。

### 就地编辑（`app/diffedit.go`）

比工作区那一边的 Diff（未暂存、未跟踪，文字 Diff，文件还在：`CanEditDiff`）标签条上有铅笔开关（命令 `diff.editInPlace`，`ToggleDiffEdit`）。

- 开了就在后面开那个文件的标签（固定），Diff 标签也固定下来，焦点留在 Diff 上；`Doc.EditDoc` 指着那个文件标签。
- `BindDiffEdit` 每帧把 `DiffState.New` 接到文件标签的缓冲区、`Editable` 设真（文件还没读完、不能编辑时先只看）。打的字、撤销、保存都是文件标签的：
  `Save` 在 Diff 上存的是它（⌘S 照用，`app/tabs.go`），标签上的圆点、标签条上的保存、状态栏的“就地编辑 · 没保存”看 `DiffDirty`。
- 编辑时不出块级暂存 / 丢弃、不能选行（磁盘上的和缓冲区里的对不上）；键绑定换成 `DiffEditBindings`（去掉不带修饰键的 J / K，那是要打的字）。
- 再点铅笔、文件标签关了、或者这个 Diff 不再是比工作区的（暂存了），就 `Doc.StopEdit`：新的一边换成一份拷贝再重读磁盘，只看的 Diff 不跟着文件标签里没保存的字变；文件标签留着，没存的改动还在。
- 标签条上“在编辑器中打开”的图标是文件（和 Git 栏行上的一样），铅笔只表示就地编辑。
- ged 那边的打字、撤销见下面“ged 的就地编辑”。

### `diffLook`：每帧的样子（两条路共用）

`diffLook`（`ui/tabs.go`）每帧给 Diff 标签、比较标签和替换预览设样子：

- 并排 / 统一：一边是空的（`oneSided`：新文件、删掉的文件）时不管 `unified` 都是统一的一栏，标签条上的并排 / 统一按钮灰着。ged 跟着 `DiffState.Layout`（`SetUnified`）。
- 软换行：`DiffState.Wrap` 跟 `WrapOf`（⌥Z 的 `canWrap` 也认文字的 Diff），ged 的 `Style.Wrap` 也取它。
- 刚读出来、切了局部 / 全文（`Doc.ToHunk`）时把光标所在那块改动滚进视图；给 ged 的是 `Doc.GedFocus`：正数是要落到的新的一边的行（+1），-1 是刚跳到的那处（还没有就第一处）。
- 底栏 `compareLabel` 对已暂存 / 未暂存写“已暂存的改动”“未暂存的改动”，`HEAD ↔ 索引` 之类放在悬停提示里。
- 下面几项（`Indicators`、`RenderSeparator`）只对 mygo 的 Diff 起作用，见最后“mygo 的 Diff”。

## ged 的 Diff（`internal/ged`）

`internal/ged` 是 Grove 自己的编辑器部件，只用 mygo 公开的接口（`ui.ShapeRichText` 排字、`Painter` 画、`Element.HandleInput` 收输入），
不碰 mygo 的 `ui/editor` 视图，也不认 Grove 的主题和 `internal/ui`。界面那边的接线在 `ui/gedview.go`（`gedDiff`、`gedStyle`、`gedMenu`、`gedKeys`、`gedFindBar`）。

### 模型（`ged/diff.go`，不碰界面）

- `Align` 把两边的行对齐成 `Row`（`Same` / `Changed` / `Deleted` / `Added`）：先去掉头尾一样的，中间用 Myers；超过 `diffMax`（1000）步就不再配，整段当删加。
  一处删 d 行加 a 行时前 min(d, a) 行配成 `Changed`，多的对面留空。
- `Folded` 把离改动 `Context`（3，`ged/view.go`）行外的没改的段折成一行 `Fold`。折叠条上数字后面写 `Style.FoldScope` 给的、下面那处改动所在的函数：
  `changeAfter` 找那处改动在旧的一边的行，界面那边 `scopeAt`（`ui/diffsep.go`）查 `DiffOutline`，和 mygo 那个 Diff 的 `sepScope` 共用。
- `Words` 按词比出改了的行里改了哪几段字节；几乎全不一样时（一样的不到两成）整行算改了，去掉缩进和行尾空白（`whole`）。
- `Hunks` 是一处处改动：对齐后连着的非 `Same` 行，两边的行范围和 `editor.DiffHunk` 一样从 0 起、半开。
- `Unified` 把行排成一栏：每处改动先列旧的一边的行成 `Deleted`，再列新的一边的成 `Added`。

### 视图（`ged/view.go`）

- 一个元素，自己管滚动：`ScrollY` 两边一起走，横着两边各滚各的；`Draw` 里只排、只画看得见的行，整个 `paint` 裁在视图里（最后一行露一半时不画到底栏上）。
- 鼠标滚轮的一格照 mygo 自己的滚动容器在 150 ms 里缓出地滑过去（`ged/glide.go`：建视图时按起点和曲线算这一帧的 `ScrollY`，连着几格接在一起、反方向立刻掉头，
  别的挪了 `ScrollY` 就不滑了）；触控板照旧跟手，“减少动态效果”时一格一下跳。
- 一行排好的 `TextLayout` 按（哪边，第几行）留着，换字体 / 颜色时作废。语法高亮用语言的 `Tokenizer`，每行开头的分词状态按需往下算。
- 改了的字的底按 `TextLayout.SelectionRects` 量位置、涂满行高；对面留空的行画 45° 斜线（`StrokePolyline`，`Painter.Line` 只画横竖）。
- 右边 14 pt 的概览条（`rulerW`）标改动，按住拖动滚。
- 状态 `ged.Diff` 存在 `Doc.Ged`，`Set` 按两边缓冲区和版本（`gedText`）认内容变没变；`SetFull` 跟着 `St.FullDiff`（全文时不折）。
- 颜色由 `gedStyle` 按 Grove 的主题给：底是和 mygo 那个 Diff 一样的 `diffAddedLine` / `diffDeletedLine`、`diffTint`；VS Code 风格（默认）下换成 VS Code 2026 的 Diff 颜色，
  `PlainNumbers` 让改了的行号不变色，括号按层上色（见 [外观](look.md)）。
- 最后一行下面能滚出来三行（`Diff.tail`）。

### 改动上的按钮、上 / 下一处

- 指针下的那块（没有时是跳到的那块）右上角的按钮由 `Style.HunkBar` 画（Grove 的 `hunkActions`，和 mygo 那个 Diff 共用；只在 `DiffState.HunkActions` 时给）。
  它是绝对定位的子元素，滚动时贴在块顶；位置在 `View` 里按这一帧的滚动现算，`Bounds()` 只取大小（上一帧的位置跳了一处改动后就不对了）。查找条开着时往下让。
- J / K 由 `Style.Keys`（`gedKeys`）按命令注册表的键（`diff.nextHunk` / `diff.prevHunk`）接走；Esc 先关查找条。
- `applyWant` 在 `View` 里把要去的行（`want`）、恢复的顶（`wantTop`）、软换行开关时留住的行（`wrapAt`）换成 `ScrollY`。

### 选字和选行

- 选字是（哪边，行，字节）的一对位置，`HandleInput` 管拖选、双击选词（`tokens`）、三击选行、⇧ 点击；`InputCommand` 的 copy / selectAll 走 `Style.Copy`（`app.CopyText`）。
- 元素 `Focusable`，按下时拿焦点；方向键、翻页、空格、Home / End 滚动，Esc 取消选择。
- 按在行号、+ − 上（`inGutter`，正文左边）是选行：`Style.Lines` / `SetLines` 读写的就是 `DiffState.SelectedLines`（和 mygo 的 Diff 共用一份），
  所以 `linesBar`、`StageLines` / `UnstageLines` / `DiscardLines` 不用改。只在 `DiffState.LineSelection` 时给。
- 拖动扩大（`picking`，到上下边自动滚），⇧ 点从起点（`anchor`；不是在这里选的从原来的头）扩到那一行；选中的行涂 `LineSel` 再加左边一道 `LineMark`。
- 选中的字（一行之内、不全是空白、`occurMax` 200 字节以内）每帧记成 `occ`（`occurText`，`ged/find.go`），画每一行时 `occurrences` 找出一样的几处、涂 `Code.Occurrence`（分大小写，选的那处自己不涂）。

### 统一视图

- `SetUnified`（跟 `DiffState.Layout == DiffUnified`）：`refold` 把行再过一遍 `Unified`。
- 两边的格子合成一整行宽（`paneX` 都是 0），左边两列行号（`numR` 旧、`numR2` 新）；一行的字是哪边的由 `rowSide` 定（删掉的行是旧的，别的按新的）。
- 行里改了的字按 `pairs`（`Set` 时从 `Changed` 行记下的旧 ↔ 新行号）找对面那一行再比；一处改动在行里的范围是 `hunkRow` / `hunkEnd`。
- 选行、选字照旧是（哪边，第几行），按下时那边按这一行的 `rowSide`，没改的行两边都算选中。

### 并排中间那道线

`Diff.split`（左边占的比例，0 是一半，夹在 15%–85%，`splitMin`）由 `splitInput` 管：按住拖改、双击还原、指针在线上换成左右拖的样子；
`geom` 按它分两边，折行宽跟着变。不记进 state.json（VS Code 也不记）。

### ged 的就地编辑（`ged/edit.go`）

- `DiffState.Editable` 时 `gedStyle` 把 `Style.Buf` 设成 `DiffState.New`（就是文件标签的缓冲区，`BindDiffEdit` 接的），`Style.Paste` 读剪贴板（`app.ReadClipboard`），
  `Style.Bindings` 是 `app.DiffEditBindings()`，`Style.Indent` 是 `a.DocIndent(d)`。
- 光标就是新的一边选中的字的 b 头（`caret`；a == b 时是一个点），点、拖、⇧ 点、双击照旧，打字替换选中的字。没选字时光标那一行垫 `Code.ActiveLine`，行号用 `ActiveLineNumber`。
- `editKey` 接方向键、⌫ / ⌦、回车（照抄缩进）、Tab、Home / End、翻页。不接的键（没按 ⌘ / ⌃、不是 Esc）`input` 直接返回 false、不问 `Style.Keys`，
  好让随后的 `InputText` 到它这里（所以有光标时 J / K 是字，不跳改动）。
- `editInput` 接字、组字（`InputCompose`，组着的字只在 `layout` 里插进那一行、不缓存）、编辑菜单的 cut / paste / delete / undo / redo。
- 改用 `replace`。`Buf.Edit` 一次一步撤销，mygo 合并打字的没导出，所以 ged 自己记每一组合了几次（`undoGroups`：`typeGroup` 1 s 内同类的连着打 / 连着删合一组；
  缓冲区版本对不上——别处改过——就清掉）；`undo` 按组连调几次 `Buf.Undo` / `Redo`，光标放到 `changedEnd`。
- 改完马上 `reload` → `update`：旧的一边没变时留着折叠展开、旧的一边的排版、选择，只扔新的一边的排版和分词状态。
- 光标不闪（静止时不多画帧，见 [mygo-pitfalls 编辑器一节](../mygo-pitfalls.md#编辑器)）；`paintCaret` 记下 `caretBox`，`View` 里 `el.TextCaret` 交给输入法；
  `revealCaret` 在 `View` 和 `Draw` 里把光标滚进视图（在折叠里就展开）。

### 编辑键（`ged/keys.go`）

- `command` 按 `Style.Bindings`（改过的键）再 `editor.DefaultBindings` 认出单键对应的 `editor.Command`，`runEdit` 自己做：挪行、复制行、删行、缩进、注释、接行、对调、⌘L、⌘D（ged 只有一个光标：
  没选字时选中光标处的词，选了字时选中下一处一样的）、配对括号……
- ged 不做的命令（多光标、补全……）交给 `Style.Fallback`（只有单栏编辑器给），没给时返回 false，键交回 `Style.Keys` / 窗口。
  和弦（⌘K ⌘L 这种）和折叠只在单栏编辑器里认，Diff 里不认。
- 改几行的命令都走 `rewriteLines`：一次 `replace` 换掉那几行（一步撤销），选择两头按那一行在哪处多 / 少了字挪。
- 缩进一级（`indentUnit`）按 `Style.Indent`：自动时看文件里 Tab 多还是空格多、空格最少几个，再没有按语言。
- 打字走 `typeChar`：跳过右括号、补另一半（`autoPairs` 是语言的 `AutoClose`，`NotIn` 按 `tokenAt` 的词种跳过字符串、注释里的引号）、括起选中的字、右括号对齐到配对那一行（`alignCloser`）。
- 配对括号 `matchBracket` / `scanBracket` 只认单字的括号、最多看 `bracketScan`（5000）行，每帧在有光标时算一次（`brk`），画成 `Code.MatchingBracket` 的框。
- 空白按 `Style.Whitespace`（设置的「显示空白」）由 `paintSpaces` 画。

### 软换行（`ged/wrap.go`）

- `Style.Wrap`（`a.WrapOf(d)`，⌥Z）开着时 `layout` 按那一格正文的宽（`setWrapWidth`，右边那格让出概览条）排，`lineLayout.n` 是折成几行。
- 一个 `Row` 占几个“屏幕行”（`lh` 高）按两边折得多的那边算（统一视图按它那一边）。`tops` 是前缀和：没排过的行按格数估（宽字两格），排出来和估的不一样就标 `topsDirty` 下一次重算，
  所以看得见的那段是准的、上面没看过的按估的。
- `top(i)` / `rowIndex(v)` 在不换行时就是 i / v，滚动、点、画、概览条、上 / 下一处都改问它们；折出来的第 k 行画在 `top(i)+k`（`rowY`），`posAt` 按折出来的行问 `IndexAt`，
  ↑ ↓ 先在一行折出来的几行里走（`vertical`）。
- 开关时滚动位置留在顶上那一行（`wrapAt`）。

### 查找和替换（`ged/find.go`）

- `Diff.Find` 是查找条的状态（开没开、输入框的字），条本身由 `Style.FindBar`（`gedFindBar`）画、`View` 摆在右上角（贴着的“暂存这一块”往下让）。
- `SetFind` 每帧拿输入框的字来比，变了才重找（`search`：只折 ASCII 的大小写，按对齐的顺序记新的一边每一行、旧的一边改了的行，最多 `findMax`（20000）处），当前那处挑视口往下的第一处。
- `FindStep` 前后跳、绕回，`revealMatch` 展开折叠、把底栏的那处改动跟过去，`revealFind` 在 `View` 里滚过去。
- `edit.find` / `edit.findNext`（`findIn`）在当前标签画着 ged 时（`findGed`：Diff 的 `UsingGed`，或单栏编辑器的 `UsingEd`）开查找条、跳下一处，不然照旧给 mygo 的编辑器。
- 替换：`Find.Replace` / `With` 是查找条下面那一行（只在能编辑时有）；`edit.replace`（⌥⌘F，`replaceIn`）开 `OpenReplace`；`ReplaceOne` 只换新的一边的当前那处（在旧的一边时先跳），`ReplaceAll` 一次 `rewriteLines`。

### 右键菜单（`ged/menu.go`）

- `Style.Menu` 先拿到右键（⌃点）的按下、按着拖、松开；按下前 `menuAt` 记好 `Spot`（哪边、哪行、哪处改动、跳到文件时去新的一边哪行），按在选中的字外面时选择挪过去（就地编辑是放光标）。
- 界面那边 `Style.Menu` 是 `menuPointer`（`ui/gmenu.go`，`menuPress` 收事件的那一半）。它不登记 `AddTarget`：菜单开着时在 Diff 别处右键只关掉，因为接收层盖着、ged 记不了新按在哪。
- 菜单是 `gedMenu`（`ui/gedview.go`）：撤销 / 重做（就地编辑）、剪切 / 复制 / 粘贴、折叠条的展开、这一块的暂存 / 取消暂存 / 丢弃、选中的行的、复制这一行（旧的一边是“复制旧版本的这一行”）、查找、上 / 下一处、在编辑器中打开这一行、统一 / 全文 / 忽略行首行尾空白 / 软换行 / 就地编辑、全选、路径（`pathItems`）。
  对着的是右键按下的那一处，不一定是当前标签，所以动作都按 d 做（软换行用 `ToggleWrapOf(d)`）。
- 剪切、复制、粘贴、全选经 `Diff.Run` 走和编辑菜单一样的 `InputCommand`。
- ⇧F10 / 菜单键由 `KeySpot` 按光标 / 当前那处改动定位置，`MenuKeyAt` 的 Build 先 `SetSpot`。

### 折叠条分步展开

- `Diff.open` 按那段没改的行在旧的一边的第一行记 `Reveal`（`Top` 接着上面的改动往下露几行、`Bottom` 接着下面的往上露、`All` 整段）。
- `foldRows` 折的时候让出这些行，顺便按 `Fold` 行的 `Old` 记 `foldInfo`（这段的键、上 / 下挨不挨着改动）。
- `ExpandMore(i, dir)` 一次露 `FoldStep`（20）行，剩的不比它多就整段；点条中间整段展开。
- 箭头（`foldArrows`）画在行号那格里，`foldZone` 认点在哪个上，悬停的那个垫一层底（`hoverZ`）。
- 右键记 `Spot.Fold`，菜单按 `FoldDirs` 给“往下 / 往上展开 20 行”。

### 缩进参考线和粘性滚动

- 缩进参考线（`ged/guides.go`，`Style.Guides`）：`prepGuides` 每帧按 `indentUnit` 定一级几格、就地编辑时找光标那一块（光标那一行最里面那道线、上下缩进比它深的连续行）；
  `paintGuides` 在每个格子的正文第 0 格起每级画一道 1 pt 的线（`Code.IndentGuide`，那一块的是 `ActiveIndentGuide`）；空行看上下 `guideScan`（200）行内最近的有字的行（取浅的再深一级，不超过深的）。
- 粘性滚动（`ged/sticky.go`，`Style.Sticky`，设置里关了就不给）：最多吸 `stickyMax`（5）行、不超过视图高的三分之一。`prepSticky` 每帧问调用方每一边（统一视图只问新的一边）顶上吸 k 行时下面那一行是第几行
  （`rowLine`，对面留空的往下找）；界面那边 `gedStyle` 用 `Doc.DiffOutlineOf(side)`（两边各解析一份，换了缓冲区或版本才重来）的 `outline.Tree.Sticky` 给。
  `paintSticky` 在那一栏顶上盖编辑器的底、行号、排好的那一行（只画第一行）和 3 px 渐淡的阴影（`StickyShadow`），`stickyInput` 悬停垫一层、按下滚到那一行刚好露在还吸着的几行下面。

### 忽略行首行尾空白

- `workspace.State.DiffTrim`（命令 `diff.ignoreTrim` / `ToggleDiffTrim`，存进 state.json）每帧经 `SetIgnoreTrim` 给 ged，`AlignTrim` 拿去掉首尾空白的行去配。
- 只改了空白的行是 `Same` 但两边的字不一样，画的时候各画各的；查找按 `sameText` 决定旧的一边借不借新的一边的结果。
- 改动、补丁都按看到的 `Hunks` 走，补丁的上下文取自要套的那一份，所以藏起来的空白改动不碍事。
- mygo 那个 Diff 不认这个开关（命令在没用 ged 时灰着）。

### 跳到行

`edit.goToLine`（⌃G）在 `gedFind` 时开命令面板的 ":"（`goToLine`）。`lineItems` 对 ged 的 Diff 按 `Ged.LineCount` 写共几行，`JumpHere` 交给 `Diff.GoToLine`
（新的一边，空的时旧的一边；`rowOfLine` 展开折叠，`want` 滚过去）。

### 测试

`internal/ui`（`gedview_test.go`）：

- `TestGedDiff`：在临时仓库里开 Diff，看折叠、点折叠条展开、滚轮、切全文。
- `TestGedHunks`：上 / 下一处，暂存这一块只暂存那一块。
- `TestGedSelect`：拖选、⌘C、双击、Esc、J、翻页。
- `TestGedLines`：拖着选行、Esc、点一行 ⇧ 点一行、暂存选中的行只暂存那两行。
- `TestGedUnified`：切到统一后一栏、上 / 下一处、点删掉的行的行号选旧的一边、暂存选中的行，再切回并排。
- `TestGedEdit`：就地编辑的光标、打字、一步撤销、回车缩进、J 打进去、组字、剪切粘贴、⌘S 存盘。
- `TestGedFoldScope`：折叠条写的函数。
- `TestGedWrap`：软换行后长行变高、短行不变、点第二截放光标、↑ ↓ 在折出来的行里走、关了还原。
- `TestGedFind`：⌘F 开条、数处数、回车跳到折叠里那处会展开、Esc 关。
- `TestGedMenu`：右键开 ged 的菜单、记的位置、只看时没有剪切粘贴、复制这一行（两边）、菜单里暂存这一块只暂存那一块、⇧F10 开菜单再选查找。
- `TestGedFoldExpand`：开头那段只有往上的箭头、点箭头露 20 行、中间那段两个箭头、右键折叠条的菜单、剩的不多时不画箭头、点条中间整段展开。
- `TestGedTrim`：菜单里勾上忽略空白后只剩真改了的那处、暂存这一块只暂存那一行。
- `TestGedGoToLine`：⌃G 开 ":"、跳到折着的第 150 行会展开并滚进视图。
- `TestGedOccurrences`：选中一个词后两边别处一样的词都涂、选的那处自己不算。
- `TestGedSplit`：拖中间那道线到 30% 处、右边正文跟着左挪、不选字，双击还原，再就地编辑放光标（截图 ged-caretline.png 看光标那一行）。
- `TestGedEditKeys`：用真键看 ⌥↓、⌘/、⇧⌘K、⌘]、打 ( 补 )、撤销，再 ⌥⌘F 替换一处、全部替换（截图 ged-keys.png 看括号框和空白、ged-replace.png 看替换条）。

`internal/ged`：

- `TestUnified`（排成一栏的顺序和折叠）、`TestAlignTrim`、`TestFoldReveal` / `TestExpandMore`（折的时候让出露的行、剩的不多整段展开）、`TestOccurrences`（全是空白、跨行的不涂）。
- `TestChangedEnd` / `TestWordMoves`（撤销后的光标、按词移动）、`TestChangeAfter`（折叠条找的那处改动）。
- `TestEditCommands` / `TestAutoClose` / `TestEditCommandUndo` / `TestCommandLookup` / `TestMatchBracket` / `TestReplace`。

暂存本身：`git` 包 `TestHunkPatchApplies` / `TestHunkPatchOneOfTwo` / `TestHunkPatchFormat` / `TestLinesPatch`；`app` 包 `TestLinesEdits`、`TestDiffEditInPlace` / `TestDiffEditBindings`、
`TestPeekHunkAt` / `TestPeekRevert` / `TestPeekStage`；`ui` 包 `TestHunkOps`、`TestDiffEditUI`。

## 文件标签的 ged 编辑器

文件标签默认也用 ged（`ged.NewEditor` 的单栏模式，`ui/gedfile.go`）：用不用它、和 mygo 的 `editor.State` 怎么对齐、补全和悬停、改动竖条、键和右键菜单、折叠、小地图见 [编辑器部件（internal/ged）](editor.md#编辑器部件internalged)。这里只说它和暂存有关的一处：有改动的行点行号（`Style.GutterClick` → `a.TogglePeek`）开的就地卡和 mygo 编辑器共用 `peekCardAt`（`edPeekCard` 按 `CharBox` 算卡顶），见下面“编辑器改动条和就地卡”。

## mygo 的 Diff（`WORKBENCH_GED=off`）

`useGed` 为假时 Diff 走 `restingDiff`（`ui/caret.go`，包着 `editor.DiffView`）。下面这些只在这条路上起作用：

- `Indicators` 是 `DiffBars`（行首红 / 绿竖条）：mygo 只在有竖条时把改了的词涂成红 / 绿，别的是灰的。
- 改了的行的底色 Grove 自己画（`ui/difftint.go`）：mygo 混进背景的比例固定、太淡，`restingDiff` 让它不涂底（`clearDiffBackground`：`NoBackground`，`Theme.Background` 透明），
  外面包一层编辑器底色的 `Column`，`Draw(diffUnder)` 每帧按 `DiffState.Hunks` / `LineTop` / `ScrollY` 现算位置（`diffTints`，`TestDiffTints` 盯着）涂 VS Code 默认主题的 `#9bb95533` / `#ff000033`，
  选中的行也在这里画。竖条、行号和改了的词还是 mygo 画在上面，颜色是 `diffTint` 换的 `Theme.Added` / `Deleted`。
- 折起来的未修改行由 `RenderSeparator = diffSep`（`ui/diffsep.go`）画成整行一条淡底，左边“N 行未修改”；点了走 `expandSep`（开头 / 结尾的往改动那边展开 `Step` 行（mygo 默认 100），中间的、不到 `Step` 行的或按住 ⇧ 全部展开）。
- 折叠条后面的字是 `sepScope`：下面那段改动第一行（间隔开头 + 上下文 + 藏着的行数 + 上下文）所在的符号那一行。`Doc.DiffOutline` 是旧的一边的大纲（按缓冲区和版本缓存），
  `outline.Tree.Scope` 找包着那一行的最里层符号。mygo 只给折叠条的行数和在不在头尾，间隔从哪行开始是没导出的 `DiffSeparator.a`，`sepStart` 用 reflect 读，读不到就不写（`TestSepStart` 盯着）。
- 块上的按钮是 `DiffState.RenderHunkActions`（同一个 `hunkActions`），选行是 mygo 自己的 `LineSelection`。
- 改动在滚动条槽里的标记是 mygo 自己画的；空出来那边的斜纹和 mygo 自带的折叠条样式在 mygo 里，Grove 不改。

## 编辑器改动条和就地卡（`app/peek.go`、`ui/peek.go`）

编辑器行号旁的改动条可以点开看那块的原文：点改动条（或改动块里的行号）、或命令 `diff.peekHunk`，在这一块下面弹一张卡。

- 状态和动作在 `app/peek.go`：`Peek`（开着的那张卡：哪个标签、第几块）、`DocHunks`、`HunkAt`、`TogglePeek`、`SyncPeek`、`PeekStep`（卡上的 ↑ ↓）；卡画在 `ui/peek.go`（`peekCardAt`）。
- 接点击：mygo 编辑器上 `peekSetup` 设 `St.OnGutterClick`（有改动块时接管整条装订线，块外的行号照旧选中那一行）；ged 编辑器上是 `Style.GutterClick`，卡由 `edPeekCard` 摆。
- 卡上能做的：
  - 还原（`PeekRevert`：`DiffState.Revert`，直接改缓冲区，⌘Z 能撤，不问）。
  - 暂存（`PeekStage`）：要先保存；补丁按 HEAD 里的原样字节和磁盘上的算，同样走 `ReplaceLines` + `HunkPatch`，`git apply --cached`。暂存区里这个文件已有挨着的改动时 git 可能不接，报出来。
  - 在 Diff 中打开（`PeekOpenDiff`：未暂存的优先；设 `Doc.HunkLine`，`diffLook` 用 `HunkFromLine` 滚到新文件里同一行所在的那块，Diff 比的是暂存区，块的序号不一定一样）。
- 块是编辑器的 `DiffState{Old: d.Base, New: d.Buf}` 算的（和 HEAD 比，历史版本和父提交比），和改动条同一种算法。`DiffState` 按标签留着（最近 `peekCacheMax` 4 个），
  缓冲区改了 mygo 只重比改动附近的行，不每次整篇 diff（装订线接管点击每帧都要问有没有块）。
- 编辑器里的块随缓冲区重算，`SyncPeek` 跟着挪：块没了就关；选区变了，光标停在某块开头（上 / 下一处改动、卡上的 ↑ ↓）卡跟过去，不然关掉。
- 卡是叠在代码上的（盖住块下面几行），mygo 编辑器和 ged 编辑器都是。想像 VS Code 那样插在两行之间、把下面的代码推下去，mygo 那边要普通编辑器（`editor.State`）能在行间留一块高度给调用方画；
  现在只有 mygo 的 Diff 查看器有（`DiffState.Annotations` + `RenderAnnotation`，内部是 `blockAnnotation`），`State` 没有对应的口子。
