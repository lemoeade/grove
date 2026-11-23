# Git

[← 架构总览](../architecture.md)

所有仓库同时可见的 Git 栏和合并的历史，git 命令怎么跑（只读、写队列、凭据），以及分支、历史、提交图、工作树、子模块等。Diff 标签和按块、按行的暂存单独在 [staging.md](staging.md)。

## 所有仓库同时可见

这是 Grove 的第一原则：文件树、Git 栏、搜索、历史都同时列出工作区里的每个仓库，没有“当前仓库 + 切换器”的视图。新功能也需遵守，不要新增只显示当前仓库的列表。

“当前仓库”（`Current()`）还在，但只用在少数地方：新终端开在哪、右栏历史的“当前仓库”页、标题栏的分支胶囊。点文件树、Git 栏或者切到某个文件标签会更新当前仓库（`UseRepo` / `SelectRepo`），⌘1–⌘9 跳到第几个仓库（`JumpRepo`）。

状态栏左边不写当前仓库，写所有仓库的总况（`reposSummary`）：几个仓库、几个有改动 / 要推送、谁在合并，读取失败的红字；悬停是每个仓库一行，点了打开 Git 栏；只有一个仓库时是它自己的改动。

### Git 栏

代码：`app/changes.go`（`GitRows` 和行的模型）、`ui/gitbar.go`（画）。

#### 行的排列

`GitRows` 把所有仓库整理成一张表。每个仓库一段，从上到下是：

1. 段头（`RowRepo`，贴顶）
2. 进行中操作的横幅（`RowOngoing`）
3. 工作树组头（设置里开了“在 Git 栏显示工作树”才有，默认收着）
4. 提交框（`RowCommit`，在改动上面：文件多时不用滚到段底）
5. 各组的标题和每个变更（冲突 / 已暂存 / 改动），说明行
6. 贮藏组头（设置里开了“在 Git 栏显示贮藏”才有，默认收着）

贮藏组头、工作树组头和已暂存 / 改动两组一样写组名和条数（`foldHeader` 的 `iconOnly` 都是假）。工作区干净的仓库没有说明行，段头上画一个绿色的勾。

段的收放是 `GitFold`：

- `FoldAuto`：有改动展开、干净的收成一行；有工作树（显示工作树开着时）、合并 / 变基停在一半的也展开，让工作树块和横幅看得见。
- `FoldOpen` / `FoldClosed`：点过箭头就按用户的，存进会话的 `git`。

#### 显示工作树 / 贮藏的开关

两个开关是 `Settings.ShowWorktrees` / `ShowStashes`（零值就是关，老的 state.json 不用迁移）。`Repo` 没有 `App` 指针，`App.newRepo` 给每个仓库接上窗口的 `&a.St.Settings`（`Repo.set`，换工作区 `*a = *n` 之后重新指一遍），`Repo.ShowWts` / `ShowStashes` 读它。

工作树关着时：

- `GitRows` 不出工作树块，`GitOpen` 的自动展开不算 `HasWorktrees`，段头不画工作树数；
- `NeedsYou` 是空的，`GoToWorktree` 什么都不做；
- `WtShown` / `WtTracked` 都是假（不算 `WtInfo`、不盯目录；会话里的 `WtOpen` 留着）。

开关变了由 `SettingsChanged` → 每个窗口的 `syncShownGroups` 收尾：关掉时收起 `WtExp`、`clearRefHistories`、扔掉 `WtInfo`、`syncWtWatch` 停掉监听；打开时按 `WtOpen` 重算、接上监听。两种都 `syncChangeSel`，选中落在没了的行上的清掉。

贮藏关着时只是 `appendStashes` 不出行、段头不画贮藏数；`r.Extra.Stashes` 照常读，`git.stash` / `git.stashPop` 照常能用。

#### 改动分组和变更行

未跟踪的文件和未暂存的改动同在“改动”一组（`groupKind`、`groupTitle`）：

