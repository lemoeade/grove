# 编辑器、搜索和本地历史

[← 架构总览](../architecture.md)

打开的文档和标签、编辑器组、格式化、跳转，Grove 自己画的编辑器部件（`internal/ged`），跨仓库搜索和替换，本地历史。

## 编辑器

文档、标签、编辑器组和围着编辑器的功能（格式化、检查、大纲、跳转、补全、悬停……）是 Grove 自己的（`internal/docs`、`internal/app`、`internal/ui`）。

画编辑器的部件有两套：

- 文件标签默认用 Grove 自己画的单栏编辑器（`internal/ged` 的 `ged.NewEditor`，`ui/gedfile.go`），Diff 默认用 ged 的 Diff（`ui/gedview.go`），见下面的“编辑器部件（internal/ged）”。
- 其余情况还是 mygo 的 `editor`：十六进制、只读的大文件、只读或编码不可写的文件、有冲突标记、开着逐行追溯栏、超过 5 万行的文件，以及历史版本、设置文档、搜索编辑器、终端输出文字这些不是可编辑文件的标签；`WORKBENCH_GED_EDITOR=off` / `WORKBENCH_GED=off` 时也全退回 mygo 的。

不管画的是哪一套，`docs.Doc` 里都有 mygo 的 `editor.State`（`Doc.St`）和缓冲区；用 ged 时 `St` 照样在，光标、选中的字、缓冲区和 ged 对齐着，App 的代码照旧问 `St`（见下面的“单栏编辑器和 `St` 的对齐”）。

### 文档和标签

`docs.Doc` 表示一个打开的文档，`Kind` 有：文件、历史版本（只读）、Diff、交互式变基、替换预览、搜索编辑器、文件比较、编辑器区的终端、设置文档、终端输出文字、并排的 Markdown 预览（`MDPreview`，见 [markdown.md](markdown.md)），另有图片和十六进制两种显示。除了文件，别的都只在内存里。

标签规则（`app/tabs.go`）：

- 单击打开是预览标签（斜体），下一个预览会替换它；双击、回车、编辑以后固定。被替换的文件记下光标和最上面那一行（`noteView`），再打开时接着看（`resumeView`，只在这次运行里）；`jumpIn` 指明了行的不用它。
- 读文件、历史版本和 diff 都在后台；写盘只在用户保存、或者在关标签 / 退出的确认框里选了保存之后。
- ⇧⌘T 重开关掉的标签，‹ › 前进后退（`Back` / `Forward`，⌃- / ⌃⇧-），同一个文件里跳行也算一步。
- 切当前组的标签（`app/tabswitch.go`）：
  - ⇧⌘[ / ⇧⌘]、⌥⌘← / ⌥⌘→ 按标签条顺序走、到头绕回（`StepTab`）。
  - ⌃Tab / ⌃⇧Tab 按最近使用走（`StepTabByUse`），顺序来自 `visit` 里记的 `tabMRU`（只在内存里，不存 state.json）。⌃Tab 是每按一次切到上一个用过的（连按在两个标签之间来回），还没有按住 ⌃ 时的标签列表浮层。
  - 键盘在终端里时这几个键先给终端标签（注册表里 `term.prevTab` 等排在前面）。
- 外部改了文件：没改过的标签直接重读；改过的标出冲突，让用户选保留自己的还是重读。

### 文件大小

| 大小 | 怎么开 |
|---|---|
| 小于 `MaxDiffBaseBytes` 2 MB | 正常，带改动条（和 HEAD 比） |
| 到 `MaxEditBytes` 8 MB | 可编辑，不与 HEAD 比较（`NoBase`，标签上显示提示），因为改动条的 diff 由 mygo 的 `DiffState` 在主线程上算 |
| 到 `MaxViewBytes` 64 MB | 只读打开（`Doc.Big`，用 mygo 的编辑器），后台高亮 |
| 更大 | 不打开，显示原因说明 |

#### 读文件

读文件在后台（`docs.Loaded`），读好了 `setText` 换上。

- 读分两步，在同一个后台 goroutine 里：先 `ReadFileText` / `ReadVersionText` 读文字、`a.Update` 放上（这时 `Doc.BasePending`），再 `Loaded.LoadBase` 读 HEAD（历史版本：父提交）里的那份、`ApplyBase` 放上。
- 读回来的是不是还是那一版看 `Gen`：`Gen` 变了（重读了）的结果和基准丢掉，重读时槽里先留着原来的基准。`readDoc` 的 `fresh` 参数是另一回事：为真时在后台建新的缓冲区，不在主线程上 `Reset` 旧的。
- 新开的文件 / 历史版本 / Diff 标签（`Loading`）主线程在 `readDoc` 里最多等 `quickLoad`（40 ms）：后台读完先试着把“放上去”经无缓冲 channel 交给等着的主线程，同步放上（预览换文件时编辑区不空一帧，issue #50）；没人等了（超时）就照旧走 `a.Update`。
- 基准不在这一步等，还是后台接着读。没读到之前和仓库里没有基准的（未跟踪）拿文字自己当 `St.DiffBase`，占着装订线的改动条那一条（`holdGutter`），基准放上时代码不横跳。
- 标签读了 `loadingDelay`（150 ms）还没读完才置 `Doc.SlowLoad`、画“读取中…”，在那之前只露编辑区的底。
- 只读的大文件外部改了，过 2 秒（`bigReloadGap`）再重读（`reloadBig`），避免日志持续写入时频繁重读。

