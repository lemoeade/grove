# 工作区、设置、会话和状态文件

[← 架构总览](../architecture.md)

工作区从哪来、设置和会话存在哪、state.json 的格式和迁移。

## 工作区、设置、会话和状态文件

### 工作区

工作区是一个 `.code-workspace` 文件（`folders` 里的每一项是一个仓库）或者一个文件夹。

- 打开文件夹时，若它位于 git 仓库中，就用该仓库；否则扫描下一层目录里的 git 仓库（跳过以点开头的目录）。没有找到仓库时，把它当普通文件夹（`Repo.Plain`，见 [Git › 普通文件夹](git.md#普通文件夹)）。
- 工作区之外临时加的仓库和隐藏的仓库记在会话的 `added` / `hidden`，不改工作区文件。

#### 空窗口和最近的工作区

没有工作区时显示空窗口，提供最近的工作区、打开文件夹和克隆入口；终端在家目录启动（`a.home`，不在仓库列表里）。

最近的工作区 `RecentWs` 最多 10 个（`MaxRecentWs`）。其中的 `.code-workspace` 文件也交给系统的“最近使用”（`AddRecentDocument`）；目录不交（从程序坞的“最近使用”打开的目录会被当成“加仓库”），设了 `WORKBENCH_STATE` 时一个都不交（`cmd/workbench/main.go` 的 `OnRecent`）。

#### 换工作区

`OpenWorkspace`（`app/workspaces.go`）：

- 就是当前这个：提示“已经开着”，什么都不做。
- 已经开在别的窗口里：把那个窗口叫到前面，这个窗口不换。
- 否则 `RequestLeave` 依次检查没保存的文件（`LeaveDirty`）、在跑的 git 写命令（`LeaveWrites`）和终端里在跑的程序（`LeaveJobs`，只有换工作区时问；关窗口、退出时终端直接关），需要时要求确认，通过后才调 `switchTo`。新工作区打不开时报错、现在的工作区不动，并把它从 `RecentWs` 里去掉。

“在新窗口中打开”调多窗口的 `Open`（见 [app.md](app.md) 的“多窗口”）。

#### files.exclude

`app/exclude.go`。三处的模式并在一起：

- `.code-workspace` 里的 `settings`（`workspace.LoadSettings`，JSONC）；
- 每个仓库的 `.vscode/settings.json`（`workspace.LoadFolderSettings`）；
- 设置“文件树”页的“隐藏的文件”（`Settings.Exclude`，逗号分开的 glob）。

`repoExclude` 把它们编成每个仓库一份 `globs.Set`，按 VS Code 的规则从仓库根算起。被排除的文件在文件树、⌘P、全文搜索、`#` 里都看不到；Git 栏的改动照常列。

- 工作区文件多半不在哪个仓库里、没有监听，所以切回窗口和 ⌘R 时 `checkWsSettings` 看一眼它改了没有。
- 仓库的 `.vscode/settings.json` 改了随增量刷新重读。
- 设置里的“隐藏的文件”停手 `ExcludeDelay`（600 ms）后才重读文件树。

### 设置

`workspace.Settings` 保存设置窗口里的选项，由进程内的所有窗口共用。改了以后 `SettingsChanged` 把设置同步到各窗口，`app/settings.go` 负责应用字体、主题、材质、缩放和终端选项等。

设置有三种改法，最后都更新同一个 `Settings`。

#### 设置窗口

`ui/settings.go`，第二个 `mygo.Window`，和主窗口共用打开它的那个 `App`。

- 每一项是 `settingRow` 表里的一行；右上角的搜索框按名字和说明筛（`settingsResults`）。
- 一页按 `settingsGroups`（页号 + 起头那一项的名字 → 小标题）分成几块，`settingsCards` 每块画一张圆角卡片，行间一道细线（`settingsSep`）。
- 一行是 `settingField`：左边名字和说明，右边控件。fn 里用 `settingControl` 记下的控件 `LabelledBy` 名字、`DescribedBy` 说明（点名字等于点控件，读屏读得到）。
- `wui.NumberInput` 里的输入框从外面够不着，所以数字项用自己拼的 `numberBox`。
- “材质”只在窗口底是通透时画，缩进 18。

#### 设置文档

编辑器区的 `settings.json` 标签（`docs.SettingsDoc`，只在内存里）。

- 内容是 `SettingsJSON` 生成的全部键（零值也写）；⌘S 应用设置，不写文件。
- `ParseSettings` 严格解析：有不认识的键或类型不对时，通过 `SettingsError` 报行列，不改任何设置；通过后走 `Clamp`，整体替换设置并应用到所有窗口。
- 别的入口改了设置后，没编辑过的设置文档由 `syncSettingsDocs` 重新生成，免得保存旧文档时盖掉新设置。

#### 导出 / 导入 / 恢复默认

导出是 `{"workbench": 版本, "exported": 时间, "settings": {全部键}}`；导入时列出会变的键（`ChangedKeys` / `SettingNames`）再确认。

### 会话

`workspace.Session` 按工作区文件的绝对路径记：打开的标签（最多 50 个）、当前标签、当前仓库、展开的仓库和目录、Git 栏各段和各组的收放、终端标签和分屏、编辑器组的布局、工作树块展开的仓库、面板开关。切走、关窗口、退出时存，打开工作区时读回来。

保存时的几个细节：

- 历史版本和 Diff 标签也记，`path` 写仓库根（读的时候会丢掉相对路径），真正的文件在 `change` 里；那个提交没了（`GoneCommit`）就不恢复。
- 标签手动选的编码、语言、缩进记在 `Tab.Enc` / `Tab.Lang` / `Tab.Indent`；换行不记。按十六进制看的记 `hex`；同一个文件在另一组里再开的镜像记 `copy`（第几份），重开时同一路径第二次出现就挂成前一份的镜像。
- 终端标签只记名字、目录、配置和分屏，以及后台会话的 id（见 [终端](terminal.md)）。只有一个没改名、在仓库根的终端时不记。开在编辑器区的终端标签（`kind` 为 `term`）按 `termNum` 对上。

### 状态目录

默认是 `~/Library/Application Support/Grove/`，`WORKBENCH_STATE=目录` 时用那个目录（测试和多开都靠它）。里面有：

| 文件 | 内容 |
|---|---|
| `state.json` | 设置、会话、最近的工作区、退出时开着的窗口，见下面 |
| `window-state.json` | 每个窗口的位置和大小（`WindowStates`，按 `StateKey`），在 `mygo.App.Run` 之前读 |
| `keybindings.json` | 自定义快捷键 |
| `history/` | 本地历史 |
| `termhist/` | 终端重开时补回的输出 |
| `snippets/` | 用户的代码片段 |
| `termd.sock` | 终端后台进程的套接字 |

日志不在这里，在 `~/Library/Logs/Grove/`（`WORKBENCH_LOGS` 可换）：`workbench.log`（超过 1 MB 时启动改名成 `workbench.old.log`）、`termd.log`、崩溃日志 `crash-*.log`（见 [app.md](app.md) 的“启动”）。

### state.json

```json
{"version":4,"appVersion":"0.2.0",
 "sidebar":true,"rightBar":true,"bottomBar":true,"sidebarMode":"files",
 "sidebarW":280,"rightW":320,"bottomH":240,"termW":420,"unified":false,"fullDiff":false,"diffTrim":false,
 "histAll":true,"histBranches":false,"histNoGraph":false,"openEditorsFolded":false,
 "recentWs":["/abs/x.code-workspace","/abs/dir"],"windows":["/abs/x.code-workspace",""],"cloneDir":"/abs/src",
 "recent":["/abs/repo/a.go"],"commitMsgs":["修好登录","…"],
 "searchHistory":[{"text":"TODO(","case":true,"word":false,"regexp":true,"include":"*.go","exclude":"vendor/","ignored":false,"big":false}],
 "taskHistory":[{"repo":"/abs/repo","cmd":"make test"}],
 "added":[],"hidden":[],
 "settings":{"theme":"dark","look":"grove","backdrop":"vibrancy","material":"menu","editorFont":"…","editorSize":13,"indent":"2",
             "autoSave":"delay","shell":"…","termFont":"…","termSize":12.5,"termTheme":"","scrollback":10,
             "termProfiles":[{"name":"fish","shell":"/opt/homebrew/bin/fish","args":["-l"],"env":["K=V"]}],
             "noHoverDefinition":true,"treeNoCompact":true,"pullMode":"rebase","zoom":110,"historyDays":90,"lang":"en"},
 "sessions":{"/abs/x.code-workspace":{
   "added":["/abs/extra"],"hidden":["/abs/other"],
   "tabs":[{"path":"/abs/a.md","pinned":true,"preview":true,"line":3,"col":1,"top":0},
           {"path":"/abs/a.go","enc":"gbk","lang":"Go","indent":"tab","hex":false,"copy":1},
           {"path":"/abs/repo","kind":"version","repo":"/abs/repo","change":{"kind":"commit","path":"a.go","code":"M","commit":"5fd1…","parent":"d1e9…"}},
           {"path":"/abs/repo","kind":"diff","repo":"/abs/repo","change":{"kind":"unstaged","path":"b.go","orig":"old.go","code":"R","sub":"","range":false}},
           {"path":"/abs/repo","kind":"term","repo":"/abs/repo","termNum":4}],
   "active":0,"repo":"/abs/repo","open":["/abs/repo"],"dirs":{"/abs/repo":["docs"]},
   "git":{"/abs/repo":"closed"},"groups":{"/abs/repo":["stash","unstaged"]},"wtOpen":["/abs/repo"],
   "terms":{"/abs/repo":{"tabs":[{"name":"构建","dir":"/abs/repo/docs","sids":["7"]},
                                 {"split":{"row":true,"ratio":0.4,"a":{"hist":"1793001600000-3.vt"},"b":{"dir":"/abs/repo/src","sid":"9","profile":"fish"}}},
                                 {"inEditor":true,"num":4}],"active":1}},
   "layout":{"row":true,"ratio":0.5,"a":{"tabs":[0]},"b":{"tabs":[1,2],"sel":1,"focus":true}},
   "panels":{"sidebar":true,"rightBar":false,"bottomBar":true,"sidebarMode":"git","sidebarW":300,"rightW":320,"bottomH":240,"termW":420,
             "unified":true,"fullDiff":false,"diffTrim":false,"histAll":true,"histBranches":false,"histNoGraph":false,"openEditorsFolded":false}}}}
```

示例为了把键写全，零值也写了；实际文件里带 `omitempty` 的零值不写。`TestStateDocKeys`（`internal/workspace/workspace_test.go`）核对 State、Session 和它们里面各个类型的每个键都在上面的示例里（`settings` 不查）。

#### 几个键的意思

- `appVersion`：上次用的应用版本，换了版本启动时状态栏提示一下。
- 顶层的布局（`sidebar` … `openEditorsFolded`）是最近存的那个窗口的，给没记过布局的工作区和空工作区用；每个工作区自己的布局在会话的 `panels`（`State.Panels()` 抄进去，打开 / 切到这个工作区时 `SetPanels` 灌回顶层）。
- 最近的在前、所有窗口共用的几个列表：
  - `recent`：最近打开的文件，最多 50 个；
  - `commitMsgs`：提交信息历史，最多 30 条；
  - `searchHistory`：最近搜索连同开关和包含 / 排除，最多 20 条（老版本只记字、键叫 `searches`，读的时候迁过来）；
  - `taskHistory`：最多 20 条；
  - `recentWs`：最多 10 个。
- `windows`：退出时开着的窗口的工作区（按槽位，空工作区是 `""`），下次启动一个个开回来；只有一个窗口、开的就是最近的工作区时不记。
- 顶层的 `added` / `hidden` 是老版本（不分工作区）的，存盘时是空的；读到时归给第一个打开的工作区。现在记在会话里。
- 会话里的：
  - `layout` 是编辑器组的拆分树（叶子记标签下标），只有一组时不记；
  - `terms` 只有一个没改名、在仓库根的终端时不记；
  - `tabs` 里 `kind` 为 `term` 的是开在编辑器区的终端，按 `termNum` 对上 `terms` 里 `num` 相同的那个。

#### 兼容规则

**新字段**一律用 `omitempty`，旧文件缺的字段当默认值；读取时忽略不认识的字段（Go 的 JSON 解码默认如此）。新增字段不升格式版本号，旧应用照样能读新文件，只是用不上新内容。`settings` 里没设的键不写。

**默认开的开关存反面**，键名带 `no` 或 `No`，这样默认状态什么都不写：

- 顶层的 `histNoGraph`；
- 设置里的 `noDiagrams`、`noRemoteImages`、`noPreviewScrollSync`、`noEditorScrollSync`、`noStickyScroll`、`noWordComplete`、`noHoverDefinition`、`noLocalHistory`、`treeNoFollow`、`treeNoCompact`、`treeNoGuides`、`treeNoStatusColors`；
- 还有默认彩色、存反面的 `treeIconsPlain`。

老版本存正面、默认关的 `treeStatusColors` 由 `legacyTreeColors` 认（写着 false 的读成 `treeNoStatusColors`）；老的 `softWrap` 开关由 `legacySoftWrap` 读成 `wrap: "all"`。

**读取时 `clamp` 清理数据**，只丢读不懂的，合法的原样留着：

- 空的会话键和 `null` 会话丢掉。
- 该是绝对路径的，相对路径的丢掉：`recentWs`、`recent`、`windows`（`""` 留着）、`taskHistory` 的仓库、`added` / `hidden`；会话里 `tabs` 的 `path`、`repo`（清空）、`open`、`wtOpen`，`dirs` / `git` / `groups` / `terms` 的仓库根键，终端标签的 `dir`（整个标签丢掉）和分屏格的 `dir`（清空）。`dirs` 里的目录是仓库内路径，绝对路径的丢掉；`git` 只认 `open` / `closed`。
- 去重、截到上限：`recentWs` 10、`recent` 50、`commitMsgs` 30（空的丢掉）、`searchHistory` 20（没有字的丢掉）、`taskHistory` 20（带换行的命令丢掉）、`windows` 100、会话的 `tabs` 50（行列、滚动为负的丢掉）、每个仓库的终端标签 20；分屏树不成形或超过 8 格的整棵不要，比例不对的换成一半，`hist` 不是 `.vt` 文件名的清掉。
- 越界下标：会话的 `active` 越界回到 -1，终端的 `active` 回到 0。
- 布局（`clampPanels`）：不认识的侧栏模式回到文件（认 `files` / `search` / `git` / `outline`）；宽高超出范围的回到默认：`sidebarW` 160–800、`rightW` 200–900、`bottomH` 80–1200、`termW` 240–1200。
- 设置（`Settings.Clamp`，设置文档保存、导入时也走它）：
  - 选项不在可选列表里的（主题、风格、窗口底、材质、强调色、缩进、换行、空白、自动保存、终端位置、拉取方式、语言）回到默认；强调色写成默认那个的存成 `""`。
  - 带控制字符或超长的终端主题名清空。
  - 数值超出范围的回到默认（存成 0）：编辑器 / 终端字号 9–32、回滚 0–1000、缩放 `zoom` 50–300、浮层不透明度 `frostOpacity` 50–100；本地历史天数 `historyDays` 只认可选的几档（7 / 30 / 90 / 365）。正好是默认值的缩放（100）、不透明度、天数也存成 0。
  - 用不了的终端配置（`Profiles`）丢掉；网页主机里种类不认识或主机名超过 255 字节的丢掉。

**格式版本号**只在字段含义变了时升。当前是 4，最近一次变的是会话里 `groups` 的含义：以前记“收起的组”，现在记“用户点过、和默认状态相反的组”。冲突 / 已暂存 / 未暂存默认展开，贮藏默认收起（`groupOpenByDefault`），所以含 `"stash"` 表示“贮藏展开着”。读 3 的文件时把 `groups` 里的 `"stash"` 去掉。

以前未跟踪单独一组（键 `"untracked"`），现在并进未暂存（`GroupKey(git.Untracked)` 是 `"unstaged"`）；旧文件里的 `"untracked"` 读的时候扔掉（`groupKeys` 不认），不升版本。

**更早的格式**由 `workspace.migratePanels` 兼容：1 的 `panels` 表、更早的四元 bool 数组、更早的 `repos.json`；顶层的 `added` / `hidden` 归给第一个打开的工作区。下次保存就是 4。

#### 几个窗口共用

所有窗口共用一份 state.json，通过 `workspace.Store` 保存各自的会话。内容没变时不写盘（`Store.Save` 和上次写的字节比）。