- 顺序照 `Status.Changes`（已跟踪的在前、未跟踪的接在后面），条数是两者之和；只有未跟踪的时组名也是“改动”。
- 每一行的 `Change.Kind` 还是各自的（字母、颜色、丢弃、暂存、目录展开照旧）。
- 组头的 + 是 `StageUnstaged`：没有冲突时是不带路径的 `git add -A`（按路径传的话，上万个改动的文件会超出 macOS 的命令行长度限制）；有冲突时按路径 `git add -A -- …`，不用不带路径的 `add -A` / `-u`（会把冲突的文件也暂存、等于标成已解决）。
- 组头的丢弃仍是 `git restore .`，不动未跟踪的；这组全是未跟踪的时不显示。

变更行、贮藏里的文件、提交里的文件是同一个写法（`changeLabel`）：左边是文件树的文件图标，状态字母（`codeLetter`）在行尾；提交里的文件排在行数右边；变更行悬停时字母让给按钮。放不下时路径先缩，文件名比整行还长才截断。

分组标题上的按钮（`showActions`）要指针移上去才有，右键（`groupMenu`）是同样的动作。

#### 段头

段头（`ui/gitbar.go` 的段头那一段）从左到右：收放箭头、仓库圆点（`repoDot`）、仓库名、分支按钮、链接工作树的标记、进行中操作的胶囊；收起时还有贮藏数 / 工作树数。右边按情况是队列按钮 / 读取中的转圈 / 读取失败的 ! / “不是 Git 仓库” / 干净的勾 / 改动数，↓↑ 数字，最后是这个仓库的拉取 / 推送按钮（`syncButton`）。

拉取 / 推送按钮和标题栏的同步按钮共用 `syncOf`（提示文字、没上游时灰着、`Repo.Pulling` / `Pushing` 在跑或排队时转圈），点了走 `App.Pull` / `Push`，没有另一套 git 调用。

#### 选中、多选和行内按钮

行的身份是 `ChangeRow.Key()`（仓库 + 行的种类 + 变更的种类 + 路径），选中 `GitSel`、悬停 `GitHover`、多选 `GitChosen` 都按键记，所以刷新后行的位置变了，选中的还是原来那项。

- 多选的操作（`StageChanges` / `UnstageChanges` / `DiscardChanges` / `OpenDiffs`）先按仓库分组（`byRepo`），再排进各自的队列；一次打开超过 20 个 Diff 时先要求确认。
- 变更行悬停（或列表有键盘时选中）出的行内按钮（`ui/gitbar.go` 的 `changeActions` / `stageActions`）用 `MenuChanges`，和右键菜单同一个约定（`app/chosen.go` 的 `actOn`）：指针所在的行在两行以上的多选里时作用于选中的全部，否则只有这一行。按钮的动作经 `gitFrame.do` 留到列表建完再做。
- 列表有焦点时的空格 / ⌘⌫ 是 `changes.toggleStage` / `changes.discard`（上下文 `GitList`，由列表自己接键，命令面板也能用），作用于 `KeyChanges`。
- 暂存 / 取消暂存 / 丢弃会让选中的那行挪走时，`markGitAfter` 先记下它后面（没有就是前面）第一个不在选中里的行，状态重读后 `syncChangeSel` 跟不上当前标签时停在那一行。

#### 提交框

每个仓库一个（`CommitOpen` / `CommitMsg`），在段头（和工作树组头）下面、改动分组上面。提交、修改上一次提交、提交信息历史在 `app/commit.go`。