#### 大文件的高亮

在 `internal/highlight`：mygo 自带的高亮器在主线程上每帧追 4 ms，64 MB 要追两三秒。这里一个 goroutine 从头分词、记下每行开头的状态，主线程只按记好的状态分看得见的几行；没算到的行先按状态 0 猜一份（`final` 为假）。`Highlight` 总是返回 true；编辑时停掉旧的 goroutine，从改动那行接着算。

### 编码

`docs/encoding.go`。读的时候自动认（`Decode`）：

- 带 BOM 的 UTF-8 / UTF-16 去掉 BOM；没 BOM 的 UTF-16 看奇偶位的 NUL（`sniffUTF16`）。
- 有 NUL 的算二进制；合法 UTF-8 原样。
- 否则在 GB18030、Big5、Shift-JIS、EUC-KR 里挑（`pickCJK`，阈值按 `encoding_test.go` 的样本调过）；都不行是未知，当 UTF-8 只读。

老编码（GB18030、Big5、Shift-JIS、EUC-KR、EUC-JP、Windows-1252、ISO-8859-1）要编回去和原来逐字节一样才能编辑（`Writable`，不一样标 `Lossy`）；保存按原编码写，有写不进去的字先问。手动“用别的编码重新打开”（`ReopenWithEncoding`）解不通的标 `Broken`、只读；选的编码记在会话 `Tab.Enc`。“转成 UTF-8…”（`ConvertUTF8`）换掉编码，下次保存按 UTF-8 写。

缓冲区里的换行一律是 `\n`，写盘时按原来的换回 CRLF；状态栏能换整个文件的换行（`Doc.SetEOL`）。

### 保存

保存前的整理（`docs/tidy.go`，设置里两个开关 `TrimTrailing` / `FinalNewline`）：去行尾空白、补末尾换行，作为一步撤销；光标所在的行不去，Markdown 里行尾正好两个空格的硬换行留着。

自动保存有三种模式：关闭、窗口失去焦点时（`blur`）、停止输入 1 秒后（`delay`）。所有文件写入都通过 `WriteDoc` → `diskWrite`，本地历史也在这里记录（见“本地历史”）。

### 格式化和检查

格式化和检查均调用外部命令，不内置工具，也不通过 LSP。命令在 login shell 里跑（`Shell` 设置或 `$SHELL`，`-lc`），这样 Homebrew 装的工具在 PATH 上；工作目录是仓库根，仓库外的文件是文件所在目录；超时整个进程组一起杀。

**格式化**（`app/format.go`）：⇧⌥F、右键或保存时（`FormatOnSave`）。

- 将缓冲区的 UTF-8 文本传入命令的 stdin，读 stdout，和原文按行比（`docs.LineEdits`，每处收窄到真正不同的那几个字），仅应用有变化的部分，作为一次可撤销操作，并保留光标、折叠和滚动位置。
- 运行期间文本再次变化时，不应用结果。
- 默认命令在 `DefaultFormatters`（按 `editor.Language.Name`），`Settings.Formatters` 能改，`"-"` 关掉；`{file}` 换成引号括好的绝对路径。

**检查**（`app/lint.go`，界面在 `ui/problems.go`）：保存时（设置“格式化”页的“保存时检查”，默认关）或手动“检查当前文件”。

- 默认命令在 `DefaultLinters`（go vet、tsc、eslint、ruff、cargo check、shellcheck），`LinterOf` 先看设置里的 `Linters`；`{file}` / `{dir}` 两个占位符，`LintTimeout` 60 秒。
- 它读的是磁盘上的文件，所以保存那一刻记下缓冲区的版本（`snapOf`）。
- 输出按 `路径:行:列: 信息` 解析（`ParseProblems`），设进 `St` 的 diagnostics（`SyncProblems`：先清掉，再按记下的版本设；对不上版本就用现在的文字重算位置）。mygo 的编辑器自己画；ged 的编辑器从 `St.Diagnostics()` 取来画成波浪线（`edSquiggles`），悬停提示里也列出来。
- 底栏第三个标签“问题”列出所有仓库的结果（按仓库、文件、行排），状态栏显示错误和警告的数目。

### 大纲和粘性滚动

`internal/outline`：

- Markdown 认标题；Go 用 `go/parser`；Python / JS / TS / Rust / Shell / YAML 按行用正则认；C / C++ / Java / Kotlin / Swift / C# 按花括号嵌套认（`brace.go`）。
- `Parse` → `Build` 摊平成 `Tree`，`At(line)` 找光标所在的符号，`Symbol.End` 是到哪行结束。
- 超过 `MaxBytes` 1 MB 不算，最多 `MaxSymbols` 2000 项。
- 侧栏的大纲模式在 `ui/outline.go`。

粘性滚动把视图最上面那几行所在的、还没结束的符号的头一行叠在编辑器顶上，最多 `StickyMax` 5 行（`Tree.Sticky` 算哪几个）。设置 `NoStickyScroll` 关掉（`ui/sticky.go` 的 `stickyOn`）。

- mygo 的编辑器：`ui/sticky.go` 的 `stickyEditor` 在编辑器上面叠一层。
- ged 的编辑器：`gedFile` 把 `Doc.StickyOutline` 的结果经 `Tree.Sticky` 交给 `ged.Style.Sticky`，ged 自己画（见下面 ged 一节）。
- 超过 `StickyQuick` 256 KB、或上次解析超过 `StickySlow` 1 ms 的文件，打字时先用旧的大纲，停手 `StickyPause` 500 ms 才重算，避免打字时掉帧。

