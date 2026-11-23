# 终端

[← 架构总览](../architecture.md)

终端标签和分屏、任务、agent 状态、回滚内容的保存，以及 termd 后台进程。

## 终端

### 标签和分屏

模型和操作在 `app/terminals.go`，底栏的界面在 `ui/bottom.go`。

- 每个仓库一排终端标签（`TermTab`），底栏显示当前仓库的那一排（`App.TermRepo`）。空工作区时终端开在家目录 `~`，不记进会话。
- 每个标签是一格或分屏的几格（`TermNode` 拆分树：叶子是一格，否则左右 / 上下分两半），最多 `MaxPanes` 8 格（会话里校验用 `workspace.MaxTermPanes`，也是 8）。每格（`TermPane`）一个 shell。
- shell 按需启动：首次需要显示（底栏开着、当前仓库是它、是它当前的标签）才起（`ui.termPane` 里调 `EnsurePane`）。切仓库再切回来还是同一个 shell。
- 记进会话的：标签的名字、顺序、分屏、每格 shell 所在的目录（OSC 7 报的）、用的终端配置。

底栏界面（`ui/bottom.go`）：`bottomBar` 画标签条（`termTab`，可改名、右键菜单）、输出和问题两页；`termNode` 按拆分树画格和拖条，`termPane` 画一格（文件树的行拖进来贴引号过的绝对路径），`termFind` 是查找条，`exitedBar` 是 shell 退出 / 和 termd 断开时的横条；`newTermButton` 是「+」，右键或 ⌥点用终端配置新建。

#### 进程号、前台命令和目录

终端里有没有程序在跑（关标签、关窗口要不要问），都在 `app/termjobs.go`：

- 认领进程号：mygo 的终端不给进程号，`pollPids` / `claimPids` 从应用的子进程里挑有终端（tty）的（`ps -A -o pid=,ppid=,tty=`），按进程号从小到大分给按启动先后排队的格。
- 前台命令：`ps -o pid,pgid,tpgid,ucomm,args` 一次拿到进程组、终端的前台进程组、可执行文件名和命令行。关格前问一格（`TermPane.Job()`）用两次 `ps -p`；轮询 agent 时一次 `ps -A`（`foregroundJobs`）。
- 名字都按 `jobName` 算：前台是解释器（node、bun、deno、python、python3、ruby）时按脚本名（改过进程名的按 argv[0]）；路径带空格时 argv[0] 被切断，按 ucomm。
- 目录没报 OSC 7 时用 `lsof` 看 cwd（`procCwd`）。
- termd 里的会话忙不忙由 termd 自己用 `TIOCGPGRP` 看（有不在任何窗口里的会话，应用不知道它们的进程号）。

#### 按键和设置

- 终端的命令走注册表：Terminal 上下文里的（分屏 ⌘D / ⇧⌘D、查找 ⌘F、清屏 ⌘K、关格 ⌘W……），和全局的新建终端 ⌃⇧`；别的键交给终端（`Options.OnKey` → `termKey`）。
- 设置里有 ⌥ 作 Meta、选中即复制、光标闪烁、配色（跟着外观、浅 / 深，或 Ghostty 主题名）。
- 终端配置（`TermProfiles`，最多 `workspace.MaxTermProfiles` 10 个）是 shell 加参数；第一格用了配置时标签名跟着它（`Repo.TermLabel`）。

#### ⌘ 点击

终端里 ⌘ 点击（`app/links.go` 的 `OpenLink`）：

- 文件路径（绝对、`~/`、相对终端当前目录，`file://` 也算）在编辑器里打开，带 `:行[:列]` 就跳过去；
- 仓库里的目录在文件树里显示；
- 别的网址交给系统浏览器。

shell 报的目录可能是解开符号链接的（`/var` → `/private/var`），对不上时两边都解开再比（`linkRepo`）。

#### 开在编辑器区