- 什么时候展开（`CommitShown`）：有已暂存的改动、写了信息、在修改上一次提交、合并的冲突都解决了，或者用户点开了提交框。
- `ShowCommit` 把段头滚到顶上（`ui.Start`），贴顶的段头不挡它。⌘↩ 提交键盘所在的那个。
- 一个框管到底（`commitRow`）：收着和没写信息时看着就是一行输入框，写了信息（或在修改上一次提交）框底才长出说明和分裂按钮（`commitFoot`）。
- Grove 风格的按钮上只有纸飞机（`iconSend`），后果写在读屏名和提示的第一行：“提交 N 个文件”、什么都没暂存时“提交全部 N 个文件”、“修改提交”。
- VS Code 风格走 `vsCommitRow`：白底细框的输入框，下面一直有满宽的“✓ 提交 …”分裂按钮（`vsCommitButton`），字就是上面说的后果；没写信息时点它 `OpenCommit`；和平常不一样的说明在按钮下面一行。
- 按钮的 ▾ 和框的右键是同一个菜单（`commitBoxMenu`）：提交并推送（`CommitPush`，不看自动推送的设置，不能推时 `CommitPushWhy` 写原因）、修改上一次提交、撤销上次提交。没写信息时 ▾ 只在指针移到框上时出现。
- 不能提交（`CanCommit`）时按钮画灰、点了不提交，但不调 `Disabled`（mygo 的 `Disabled` 不出悬停提示），提示里写原因（`CommitWhy`）。
- 最近 30 条提交信息（`MaxCommitMsgs`，所有仓库共用）存在 `St.CommitMsgs`，框里空着时 ↑↓ 翻。

“提交全部 N 个文件”（`CommitsAll`）：先 `git add -A` 再提交，含未跟踪的文件；add 失败就不提交；有冲突、进行中操作时不走这条。仓库里有 git 命令在跑 / 排队，或者状态正在读时，先不提交（toast 让你稍后再点），免得把别的文件一起提交了。

修改上一次提交时，上一次提交已经推送过（`HeadPushed`）的先 `AskDanger`：改了要强推。

#### 横幅和其余按钮

- 进行中横幅（`ongoingRow`）的继续 / 跳过 / 放弃是带字的小按钮（`bannerButton`，继续是强调色底，有冲突时灰着、提示里写原因）。
- 展开的工作树下面那行（`wtSummaryRow`）写“N 个提交 · N 个文件”和 +/−。
- 其余面板里的动作（冲突条、Diff 选行条和块按钮、搜索的全部替换、变基向导、终端退出浮条）都用同一个 `iconAction`（`ui/widgets.go`）：只有图标，读屏名是原来按钮上的字，`why` 不空时画灰、提示里写原因、点了没反应。

### 合并的历史

右栏历史除“当前仓库”页外，还有“所有仓库”页（`histAll`）。

- `AllHistRows` 把每个仓库的一页提交按作者时间（`%aI`）做 k 路归并。各仓库的读取进度不同，所以只合并到所有仓库都已读到的时间（地平线），往下滚 `LoadMoreAllHistory` 给落后的仓库补一页。
- 最上面是有改动的仓库的“未提交的改动”行，一个仓库一行。
- 选中是 `HistAllSel`。
- 所有仓库页不画提交图，每行左边一个仓库圆点，行尾的仓库名是同色淡底的胶囊（`repoChip`）。

仓库的颜色（`repoColor`，按仓库在工作区里的顺序在六个灰调的颜色间轮换）也用在文件树和 Git 栏的仓库行、终端标签、起始页上的圆点（`repoDot`）。

## Git

### 只读和刷新

只读命令都带 `GIT_OPTIONAL_LOCKS=0 LC_ALL=C`：前者防止 `git status` 刷新 index，避免与终端中的 git 操作争用锁；后者固定输出语言，便于解析。

一个仓库的状态是 `git status --porcelain=v2 -z --branch` 加 `git.Extra`：

- 进行中的操作（`Ongoing`）和它的进度（`Progress`）
- HEAD，`HeadRoot`（HEAD 没有父提交）
- 未推送的提交（`rev-list -n 200`，`MaxUnpushed`）
- 贮藏（最多 50 个，`MaxStashes`）
- 远程列表、远程的网页地址
- `GitDir` / `CommonDir` / `Linked`（是不是链接工作树）
- 子模块（`Subs`，最多 200 个）

刷新只有一个入口：`app/watch.go` 的 `RefreshRepo(r, why)`。

- `why` 是 `WhyManual` / `WhyOpen` / `WhyOp` / `WhySave` / `WhyFileOp` / `WhyWatchGit` / `WhyWatchFiles` / `WhyFocus` 之一，记在 `Repo.LastWhy` 里给日志和测试看。
- 它在后台读状态、分支、文件树（必要时）、重读开着的文档（`syncDocs`），回到主线程一起换上。
- 只有工作区文件变化（`filesOnly`）时不重读历史，也不重读 `git.Extra`；监听和保存带来的路径提示让文件树只改动到的那几处。
- 读取期间再次收到刷新请求时，先记到 `r.again`，本次完成后再读一次，不并发读。
- 窗口回到前台时 `RefreshOnFocus` 只给没有监听的仓库（出错、目录没了、没开起来）补读一遍（3 秒内只读一次）；在监听的不读。