### 跳转和命令面板

⌘P 的面板（`app/palette.go`）按前缀分模式：

| 输入 | 内容 |
|---|---|
| 空 | 最近用过的 |
| 文字 | 所有仓库的文件（`fuzzy`） |
| `名字@符号` | 某个文件里的符号（`app/palettesym.go`） |
| `名字:行[:列]` | 某个文件，打开后跳到那一行 |
| `@` | 当前文件的符号 |
| `#` | 工作区符号（`app/symindex.go`） |
| `>` | 命令 |
| `>git ` | 跑一条 git 命令 |
| `>term ` / `>snip ` | 用某个终端配置开新终端；插入代码片段 |
| `>restore ` | 从本地历史恢复已删除的文件 |
| `!` | 任务 |
| `:行` | 跳到行 |

工作区符号的索引第一次用时建：后台几个 worker 读盘、解析，分批交回主线程；之后监听报的目录标脏，下次用时只重新 stat 脏的、版本变了才重新解析。开着的文件用标签的大纲（没保存的改动也算）。索引不常驻、不存盘，每个窗口一份，最多 20 万个符号（`maxIndexSymbols`）。

### 编辑器组

`app/groups.go`。主区能左右 / 上下分成几组，可以嵌套，每组一条标签条。拆分树是 `EdNode`（叶子是一组，否则是左右或上下的两半和比例），存进会话的 `layout`。

#### 标签条

- 标签条（`ui/tabs.go` 的 `tabStrip`）和底栏的终端标签（`ui/bottom.go`）共用 `scrollStrip`：横向滚动、自己在底边画 2 pt 的细条。
- 滚动位置分别在 `Group.TabStrip` 和 `Repo.TermStrip`，换了当前标签滚到看得见（`Group.TabShown` / `Repo.TermShown`）。
- 每个标签是 `panelTab`（`ui/widgets.go`）：通栏平铺、同高 `headerHeight`、没有圆角，右边一条 `cardLine`，当前的用卡身底色盖住卡片头底下那条线；第一个贴着卡片左边（卡片头左 padding 0）。VS Code 样式下当前标签顶上另画一条 1 px 的线（`vsActiveLine`）。

#### 当前组和别的组

当前有焦点的组，其标签保存在 `a.Docs` / `a.DocSel` 里，所以“当前标签”的代码不用管组；别的组的标签放在组自己那里，换组时两边对调（`swapTo`）。要遍历所有标签用 `AllDocs()`。

#### 拆分和拖动

- 一个标签同时只在一组里：拖标签、⌘\（向右拆分）、⌥⌘\（向下拆分）都是把当前标签搬过去（`SplitEditor`）。
- 拖标签时落在一组的中间是搬进来，离某条边不到 1/4 是在那边分出新的一组（`DropSide` / `DropRect`）；一组不到两倍最小尺寸（160×120）就不让分。
- 组内最后一个标签移走后，收起该组（`collapse`），由相邻组填满空间。

#### 镜像标签

同一个文件要在两组里同时看，用镜像标签（`app/mirror.go`，⇧⌘\）：和正本共用缓冲区和对比基准，光标、滚动、折叠和查找状态各自独立。读盘、保存和切换编码等操作只对一份文档执行，再通过 `relink` 同步到其他镜像。打开文件总是找正本（`Copy` 为 0）。

#### 最大化底栏和禅模式

最大化底栏（`app/bottommax.go`，⇧⌘J）：`App.BottomMax`，每个窗口独立、只在内存里；`ui/view.go` 的 `center` 在 `BottomMaxed()` 时不画主区和底栏旁边的拖条，底栏 `Grow(1)` 占掉那块，`St.BottomH` / `St.TermW` 不动。关底栏（`ToggleArea` / `ToggleProblems`）、进禅模式、切到或打开编辑器标签（`SelectDoc` / `FocusGroup` / `MoveDocTo` 里的 `showEditor`）时还原。

禅模式（`app/zen.go`，⌃⌘Z）：三个面板全关，编辑器组居中；每个窗口独立，不持久化，存 state.json 时写进入前的面板状态。连按两下 Esc 退出。

#### 起始页

编辑组里没有标签时画起始页（`ui/startpage.go`）：

- 内容：工作区名和 `reposSummary`，有改动的文件（每个仓库最多 5 个，点了 `OpenDiff`）、`St.Recent` 里还在工作区里的文件（`RepoOfPath`）、不是主工作树的工作树（`WtInfo` 算过的才写状态）。
- 行是 `ButtonBase`，`FocusBind(&a.StartFocus, 第几行)`，页面的 `OnShortcut` 里 ↑↓ 按 `FocusedValue` 挪；行的键由调用方给（同一个文件可以既暂存又未暂存，名字会重）。

### 跳到定义、补全、悬停、查找引用

这几样都不通过 LSP。文件标签用 mygo 的编辑器时由 mygo 的 `State.OnDefinition` / `OnCompletion` / `OnHover` 回调进来；用 ged 的编辑器时由 `ui/gedfile.go` 接（⌘点击走 `Style.Definition`，补全列表和悬停提示 Grove 自己画），后面查什么、怎么排是同一套 App 代码。`Doc.ShowCompletions` / `ShowHover` / `Run` 等按 `UsingEd()` 分给 ged 或 `St`（`docs/edit.go`）。

#### 跳到定义