终端标签也能开在编辑器区（`app/termeditor.go`，⌃⇧⌘`；底栏标签的右键菜单也能挪过去）：一个 `TermTab` 要么在底栏（`Repo.Terms`），要么在编辑器区（`Repo.EdTerms`，对应一个 `docs.Term` 标签）。移动时不重启 shell，只改变显示位置；画法和底栏同一套（`ui.termNode`）。

#### 把终端内容开成标签

终端没有键盘选区，所以有“把终端内容开成标签”（⇧⌘C，`app/termtext.go`）：把一格的屏幕和回滚开成只读标签，在编辑器里选、搜、复制。内容是打开时的快照，可通过横幅刷新（`RefreshTermText`）。同一个文件里还有“复制全部输出”。

### 任务

`internal/tasks` 认仓库根上的任务，只读文件、不跑东西：

- package.json 的 scripts（锁文件定用哪个包管理器）、Makefile / justfile 的目标、Cargo.toml / go.mod 的常用命令；
- 最多 200 条（`tasks.Max`）；`Stamp` 按那几个文件的名字、大小和修改时间决定要不要重扫。

跑在 `app/tasks.go`：

- ⇧⌘B（`task.run`）或面板 `!` 选一个（所有仓库的任务一起模糊匹配），文件树里 package.json 这些的右键也能跑。
- `RunTask` 在那个仓库里跑：有叫任务名、前台没在跑命令的终端标签就用它，不然新开一个以任务名命名的；切到这个仓库、打开底栏、键盘给它。
- 最近跑过的记在 `TaskHistory`（最多 `workspace.MaxTaskHistory` 20 条），面板里排前面。

### agent 状态

终端里跑着 claude、codex 这类 agent 时，按格算它的状态，再按工作树汇总（`app/agents.go`）。全在本机算，只看 Grove 自己的终端，不读 agent 的日志、不接它的接口。

#### 三种状态

- 等你（`AgentWaiting`）：终端响铃（BEL）或要通知（OSC 9 / OSC 777），而键盘不在这一格（`paneLooked`）。
  - 记在 `TermPane.bellAt`，用户切进这一格（`FocusPane`、`GoToPane`）时清。
  - 认序列靠 mygo 终端的 `OnBell` / `OnNotify`（OSC 0/2/7/52 里当结束符的 BEL 它不算）。
  - termd 的会话在无头终端上接这两个回调，发 `frameBell` 给窗口（没窗口接着时记下，接上时补一帧），窗口端是 `Conn.OnBell`。
- 运行中 / 空闲（`AgentRunning` / `AgentIdle`）：前台命令在 agent 名单里（`DefaultAgents`：claude、codex、gemini、aider、opencode，加设置的「agent 命令」`AgentNames`），最近 10 秒（`agentQuiet`）有输出算运行中，否则空闲。

#### 后台轮询

- 前台命令由后台 goroutine（`agentPoller`）每 2 秒（`agentPoll`）用一次 `ps -A` 读所有开着的格（`termjobs.go` 的 `foregroundJobs`）。
- 读到的前台命令和运行中 / 空闲都和上一轮一样就不回主线程，免得空闲时每 2 秒多一帧。
- 输出时间：termd 的格每来一帧屏幕记一次（`Conn.OnOutput`，原子量）；本地的格没有输出回调，前台是 agent 时看屏幕文字的指纹变没变。

#### 终端 → 工作树

- 格现在的目录（`paneDir`：OSC 7，没有时上次 lsof 读的）落在哪个工作树里（`git.Under` / `SamePath`，套在里面的别的工作树归那个工作树）。
- `WtAgent(r, w)` 取最需要你的一格（等你 > 运行中 > 空闲），带上开始运行 / 等你的时间和是哪个终端。
- `GoToPane` 切到那一格（底栏的打开底栏、切仓库和标签，编辑器区的切到那个标签）。

#### 界面

终端标签（底栏和编辑器区）的图标后面，等你画 7px 琥珀色圆点，运行中画 13px（`iconRow`）的灰色转圈（`ui/agents.go` 的 `agentMark`），空闲和没有 agent 不画。Git 栏的工作树行用同一套符号（`ui/wtrows.go` 的 `wtMark`）。

### 回滚内容的保存

`app/termhist.go`。未启用后台进程时，shell 随窗口关闭而结束；为便于重开后查看原有输出：

- 关闭窗口、退出或切换工作区前，每个启动过、在应用进程里的格都把回滚内容（`Terminal.History`，VT 序列）写入 `<状态目录>/termhist/<工作区哈希>/<毫秒>-<格 ID>.vt`，文件名保存在会话中对应分屏的 `hist`（`workspace.TermSplit.Hist`）。不定时写；全屏程序开着时（`History` 是空的）留着上一次写的。
- 下次启动该格时（`takeHist`），先将内容输入新终端，并在前面加一行淡色分隔；读取后删除文件。
- 每格最多保留 `TermHistCap` 256 KB（从行首截取，留最后的），整个目录上限为 `TermHistMax` 20 MB，超出时删除最早的文件；会话里没记着的文件由 `gcTermHist` 清掉。
- termd 里的格不写：会话本来就在。

### termd

设置里开了“关掉窗口后终端继续在后台运行”（`TermDaemon`），新建分屏的 shell 会在终端后台进程 termd 中运行，窗口里的终端只是镜像。包在 `internal/termd`（`server.go`、`session.go`、`client.go`、`proto.go`、`spawn.go`、`pty_darwin.go`），应用这边在 `app/termd.go`。

- 进程：就是 Grove 自己带 `termd` 参数（`Grove termd <sock> <log>`），`main` 第一行分出去，不起 `mygo.App`。由 `termd.Spawn` 用 `Setsid` 起，等它能连上。
- 套接字：状态目录里的 `termd.sock`，0600，不做别的认证（只有这个用户能连）。路径太长（macOS 的 `sun_path` 只有 104 字节）时换成 `$TMPDIR/grove-termd-<sha1 前 6 字节>.sock`（`SockPath`）。同时起了两个时靠 `<sock>.lock` 的 flock 只留一个。一个状态目录一个 termd，所以 `WORKBENCH_STATE` 的实例有自己的。
- 会话：由一个 pty（自行实现的 `pty_darwin.go`，使用 `/dev/ptmx` + `ioctl`）、一个 shell 和一个无头 `terminal.Terminal` 组成。shell 输出传给无头终端，窗口中的终端通过 `terminal.Options.Conn` 获取镜像帧（`ResizeByConn`：屏幕大小跟着 termd 那边），保持屏幕和回滚内容一致。
- 一个会话同时只允许一条连接：新连接会替换旧连接，旧的收到 `frameGone "kicked"`，那个窗口的格显示“这个终端在别的窗口里接上了”。
- 断开和关掉：关窗口、⌘Q、换工作区是断开，会话留在 termd 里，会话记下 sid，再开同一个工作区接回去（会话不在了就开新的，并提示）；关格、关标签、移除仓库、重新启动是关掉会话（shell 收到 SIGHUP）。
- 响铃：会话的无头终端响铃或要通知时发 `frameBell`（镜像帧里没有），见上面“agent 状态”。
- 退出：shell 退出了、又没窗口接着的会话过 `Linger`（`DefaultLinger` 10 秒）拿掉；没有会话也没有连接 60 秒（`DefaultIdle`）后 termd 自己退出、删掉套接字。SIGTERM / SIGINT 挂断所有 shell 再退。
- 设置窗口终端页显示 termd 在不在跑、几个会话（`TermdStatus`，2 秒没查就在后台重查），能“停止”（`StopTermDaemon`，菜单里是“停止终端后台进程…”）：先问，再挂断 termd 里所有的 shell，开着的格显示已退出，termd 没有会话、没有连接后自己退出。
- 关掉设置只影响新开的格。