### 写命令和队列

写命令走 `git.RunRW` / `RunOp`：不交互（`GIT_TERMINAL_PROMPT=0`，`GIT_EDITOR` / `GIT_SEQUENCE_EDITOR` 换成 `true`，分页器换成 `cat`），默认 2 分钟超时，在自己的进程组里跑，取消时整组先 SIGTERM 再 SIGKILL。

每个仓库有一条队列（`app/queue.go`），`App.RunOp` 排进去，一次只跑一条。

- 队列按钮（`ui/queue.go`）在 Git 栏的段头和标题栏上，跑着时转圈；菜单里 `CancelOp` 取消正在跑的，`Dequeue` 移除还没开始的。
- 取消后检查是否留下 `index.lock` 之类的锁文件（`git.StaleLocks`），有就只提醒、不删，因为可能有别的 git 进程正在跑。
- “全部抓取 / 拉取”是 `Batch`：按 Git 栏顺序依次处理各仓库，单个仓库失败不影响后面的，状态栏显示进度，`StopBatch` 停掉还没开始的。
- 拉取默认 `--ff-only`，设置里可改成 rebase 或 merge（`PullArgs`）。

持续输出的长任务（如克隆）用 `RunStream` + `Keep`：`readChunks` 分段读输出，有进展时 `Touch` 重置超时计时，所以只在任务不再推进时才超时。

失败时，`git.Explain` 认出常见错误（没有上游、被拒绝、认证失败、合并冲突等），给出简明说明；`AlertDetail` 把 git 原始输出的最后几行放进“详细信息”。

需要终端的 git 命令（`git.NeedsTerminal`：`-i`、`-p`、`commit` 不带 `-m` 之类）在命令面板 `>git` 里输入时不在后台跑，而是开一个终端去跑（`runGitInTerm`）。

### 凭据

git 要用户名、密码或 SSH 口令时，由 Grove 弹凭据对话框。代码在 `internal/askpass`（进程间那一层）和 `app/askpass.go`（对话框）。

- 第一条写命令跑之前才（懒）起：在 `os.TempDir()` 下建 `grove-askpass-<pid>.sock`（0600 的套接字）和 `grove-askpass-<pid>.sh`（0700 的小脚本，内容就是 `exec Grove askpass "$@"`）。`Listen` 里先 `CleanStale` 清掉已经退出的进程留下的这两样。
- 只有写命令的环境里带 `GIT_ASKPASS` / `SSH_ASKPASS` / `SSH_ASKPASS_REQUIRE=force` 指向它，另带 `WORKBENCH_ASKPASS_SOCK` / `WORKBENCH_ASKPASS_ID`；读命令不带，不会在刷新时弹框。
- `Grove askpass <提示>` 子进程（`askpass.Main`）连接套接字，发一行 JSON（提示、`WORKBENCH_ASKPASS_ID` 的值），等回答后写到标准输出。
- 应用端按 id 找到对应的窗口和命令（`askRegister` / `askDone`），`Classify` 认出提示的种类（四种），放进 `a.Cred` 显示对话框，用户回答后 `AnswerCred`；取消时 toast“已取消”。回答不进日志。
- 对话框开着时命令的超时钟暂停（`Keep` 的 `Hold` / `Release`）。
- HTTPS 的用户名对话框里可以顺手填密码：密码在 `credCache` 里留 2 分钟、只用一次，接着来的密码提示就不再弹框。
- 测试的 `TestMain` 也认 askpass 模式，所以测试二进制也能当 askpass 脚本的入口。

### 冲突和三方合并

冲突标记由 `docs/conflicts.go` 解析：`ParseConflicts`，以及 `Doc` 的方法 `ConflictBlocks`（按 `Buf.Version` 缓存）；`Resolved` 算出解决一块后的替换文字和范围。