`app/definition.go`。⌘点击、F12、右键菜单调 `GoToDefinition`：

- `identAt` 取光标下的标识符和 `pkg.` 限定词。
- 在开着的文件的 `Doc.Outline()` 和符号索引（`app/symindex.go`）里找同名的声明（标题、分组不算），`narrowDefs` 按限定词 = 目录名、同文件、同目录缩。
- 一处直接 `OpenAt`（固定标签，记进 ‹ › 历史），几处开面板 `#名字`，`Palette.defs` 在时只列同名的（`definitionItems`）。
- 文件列表没读完或索引在建时记进 `App.defWait`，`LoadFiles` 读完（`defFilesLoaded`）和 `mergeSymbols` 建完最后一批（`defIndexed`）时再看：标签、内容版本、光标都没变才接着跳。

#### 补全

`app/wordcomplete.go`。`Complete` 把片段（`snippetCompletions`）和词（`wordItems`）一起 `ShowCompletions`。

- 词来自：这个文件光标前后 128 KB（近的在前、大纲里有的带种类；种类看 `Doc.LastOutline()`，不在每个键上整篇解析）；开着的同一族语言的文件（`langFamily`；按标签和内容版本缓存在 `App.words`，超过 1 MB 不扫）；同一仓库索引里的名字（索引已经建过才用，补全不去建）。
- 前缀开头的在前，其次按字的顺序出现的；不给正好等于前缀的。
- `Settings.NoWordComplete` 关掉敲字时的词（⌃Space 照样给），Markdown / 纯文本（`docs.IsProse`）只在 ⌃Space 时给。
- mygo 的编辑器：mygo 的回车会接补全，所以 `CompleteTick`（每帧画编辑器前）在打完一个整词时把列表里同名的那项拿掉重给；mygo 没有读补全状态的接口，退格时不重给。
- ged 的编辑器：列表在 `Doc.EdCompl`，`ui/gedfile.go` 画；打了字母数字后由 `Style.Changed` 触发，↑↓ 选，回车 / Tab 接受，Esc 关。

#### 悬停提示

`app/hover.go` + `ui/hover.go`。找声明和跳到定义同一套（`definitions` + `narrowDefs`，停在声明那一行上不给）；索引没建好时这次不给，顺手 `EnsureFiles` / `refreshSymbols` 让它去建。

- 触发：mygo 的编辑器是 `State.OnHover`（指针停 600 ms，或命令 `editor.showHover`；mygo 默认的 ⌘K ⌘I 被菜单栏的 ⌘K 先接走，见 [docs/shortcuts.md](../shortcuts.md)），同步调 `HoverAt`——mygo 只认当场的答复，晚到的 `ShowHover` 会挂在已经关掉的提示上。ged 的编辑器是 `Style.Hover`（指针停 500 ms），`edHover` 调 `HoverAt`，再加上那个位置的检查问题，`edTipCard` 画。
- 声明的源码：开着的文件读缓冲区，别的读磁盘（超过 1 MB、二进制不读，只给位置）。
- `declText` 从声明行起取到括号配平、截在括号外第一个 `{` 前，最多 6 行；`commentAbove` 往上取紧挨着的行注释 / 块注释（跳过 `@注解`、`#[属性]`，空行断开），Python 先看声明后面的文档字符串。
- 提示的字是声明（编辑器的字体），下面是注释和位置的链接按钮、同名的“还有 N 处”（点了走 `GoToDefinition` 开面板）。
- `Settings.NoHoverDef` 关掉声明（mygo 的编辑器不装 `OnHover`），检查报的问题照样悬停显示。

#### 查找引用

`app/references.go`。`editor.findReferences`（⇧F12，编辑器的键表里没有，落到窗口的命令）和编辑器右键菜单调 `FindReferences`：光标下的名字做成整词、区分大小写的 `search.Query`，“包含”是同一族语言的扩展名（`referenceGlobs`，没有扩展名的不限），清掉 `Search.Scope` 搜整个工作区，结果就是侧栏搜索。

### 其他

- 状态栏的语言、缩进、换行、编码都能点（`app/docstatus.go`），改了的语言和缩进只对这个文件、记进会话。
- 代码片段（`internal/snippets`）：内置的加状态目录里的，Tab 展开，Tab / ⇧Tab 跳位。
- 图片：预览、和 HEAD 的图片 Diff；Markdown 和 SVG 用 ⇧⌘V 在源码和预览之间切（`TogglePreview`）；`app/image.go` 的 `ToggleHex` 切十六进制视图，也能用默认应用打开。
- Diff 有并排和统一两种，全文 / 只看改动两种（`unified` / `fullDiff`）；ged 的 Diff 还能忽略行首行尾空白（`diffTrim`）。
- 两个磁盘文件、或文件和剪贴板比较：`app/filecompare.go`（先“选择以比较”，再“与已选的比较”）。
- 代码配色（`ui/occur.go` 的 `codeTheme` / `softTheme`，每帧设进 `St.Theme`）：VS Code 样式整套换成 `fullCodeTheme`；Grove 样式只改两处，其余跟 mygo 的默认——选中里画出来的空白符号（点、箭头）用 `TextMuted` 的 0.22（mygo 默认 0.45），运算符和标点用 `TextMuted`（`softTokens`）。

#### 只在 mygo 的编辑器上的

下面几样是给 mygo 的编辑器补的，ged 有自己的做法（见下一节）：

