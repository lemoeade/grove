# 架构

本文是 Grove 结构的总览：分了哪几层、各层之间怎么依赖、每个包管什么、改代码时要守哪些约定。各子系统的细节在
[`docs/architecture/`](architecture/) 下按主题分文件，见下面的[子系统](#子系统)。各版本改了什么记在根目录的
[`CHANGELOG.md`](../CHANGELOG.md)。

## 分层

```
cmd/workbench ─► internal/ui ─► internal/app ─► 逻辑包 internal/{git, files, docs, workspace, search, globs, fuzzy,
（三个窗口、单实例、  （建控件、画、   （状态机、命令注册表、   watch, graph, outline, history, askpass, termd,
  日志、子命令入口、    接事件）       界面交互的状态）         tasks, snippets, render, fonts, highlight}
  自动更新的接线）       │                                       （不建控件；docs、render 等用到 mygo 的类型）
      │                  ├─► 部件包 internal/{ged, gmd}（自己排字、画、接输入；gmd 用 css）
      │                  └─► 逻辑包的类型和纯函数（不做 IO）、internal/{mdprep, appicon}
      └─► internal/{workspace, render, fonts, askpass, termd, version, crash, appicon}、mygo 的 ui（接线）
                                internal/i18n（文案表，各层都用）
```

依赖方向从左到右，没有环（Go 也不允许）。

### `ui` 和 `app`

- `ui` 读 `app` 的状态、调它的方法和命令；`app` 不 import `ui`。
- `ui` 还直接 import 了 `docs`、`git`、`workspace`、`files`、`search`、`outline`、`graph`、`render`、`snippets`、`highlight`、
  `history`、`askpass`、`fonts`、`version`、`appicon`、`mdprep`，但只用类型、常量和纯函数（`git.Change`、`workspace.Settings`
  的可选值、`files.JoinPath`、`askpass` 的提示种类……）。读写文件、跑 git 都经 `app`（`ui` 里没有 `os.*` / `exec`）。
- 两者的边界不是“逻辑 / 界面”，而是“谁建控件”：
  - `app` 不建控件、不画。除了 `BindShortcuts` 在传进来的 `ui.Context` 上登记快捷键外，不碰 `wui.Context`。
  - 为了无头测试，界面交互的状态机放在 `app`：
    - 自画的右键菜单（`popmenu.go`：项、位置、命中、子菜单计时）；
    - 侧栏滚轮的手感（`sidescroll.go`）；
    - 文件树悬停提示的延迟和暖启动（`treetip.go`）；
    - 编辑器组拖放标签的落点几何（`groups.go` 的 `DropSide` / `DropRect`）；
    - `App` 里由 `ui` 每帧写回的“这一帧 / 上一帧”字段（`GitHover`、`TreeSpans`、`TreeDragging`、`MenuKeyAt`、`TermFocused`……）。
  - 所以 `app` 也 import mygo 的 `ui`、`editor`，用的是键位、`ListState`、`Rect` 这类类型。

### 逻辑包和部件包

- 逻辑包之间只有少量依赖：`search` 用 `docs` / `files` / `git` / `globs`；`docs` 用 `git` / `highlight` / `outline` / `snippets`
  （还有部件包 `ged` / `gmd` 的状态类型）；`files` 用 `fuzzy` / `git`；`watch` 用 `git`；`mdprep` 用 `emoji`。它们都不依赖
  `app` 和 `ui`。
- “逻辑包”指不建控件，不是不碰 mygo：
  - `docs.Doc` 持有 mygo 编辑器的 `editor.State` / `editor.Buffer`（mygo 的编辑器还在用的那些情况）和 Markdown 预览的选项；
  - `highlight` 复制 mygo 的语言定义；
  - `render` 用不可见的 `mygo.WebView`；
  - `watch` 包装 `mygo.WatchFiles`；
  - `termd` 用 mygo 的终端插件；
  - `fonts` 注册字体；
  - `outline` 用 mygo 的 Markdown 解析器。
- 部件包 `ged`（编辑器和 Diff）、`gmd`（Markdown 预览）只用 mygo 公开的排字、画图、输入接口，自己排版、绘制、处理输入，
  不认 Grove 的主题，也不 import `app` / `ui`。样式由 `ui` 传进去，`app` / `docs` 只持有它们的状态。`gmd` 用 `css`，`css` 不依赖别的包。

### `cmd/workbench`

除了 `app` / `ui` 还直接用：

- `askpass` 和 `termd`：同一个可执行文件也是这两个子命令的入口；
- `render`：`main.go` 里 `render.Register` 注册 WebView 后端；
- `workspace`：设置窗口的回调、材质；
- `fonts`、`crash`（日志）、`version`、`appicon`（更新窗口里的图标）；
- mygo 的 `ui`（`wui.View` 把三个窗口的界面挂上去）和更新插件 `plugins/updater`（见 [app.md 的自动更新](architecture/app.md#自动更新)）。

### 线程和状态

- `app.App` 保存一个窗口的全部状态。界面每帧读状态，只通过命令（`a.Exec(id)`）或 `App` 的方法改。
- 后台 goroutine 不碰 `App`，把结果交给 `a.Update(func())`，回主线程再改。开始前先拿好 `up := a.Update`：切换工作区（`switchTo`）
  会重写整个 `*a`，回来时还要核对原来的仓库、文档还在不在（比仓库指针；文档比 `Doc.Gen`）。
- 要把键盘给某个控件时，`app` 置一个一次性的 `FocusXxx` 请求字段，画那个控件的界面函数看到就 `Focus(c)` 并清掉。

细节（`switchTo` 保留哪些字段、`FocusXxx` 的全部字段、命令注册表、多窗口）见 [App、命令和窗口](architecture/app.md)。

## 子系统

| 文档 | 内容 |
|---|---|
| [App、命令和窗口](architecture/app.md) | `App`、`FocusXxx`、`switchTo`；命令注册表和由它生成的菜单、快捷键、面板；自定义快捷键；多窗口、启动、关窗口和退出、设置窗口和关于窗口、版本、自动更新 |
| [工作区、设置、会话和状态文件](architecture/state.md) | `.code-workspace` 和普通目录、`Settings`、按工作区的 `Session`、状态目录、state.json 的格式和迁移 |
| [Git](architecture/git.md) | 所有仓库同时可见、Git 栏、合并的历史；只读和刷新、写命令队列、凭据、冲突、进行中的操作、分支 / 标签 / 远程、历史、提交图、工作树、子模块、交互式变基、克隆、远程网页、普通文件夹 |
| [Diff 和暂存](architecture/staging.md) | 按块、按行暂存 / 取消暂存 / 丢弃，补丁怎么拼；Diff 标签的两条代码路径（ged / mygo）、ged Diff 的模型和视图、就地编辑；编辑器改动条和就地卡 |
| [文件监听和文件树](architecture/files.md) | 每个仓库一个监听、两类提示；文件树的模型（`git ls-files`、增量更新、被忽略的懒读、合并目录）、交互和外观 |
| [右键菜单和按钮的菜单](architecture/menus.md) | Grove 自己画的菜单：哪些地方用、`app.PopMenu` 状态机、`ui/gmenu.go` 的事件和绘制、磨砂、还用 mygo 菜单的几处 |
| [编辑器、搜索和本地历史](architecture/editor.md) | 文档和标签、大文件、编码、保存、格式化、大纲和粘性滚动、跳转、编辑器组；Grove 自己的编辑器部件 `ged`（单栏编辑器和共用部分）；跨仓库搜索和替换；本地历史 |
| [Markdown 渲染](architecture/markdown.md) | `mdprep` 的预处理、`gmd` 的排版和绘制、`css` 的层叠、emoji 短代码、Mermaid 和公式（`render`）、并排预览和滚动同步 |
| [终端](architecture/terminal.md) | 标签和分屏、任务、agent 状态、回滚内容的保存、termd 后台进程 |
| [外观、语言和快捷键](architecture/look.md) | 窗口底、材质和颜色、应用图标、中英文、快捷键的分派域 |

别的文档：[测试](testing.md)、[发布](release.md)、[快捷键表](shortcuts.md)、[术语](terms.md)、[踩过的坑](mygo-pitfalls.md)。

## 目录

| 目录 | 职责 |
|---|---|
| `cmd/workbench/` | 入口。第一个参数是 `askpass` / `termd` 时交给 `askpass.Main` / `termd.Main`；否则解析参数（工作区路径）、搬旧目录（`migrate.go`）、载字体、接自动更新（`setupUpdates`）、建 `app.Windows` 并把 mygo 的窗口回调接上去（`NewWindow`、`OnClose` → `CanClose`、`OnClosed` → `Closed`、`OnFocus`、`OnBeforeQuit` → `RequestQuit`），单实例锁和 `deliver`，日志和崩溃日志，设置窗口和关于窗口，`WORKBENCH_*` 环境变量 |
| `internal/app/` | 状态机：工作区和仓库、面板开关、编辑器组和标签、文件树、Git 栏、历史、搜索、终端、git 写命令队列、对话框状态、命令注册表和由它生成的菜单栏、state.json 读写、多窗口注册表、自动更新的钩子（`updates.go`）、界面交互的状态（自画菜单、滚轮、拖放几何，见上）。一个窗口一个 `App` |
| `internal/ui/` | 纯界面，按区域分文件：`view.go` 总布局，`titlebar.go`，`sidebar.go`（文件树），`gitbar.go`，`search.go`，`outline.go`，`openeditors.go`，`tabs.go`（编辑器组和标签内容），`history.go` / `graph.go`（右栏），`bottom.go`（终端 / 输出 / 问题），`statusbar.go` / `statusdoc.go`（状态栏；后者是当前标签的几项和语言 / 缩进 / 换行 / 编码菜单），`palette.go`，`dialogs.go` / `dialogframe.go`，`menus.go`（右键菜单），`settings.go` / `about.go`（两个小窗口），`widgets.go` / `icons.go` / `fileicons.go` / `accent.go` |
| `internal/workspace/` | `.code-workspace` 解析、state.json 的 schema 和旧格式迁移、`Settings`、`Session`（按工作区的标签、展开目录、终端、布局）、几个窗口共用的 `Store`、设置的 JSON 形式（`settingsjson.go`）、`files.exclude` 的读取 |
| `internal/git/` | Git 命令调用。只读：status（porcelain v2）、diff 两侧、log、分支 / 标签 / 远程、blob、ls-files、blame、工作树、子模块；写入：`RunRW` / `RunOp`（非交互环境、超时、进程组）；块级和行级补丁（`hunk.go`）、远程地址转网页（`web.go`）、常见错误说明（`explain.go`）、需要终端的命令识别（`interactive.go`） |
| `internal/askpass/` | 凭据提示的两端：`Main` / `Run` 处理 `Grove askpass <提示>` 子进程（连接套接字、发送一行 JSON、等待回答），`Listen` / `Server` / `Close` 处理应用端；`CleanStale` 清理上次遗留的目录，`Classify` 识别用户名、密码或口令提示 |
| `internal/files/` | 文件树模型：`git ls-files` 建，普通文件夹用 `Walk`；展开收起、筛选、改动标记、被忽略的文件懒读、符号链接、子模块行、合并目录、增量 `Diff` / `Apply`、空目录 |
| `internal/docs/` | 打开的文档：各种 `Kind`、读写、外部改动检测、编码、大文件、历史版本、Diff 两侧、冲突块解析、保存整理、按行 diff（`LineEdits`）、交互式变基的计划、光标信息 |
| `internal/ged/` | Grove 自己画的编辑器部件：Diff（并排、统一、就地编辑）和文件标签的单栏编辑器，默认都用它（`WORKBENCH_GED=off` / `WORKBENCH_GED_EDITOR=off` 退回 mygo 的）；只用 mygo 公开的排字、画图、输入接口，不认 Grove 的主题和 `internal/ui`。见 [editor.md](architecture/editor.md) |
| `internal/highlight/` | 大文件的后台高亮：`For(name)` 复制 mygo 的语言定义，只替换 `Highlighter`；一个 goroutine 从头分词并记录每行开头的状态，主线程据此处理可见行 |
| `internal/search/` | 跨仓库全文搜索和替换的计算：候选、worker、字面 / 整词 / 正则、glob、编码、`advanced.go` 的 regexp2 后备、`replace.go` |
| `internal/globs/` | 路径 glob（`**` 跨目录，`*` `?` 不跨，`{a,b}`，不分大小写）：`Set.Match` 给搜索的包含 / 排除，`Set.MatchRoot` 给 `files.exclude`，带目录缓存的 `Matcher` |
| `internal/fuzzy/` | 快速打开和命令面板的模糊匹配：优先匹配文件名，无法匹配时再包含目录；连续、词首和完整文件名匹配会加分，命中位置返回给界面用于高亮 |
| `internal/watch/` | 文件监听：每个仓库一个 `Watcher`，封装 `mygo.WatchFiles` 的 HintsOnly 模式，报“工作区变了 / git 变了”两类提示 |
| `internal/graph/` | 提交图的泳道算法，纯算法，不碰 git 和界面 |
| `internal/outline/` | 大纲：解析 Markdown 标题、Go（`go/parser`），其他语言按行正则或花括号嵌套识别；计算粘性滚动的吸顶行（`sticky.go`） |
| `internal/history/` | 本地历史存储：每个文件一个目录，包含 `index.json` 和内容文件；记录时机和恢复流程在 `app/localhist.go` |
| `internal/termd/` | 终端后台进程：帧协议（`proto.go`）、`Server`、会话（pty + shell + 无头终端）、应用端的 `client.go`、自行实现的 pty（`pty_darwin.go`）、`Spawn` |
| `internal/tasks/` | 读取仓库根目录中的任务定义：package.json 的 scripts、Makefile / justfile 的目标、Cargo.toml / go.mod 的常用命令；仅读取文件 |
| `internal/snippets/` | 代码片段：内置的（embed）加状态目录里 `snippets/*.json` 的；`Expand` 把 VS Code 片段体展开成文字和跳位，`Session` 管 Tab / ⇧Tab 跳位 |
| `internal/mdprep/` | Markdown 交给预览之前的整理：front matter、跨块的 HTML 容器（`<details>`、`<div align>`…）换成信息串 `grove-region N` 的围栏代码块，内容记在 `Regions`；emoji 短代码换成字符（`emoji`）；只换开头、结尾那一行，行号不变 |
| `internal/emoji/` | GitHub 的 emoji 短代码表（`gemoji.txt`，来自 github/gemoji）：`:tada:`、`:+1:` 换成 emoji 字符，代码块和行内代码里的不换 |
| `internal/gmd/` | Grove 自己画的 Markdown 预览（解析用 mygo 的）：块和行内排版、选择复制、代码块、表格、图片；带样式的 HTML 块按 `internal/css` 的结果画成 grid / flex / 盒子（`htmlrender.go`）；`WORKBENCH_GMD=off` 退回 mygo 的 |
| `internal/css/` | CSS：颜色、长度、grid 轨道、声明和样式表的解析，选择器匹配、优先级层叠、继承、`var()`、UA 样式表；元素由调用方包成 `css.Element`，不碰界面 |
| `internal/render/` | Markdown 预览中 Mermaid 图和公式的渲染：`Renderer`（缓存、队列、一个 goroutine）和 `Backend`；`web.go` 使用不可见的 `mygo.WebView` 作为后端 |
| `internal/fonts/` | 编辑器 / 终端等宽字体的选择和内置字体注册；`app` 渲染 Markdown 时也会使用，因此不放在 `ui` 中 |
| `internal/i18n/` | 文案表：`T` / `Tf` / `Tn` / `List`，`zh-CN.json`（测试从代码里收）、`en.json`；`Set` / `Lang` / `Locale` |
| `internal/version/` | 版本号：`App`（`mygo build` 用 ldflags 编进来，`go run` 和测试里是 `dev`）、`Mygo()`、`Go()`、`Platform()`、`String()` |
| `internal/crash/` | 日志目录：`workbench.log`（满 1 MB 轮换）和崩溃日志 |
| `internal/appicon/` | 用有向距离场绘制应用图标（浅色圆角方块上的钢蓝色 G 形 git 分支）；`-update` 生成 `assets/icon.png`，关于窗口和更新窗口使用同一套绘制代码 |
| `internal/testutil/` | 测试用：临时仓库（`TempRepo` / `NewRepo` / `ThreeRepos` / `SubmoduleRepo`）、后台回调队列 `Queue`、推帧等待、`Budget` / `Race`、`WaitTermReady` |
| `tools/measure/` | 测量内存和启动用时的脚本，不包含在应用中：`measure.sh`、`report.py`、`longrun.sh`、`seed/`；Go 端仅有 `TestMeasureScripts` |
| `.github/workflows/` | `ci.yml`：推送到 main 和 PR 时 vet、测试、构建 .app（`-race` 手动触发或每周跑）；`release.yml`：推 `v*` 标签时构建、签名、把更新包传到草稿 Release（见 [release.md](release.md)）；`claude.yml`：issue / PR 评论里写 `@claude` 时跑 Claude Agent SDK（`.github/claude/`） |

## 开发约定

新增代码需遵守以下约定：

- 界面只读状态；改状态走命令（`a.Exec(id)`）或 `App` 的方法。后台 goroutine 只经 `a.Update` 回主线程，开始前先拿好 `up := a.Update`（`switchTo` 会重写 `*a`）。
- 新动作先在 `app/commands.go` 登记，菜单、快捷键、面板、提示、快捷键文档都从这里来。改了键跑 `TestShortcutDoc -update`。
- git 的读命令带 `GIT_OPTIONAL_LOCKS=0 LC_ALL=C`；写命令非交互、有超时、经每个仓库的队列（`RunOp`）。
- 只在用户明确操作时写磁盘或改仓库：刷新、监听、打开、切换都不写。文件写盘只经 `WriteDoc` → `diskWrite`。
- 危险操作（丢弃改动、强推、硬重置、删除分支等）通过 `AskDanger` 确认，使用红色按钮；盖掉用户的配置（恢复默认设置、导入设置、重置全部快捷键）或没保存的改动（本地历史恢复）、不能撤销的也算。完整名单见 `app/danger_test.go`。作用于整个仓库的确认（丢弃全部、强推、重置、删分支、丢弃贮藏……）标题用 `a.inRepo(r, …)` 包一层，工作区里不止一个仓库时前面写上仓库名（`TestConfirmInRepo` 按源码数）。
- 菜单栏和命令面板里的 Git 命令作用于当前仓库：菜单栏没有别的上下文，当前仓库跟着上一次点的仓库走（`a.UseRepo`），确认框写明是哪个（上一条）。Git 栏、文件树、历史里的按钮和右键作用于那一行所在的仓库，并把当前仓库切过去。
- 主线程不跑 git、不遍历目录。同步读盘只限三种、都是读一个文件且有大小上限：
  - 悬停（`app/hover.go`，不开着的文件 `hoverMaxFile` 1 MB 以内，编辑器要马上拿到结果）；
  - 冲突行的“标记为已解决”（数磁盘上的冲突标记）；
  - 替换和替换预览（按搜索的文件大小上限，替换是用户点的写操作，开着的文件要在主线程改缓冲区）。

  别的读盘放后台 goroutine。
- 对话框只用 `ui/dialogframe.go` 的 `dialogFrame` 和它的 `dlgButton` / `dlgField` / `dlgInput` 等，样子统一。新对话框的状态照“新建分支”的写法：`App` 上一个 `XxxDialog` 结构、带 `Open` 字段，`a.OpenXxx(…)` 一次填好打开（`OpenNewBranch`）。现有几种别的写法（`ConfirmOpen` + `Confirm`、单独的 `bool` 等）是历史留下的，不再新增。
- 设置里默认开着的开关存反面（`noHoverDefinition`、`noStickyScroll`、`treeNoCompact` …，零值 = 默认，`omitempty` 不写进文件）。设置文档里看到的就是这些键名；改成正面会让用户的设置文件和导出的 JSON 失效，不改。新的默认开的开关也这么存。
- 每个可点击元素的标签（Label）需在窗口内唯一，供测试和无障碍功能使用：同名的加上仓库或位置，写法照现有的来（`名（仓库）`、`标签 x`、`编辑 x`、`分支 x`、`位置 仓库/路径`、`打开 …`、`命令 …`）。
- 文案一律 `i18n.T` / `Tf` / `Tn`，写字面量；跑 `TestCatalog -update` 收进 `zh-CN.json`，再补 `en.json`。
- 所有仓库同时可见：不要加“当前仓库 + 切换器”式的列表。
- 控制内存占用：大列表按需计算、后台读取；批量操作后调用 `debug.FreeOSMemory()`；渲染后端、termd 等组件空闲后关闭。
- 改了行为同步文档：功能和快捷键写进 README，`CHANGELOG.md` 的“## 未发布”，这里或 `docs/architecture/` 下对应的那篇。

完整开发约定见 [`AGENTS.md`](../AGENTS.md)，mygo 的使用注意事项见 [mygo-pitfalls.md](mygo-pitfalls.md)，测试方法见 [testing.md](testing.md)。