`app/conflicts.go` 的 `ResolveConflict` / `ResolveAll` 只改缓冲区：保留我的、他们的、两者。只有 `MarkResolved` 跑 git 写命令：先保存文件，再 `git add`；文件里还有冲突标记时先问。

- 冲突的文件在 Git 栏的“冲突”组（`Change{Kind: Conflict}`），单击在编辑器里打开（`OpenConflict`，文件不在了就退回开 Diff），冲突就在那里解决。
- 右键能开两种 Diff：HEAD 和工作区比，或者“两边”比（`Code: 'S'`，`git ls-files -u` 找到 `:2:` 我的和 `:3:` 他们的）。
- 在文件标签里，`ui/merge.go` 每帧按解析结果给冲突块上色、在行号栏画红点（`conflictMarks`）；按钮在冲突条上（`mergeBar`）。
- 快捷键（`mergeKeys`）：⌃⌥1 / 2 / 3 选我的 / 他们的 / 两者，⌥F7 / ⌥⇧F7 或 ⌃⌥↓ / ↑ 跳到下一处 / 上一处。
- 改动条（GutterMarks）只给冲突用。

### 进行中的操作

合并、变基、拣选、还原停在一半时（`git/state.go` 只认这四种），仓库段顶上有横幅（`RowOngoing`），带进度（`ReadProgress`：变基第几步共几步）。

按钮是继续 / 跳过 / 放弃（`app/ongoing.go`：`ContinueOngoing`、`SkipOngoing`、`AbortOngoing`）：

- 还有冲突的文件时不能继续。
- 跳过和中止先 `AskDanger`；合并没有跳过。
- 继续和跳过走 `runStep`：退出码非 0 且输出里有 `CONFLICT (` 当成又停下了，不报错；别的非 0 才是失败。

日常操作在 `app/gitdaily.go`：`Merge`、`RebaseOnto`、`CherryPick`、`Revert`、`ResetTo`（soft / mixed / hard，hard 要确认）、`UndoCommit`（软重置一个提交，把信息填回提交框）、`PushForce`（`--force-with-lease`）、`Sync`（拉了再推）。停在冲突上时交给横幅（`stopOp`）。`inHead` 判断某提交在不在当前分支上。

#### 不能做的原因

都在 `app/gitwhy.go`：每种 Git 操作一个 `XxxWhy`（`FetchWhy`、`PullWhy`、`PushWhy`、`PushForceWhy`、`SyncWhy`、`StageAllWhy`、`StashWhy`、`RefWhy` / `MergeWhy` / `RebaseOntoWhy`、`SwitchWhy`、`DeleteBranchWhy`、`RemoveWorktreeWhy`、`UndoWhy`、`PickWhy`、`ResetWhy`、`RebaseWhy`…），空串 = 能做，按“最根本的先”的固定顺序只报一条。用在：

- 原生菜单：ui 的 `whyItem` / `withWhy` 括在标签后面；
- 按钮：写进提示（`syncOf`）；
- 命令注册表的 `Command.Why`：给命令面板（写在灰着的命令后面）和菜单栏（点了提示原因）；
- 动作本身开头再查一次（`a.Blocked(XxxWhy(…))`：不为空就 toast、不执行），菜单开着时状态变了也不会照做。

提交后自动推送、同步拉完再推用不查的 `pushNow`（那时状态还没刷新）。

### 分支、标签、远程

`app/gitrefs.go`：`RefDialog` 一个对话框五种用法（`RefDlgKind`：分支改名、新建标签、删除标签、远程改名、添加远程），底下实时预览要跑的命令，`refNameErr` 查重名和 git 不收的字符。

- 删除分支前在后台数有几个提交没合并再问；删除远程分支用 `splitRemote` 拆出远程名。
- `TagRemote` 找标签该推到哪个远程。
- 标签和远程的读取在 `git/refs.go`。

右栏历史可以看某个分支或标签的历史（`Hist.RefKind`），也可以看某个工作树的。

### 历史

#### 读和搜索