- 键盘移动光标后，编辑器让光标上下各留 min(5 行, 视图高 1/5)（`ui/caret.go` 的 `caretMargin`）。mygo 自己只留 3 行；点击和滚轮不受影响。ged 挪光标只滚到刚好看得见（`revealCaret`）。
- 光标在标识符上（没有选区、只有一个光标）停稳 250 ms（`ui.OccurRest`）后，本文件里同名的其他出现（整词、区分大小写）铺上 mygo 的 `Occurrence` 底色（`ui/occur.go` 的 `occurMarks`，写进 `St.TextBackgrounds`，状态在 `Doc.Occur`）；打字、换词马上清掉。
  - mygo 自己只在有选区时标选中文字的其他出现，也没有“光标下的词”这个选项，所以用 `TextBackgrounds` 在 Grove 这边做。
  - 按内容版本和词算一次，大于 `StickyQuick` 的文件只搜看得见的行、滚了重搜。搜索编辑器（底色是搜到的那几处）和十六进制视图不标。
  - ged 不画 `St.TextBackgrounds`；它标的是选中的字的其他出现（见下一节）。
- 编辑器和 Diff 中的光标在停止操作一段时间后不再闪烁（`ui/caret.go` 的 `restingEditor` / `restingDiff`，`WORKBENCH_CARET_REST=off` 关掉）。mygo 的编辑器有键盘时每 500 ms 排一帧闪光标，而 mygo 要 2 秒没有新帧才放掉整窗的帧内存，因此持续闪烁会额外占用一两百 MB 内存，并持续消耗 CPU。ged 的光标本来就不闪。

## 编辑器部件（internal/ged）

`internal/ged` 是 Grove 自己画的编辑器部件，一个类型 `ged.Diff` 管两种用法：

- **Diff**（`ged.NewDiff`）：并排、统一、就地编辑新的一边。Diff 标签、替换预览、文件比较默认用它（`ui/gedview.go` 的 `gedDiff`）。
- **单栏编辑器**（`ged.NewEditor`）：`plain` 模式，每行都是“相同”、新旧是同一份文字，只画一栏。文件标签默认用它（`ui/gedfile.go` 的 `gedFile`）。

它只用 mygo 公开的排字、画图和输入接口（`ui.ShapeRichText`、`Painter`、`HandleInput`），只 import `mygo/ui` 和 `mygo/ui/editor`（缓冲区、选择、语言分词、键表），不认 Grove 的主题，也不碰 `internal/ui`：颜色、字体、键表、要画的额外东西都由调用方经 `ged.Style` 给。Diff 那一面的内部（对齐、折叠、词级高亮、改动块、选行、统一视图、就地编辑、折行、查找）在 [staging.md](staging.md) 里讲，这里讲单栏编辑器和两边共用的部分。踩过的坑见 [../mygo-pitfalls.md](../mygo-pitfalls.md) 的“编辑器”一节。

### 文件

| 文件 | 管什么 |
|---|---|
| `diff.go` | Diff 的模型：`Align` / `AlignTrim` 对齐、`Folded` 折叠、`Words` 词级高亮、`Hunks`、`Unified`，`Row` / `Span` / `Hunk` 等类型 |
| `view.go` | `Style`（调用方给的全部配置）、`View(c, key, d, s)` 画和收输入、每行的排字缓存和分词、滚动、光标 |
| `edit.go` | 就地编辑：改缓冲区、撤销分组、`Caret` / `PlaceCaret`、`revealCaret` |
| `editor.go` | 单栏编辑器：`NewEditor`、`SetText`、折叠、选择、`VisibleLines` / `ScrollToLine` / `TopLine` 等给 App 问的、`Command` / `Lookup` |
| `keys.go` | 键表和编辑命令 |
| `find.go` | 查找和替换、选中的字的其他出现 |
| `wrap.go` | 自动换行（折行），每行占几屏行的前缀和、没排的先估 |
| `sticky.go` | 粘性滚动 |
| `minimap.go` | 小地图 |
| `guides.go` | 缩进参考线 |
| `marks.go` | 装订线的改动条、波浪线、行尾的注记（行内追溯） |
| `glide.go` | 鼠标滚轮的平滑滚动 |
| `menu.go` | 右键菜单要的位置和动作：`Spot` / `MenuSpot` / `KeySpot`、`Run`、`CanEdit`、`HasSelection` |

### 界面怎么配置

ged 不认主题，`internal/ui` 每帧拼一个 `ged.Style` 交给 `ged.View`：

- `gedStyleBase`（`ui/gedview.go`）：两种用法共用的底子——代码配色（`fullCodeTheme`）、编辑器字体、Grove 样式的增删颜色、折叠条的字（“%d 行未修改”）等。
- `gedStyle`：Diff 用，在底子上加 Diff 的底色和增删色（VS Code 样式另有一套词 / 行颜色、空白填充、括号上色）。
- `gedEditorStyle`（`ui/gedfile.go`）：单栏编辑器用；VS Code 样式下行号用 VS Code 的颜色、括号按层上色（`bracketColors`）、开小地图。
- `gedFile` / `gedDiff` 再按标签填行为：缓冲区（`Buf`）、剪贴板（`Copy` / `Paste`）、键表（`Bindings`：`app.EditorBindings()` / `app.DiffEditBindings()`）、缩进（`a.DocIndent`）、空白显示、参考线、折行（`a.WrapOf`）、粘性滚动、查找条（`gedFindBar`）、右键菜单（`Menu`）。
- 单栏编辑器另有：`Intercept`（⌥点击加光标等）、`Changed`（改了以后同步回 `St`、触发补全）、`Fallback`（ged 不认的命令）、`Hover`、`Definition`（⌘点击）、`Changes`（改动条，来自 `app.DocHunks`）、`GutterClick`（点改动条开 peek，`a.TogglePeek`）、`Squiggles`（`St.Diagnostics()`）、`Notes`（行内追溯 `d.BlameInlay`）、`Extra`（`St` 的其他光标）。

