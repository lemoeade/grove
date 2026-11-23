# App、命令和窗口

[← 架构总览](../architecture.md)

一个窗口一个 `App` 的状态机、命令注册表和快捷键、多窗口、启动和退出、设置 / 关于窗口、版本和自动更新。

## App 和命令注册表

### App

`app.App` 保存一个窗口的全部状态。界面每帧读取状态，只通过命令（`a.Exec(id)`）或 `App` 的方法修改状态。后台 goroutine 不直接访问 `App`，而是把结果交给 `a.Update(func())`，由主线程更新。

#### 焦点请求

要把键盘交给某个控件时，`app` 不碰控件（拿不到 `Focus(c)`），而是置一个一次性的请求字段，名字一律 `FocusXxx`、注释写“下一帧把焦点给 ……”：

- `App.FocusDoc` / `FocusTerm`（`FocusTab` 指名哪个标签）/ `FocusTree` / `FocusHist`
- `Repo.FocusCommit`
- `SearchState.Focus` / `FocusRepl`
- `TermFind.Focus`

画那个控件的界面函数看到它就 `Focus(c)`、清掉。请求跟着它要找的东西走（哪个仓库的提交框、哪一格的查找条），所以是分散的字段，不合成一个全局的“焦点给谁”。同一帧里置了两个时，后画的那个控件拿到键盘。

#### 切换工作区

切换工作区（`switchTo`，`app/workspaces.go`）会重写整个 `*a`。所以后台任务开始前先存 `up := a.Update`，回来时通过 `up` 更新，并核对原来的仓库还在（比较仓库指针，文档比较 `docs.Doc.Gen`：`switchTo` 给每个开着的文档 `Gen++`）。

属于窗口、不随工作区重置的字段（`workspaces.go` 里 `switchTo` 抄回去的那些）：

- 窗口本身：`Win`、`Update`、`HasMenu`、`StateDir`、`Windows`、`Slot`、`ownStore`、`LastBar`
- 渲染器：`Rend`、`RenderOpen`
- 设置 / 关于窗口和菜单回调：`SettingsWin`、`OnSettings`、`OpenSettings`、`OpenAbout`、`OnRecent`、`OnClearRecent`
- 测试换掉的系统钩子：`Trash`、`OpenURL`、`ShowPath`、`InFinder`、`OpenDialog`、`SaveDialog`、`Preview`
- 日志：`LogDir`、`LastCrash`；输出面板：`Output`、`OutList`、`BottomTab`
- 命令面板的历史 `Palette.History`、还没处理的提示框（`Alert*`）

### 命令注册表

`app/commands.go` 里每个动作只定义一次（`Command`）：

- id（如 `view.sidebar`）、名字 `Title`、菜单 `Group`、前面画分隔线的 `Sep`
- 快捷键 `Mods` / `Key` 和生效的焦点上下文 `Context`：全局 / 编辑器 / Diff / 文件树 / 终端 / 提交框 / 搜索结果 / Git 栏
- `Enabled`（前提，比如有没有当前仓库）、`Why`（Git 操作现在不能做的原因，`app/gitwhy.go`；不空也算不能用，命令面板写在灰着的命令后面，从菜单栏点了就提示它）、`Run`、`Checked`
- `Section`：命令面板里分组的后半截（“编辑器 · 折叠”的“折叠”）；`Hint`：`Key` 为 0 时命令面板里显示的键（编辑器自己接的默认键，只显示、不登记）
- `Local`：有 `Run` 但键不在窗口上登记，由界面元素自己接（编辑器的 ⇧⌥F、编辑器自带的命令），`Run` 给命令面板
- `Custom` / `Shadowed`：自定义快捷键用，见下面

由注册表生成：

- **菜单栏**（`app/menu.go` 的 `BuildMenu`）。`SyncMenu` 每帧比对 `menuSignature`，变了才重建；签名包含勾选状态、仓库列表、最近的工作区、终端配置、界面语言、`keyRev` 和 `KeyRecording`。菜单栏是整个应用的（`mygo.App.SetMenu`），只有前台窗口的 `App` 同步它；菜单项的 Click 按传进来的窗口找 `App`（`target(win)`）。菜单项一律不置灰，点了不能用的命令什么都不做。
- **窗口快捷键**。有菜单栏时菜单先收到按键，所以菜单快捷键会盖过编辑器的同名快捷键；没有菜单栏时（测试、`WORKBENCH_PLAIN`）由 `BindShortcuts` 在窗口根上登记同一套。同一个键在不同上下文里可以是不同命令（⌥⌘F 在 Diff 里是全文 diff，在文件标签里是替换；⌘D 在终端里是分屏，在文件树里是创建副本），按键和菜单项都走 `RunKey(mods, key)`：按注册顺序找第一个当前能执行的命令。`RunKey` 只分派有 `Run` 的：全局 / Diff / 编辑器里不带 `Local` 的（`windowKey`），加上终端和文件树的。
- **悬停提示**：`Tip(text, id)` 在句尾加括号里的键，`KeyText` / `KeyHint` / `KeyList` 给句子中间用的键。每帧计算，改了快捷键马上跟着变。
- **`docs/shortcuts.md`**（`TestShortcutDoc -update` 生成）。
- **冲突检查** `TestShortcutConflicts`：id 不重复，同一分派域里同一个键只能有一个命令。
- **命令面板**（`>`）。