`git.LogQuery` 拼 log 的参数：搜索词、所有分支、路径。右栏的搜索框由 `ParseHistQuery` 解析：

- `键:值`：`author:` / `作者:`、`path:` / `路径:`、`after:` / `since:` / `之后:`、`before:` / `until:` / `之前:`；
- `/正则/`；
- 日期交给 git 自己的写法认（`git.ResolveDate`），所以 `2.weeks.ago` 之类都行。

文件历史带 `--follow`；`--follow` 和搜索词 git 不能一起用，所以这时读最多 `MaxFollowSearch`（5000）条，在 Go 里过滤（`matchLocal`）。

一页 `LogPage`（200）条，往下滚读下一页。

#### 提交详情、比较、贮藏

- 提交详情里的文件列表是 `diffTreeFiles`：先 `--name-status` 再 `--numstat` 拿增删行数。
- 比较两个提交在 `app/compare.go`（`docs.Key.Parent` 记左边是哪个）。
- 贮藏（`app/stash.go`）可以展开看文件（`-u` 贮藏的未跟踪文件在 `^3`）；`ApplyStashFiles` 用 `restore --source` 只取几个文件，工作区里这些文件有改动时先问。

#### 列表行

右栏历史的列表行在 `ui/history.go`，一个提交一行。

- 日期段头是单独的列表行（`HistRow.CI == DayRow`，所有仓库是 `AllDay`）。分段是 `app.Bucket`：今天、昨天、本周（从周一起）、上周、本月，再早的一个月一段；字是 `groupLabel`，行尾的时间 `rowWhen` 跟着段变格式。段头用 `ListState.Header`：滚动时钉在顶上、下一天推走它，键盘和鼠标选不中，要有底色。
- 第一行“未提交的改动”只在工作区有改动（或没读到状态、看工作树 / 标签）时有（`uncommittedRow`），它来去时 `keepRowSel` 把 `Hist.Sel` 挪一行。
- 展开的提交不画选中的底：`commitTitle` 是卡片的第一行，下面的 `commitCard` 在选中的底外面（`histParts`），展开的文件是单独的列表行，接着画卡片的两边和底（`histFileRow`）。
- 正文 `bodyItems` 拆成一条条，`Hist.BodyOpen` 记哪个提交的正文展开着；列表里量不了字，`bodyFits` 按字宽估要不要“展开全文”。
- 分支标记走 `foldRefs`：同名的远程分支折进本地分支。

#### agent 记号

`git.AgentOf` 认提交正文里的 `Co-Authored-By:` trailer（名字里的词或邮箱域名对上 `agentPats` 表），读 log 时填进 `Commit.Agent`。列表行不画记号，写在展开卡片的信息行里（`commitMeta`）。

“只看 agent 写的”（`App.HistAgent`，不进 state.json）读一页再筛，`Hist.read` 记 git log 读了几条，下一页从那里接着读。

#### blame

`app/blame.go`、`git/blame.go`：`ReadBlame` 读 `blame --porcelain`，按 `BlameKey`（文件 + 版本）缓存。编辑器左边叠一列（`ui/blame.go`，用编辑器的 Margin），也有只显示光标行的行内 blame。点一段可以在右栏历史里找到那次提交（`ShowInHistory`）。

### 提交图

`internal/graph` 只负责算：按显示顺序把提交传给 `Lanes.Next(hash, parents)`，返回这一行的泳道及上下连接（`Row{Lane, Edges}`）。状态存在 `Lanes` 里，下一页接着用。只看所有分支时用 `Reserve` 把 HEAD 固定在第 0 道。log 用 `--date-order`，避免分支连线过长。

`ui/graph.go` 画：