### 什么时候还用 mygo 的

- Diff：`useGed(d)`（`ui/gedview.go`）是 `WORKBENCH_GED` 不为 `off` 且有 `DiffSt`；否则画 mygo 的 Diff（`restingDiff`）。
- 文件标签：`useGedEditor`（`ui/gedfile.go`）要求 `WORKBENCH_GED_EDITOR` 不为 `off`、是文件、有缓冲区、不是十六进制、不是只读的大文件（`Big`）、`St` 不是只读、`Doc.Editable()`（不是 `Lossy` / `Broken`、编码可写、不是本地历史的快照）、没有冲突标记、没开逐行追溯栏、不超过 `gedEditorMaxLines`（5 万行）。不满足就按原来的走：开着追溯栏是 `blameEditor`，开着粘性滚动是 `stickyEditor`，否则 `peekEditor`（`ui/tabs.go`）。
- 别的 `Kind`（历史版本、设置文档、搜索编辑器、终端输出文字）都是 mygo 的编辑器。

### 单栏编辑器和 `St` 的对齐

用 ged 时 `Doc.Ed` 是那个 `*ged.Diff`，`Doc.EdOn` 是上一帧画的是它（`UsingEd()`）。

mygo 的 `editor.State`（`d.St`）照旧在、没有视图也能用（`Selections` / `GoTo` / `Insert` / `Run` 都只碰缓冲区），所以缓冲区、光标和 St 对齐着，
App 那边问光标、`St.GoTo` / `St.Select`、代码片段、词补全、格式化、拖进来的字都不用改。对齐两条：

- 每帧画之前 `edPull`：St 的选择和上次对齐时（`Doc.EdSel`）不一样就是 App 挪过光标，主的那个 `SetSelection` 给 ged（滚进视图），别的光标当 `Style.Extra` 画。
- ged 接了事件以后 `Style.Changed` → `edPush`：ged 的选择变了就 `St.SetSelections`（只剩一个）。

其余：

- 多光标由 St 做：St 有不止一个选择时 `Style.Intercept` → `edMulti` 把打字（`St.Insert`）、按键（`Diff.Lookup` 查成命令，`edRunMulti` 在 St 上 `Run`，剪贴板走 Grove 的）交给 St，Esc 只留一个；⌥点击 `PointAt` 加一个。
- ged 自己不做的命令走 `Style.Fallback`（`edFallback`）：跳行开 ":"、补全、跳到定义、显示悬停自己接，其余的交给 `St.Run` 再 `edPull`。
- 看得见的行、滚到某行、执行编辑器命令、开补全 / 悬停提示经 `Doc.VisibleLines` / `ScrollTo` / `Run` / `ShowCompletions` / `ShowHover`（`docs/edit.go`）分到 ged 或 St，
  App 里原来直接碰 St 视图的地方都改走它们；`findGed` 让查找、替换、跳到行在 `UsingEd` 时也给 ged。
- ⌘点是 `Style.Definition`（跳到定义）。

### 补全和悬停提示

- 补全列表（`edCompletionList`）和悬停提示（`edTipCard`）在 `gedFile` 那层 Row 里按 `CharBox` 摆。
- 打标识符里的字时 `Changed` 里 `edComplete` → `a.Complete` → `Doc.ShowCompletions`（`Doc.EdCompl`）；`edCompletions` 先按前缀、再按子序列筛；
  ↑ ↓ 回车 Tab Esc 在 Intercept 里接，接受是 `St.Select` + `St.Insert`（片段照旧由 `SnipTick` 认）。
- 指针停稳（`HoverDelay`）`Style.Hover` → `edHover`：`a.HoverAt` 的声明加上盖住那处的诊断（`edDiagnosticsAt`），指针在卡上时不关（`Doc.EdOut`）。

### 画和滚动

- 每行一份排好的 `TextLayout` 缓存，分词用 mygo 的 `lang.Tokenizer`，按行记分词状态和括号层数（括号上色用）。一行超过 `lineMax`（4000 字节）只排前面那段。
- 光标 2 pt 宽，不闪，没有计时器排帧；光标所在的行垫一层底色（VS Code 的 current line highlight），没有键盘时也画。
- 键盘挪光标只滚到刚好看得见（`revealCaret`）。
- 末尾能滚过最后一行（`tail()`）：单栏编辑器在内容比视图高时多留 max(3 行, 视图高减一行)，也就是最后一行能滚到顶上（VS Code 的 scrollBeyondLastLine）；Diff 多留 3 行。
- 鼠标滚轮（`glide.go`）：不是触控板的一格在 `glideTime` 150 ms 里缓出地滑过去，连着几格接在一起、反方向立刻掉头；别的挪了滚动位置（触控板、键、拖滚动条、预览带着）就不滑了。触控板照旧跟手；系统设置里“减少动态效果”时一格一下跳。
- 同一帧里既有恢复的顶（`wantTop`）又有要露出来的光标（`want`），光标在滚好的视野里就不再为它挪。
- 自动换行在 `wrap.go`，跟 Diff 共用。
- 测 mygo 编辑器自己的东西（它的 FindBar、`LineTop` / `ScrollY`、`TextBackgrounds`、它画的粘性滚动行）的测试钉在 `gedEditor(t, false)` 上。