#### 编辑器自带的命令

编辑器自带的动作（复制行、多光标、折叠……）也登记成命令，id 是 `editor.<mygo 的英文名>`，由 `editorCommands` 从 `editorTable` 生成。它们 `Key` 留 0、`Local`，不进菜单栏和窗口快捷键，键由编辑器自己接；注册表只用来生成命令面板和快捷键文档。`Run` 外面套了 `inEditorChecked`：执行前再查一遍 `Enabled`（mygo 的命令不管只读），执行完把焦点还给编辑器。扫整篇文字的命令（全部折叠这类）在文件超过 `MaxWholeTextBytes`（1 MB）时不能用。

#### 只登记键的命令

没有 `Run` 的命令只登记键，不进菜单栏、也不进命令面板（`Group` 空）：键由界面上那个元素按 id 自己接，因为要用的状态只在那一帧的界面里（光标所在的冲突块、提交框里的字、列表的选中行）。现在有：

- 合并冲突的 `merge.*`
- 提交框的 `commit`
- 搜索结果的 `search.replaceOne` / `search.dismiss`
- 文件树列表的 `tree.*`（`tree.duplicate` 除外，见下）

文件树的这些键不能补上 `Run`：`RunKey` 会在窗口上分派有 `Run` 的文件树命令，键就不归列表了。`tree.duplicate`（⌘D）是故意有 `Run` 的：⌘D 在菜单栏里是终端的分屏，菜单接到按键后由 `RunKey` 分派，键盘在文件树里时落到它。`tree.undoKey`（⌘Z）只进文档，按键由 `treeCommands` 接。

Git 栏的 `changes.toggleStage` / `changes.discard` 有 `Run`（给命令面板），但列表有焦点时键由列表自己接（`ui/gitbar.go`）。

`TestShortcutConflicts` 查有 `Group` 的都有 `Run`；`TestRunlessCommandsHandled` 查没有 `Run` 的都有键、不是全局的、id 在 `internal/ui` 里有人接。

#### 普通文件夹

当前仓库是普通文件夹（不是 Git 仓库）时 `git.*` 命令都不能用；不看当前仓库的刷新、取消、全部抓取 / 拉取 / 停止，和“运行 git 命令…”（可以 `git init`）除外。

### 自定义快捷键

`app/keybind.go`。用户改过的键存在状态目录的 `keybindings.json`：

- 只记和默认不一样的，`key` 为空表示去掉默认键。
- 写法由 `ParseKeys` 读（`cmd+shift+p`、`⌘⇧P` 都行）；先按 JSON 解，不行再去掉注释和尾逗号（`workspace.StripJSONC`）按 JSONC 解。
- 每次 `applyKeybindings` 都从默认键（`keyDefaults`）起重算，所以“重置”就是从文件里删掉那条。命令的 `Mods` / `Key` 就是生效的键，菜单、快捷键、提示照旧从注册表读。命令表由整个进程共用。

**冲突**按分派域算（`keyDomain` / `clash`）：编辑器和 Diff 标签的键都归到全局那个域，两条命令同域、同上下文（或有一条是全局）、同键才算冲突。冲突时用户配置优先，原命令的键被去掉，`Shadowed` 记着被谁顶掉。编辑器自带的命令（`editor.*`）改了键时，由 `editorKeys` 生成 `editor.Binding` 交给编辑器（`EditorBindings` / `DiffBindings` / `DiffEditBindings`），同时去掉 mygo 原来的默认键。不支持和弦快捷键和 when 条件。

**改键的入口**：

- 设置窗口的“快捷键”页。录键时 `SetKeyRecording`，菜单暂时不接键。
- 直接在编辑器里改 `keybindings.json`：保存时（`keybindingsSaved`）重读。解析不了的条目提示一次（`keyToasted`），并列在设置页（`KeyErrors`）；整个文件读不了时（`keyBroken`），设置页不再写入，免得盖掉原文件。
- 写文件是原子的（`writeFileAtomic`）。

