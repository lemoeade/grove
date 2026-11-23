# 文件监听和文件树

[← 架构总览](../architecture.md)

每个仓库的文件监听，文件树的模型、交互、外观，以及 Grove 自己画的右键菜单（最早给文件树做的，现在大多数地方都用它）。

## 文件监听

代码在 `internal/watch/watch.go`，每个仓库一个 `Watcher`，包的是 `mygo.WatchFiles` 的 HintsOnly 模式：只要“这个目录里有东西变了”的提示，不留快照、不 stat 文件、不要事件列表。app 这边在 `app/watch.go`（`watchRepo` 开、`watchChanged` 收、`watchStopped` 停）。

### 看哪些地方

- 工作区，递归。不看 `.git`（嵌套的仓库）、`node_modules` 和被 git 忽略的目录。忽略的目录由 `git.Ignored` 读（`ls-files --others --ignored --exclude-standard --directory`，只列最上面那层），开始监听时读一次；之后离上次读超过 `ignoredTTL`（30 秒）再报变化时重读一次（`.gitignore` 可能改了、新建了 `dist/` 之类）。
- git 目录里的 `HEAD`、`index`、`packed-refs`、`refs/`，和 `worktrees/<名>` 那一层（链接工作树各自的 `HEAD`、`index`：新建 / 移除工作树、在工作树里切分支，主仓库的工作树列表要跟着变）；`objects`、`logs` 等不看。
- `.git` 在仓库根下时这些都在工作区那一个监听里；git 目录不在根下（链接的工作树）时另开一个看 git 目录，公共目录也不在时再开一个看公共目录。

### 合并和 `Change`

变更提示按 `Debounce`（300 ms）合并：最后一次变化之后等 300 ms 再报，一直在变时离第一次变化最多 `MaxDelay`（1 s）就报。报出来的是一个 `Change`：

- `Git`：git 目录里变了（提交、暂存、切分支、拉取……）。
- `Files`：工作区里的文件变了。
- `Dirs`：`Files` 且不 `Full` 时变了的目录（仓库内路径，去重，祖先在就不记子孙），最多 `MaxDirs`（200）个，多了就当 `Full`。文件树按它做增量刷新（见下面“增量刷新”）。
- `Full`：说不清哪里变了。系统只给 `.`（根下的文件变了，或系统丢了事件要全看）时 `Git`、`Files`、`Full` 都算上。

app 收到后（`watchChanged`）把它记进 `Repo.hints`（`fileHints.note`），调 `RefreshRepo`；`Files` 时还通知全文搜索、符号索引和工作树块（`SearchFilesChanged`、`symFilesChanged`、`wtTouched`）。

### 三种监听

- `Start`：Git 仓库。
- `StartPlain`：普通文件夹，只看工作区；不看 `node_modules` 和点开头的目录，但根下的 `.git` 照看（只看 `refs/`），这样 `git init` 之后能知道、把它当仓库重读。
- `StartTree`：Git 栏工作树块里展开的链接工作树（`app/wtwatch.go`，一个仓库最多 `WtWatchMax` 16 个），只看工作区，不看 git 目录（它的 `HEAD`、`index`、refs 都在主仓库的 `.git` 里，主仓库自己的监听已经在看）。

### 停了