### 折叠

只在单栏编辑器上（`editor.go`）：

- 规则：行尾是开括号的，折到配对的闭括号的前一行；否则折起后面缩进更深的那些行。
- 折起的行后面画“⋯”；指针移到装订线上才出折叠箭头（`foldGutterW` 16）；光标进了折起的范围自动展开。
- 命令：`Fold` / `Unfold` / `ToggleFold` / `FoldAll` / `UnfoldAll` / `FoldRecursive` / `UnfoldRecursive` / `FoldAt` / `UnfoldAt`，`FoldImports` 折前 200 行里第一段能折的导入（以 import / use / from 开头的行）。
- Diff 的“未修改的行折起来”是另一套（`Folded`，见 [staging.md](staging.md)）。

### 粘性滚动

`sticky.go`：最多 `stickyMax` 5 行，视图矮时不超过视图高的 1/3。哪几行叠在顶上由调用方经 `Style.Sticky` 给（单栏编辑器用大纲，Diff 用 `DiffOutlineOf` 每边各一份，统一视图用新的一边），ged 只管画和点了跳过去。

### 小地图

`minimap.go`，只在单栏编辑器上、`Style.Minimap` 为真（`gedEditorStyle` 只在 VS Code 样式下开）、编辑器宽不小于 `minimapMinW`（480）时画：右边 `minimapW` 100 宽，每行 2 点高、每字 1 点宽。当前一屏的滑块一直淡淡地画着（照 VS Code 的 `minimap.showSlider: always`），指针在上面深一档、拖着再深一档；按滑块外把滑块挪到指针下再拖。按词的颜色画细条（每行按字和分词状态缓存）。

比编辑器高时按比例跟着滚（编辑器滚到底它也到底）；`geom.right` 是右边让出的宽，正文、粘性滚动、横滚、软换行的宽都按它算。`ged` 包里 `TestMinimap`。

### 参考线、括号、空白

- 缩进参考线（`guides.go`，`Style.Guides`）：光标所在的那一块深一档。
- 括号按层上色（`Style.Brackets`，只在 VS Code 风格里给，Light+ / Dark+ 的三种色；ged 的 Diff 在 VS Code 风格里也给）：每行开头的括号层数和分词状态一起缓存（`depths` / `stateAt`），`walkBrackets` 跳过字符串、注释里的括号。
- 空白显示（`Style.Whitespace`）跟设置。

### 查找和替换

`find.go`：

- 查找条由调用方经 `Style.FindBar` 画（`gedFindBar`）；ged 管匹配、跳上 / 下一处、替换一处 / 全部。不区分大小写时按 ASCII 折。
- 选中一行之内、不全是空白、不超过 `occurMax`（200 字节）的字时，别处一样的（区分大小写）都涂上底色（VS Code 的 selection highlight）。

### 装订线和行上的标记

`marks.go`（画）和 `ui/gedfile.go`（给数据），单栏编辑器用：

- 行号右边的改动竖条是 `Style.Changes`（`edChanges` 从 `app.DocHunks`：加的绿、改的蓝、删的在下一行顶上一个红三角，`ged/marks.go`）。
- 有改动时点那一行的行号 `Style.GutterClick` → `a.TogglePeek`，卡是和 mygo 编辑器共用的 `peekCardAt`（`edPeekCard` 按 `CharBox` 算卡顶），见 [staging.md](staging.md) 的“编辑器改动条和就地卡”。
- 诊断（`St.Diagnostics`，`SyncProblems` 照旧设）画成 `Style.Squiggles` 的波浪线；行内 blame（`Doc.BlameInlay`）是 `Style.Notes` 的行尾淡字。

### 键、和弦和右键菜单

- 单栏时 ged 和 mygo 的编辑器一样把键、字、按下先给 `Style.Keys`（`docKeys`：片段跳位的 Tab、⇧⌥F 和它跟着送来的 ⌥ 字），再自己处理。
- 和弦（⌘K ⌘0 这种）由 ged 的 `command` 按键位表认（第一个键返回 `chordPending`，只在单栏编辑器里）；⌘K 开头的被菜单的“运行 git 命令…”先拿走，和 mygo 的编辑器一样。
- 右键菜单是 Grove 画的（`gedFileMenu`，测试用 `shownMenu` / `chooseMenu`），照 VS Code 编辑器的右键菜单只到命令面板：跳到定义、查找引用；更改所有匹配项、格式化文档；剪切、复制、粘贴；命令面板。
  拆分、本地历史、Finder、复制路径和 VS Code 一样在标签的右键菜单（`tabMenu`），保存、检查、blame 在菜单栏和命令面板；mygo 编辑器的菜单照旧接 `docFileItems`。
- 编辑器自己的命令（跳到定义、更改所有匹配项）不在菜单栏的注册表里，键按 `edBindingOf` 从 `app.EditorBindings()` 再 `editor.DefaultBindings()` 找（只要一下的，不要和弦）；
  一个命令有几个键时挑 `edMenuKeys` 里 VS Code 菜单写的那个（更改所有匹配项 ⌘F2，`TestEdMenuKeys`）。