- 泳道间距 12，最多画 6 道，更右边的不画（线到边上淡出）。
- 颜色是淡的（不用红色）：第 0 道是强调色往次级字色挪一点（`accentLane`），后面是灰绿、灰沙、灰紫、灰青、灰褐、灰蓝，去掉和第 0 道最像的一个（`laneColors`，按强调色记住一套）。
- 线宽 1.5，换泳道的线走 `graphBend`（竖、圆角、平、圆角、竖）。节点不描底色的边（行悬停、选中时不出白晕）。
- 只有一条泳道（`GraphW <= 1`）时走 thin：淡灰细线和小点，线在节点前后留缝（`nodeGap`），HEAD 是强调色的细圈。
- “未提交的改动”那行是橙色虚线圈，用橙色虚线（`dirtyLine`）接到 HEAD 那道。
- 列表是筛过的（有搜索词、只看 agent、文件历史）时不算泳道，只画点（`HistDots` / `dotGraph`），相邻的点是父子才连一段线。
- 头部右键菜单里的“提交图”（`St.HistNoGraph`）关掉整个图。

### 工作树

`git worktree` 的链接工作树在 Git 栏仓库段里是一块“工作树”，按“工作树就是 agent 的工位”来设计。整块只在设置开了“在 Git 栏显示工作树”时有（见上面的[显示工作树 / 贮藏的开关](#显示工作树--贮藏的开关)）。代码：`app/worktrees.go`、`app/wtstate.go`、`app/wtwatch.go`、`git/worktree.go`。

#### 读取

- 只有 `<common>/worktrees` 目录存在时才读 `worktree list --porcelain -z`，免得给普通仓库多跑一条命令。
- 每个工作树的改动状态和领先 / 落后数只在要算状态时算（`WtInfo`，最多 4 个并发 `WtConcurrency`），见下面的 `WtTracked`。
- 路径比较一律走 `git.SamePath` / `Under`（处理大小写和符号链接）。
- 工作树的目录嵌在仓库根目录里时，仓库的文件树不列它下面的文件（`dropNested`）。

#### 状态和排序

`WtStateOf` 把 agent 状态（`WtAgent`）和 `WtInfo` 算成七种之一：等你 → 运行中 → 可合并 → 有改动 → 空闲 → 没有 agent → 已合并（值越小越需要你）。

- `SortWts` 按它排（主工作树在最前），`WtTop` 是收起的组头放的那一档。
- “已合并”是领先 0、落后至少 1（分支已经进了主线），目录没了（可清理）的也算；刚建、领先落后都是 0 的不算已合并。

#### 展开、收起和“需要你”

- 组默认收着：列不列行（`Repo.WtShown`）只看用户点开的（`WtOpen`，存会话；`GoToWorktree` 也会点开），有工作树在等你 / 在跑 / 可合并也不自动展开。
- 算不算状态、盯不盯目录另算：`Repo.WtTracked` = `WtOpen` 或 `wtAuto`（有工作树在等你 / 在跑 / 可合并，每帧 `SyncWtAuto` 对一次），所以收着时“需要你”条和组头的符号照样跟得上。
- 同时只展开一个工作树（`App.WtExp`，不存）：`git.BranchFiles` 先求 merge-base 再列 merge-base → 分支的文件（就是 `main...分支`），行是 `RowWtSummary` / `RowWtFile` / `RowWtTerm`。
- “合并到 main”走 `App.Merge`，先 `Ask`；灰着的原因是 `MergeWorktreeWhy`。
- “需要你”条是 `NeedsYou`（先等你、再可合并；可合并只有算过 `WtInfo` 的组才认得出），上一个 / 下一个是 `StepNeedsYou` → `GoToWorktree`。

#### 操作

- 单击一个工作树在右栏看它分支的历史（`viewWorktree`），“在工作区中打开”把它加成一个仓库（`OpenWorktree`）。
- 新建 / 锁定 / 解锁 / 移除 / 清理都是对话框 + `RunOp`。清理先 `git.PruneList`（`prune --dry-run -v`，从 stderr 读要清掉哪些），确认后再 `worktree prune -v`。

#### 监听

仓库的工作树块在跟踪时（`WtTracked`），里面的工作树各有一个监听（`app/wtwatch.go` 的 `syncWtWatch`，`watch.StartTree`，每个仓库最多 `WtWatchMax` 16 个；可清理的、裸的、已经作为仓库打开的不盯），改了只刷新那一行。

### 子模块

`git.ReadSubmodules` 读 `.gitmodules`（`git config -f .gitmodules`）和 `submodule status --cached`，最多 200 个。porcelain v2 的 `S<c><m><u>` 记在 `Change.Sub`。

- 子模块在文件树里是一行（不展开），在 Git 栏里的 Diff 是一段说明（`subDiffNote`：指针从哪个提交到哪个、里面有没有改动）。
- 丢弃子模块的改动是 `submodule update`（检出回父仓库记录的提交），只在指针变了（`Sub` 里有 `C`）时做；否则只 toast：里面的改动要在子模块自己里丢弃。
- 右键能初始化、更新、在新窗口打开、加进工作区（`OpenSubmodule`：没初始化的先初始化，初始化过的加成一个仓库）。

### 交互式变基

`app/rebase.go` 和 `docs.RebasePlan`。从历史里某个提交“从这里开始交互式变基…”打开一个向导标签：每行一个提交，选 pick / reword / squash / fixup / drop / edit、拖动排序、直接改信息。

- 只从当前分支的完整历史里开（`RebaseView`），范围必须是一条线、没有合并提交、最多 `RebaseLimit` 100 个。
- 开始时生成 todo：要改信息的（reword、squash 合起来的）写成 `pick` / `fixup` 加一行 `exec git commit --amend -F <信息文件>`，所以整个变基不开编辑器。
- todo 用 `GIT_SEQUENCE_EDITOR=cp <文件>` 换进去，经 `RunOpEnv` 跑。
- 停在冲突上交给横幅，`followRebase` 跟着进度。

### 克隆

`app/clone.go`。

- 对话框填地址、父目录（默认上次的 `St.CloneDir`）和目录名（`CloneName` 从地址推），可勾“含子模块”（`--recurse-submodules`）；剪贴板里像 git 地址（`LooksGitURL`）就预填。
- `git clone --progress` 在父目录里经 `RunStream` 跑，stderr 一段段读成进度（`CloneProgress`），侧栏文件页顶上一行显示。
- 克隆不属于哪个仓库的队列（仓库还没有），跟着窗口走（`a.Clones`），凭据提示和超时钟和写命令一样。
- 日志里的地址先抹掉密码（`redactURL`：用户名留着，密码换成 `***`）。
- 克隆完加进列表（空工作区时当工作区打开）；取消或失败时删掉没克隆完的目录，但只删克隆前不存在的目录。

### 远程网页

`git/web.go`：`ParseWeb` 把远程地址（ssh / https / scp 写法）转成网页地址，`KindOf` 认 GitHub / GitLab / Gitea / Bitbucket，自建的主机在 `Settings.WebHosts` 里指定风格。

`Web.Commit` / `File` / `Lines` 拼出提交、文件、文件某几行的链接；`App.WebOf`（`app/stash.go`）套上设置里按主机指定的风格，`Repo.WebRef` 是看工作区文件时钉的版本（HEAD 推送过就用它，否则远程的默认分支）。

### 普通文件夹

不是 git 仓库的文件夹（`Repo.Plain`）也能开：

- 文件树用 `files.Walk` 遍历（最多 `WalkCap` 50 000 个），监听用 `watch.StartPlain`。
- Git 栏里是一行灰着的段头，写“不是 Git 仓库”，不能展开；历史里不出现。需要 git 的命令遇到 `git.NotRepo` 时禁用。
- 每次刷新都重新看一下它是不是已经 `git init` 了；监听照样盯着根下 `.git` 的变化，`git init` 之后能跟上。

`Walk` 不进 `node_modules` 和点开头的目录（`.venv`、`.pytest_cache`……）：

- 把它们列成第四个返回值（名字后面带 `/`，同 `LsIgnored`），`LoadFiles` 当被忽略的目录传给 `Tree.Done`：树里灰显、收着，展开时 `ReadLazy` 读一层，`Paths` 里没有它们里面的（⌘P、全文搜索不进）。
- 这一条不看“显示被忽略的文件”开关，`files.exclude` / “隐藏的文件”照样能藏掉；`.git` 不列也不进，`.DS_Store` 不列。
- 监听（`ignorePlain`）照旧不看这些目录里面，根下增删这样的目录是根目录的变化，整棵重读时跟上。