## 多窗口和生命周期

### 多窗口

`app/windows.go`。一个窗口一个工作区、一个 `App`；同一个工作区只开在一个窗口里，再开就把那个窗口置于前台。

- `Windows` 是开着的工作台窗口的注册表（设置、关于窗口不算）。
- 新窗口 1440×880、最小 960×600，比前一个错开 24 pt。
- 每个窗口有个槽位 `Slot`，决定它的 `StateKey`（1 是 `main`，2 是 `main-2`……，关掉的槽位新窗口先用），mygo 按 `StateKey` 记窗口位置。
- 几个窗口共用一个 `workspace.Store`；跟着进程走的东西（设置、最近的工作区）改了由 `syncShared` 同步到其他窗口。
- 同一仓库在两个窗口中打开时，各自有独立的监听和终端。

#### 外面交进来的路径

命令行参数、第二个实例转过来的、`open -a Grove`、拖到程序坞图标上的（`OnOpenFile` 分不出来；冷启动时攒着的在 `Restored` 里一起派）都进 `Route`：

- 工作区文件：开新窗口（开着就置于前台）。
- 目录走 `routeDir`：已经是哪个窗口的工作区就叫到前面；是哪个开着的窗口里的仓库（或它的 git 根是）就叫到前面并跳到那个仓库；前台窗口是空工作区就在它里面开；别的一个目录开一个新窗口。
- 文件：交给前台窗口的 `OpenArgs`。

拖进窗口里的（`OnFileDrop`）不走 `Route`，直接交给那个窗口的 `OpenArgs`。`OpenArg` 对目录的处理：已在工作区里的仓库跳到那一段；不在的先问要不要加进来（只记进会话，不改 `.code-workspace`）；空工作区的窗口直接把目录当工作区打开。

### 启动

`cmd/workbench/main.go` 的 `main`，按顺序：

1. 记下 `app.Launched`（启动计时）；第一个参数是 `termd` / `askpass` 时分出去当终端后台进程 / 凭据助手，不往下走。
2. 版本：`mygo build` 编进来的版本号写进 `version.App`。
3. 单实例：`mygo.App.RequestSingleInstanceLock`。拿不到锁就把参数交给已经在跑的实例、自己退出。设了 `WORKBENCH_STATE` 的实例默认不拿锁（测试和多开），要拿就设 `WORKBENCH_SINGLE=1`。
4. `cmd/workbench/migrate.go` 的 `migrateDirs`：在第一次调 `mygo.App.Path` 之前把旧名字（Workbench）的状态目录和日志目录搬过来。
5. 日志（`internal/crash` 的 `Setup`）：日志目录是 `WORKBENCH_LOGS`，没设时是 mygo 的 `PathLogs`（`~/Library/Logs/Grove`）。
   - 标准库 `log` 写 `workbench.log`；启动时超过 1 MB 就改名成 `workbench.old.log`，重新开一份。
   - `debug.SetCrashOutput` 指向 `crash-pending.log`；下次启动时如果它不空，就改名成 `crash-日期-时间.log` 留下（最多 10 份），`LastCrash` 记着，关于窗口里能打开。
   - 终端后台进程的日志是同目录的 `termd.log`。
6. 状态目录：`WORKBENCH_STATE`，没设时是 mygo 的用户数据目录；`SetWindowStateStore(app.WindowStates(目录))` 读 `window-state.json`，要在 `mygo.App.Run` 之前设好。
7. `render.Register` 注册 Mermaid / 公式渲染页的协议。
8. `OnOpenFile` / `OnSecondInstance` 收外面交进来的路径：`deliver` 去重（2 秒内同样的路径只算一次），窗口还没开好（或还在还原）时攒着，否则交给 `Route`。
9. `setupUpdates`（见“自动更新”），然后 `SetLanguages(i18n.Locale())`。
10. `WhenReady`：载字体、`NewWindows`、设置的回调（主题、材质、语言……），`Windows.Restore` 开回上次退出时开着的窗口（`windows`），再 `Greet`。

`Restore` 按队列依次开：每个窗口启动时要数块整窗大小的帧内存，同时开会叠出内存峰值，所以前一个画完第一帧（`OnReadyToShow` → `Ready`）再隔 `RestoreGap`（2.5 秒，`WORKBENCH_RESTORE_GAP` 可改）开下一个。只开一个窗口时不排队。