- `menu.go` 给出点在哪一行哪一列（`Spot` / `MenuSpot`，键盘打开菜单时 `KeySpot`）和能不能编辑、有没有选中，菜单本身由调用方画；Diff 的是 `gedMenu`（见 [staging.md](staging.md)）。

## 搜索和替换

### 搜索

`internal/search` 只负责搜索计算。候选来自 `git ls-files`（或文件树已经读到的列表），减去 `files.exclude` 和包含 / 排除 glob；每个 CPU 一个 worker。字面 / 整词 / 正则、区分大小写都在一个 `Matcher` 里。

- 默认跳过二进制和超过 `MaxFileSize` 2 MB 的文件；勾“也搜大文件”放宽到 `MaxBigFileSize` 32 MB，勾“也搜被忽略的”加上被忽略的文件（`Query.Big` / `Ignored`）。
- 最多 `MaxMatches` 5000 处，“再读 5000”往上加，到 `MaxLimit` 50 000 为止。
- 不是 UTF-8 的文件按 `docs.Sniff` 认编码、解成文字再搜，所以位置就是编辑器缓冲区里的位置（`Match.Enc`）。
- 多行的字面词转成正则，换行写成 `\r?\n`。`Match.Len` 只到行尾，`Match.Span` 是整个匹配的长度，`CR` / `BufLen` 用来把 CRLF 文件的字节位置换成缓冲区位置。
- 正则先用 Go 的 RE2；编不了又用了反向引用或环视时换 `regexp2`（`advanced.go`），一次匹配限时 2 秒（`advTimeout`），超时的文件当跳过、结果上标出来（`Result.TimedOut`）。

应用端（`app/search.go`）：停止输入 150 ms 后开始搜索，新搜索会取消上一次搜索；结果按文件分组。完成一批搜索后调用一次 `debug.FreeOSMemory()`，把候选列表和读过的文件内容的内存还给系统。

- 最近搜索 `SearchHistory`（最多 20 条，带开关和 glob），框里 ↑↓ 翻。
- 范围：右键目录“只在这个目录里搜”（`SearchScope`，`Root.Sub`）。
- 排除：结果里点 × 去掉一处或一个文件（`DismissMatch` / `DismissFile`），替换时跳过。
- 跟随：结果里的文件改了，停 `FollowDelay` 300 ms 只重读这些文件再搜（`app/searchfollow.go`），不整个重搜。
- ⇧⏎ 把搜索框换成多行的（`multiLine`）。
- 搜索编辑器（`app/searchdoc.go`）：把结果开成一个只读的文本标签，带上下文行，回车跳到那一处。

### 替换

`search.Plan` 先核对每一处还在原位（文件可能刚改过），再算出改动；正则替换里的 `$1` 由 `Expand` 展开。

`app/replace.go` 的 `ReplaceMatches`：

- 开着的文件改缓冲区（能撤销、不存盘），没开的直接写盘（经本地历史）。
- 替换后不整个重搜：只对换过的文件按新内容在内存里重新匹配，`splice` 换进结果，被排除的也跟着挪（`moveDismissed`）。
- 替换操作记录在输出栏。

替换预览（`app/replacepreview.go`）：打开一个 Diff 形式的标签，左侧为原文，右侧为替换后的内容；预览开着时文件又改了，标出“过期”并能重生成。

“撤销上次替换”（`app/replaceundo.go`）只处理直接写入磁盘的文件：

- 记下替换前的内容（一次最多 `MaxUndoReplace` 20 MB，超出时本次替换无法撤销），撤销时只恢复替换后未再次改动的文件。
- 只记录最近一次操作，不持久化；已打开的文件通过编辑器的 ⌘Z 撤销。

## 本地历史

`internal/history` 只负责存储：`Store{Dir, Now}`，每个文件一个目录 `<状态目录>/history/<路径 sha256 的前 12 位>/`，里面 `index.json` 加内容文件（按毫秒时间 `T` 命名）。每条有 `Why`：保存、保存前磁盘上的、恢复前、改名自。

- 和上一条“保存”隔不到 `MergeWindow` 10 秒就替换它，避免自动保存产生大量记录。
- 超过 `MaxSnapshot` 256 KB 的文件不记，每个文件最多 `MaxEntries` 50 条，全部加起来最多 `MaxTotal` 200 MB。
- `Prune(days)` 删过期的（默认 30 天，`historyDays`）、超总量的从最早删起；启动后 `PruneDelay` 30 秒在后台做一次。`noLocalHistory` 整个关掉。

`app/localhist.go` 决定什么时候记：

- 文件写入统一通过 `diskWrite`（保存、全部保存、格式化后保存、自动保存、转成 UTF-8、替换都走它）。
- 写之前磁盘上那份还没记过就先记一条“保存前磁盘上的”（第一次保存，或者别的程序改过），写成功后记写进去的那份。
- 状态目录里的、`.git` 下面的、图片、`files.exclude` 藏掉的不记（`keepsHistory`）。
- 应用里的重命名和移动跟着挪历史（`localMoved` → `Store.Move`）。

右栏的文件历史里，本地历史的条目和提交按时间混排（`Hist.Local`）。点一条开比较标签（`OpenLocalCompare`，左边是那一条、右边是现在的文件，`docs.Doc.Snapshot` 记着是哪条），能恢复（`RestoreSnapshot`，恢复前先记一条“恢复前”）。误删文件可通过“从本地历史恢复文件…”（命令面板 `>restore `）找回（`RestoreDeleted`）。