仓库目录（或它的 git 目录）被删了或移走了，报 `ErrGone`；监听出错或没开起来时 app 在输出栏记一条“文件监听停了”（`watchStopped`），之后靠切回窗口（`RefreshOnFocus` 给监听停了的仓库补读）或手动刷新。刷新的入口见 [Git 的“只读和刷新”](git.md#只读和刷新)。

## 文件树

### 模型

模型是 `internal/files` 的 `files.Tree`（`files.go`，一个仓库一棵），读文件列表在 `app/tree.go` 的 `LoadFiles`（后台读，回主线程 `Tree.Done`）。

- Git 仓库：由 `git ls-files --cached --others --exclude-standard` 建（`git.LsFilesModes`，带 `-s` 拿模式位，认得出符号链接和子模块；未跟踪的文件也在这一次里，去掉 `ls-files --deleted` 列的已删文件）。
- 普通文件夹：`files.Walk` 遍历，最多 `WalkCap`（50 000）个文件；`node_modules` 和点开头的目录不往里走，当被忽略的目录列着。
- 读完还要：并上应用里新建、`ls-files` 列不出来的（`Tree.Extra`：空目录、被忽略的文件，`files.WithExtra`）；去掉套在仓库下面的别的工作树（`Repo.nested`，`dropNested`）；去掉 `files.exclude` 藏起来的（`app/exclude.go`，见 [工作区和设置](state.md)）。
- 树里每个仓库是一段，仓库行贴顶；仓库展开着记在 `Repo.Expanded`（会话的 `open`），展开的目录记在会话的 `dirs` 里（`workspace.Session.Dirs`，仓库根 → 目录）。

#### 增量刷新

代码在 `app/treediff.go`：

1. 监听报的 `Change.Dirs`（和保存的文件）记进 `fileHints`；`.git` 变了、`Full`、或加起来超过 `MaxDirs`（200）个时整棵读。
2. `RefreshRepo` 开始时 `startTreeDiff` 拿着当时的树建一个 `treeDiff`，在后台 `run`：`Tree.Diff` 只对比这些目录（和文件）在磁盘上和树里的不同（不跑 `ls-files`），多出来的文件和新目录问一次 git（`git.Listed`，一个进程）要不要列（被忽略的不列）。新目录里有要列的文件、要问的超过 200 个、树里的目录没了、`files.exclude` 改了、git 出错，都改成整棵读。
3. 回主线程 `applyTreeDiff`：`Tree.Apply` 放进加减。后台计算期间整棵树被重读过（`Gen` 变了）或正在读，结果丢掉。

普通文件夹、开着“显示被忽略的文件”、树还没读完或读失败时不走增量，整棵读。

#### 几种特殊行

- 符号链接：已跟踪的看索引里的 mode，未跟踪的和增量加进来的才看磁盘（`ReadLinks`），得到 `Link{Target, Broken, Dir, Up, Deep}`。指向目录的链接能展开（展开时一层层懒读，`lazyLinks`）；穿过的链接目录已有 `MaxLinkDepth`（8）层（`Deep`）、或指向祖先（`Up`）、或断了的不展开，画成文件行。链接里面的行（`Row.Linked`）没有 git 状态。
- 被忽略的文件：默认不显示；`ShowIgnored` 打开时，被忽略的目录由 `git.LsIgnored` 列出、灰显（普通文件夹不看开关，是 `Walk` 不往里走的那些目录）。展开时一层层读（`Lazy` / `ReadLazy`，一层最多 `FilterCap` 500 项，多的显示一行“还有 N 项没列”）。它们不在 `Paths` 里，快速打开和全文搜索都不含。
- 空目录：git 不记空目录，所以另外找。树先画出来，后台再 `find` 一遍（`FindEmptySkip`：不进被忽略的目录，最多跳过 `MaxEmptySkip` 500 个）补上（`SetEmpty`）。仓库文件数超过 `MaxEmptyScan`（20 000）时不整棵找，展开到哪一层读哪一层（`app/emptylazy.go` 的 `lazyEmpty`）。增量刷新里冒出来的空目录由 `emptyIn` 算。
- 子模块：一行，不展开（`Row.Sub`）。
- 只有一个子目录、没有文件的目录和它的子目录合成一行（`CompactRows`，`Row.Chain`，显示成 “internal / ui”），每段能单独点；设置 `TreeNoCompact` 关掉。

### 交互

#### 筛选、跟随、吸顶

- 筛选：输入框里打字按模糊匹配筛（`files.Tree.Match`，`fuzzy.Pattern.ScorePath`），所有仓库的命中放在一起比分，只留最好的 `FilterCap`（500）个（`app/tree.go` 的 `topHits`，用堆）。筛选结果是平铺的：按仓库分组（每组前面是仓库行），组里按分排，一行一个文件，命中的字高亮（`HitRow`）。
- 跟随：切标签时在树里展开到那个文件、选中、滚到（`FollowCurrent`，设置 `TreeNoFollow` 关掉）；⌥⇧⌘E 手动定位（`RevealCurrent`，命令 `file.revealCurrent`）。
- 吸顶：滚动时仓库行和祖先目录贴在顶上（`stickyStack`，最多 `maxSticky` 3 层），最底下那层往下投阴影（`stickyShadow`：浅色同编辑器的粘性滚动，深色加深），不画边线。筛选时不钉。
  - 吸顶栈读 `TreeSpans`（行上一帧的位置），建完列表后按 `sideShift`（列表上一帧画时的偏移减这一帧的）挪到这一帧，和行同帧。
  - 光标那行被盖住时栈让位（那层和下面的不画），只限光标刚走到的行：`stickyYields` 看 `App.TreeYield`，光标换行后那行落进列表，之后列表再滚动就不让了（不然滚过选中的行时栈整个没了）；整行在列表上面的（列表多建的两行）不让。
  - `TreeRootH` 没量过时先按 `treeRowH()` 算，钉着的那层画出来再量。
- 缩进参考线由 `drawGuides`（`el.Draw`）画，设置 `TreeNoGuides` 关掉。

#### 滚动

`ui/widgets.go` 的 `sideList`，文件树、Git 栏、搜索结果共用；状态机在 `app/sidescroll.go`。

- 列表 `TrackScroll` 一份 `app.SideScroll`（`App.SideScrolls`，按 `*ListState`）。
- 触控板（precise）的交给 mygo；鼠标滚轮的由 `sideWheel` 接（带 ⇧ / ⌃ 的不接）：一格（mygo 的 40 DIP，`WheelLine`）一个行高，系统加速后的 n 行走 n 行、最多 `WheelMaxRows` 6 行；`WheelGlide` 160 ms 三次缓出滑到累计目标，同方向接着累加、反方向从当前位置重来。
- 滑动是每帧建列表前自己写 `ScrollState.Y`（`SideScroll.Step`），不用 `ScrollState.ScrollTo`：后者每帧在布局前把偏移拽回滑动的路上，半路的 `ListState.ScrollTo` / `ScrollIntoView` 会被吞掉。自己写的话 `S.Y` 不是上一帧写的那个就知道别处挪过了：差一行以上是别处滚走了，停下让给它；不到一行是列表量到新行修的偏移，整段滑动跟着平移，接着走。
- 从底往上时每帧至少离底 1 DIP，不然第一帧走得太少会被 `ListState` 拉回底上。
- 文件树的列表 `HandleInput` 被 `menuPress` 换掉，所以它在自己的回调里先调 `sideWheel`。

#### 鼠标点击（issue #46）

- 没选中的文件 / 目录行、没按 ⌘ ⇧ ⌃ 的单击由行自己接（`ui.treePress`，`HandleInput` 接下按下、移动、松开，不交给列表）：
  - 按下那一帧就选中（`PressTreeRow`），不画单独的“按下”底色（比选中深一档的话，点一下先深后浅，看着是闪）。
  - 按着拖到别的行是拖着选（`DragTreePress`，按 `TreeSpans` 找行，不会在列表边上自动滚）。
  - 松开才打开文件 / 展开目录（`ReleaseTreePress`）；那次松开 mygo 下一帧还会报成这一行的单击，`TreeRowClicked` 按 `eat` 吃掉。`eat` 只管松开后紧接着的那一帧（`ApplyTreeClick` 每帧末尾清掉），不按时间算：按住多久再松开都只做一次（issue #50）。
- 双击：只有文件行认（固定打开，`treeDoubleClick` 500 ms 内同一行的第二下）；目录 / 仓库行不认双击，每一下单击都展开 / 收起（同 VS Code、Zed），行自己数的双击（第二下按下时报的 `DoubleClicked`）对目录不算，只看松开的 `Clicked`。
- ⌘ / ⇧ 点、双击、按在箭头上的照旧交给列表 / 箭头；已经选中的行照旧自己接（可拖）。箭头不换手形光标。
- 右键不改选中（原来的多选留着），菜单只对那一行，那一行画 1 pt 强调色描边（`menuRing`：菜单开着、`App.Menu.Owner` 是这一行，`menuOnRow` / `menuOwned`）。

#### 悬停提示

- 行的悬停提示不用 mygo 的 `Tooltip`（0.6 s，写死在 mygo 里）：`app.TreeTip`（`app/treetip.go`）记指针从什么时候起停在哪一行，停满 `TreeTipDelay`（0.5 s）后 `ui/treetip.go` 才在 Overlay 里画，贴在行的下方（`AttachTo`，底下放不下自己翻上去）。
- 暖启动：出过一条以后换一行直接出；指针离开所有行、按下 / 右键以后又要等。点了那行（`Dismiss`）指针离开前不再出。
- 行里自己带提示的小图标（改动的点、链接图标）悬停时让给它们。名字被截断（`cutName` 量字宽）时提示第一行是全名。
- 目录行的提示带大小和最近改动（`app/dirtip.go`）：只 `Lstat` 树里列着的文件，第一次悬停时在后台算，一个目录最多算 `dirInfoCap`（50 000）个文件。

#### 多选和拖放

- 多选（`FileChosen`）：⌘ 点、⇧ 点。
- 树里拖动（`app/treedrag.go`，拖着的是 `TreeDrag`；`DropTree` 决定移动还是复制）：同仓库是移动（`MoveTree`），别的仓库来的是复制（`CopyTree`，先问），按着 ⌥ 是复制（不问）；选中的跨了仓库的不动。移动时目录、多项、有没保存的标签先问。写盘只在松手之后。
- 悬停在收起的目录上一会儿会展开（`DwellTree`）。
- 从访达拖到目录行上是复制进来（先问；本来就在这个仓库里的是移动，`DropFiles`），拖到窗口别处是打开。
- 复制由 `copyIn` / `copyDir` / `copyNode` 实现：`os.CopyFS` 遇到符号链接直接报错，这里链接照原样复制一个、普通文件和目录保留权限位，管道之类不复制、报错。
- 树里的行拖到编辑器组上是在那组打开（`OpenDropped`），拖到终端上是贴绝对路径，都不写盘、不问。
- 别的应用拖进来的（`app/datadrop.go`）：拖到编辑器里插入文字或打开文件（`DropOnDoc`），拖到终端里插入路径（`DropOnTerm` / `ShellFiles`）。
- 仓库行能拖动排序（`RepoDrag`，仓库菜单的上移 / 下移也是），落在 `MoveRepo`：工作区文件里的那组写回 `.code-workspace` 的 `folders` 顺序（`workspace.Reorder`，只换那几项、别的原样，写不了就退回去）；“添加仓库”加的那组记进会话；打开的是文件夹时只改内存。拖动和移除仓库在界面里只发请求（`MoveReq` / `RemoveReq`），`ui/view.go` 的 `pending()` 在列表画完后统一处理，避免在一帧绘制过程中修改列表。

#### 文件操作

- 复制 / 粘贴 / 创建副本（`app/treeclip.go`）：重名时用 `freeName` 起 “名字 副本”“名字 副本 2”（文件的扩展名留在后面，如 “main 副本.go”）。
- 新建、重命名、移到废纸篓在 `app/fileops.go`。
- 文件树的 ⌘Z（`app/fileundo.go`）：重命名 / 移动改回去，新建 / 复制进来的移到废纸篓；移到废纸篓的不记（放回靠访达），没有重做；`undoStale` 发现磁盘在那之后变了就不撤。
- 快速查看：空格（命令 `tree.quickLook`，`QuickLookChosen`）用系统的 Quick Look 面板看光标那一行（文件、目录、仓库行；选了多行也只看光标那行，面板一次一项），右键“更多”里也有。
  - 走 `App.Preview`（默认 `mygo.Shell.PreviewItem`，mygo v0.9.0 起；测试里换掉），只看、不写盘；面板开着时再调是换一项，关它是面板自己的事。
  - 面板是系统的窗口，开着时键盘在它那里：↑↓ 不会带着文件树换行，要换就点回文件树再按空格。
  - 文件树能打字选行（mygo 的 type-select）：空格登记成了快捷键，所以连着打带空格的名字时，空格那一下是快速查看、不进打的字里。

### 右键菜单

文件树的右键菜单（和应用里大多数右键菜单、按钮的菜单）是 Grove 自己画的，状态机在 `app/popmenu.go`、界面在 `ui/gmenu.go`，见 [右键菜单和按钮的菜单](menus.md)。

### 外观

#### 字号和行高

侧栏（文件树、Git 栏、搜索、大纲、打开的编辑器）的字号统一用常量，跟着风格（`ui/look.go` 的 `lookSize`；默认的 VS Code 风格和 Grove 风格见 [外观](look.md)）：

| | Grove 风格 | VS Code 风格 |
|---|---|---|
| 正文 `sideFont()`（文件名、仓库名、变更、结果） | 14 | 13 |
| Git 栏仓库段头的仓库名 `repoFont()` | 14.5 | 13 |
| 文件树目录 / 文件行 `treeRowH()` | 24 | 22 |

- 次要信息 `sideSub` 12（分支、计数、目录、分组标题、说明）、搜索结果的代码行 `sideMono` 12.5，两种风格一样，常量在 `ui/sidebar.go`。
- 吸顶层和“打开的编辑器”的行跟 `treeRowH()` 对齐。
- Git 栏的仓库段头 34、分组标题、贮藏行和工作树组头 26、工作树行 28、展开的工作树摘要行 36。
- 界面整体大小走界面缩放（⌘= / ⌘-），没有单独的侧栏字号设置。

#### 颜色和图标

- 文件按状态上色（改了、新加、冲突、被忽略），`TreeNoStatusColors` 关掉。
- 文件图标默认是 `ui/fileicons.go` 的彩色小方块（按扩展名）；文件树和“打开的编辑器”那一档 `iconTool` 用 `quietGlyphOf`：不画底块、颜色往次级字色混 `quietMix`，一屏几十个不那么重。`TreeIconsPlain` 换成线条图标。
- 目录图标按名字上色、带用途符号（`folderKinds`）。

界面图标是 `ui/icons/` 里的 SVG（Phosphor 2.1.1，MIT，文件名就是图标名），`icons.go` 用 `go:embed` 读，`icon("git-branch")` 取一个。要新图标就把 Phosphor 的 Regular 放进这个目录。

- `lang-*.svg` 是语言的单色图标（Simple Icons，每个 logo 的许可不一样，见 `resources/licenses/simple-icons.txt`；Go、C#、Objective-C、Vue、SQL、proto 是自己画的）。
- 文件夹也是自己画的：`fileicons.go` 的 `folderKinds` 按目录名给颜色和用途符号，文件夹和符号在那里拼成 SVG。
- `TestIconFilesUsed` 查目录里没有没人用的文件。

#### 打开的编辑器

侧栏顶上可以加一段“打开的编辑器”（`ShowOpenEditors`，默认关，`app/openeditors.go`，界面在 `ui/openeditors.go`）：按组列出所有标签（拆分了时每组前面一个小标题），最多显示 `OpenEditorsMax` 9 行高，多了滚；收起状态在工作区的布局里（`Panels.OpenEditorsFolded`）。这一段不另存列表，每帧由 `OpenEditorRows()`（按 `Groups()` / `GroupDocs`）现算。