`Greet`（`app/openarg.go`）在状态栏提示一句：上次崩溃了就说崩溃日志在哪；第一次用是“欢迎使用 Grove …”；版本变了（上次的版本记在 `appVersion`）是“已更新到 Grove …”。

#### 启动用时和内存

启动用时记在日志里（`app/ready.go`）：第一个窗口的仓库都读完时，每个仓库一行“就绪”（状态、树、空目录、历史各自的用时），最后一行“全部就绪”从进程启动（`Launched`）算起。

`WORKBENCH_ALLOPEN=搜索词`（`app/allopen.go`）是量内存用的“全开”：仓库读完后开两个 Diff、搜一次、起当前仓库的终端，然后打一行日志。

一次搜索、关掉一个窗口这类大批释放之后调一次 `debug.FreeOSMemory()`，把内存还给系统。

### 关窗口和退出

#### 关一个窗口

`OnClose` → `CanClose`：有没保存的文件或正在跑的 git 写命令时先问（`RequestLeave`）。终端里还在跑的程序关窗口和退出时不问、直接关；只有切换工作区时才问（见 [state.md](state.md) 的“工作区”）。

问完关掉窗口，`OnClosed` → `Closed`：

1. `freeze` 记下开着的窗口（见下面“退出”）。
2. 记下终端目录（`NoteTermDirs`：shell 没报 OSC 7 的用 `lsof` 读）和终端的回滚。
3. 存会话、关掉窗口、关掉这个窗口的渲染器。
4. `debug.FreeOSMemory()`。

#### 退出

⌘Q、程序坞、关机：mygo 先发 `OnBeforeQuit`，没被拦就依次关窗口；中途有窗口拦了，已经关掉的窗口不会回来。所以所有退出确认都在 `OnBeforeQuit` 里拦下来处理（`RequestQuit`）：一个窗口一个窗口地问，问完、关掉再问下一个，全关完再退一次。中途取消就停下，已经关掉的不再开回来。

- `freeze` 在退出或关最后一个窗口时记下开着的窗口（还原中的连同还没开的），之后一个个关掉也照这个存进 `windows`。
- `quitting` 是正在问的那个窗口；`ending` 表示问完了、要退出了，不再拦，最后一个窗口关掉时也不再退一次。

### 设置窗口和关于窗口

都在 `cmd/workbench/main.go` 里开：

- 设置窗口 640×480，`StateKey` 是 `settings`。
- 关于窗口 460×640，`StateKey` 是 `about`。

两者各只有一个，属于打开它们的工作台窗口；那个工作台窗口关掉时一并关掉（`closeAux`）。设置窗口和工作台窗口共用一个 `App`（切换工作区时 `SettingsWin` 留着），所以设置窗口里的改动走的也是那个 `App` 的方法。界面缩放、强调色和风格每帧也套到这两个窗口上。

### 版本

`internal/version`：

- `App` 是 `mygo build` 用 ldflags 编进来的 `mygo.json` 里的版本，`go run` 和测试里是 `dev`。
- 日志第一行是 `version.String()`（应用版本、mygo 版本、Go 版本、系统）。
- `TestVersions` 核对：`mygo.json` 的版本在 CHANGELOG 里有 `## v<版本> ` 一节；go.mod 里的 mygo 版本和 `MygoPinned` 一致。`TestAppName` 核对 `mygo.json` 的名字和 bundle id。

### 自动更新

mygo 的 updater 插件（`plugins/updater/native`，原生的更新窗口）在 `cmd/workbench/main.go` 的 `setupUpdates` 里装上（`SetLanguages` 之前，图标用 `appicon.Draw(256)`）。更新源和公钥在 `mygo.json` 的 `updates`（GitHub Releases 的 `lemoeade/grove`，Ed25519）。

界面不直接引插件：`internal/app/updates.go` 的 `app.Updates`（`UpdateHooks`：能不能更新、检查、两个开关、上次检查、找到的版本）由 main 填好，测试和 `go run` 里是 nil。

- 菜单 / 命令面板的“检查更新…”是命令 `app.checkUpdates`（`Updates` 为 nil 时命令面板里灰着；菜单栏的项从不灰，点了也不做事），开插件的更新窗口。
- 设置窗口的“更新”页是两个开关和“版本”一行；插件的状态变了（`updater.OnChange`）就重画设置窗口。
- 开关存在插件自己的 `updater.json`（用户数据目录），不进 state.json。
- 设了 `WORKBENCH_STATE` 的实例关掉后台检查（`DisableAutomaticChecks`），量内存、冒烟时不会弹更新窗口抢前台。

发布、签名和密钥见 [release.md](../release.md)。
