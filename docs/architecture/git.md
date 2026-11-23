# Git

[← Architecture overview](../architecture.md)

The Git panel and merged history that show every repository at once; how git commands run (read-only, the write queue,
credentials); and branches, history, the commit graph, worktrees, submodules and more. Diff tabs and hunk / line staging
live in [staging.md](staging.md).

## All repositories visible at once

This is Grove's first principle: the file tree, Git panel, search and history all list every repository in the workspace
at the same time; there is no "current repository + switcher" view. New features must follow it too — don't add a list
that shows only the current repository.

The "current repository" (`Current()`) still exists, but it is used in only a few places: where a new terminal opens, the
"Current repository" page of the right-panel history, and the branch pill in the title bar. Clicking in the file tree or
Git panel, or switching to a file tab, updates the current repository (`UseRepo` / `SelectRepo`); ⌘1–⌘9 jump to the nth
repository (`JumpRepo`).

The left side of the status bar doesn't show the current repository; it shows a summary of all of them (`reposSummary`):
how many repositories, how many have changes / need pushing, which one is merging, and read failures in red. Hovering shows
one line per repository, clicking opens the Git panel; with a single repository it shows that repository's own changes.

### Git panel

Code: `app/changes.go` (`GitRows` and the row model), `ui/gitbar.go` (drawing).

#### Row layout

`GitRows` arranges all repositories into one table. Each repository is a section; from top to bottom:

1. The section header (`RowRepo`, sticky)
2. The banner for an operation in progress (`RowOngoing`)
3. The worktree group header (only when "Show worktrees in the Git panel" is on in Settings; collapsed by default)
4. The commit box (`RowCommit`, above the changes, so you don't have to scroll to the end of the section when there are many files)
5. Each group's title and its changes (Conflicts / Staged / Changes), plus note rows
6. The stash group header (only when "Show stashes in the Git panel" is on in Settings; collapsed by default)

The stash and worktree group headers show their name and count just like the Staged / Changes groups (`foldHeader`'s
`iconOnly` is false for all of them). A repository whose working tree is clean has no note row; its header shows a green check.

Section folding is `GitFold`:

- `FoldAuto`: expanded when there are changes, collapsed to one line when clean. Repositories with worktrees (while
  worktrees are shown) or with a merge / rebase stopped halfway are also expanded, so the worktree block and the banner are visible.
- `FoldOpen` / `FoldClosed`: once the user clicks the arrow, their choice wins; it is saved in the session's `git`.

#### Show worktrees / stashes toggles

The two toggles are `Settings.ShowWorktrees` / `ShowStashes` (the zero value is off, so old state.json files need no
migration). `Repo` has no `App` pointer, so `App.newRepo` hands each repository the window's `&a.St.Settings`
(`Repo.set`, re-pointed after switching workspaces with `*a = *n`), and `Repo.ShowWts` / `ShowStashes` read it.

With worktrees off:

- `GitRows` emits no worktree block, `GitOpen`'s auto-expand ignores `HasWorktrees`, and the header draws no worktree count;
- `NeedsYou` is empty and `GoToWorktree` does nothing;
- `WtShown` / `WtTracked` are both false (no `WtInfo` is computed, no folders are watched; the session's `WtOpen` is kept).

When a toggle changes, `SettingsChanged` → each window's `syncShownGroups` finishes up: turning it off collapses `WtExp`,
runs `clearRefHistories`, drops `WtInfo`, and stops watching via `syncWtWatch`; turning it on recomputes from `WtOpen` and
reattaches the watchers. Both run `syncChangeSel`, clearing a selection that sat on a row that no longer exists.

With stashes off, `appendStashes` simply emits no rows and the header draws no stash count; `r.Extra.Stashes` is still read,
and `git.stash` / `git.stashPop` still work.

#### Change groups and change rows

Untracked files and unstaged changes share the "Changes" group (`groupKind`, `groupTitle`):

- Order follows `Status.Changes` (tracked first, untracked after them), and the count is the sum of both; when there are
  only untracked files the group is still called "Changes".
- Each row keeps its own `Change.Kind` (letter, color, discard, stage and folder expansion work as before).
- The group header's + is `StageUnstaged`: with no conflicts it is a path-less `git add -A` (passing paths would exceed
  macOS's command-line length limit with tens of thousands of changed files); with conflicts it passes paths,
  `git add -A -- …`, never a path-less `add -A` / `-u` (that would stage the conflicted files too, which marks them resolved).
- The group header's discard is still `git restore .`, which leaves untracked files alone; it is hidden when the group is entirely untracked files.

Change rows, files in a stash and files in a commit are all rendered the same way (`changeLabel`): the file tree's file
icon on the left, the status letter (`codeLetter`) at the end of the row; for files in a commit it sits to the right of the
line counts; on a hovered change row the letter gives way to the buttons. When space is short the path shrinks first; the
file name is truncated only if it is longer than the whole row.

The buttons on group titles (`showActions`) appear only when the pointer is over them; right-click (`groupMenu`) offers the same actions.

#### Section header

The section header (the header part of `ui/gitbar.go`), left to right: the fold arrow, the repository dot (`repoDot`), the
repository name, the branch button, the linked-worktree marker, and the pill for an operation in progress; when collapsed,
also the stash count / worktree count. On the right, depending on the situation: the queue button / a reading spinner /
a ! for a read failure / "Not a Git repository" / a check when clean / the change count, then the ↓↑ counts, and finally
this repository's pull / push buttons (`syncButton`).

The pull / push buttons share `syncOf` with the title bar's sync button (tooltip text, grayed out with no upstream,
spinning while `Repo.Pulling` / `Pushing` is running or queued); clicking goes through `App.Pull` / `Push` — there is no
separate set of git calls.

#### Selection, multi-select and inline buttons

A row's identity is `ChangeRow.Key()` (repository + row kind + change kind + path). Selection `GitSel`, hover `GitHover`
and multi-selection `GitChosen` are all recorded by key, so after a refresh moves rows around, the selection still points at the same item.

- Multi-select operations (`StageChanges` / `UnstageChanges` / `DiscardChanges` / `OpenDiffs`) are first grouped by
  repository (`byRepo`), then queued in each repository's own queue; opening more than 20 Diffs at once asks for confirmation first.
- The inline buttons that appear on a hovered change row (or the selected row when the list has the keyboard;
  `changeActions` / `stageActions` in `ui/gitbar.go`) use `MenuChanges`, with the same convention as the context menu
  (`actOn` in `app/chosen.go`): if the row under the pointer is part of a multi-selection of two or more rows, the action
  applies to all selected rows; otherwise only to that row. Button actions are deferred via `gitFrame.do` until the list has been built.
- When the list has focus, Space / ⌘⌫ are `changes.toggleStage` / `changes.discard` (context `GitList`; the list handles
  the keys itself, and they are also available in the Command Palette), acting on `KeyChanges`.
- When staging / unstaging / discarding will move the selected row away, `markGitAfter` first records the first row after
  it (or before it, if none) that isn't selected; after the status is reread, if `syncChangeSel` can't follow the current tab, the selection lands on that row.

#### Commit box

There is one per repository (`CommitOpen` / `CommitMsg`), below the section header (and worktree group header) and above
the change groups. Committing, amending the last commit and the commit message history live in `app/commit.go`.

- When it is expanded (`CommitShown`): there are staged changes, a message has been written, an amend is in progress,
  the conflicts of a merge are all resolved, or the user opened the commit box.
- `ShowCommit` scrolls the section header to the top (`ui.Start`) so the sticky header doesn't cover it. ⌘↩ commits the box that has the keyboard.
- One box does it all (`commitRow`): collapsed or with no message it looks like a single-line input; once a message is
  written (or while amending), the explanation and split button (`commitFoot`) grow out of the bottom of the box.
- In the Grove style the button shows only a paper plane (`iconSend`); the consequence is spelled out in the
  screen-reader name and the first line of the tooltip: "Commit N files", "Commit all N files" when nothing is staged, or "Amend Commit".
- The VS Code style uses `vsCommitRow`: a white input with a thin border, with a full-width "✓ Commit …" split button
  (`vsCommitButton`) always below it, whose text is the consequence described above; clicking it with no message runs
  `OpenCommit`; any unusual explanation goes on a line below the button.
- The button's ▾ and the box's context menu are the same menu (`commitBoxMenu`): Commit and Push (`CommitPush`, which
  ignores the auto-push setting; when it can't push, `CommitPushWhy` gives the reason), Amend Last Commit, Undo Last Commit.
  With no message, the ▾ only appears while the pointer is over the box.
- When committing isn't possible (`CanCommit`), the button is drawn gray and clicking it doesn't commit, but `Disabled` is
  not used (mygo's `Disabled` suppresses the tooltip); the tooltip gives the reason (`CommitWhy`).
- The last 30 commit messages (`MaxCommitMsgs`, shared by all repositories) are stored in `St.CommitMsgs`; ↑↓ cycles through them when the box is empty.

"Commit all N files" (`CommitsAll`): runs `git add -A` first and then commits, including untracked files; if the add
fails, nothing is committed. This path isn't taken when there are conflicts or an operation in progress. If a git command
is running / queued in the repository, or its status is being read, the commit is held off (a toast asks you to click
again in a moment) so that other files don't get committed along with it.

When amending, if the last commit has already been pushed (`HeadPushed`), `AskDanger` asks first: amending means a force push.

#### Banner and other buttons

- The in-progress banner's (`ongoingRow`) Continue / Skip / Abort are small labeled buttons (`bannerButton`; Continue has
  an accent background, and is grayed out with the reason in its tooltip while there are conflicts).
- The row under an expanded worktree (`wtSummaryRow`) shows "N commits · N files" and +/−.
- Actions in the other panels (the conflict bar, the Diff line-selection bar and hunk buttons, search's Replace All, the
  rebase wizard, the terminal's exit overlay) all use the same `iconAction` (`ui/widgets.go`): icon only, its screen-reader
  name is the text the button used to carry; when `why` is non-empty it is drawn gray, the reason goes in the tooltip, and clicking does nothing.

### Merged history

Besides the "Current repository" page, the right-panel history has an "All Repositories" page (`histAll`).

- `AllHistRows` does a k-way merge of each repository's page of commits by author time (`%aI`). Repositories load at
  different rates, so it only merges down to the time every repository has reached (the horizon); scrolling down calls
  `LoadMoreAllHistory`, which loads another page for the repositories that are behind.
- At the top are the "Uncommitted changes" rows of repositories with changes, one row per repository.
- The selection is `HistAllSel`.
- The All Repositories page draws no commit graph: each row has a repository dot on the left, and the repository name at
  the end of the row is a pill with a faint background of the same color (`repoChip`).

Repository colors (`repoColor`, cycling through six muted colors in workspace order) are also used for the dots
(`repoDot`) on repository rows in the file tree and Git panel, on terminal tabs, and on the start page.

## Git

### Reads and refresh

All read-only commands carry `GIT_OPTIONAL_LOCKS=0 LC_ALL=C`: the former keeps `git status` from refreshing the index, so
it doesn't fight over the lock with git operations in a terminal; the latter pins the output language so it can be parsed.

A repository's status is `git status --porcelain=v2 -z --branch` plus `git.Extra`:

- The operation in progress (`Ongoing`) and its progress (`Progress`)
- HEAD, and `HeadRoot` (HEAD has no parent)
- Unpushed commits (`rev-list -n 200`, `MaxUnpushed`)
- Stashes (at most 50, `MaxStashes`)
- The list of remotes and the remotes' web URLs
- `GitDir` / `CommonDir` / `Linked` (whether it is a linked worktree)
- Submodules (`Subs`, at most 200)

Refreshing has exactly one entry point: `RefreshRepo(r, why)` in `app/watch.go`.

- `why` is one of `WhyManual` / `WhyOpen` / `WhyOp` / `WhySave` / `WhyFileOp` / `WhyWatchGit` / `WhyWatchFiles` /
  `WhyFocus`, recorded in `Repo.LastWhy` for logs and tests.
- It reads the status, branches, file tree (when needed) and rereads open documents (`syncDocs`) in the background, then
  swaps them all in together on the main thread.
- When only working-tree files changed (`filesOnly`), history and `git.Extra` are not reread; the path hints from the
  watcher and from saves let the file tree update only the affected spots.
- A refresh request that arrives during a read is recorded in `r.again`, and one more read runs after the current one
  finishes; reads never run concurrently.
- When the window comes back to the foreground, `RefreshOnFocus` rereads only the repositories without a watcher (errored,
  folder gone, never started), at most once per 3 seconds; watched repositories aren't reread.

### Write commands and the queue

Write commands go through `git.RunRW` / `RunOp`: non-interactive (`GIT_TERMINAL_PROMPT=0`, `GIT_EDITOR` /
`GIT_SEQUENCE_EDITOR` replaced with `true`, the pager replaced with `cat`), a 2-minute timeout by default, run in their own
process group; on cancel the whole group gets SIGTERM, then SIGKILL.

Each repository has one queue (`app/queue.go`); `App.RunOp` enqueues into it, and only one command runs at a time.

- The queue button (`ui/queue.go`) sits in the Git panel's section header and in the title bar, spinning while something
  runs; in its menu, `CancelOp` cancels the running command and `Dequeue` removes one that hasn't started.
- After a cancel, Grove checks for leftover lock files such as `index.lock` (`git.StaleLocks`) and only warns about them,
  never deletes them, since another git process may be running.
- "Fetch All / Pull All" is a `Batch`: repositories are processed one after another in Git panel order, a failure in one
  doesn't affect the rest, the status bar shows progress, and `StopBatch` stops the ones that haven't started.
- Pull defaults to `--ff-only`; Settings can switch it to rebase or merge (`PullArgs`).

Long tasks with continuous output (like clone) use `RunStream` + `Keep`: `readChunks` reads output in chunks and calls
`Touch` on progress to reset the timeout clock, so a task only times out once it stops making progress.

On failure, `git.Explain` recognizes common errors (no upstream, rejected, authentication failed, merge conflict, etc.)
and gives a short explanation; `AlertDetail` puts the last few lines of git's raw output into the details section.

Git commands that need a terminal (`git.NeedsTerminal`: `-i`, `-p`, `commit` without `-m` and the like), when typed in the
Command Palette's `>git`, aren't run in the background; a terminal opens to run them instead (`runGitInTerm`).

### Credentials

When git needs a username, password or SSH passphrase, Grove shows a credentials dialog. The code is in `internal/askpass`
(the inter-process layer) and `app/askpass.go` (the dialog).

- It starts lazily, just before the first write command runs: it creates `grove-askpass-<pid>.sock` (a 0600 socket) and
  `grove-askpass-<pid>.sh` (a 0700 script whose content is just `exec Grove askpass "$@"`) under `os.TempDir()`. `Listen`
  first runs `CleanStale` to remove these two files left behind by processes that have exited.
- Only write commands' environments carry `GIT_ASKPASS` / `SSH_ASKPASS` / `SSH_ASKPASS_REQUIRE=force` pointing at it,
  plus `WORKBENCH_ASKPASS_SOCK` / `WORKBENCH_ASKPASS_ID`; read commands don't, so a refresh never pops up a dialog.
- The `Grove askpass <prompt>` child process (`askpass.Main`) connects to the socket, sends one line of JSON (the prompt
  and the value of `WORKBENCH_ASKPASS_ID`), waits for the answer and writes it to standard output.
- The app finds the matching window and command by id (`askRegister` / `askDone`), `Classify` recognizes the kind of
  prompt (four kinds), and the prompt goes into `a.Cred` to show the dialog; once the user answers, `AnswerCred` replies.
  Canceling shows a "Canceled" toast. Answers never go into the log.
- While the dialog is open, the command's timeout clock is paused (`Keep`'s `Hold` / `Release`).
- In the HTTPS username dialog you can fill in the password at the same time: the password stays in `credCache` for
  2 minutes and is used once, so the password prompt that follows doesn't show a dialog.
- The tests' `TestMain` also recognizes askpass mode, so the test binary can serve as the askpass script's entry point too.

### Conflicts and three-way merge

Conflict markers are parsed by `docs/conflicts.go`: `ParseConflicts`, plus the `Doc` method `ConflictBlocks` (cached by
`Buf.Version`); `Resolved` computes the replacement text and range after resolving one block.

`ResolveConflict` / `ResolveAll` in `app/conflicts.go` only change the buffer: keep ours, theirs, or both. Only
`MarkResolved` runs a git write command: it saves the file first, then runs `git add`; if the file still contains conflict markers, it asks first.

- Conflicted files are in the Git panel's "Conflicts" group (`Change{Kind: Conflict}`); a single click opens them in the
  editor (`OpenConflict`, falling back to opening a Diff if the file is gone), and the conflicts are resolved there.
- The context menu can open two kinds of Diff: HEAD vs. the working tree, or "both sides" (`Code: 'S'`, where
  `git ls-files -u` finds `:2:` ours and `:3:` theirs).
- In a file tab, `ui/merge.go` colors the conflict blocks each frame from the parse result and draws red dots in the
  line-number gutter (`conflictMarks`); the buttons are on the conflict bar (`mergeBar`).
- Shortcuts (`mergeKeys`): ⌃⌥1 / 2 / 3 accept ours / theirs / both; ⌥F7 / ⌥⇧F7 or ⌃⌥↓ / ↑ jump to the next / previous conflict.
- The change bar (GutterMarks) is used only for conflicts.

### Operations in progress

When a merge, rebase, cherry-pick or revert stops halfway (`git/state.go` recognizes only these four), the top of the
repository's section shows a banner (`RowOngoing`) with progress (`ReadProgress`: rebase step n of m).

The buttons are Continue / Skip / Abort (`app/ongoing.go`: `ContinueOngoing`, `SkipOngoing`, `AbortOngoing`):

- You can't continue while there are still conflicted files.
- Skip and Abort go through `AskDanger` first; a merge has no Skip.
- Continue and Skip go through `runStep`: a non-zero exit code with `CONFLICT (` in the output is treated as stopping
  again, not as an error; only other non-zero exits are failures.

Everyday operations are in `app/gitdaily.go`: `Merge`, `RebaseOnto`, `CherryPick`, `Revert`, `ResetTo` (soft / mixed /
hard; hard needs confirmation), `UndoCommit` (soft-resets one commit and puts its message back in the commit box),
`PushForce` (`--force-with-lease`), `Sync` (pull, then push). When one stops on a conflict it hands off to the banner
(`stopOp`). `inHead` tells whether a commit is on the current branch.

#### Why an action is unavailable

All in `app/gitwhy.go`: one `XxxWhy` per kind of Git operation (`FetchWhy`, `PullWhy`, `PushWhy`, `PushForceWhy`,
`SyncWhy`, `StageAllWhy`, `StashWhy`, `RefWhy` / `MergeWhy` / `RebaseOntoWhy`, `SwitchWhy`, `DeleteBranchWhy`,
`RemoveWorktreeWhy`, `UndoWhy`, `PickWhy`, `ResetWhy`, `RebaseWhy`…). An empty string means the action is allowed;
otherwise exactly one reason is reported, in a fixed "most fundamental first" order. They are used in:

- Native menus: the ui's `whyItem` / `withWhy` append the reason in parentheses after the label;
- Buttons: in the tooltip (`syncOf`);
- The command registry's `Command.Why`: for the Command Palette (shown after the grayed-out command) and the menu bar (clicking shows the reason);
- The action itself checks again at the start (`a.Blocked(XxxWhy(…))`: if non-empty, show a toast and don't run), so a
  state change while a menu is open can't make it go ahead anyway.

Auto-push after commit, and the push half of sync after pulling, use the unchecked `pushNow` (the status hasn't been refreshed at that point).

### Branches, tags, remotes

`app/gitrefs.go`: `RefDialog` is one dialog with five uses (`RefDlgKind`: rename branch, new tag, delete tag, rename
remote, add remote), with a live preview of the command to run at the bottom; `refNameErr` checks for duplicate names and characters git won't accept.

- Before deleting a branch, Grove counts in the background how many commits aren't merged and then asks; deleting a
  remote branch uses `splitRemote` to split off the remote name.
- `TagRemote` figures out which remote a tag should be pushed to.
- Tags and remotes are read in `git/refs.go`.

The right-panel history can show a branch's or tag's history (`Hist.RefKind`), or a worktree's.

### History

#### Reading and searching

`git.LogQuery` builds the log arguments: search terms, all branches, path. The right panel's search box is parsed by `ParseHistQuery`:

- `key:value`: `author:` / `作者:`, `path:` / `路径:`, `after:` / `since:` / `之后:`, `before:` / `until:` / `之前:`;
- `/regex/`;
- Dates are handed to git's own date parser (`git.ResolveDate`), so things like `2.weeks.ago` work.

File history uses `--follow`; git can't combine `--follow` with search terms, so in that case up to `MaxFollowSearch`
(5000) commits are read and filtered in Go (`matchLocal`).

A page (`LogPage`) is 200 commits; scrolling down loads the next page.

#### Commit details, comparison, stashes

- The file list in commit details is `diffTreeFiles`: first `--name-status`, then `--numstat` for added / removed line counts.
- Comparing two commits is in `app/compare.go` (`docs.Key.Parent` records which one is on the left).
- Stashes (`app/stash.go`) can be expanded to show their files (untracked files of a `-u` stash are in `^3`);
  `ApplyStashFiles` uses `restore --source` to take just a few files, and asks first if those files have changes in the working tree.

#### List rows

The right-panel history's list rows are in `ui/history.go`, one row per commit.

- Date headers are separate list rows (`HistRow.CI == DayRow`; `AllDay` for All Repositories). Grouping is `app.Bucket`:
  Today, Yesterday, This week (starting Monday), Last week, This month, then one group per month for anything older; the
  text is `groupLabel`, and the time at the end of each row (`rowWhen`) changes format with the group. Headers use
  `ListState.Header`: pinned to the top while scrolling, pushed away by the next day, not selectable by keyboard or mouse, and they need a background.
- The first row, "Uncommitted changes", only exists when the working tree has changes (or the status hasn't been read,
  or you're viewing a worktree / tag) (`uncommittedRow`); when it comes and goes, `keepRowSel` moves `Hist.Sel` by one row.
- An expanded commit doesn't draw the selection background: `commitTitle` is the card's first line, the `commitCard`
  below it sits outside the selection background (`histParts`), the expanded files are separate list rows, and then the
  card's sides and bottom are drawn (`histFileRow`).
- The message body `bodyItems` is split into items, and `Hist.BodyOpen` records which commit's body is expanded; text
  can't be measured in the list, so `bodyFits` estimates from character widths whether "Show all" is needed.
- Branch labels go through `foldRefs`: a remote branch with the same name is folded into the local branch.

#### Agent marker

`git.AgentOf` recognizes the `Co-Authored-By:` trailer in a commit message (a word in the name or the email domain
matching the `agentPats` table) and fills in `Commit.Agent` while reading the log. List rows don't draw the marker; it goes
on the info line of the expanded card (`commitMeta`).

"Agent commits only" (`App.HistAgent`, not saved to state.json) reads a page and then filters it; `Hist.read` records how
many commits git log has read, and the next page continues from there.

#### Blame

`app/blame.go`, `git/blame.go`: `ReadBlame` reads `blame --porcelain` and caches it by `BlameKey` (file + revision). A
column is overlaid on the left of the editor (`ui/blame.go`, using the editor's Margin), and there is also an inline blame
for the cursor line only. Clicking a block finds that commit in the right-panel history (`ShowInHistory`).

### Commit graph

`internal/graph` only does the computation: commits are passed to `Lanes.Next(hash, parents)` in display order, which
returns the row's lane and its connections above and below (`Row{Lane, Edges}`). State lives in `Lanes` and carries over
to the next page. When viewing all branches, `Reserve` pins HEAD to lane 0. The log uses `--date-order` so branch lines don't get too long.

`ui/graph.go` draws it:

- Lanes are 12 apart and at most 6 are drawn; anything further right isn't drawn (lines fade out at the edge).
- Colors are muted (no red): lane 0 is the accent color shifted slightly toward the secondary text color (`accentLane`),
  then gray-green, gray-sand, gray-purple, gray-cyan, gray-brown and gray-blue, minus the one most similar to lane 0
  (`laneColors`, one set remembered per accent color).
- Lines are 1.5 wide; lines that change lanes follow `graphBend` (vertical, rounded corner, horizontal, rounded corner,
  vertical). Nodes have no background-colored outline (no white halo on hovered or selected rows).
- With a single lane (`GraphW <= 1`) it goes thin: a faint gray thin line and small dots, with gaps in the line before and
  after each node (`nodeGap`); HEAD is a thin ring in the accent color.
- The "Uncommitted changes" row is an orange dashed ring, connected to HEAD's lane by an orange dashed line (`dirtyLine`).
- When the list is filtered (search terms, agent commits only, file history), no lanes are computed and only dots are drawn
  (`HistDots` / `dotGraph`); adjacent dots are connected by a line segment only if they are parent and child.
- "Graph" in the header's context menu (`St.HistNoGraph`) turns the whole graph off.

### Worktrees

`git worktree`'s linked worktrees appear as a "Worktrees" block in the repository's Git panel section, designed around
"a worktree is an agent's workstation". The whole block only exists when "Show worktrees in the Git panel" is on in
Settings (see [Show worktrees / stashes toggles](#show-worktrees--stashes-toggles) above). Code: `app/worktrees.go`,
`app/wtstate.go`, `app/wtwatch.go`, `git/worktree.go`.

#### Reading

- `worktree list --porcelain -z` is only run when the `<common>/worktrees` directory exists, to avoid an extra command for ordinary repositories.
- Each worktree's change status and ahead / behind counts are only computed when its state is needed (`WtInfo`, at most 4
  concurrently, `WtConcurrency`); see `WtTracked` below.
- Path comparison always goes through `git.SamePath` / `Under` (handling case and symlinks).
- When a worktree's folder is nested inside the repository root, the repository's file tree doesn't list the files under it (`dropNested`).

#### State and sorting

`WtStateOf` combines the agent state (`WtAgent`) and `WtInfo` into one of seven states: waiting for you → running →
ready to merge → has changes → idle → no agent → merged (lower values need you more).

- `SortWts` sorts by it (the main worktree first), and `WtTop` is the level shown on the collapsed group header.
- "Merged" means 0 ahead and at least 1 behind (the branch is already in the mainline); a worktree whose folder is gone
  (prunable) also counts. A freshly created worktree that is 0 ahead and 0 behind doesn't count as merged.

#### Expanding, collapsing and "Needs you"

- The group is collapsed by default: whether rows are listed (`Repo.WtShown`) depends only on the user opening it
  (`WtOpen`, saved in the session; `GoToWorktree` opens it too). Even with worktrees waiting for you / running / ready to merge, it doesn't auto-expand.
- Whether state is computed and folders are watched is separate: `Repo.WtTracked` = `WtOpen` or `wtAuto` (some worktree
  is waiting for you / running / ready to merge, checked every frame by `SyncWtAuto`), so even when collapsed, the
  "Needs you" bar and the group header's symbol stay current.
- Only one worktree is expanded at a time (`App.WtExp`, not saved): `git.BranchFiles` first finds the merge-base and then
  lists the files from merge-base → branch (i.e. `main...branch`); the rows are `RowWtSummary` / `RowWtFile` / `RowWtTerm`.
- "Merge into main" goes through `App.Merge` after an `Ask`; the reason it's grayed out is `MergeWorktreeWhy`.
- The "Needs you" bar is `NeedsYou` (waiting-for-you first, then ready-to-merge; ready-to-merge is only recognized for
  groups whose `WtInfo` has been computed); previous / next are `StepNeedsYou` → `GoToWorktree`.

#### Operations

- Clicking a worktree shows its branch's history in the right panel (`viewWorktree`); "Open in Workspace" adds it as a repository (`OpenWorktree`).
- New / Lock / Unlock / Remove / Prune are all a dialog + `RunOp`. Prune first runs `git.PruneList` (`prune --dry-run -v`,
  reading what would be removed from stderr), and only after confirmation runs `worktree prune -v`.

#### Watching

While a repository's worktree block is tracked (`WtTracked`), each worktree in it gets its own watcher (`syncWtWatch` in
`app/wtwatch.go`, `watch.StartTree`, at most `WtWatchMax` 16 per repository; prunable, bare and already-opened-as-a-repository
worktrees aren't watched); a change refreshes only that row.

### Submodules

`git.ReadSubmodules` reads `.gitmodules` (`git config -f .gitmodules`) and `submodule status --cached`, at most 200.
porcelain v2's `S<c><m><u>` is recorded in `Change.Sub`.

- A submodule is a single row in the file tree (it doesn't expand); its Diff in the Git panel is a note (`subDiffNote`:
  which commit the pointer moved from and to, and whether there are changes inside).
- Discarding a submodule's changes is `submodule update` (checking out the commit recorded by the parent repository), and
  only happens when the pointer moved (`Sub` contains `C`); otherwise it just shows a toast: changes inside must be discarded in the submodule itself.
- The context menu can initialize, update, open in a new window, and add to the workspace (`OpenSubmodule`: initializes
  it first if needed; an initialized one is added as a repository).

### Interactive rebase

`app/rebase.go` and `docs.RebasePlan`. "Interactive Rebase from Here…" on a commit in the history opens a wizard tab: one
row per commit, where you choose pick / reword / squash / fixup / drop / edit, drag to reorder, and edit messages directly.

- It only opens from the full history of the current branch (`RebaseView`); the range must be linear, with no merge
  commits, and at most `RebaseLimit` 100 commits.
- When it starts, it generates the todo: commits whose message changes (reword, or squashed together) are written as
  `pick` / `fixup` plus a line `exec git commit --amend -F <message file>`, so the whole rebase never opens an editor.
- The todo is swapped in with `GIT_SEQUENCE_EDITOR=cp <file>` and run via `RunOpEnv`.
- When it stops on a conflict it hands off to the banner, and `followRebase` follows the progress.

### Clone

`app/clone.go`.

- The dialog takes the URL, the parent folder (defaulting to the last one, `St.CloneDir`) and the folder name (`CloneName`
  derives it from the URL), with an "Include submodules" checkbox (`--recurse-submodules`); if the clipboard holds
  something that looks like a git URL (`LooksGitURL`), it is prefilled.
- `git clone --progress` runs in the parent folder via `RunStream`; stderr is read in chunks as progress (`CloneProgress`),
  shown on a line at the top of the sidebar's files page.
- A clone doesn't belong to any repository's queue (the repository doesn't exist yet); it follows the window (`a.Clones`).
  Credential prompts and the timeout clock work the same as for write commands.
- URLs in the log have the password scrubbed first (`redactURL`: the username stays, the password becomes `***`).
- When the clone finishes, it is added to the list (or opened as the workspace if the workspace is empty); on cancel or
  failure, the half-cloned folder is deleted — but only if that folder didn't exist before the clone.

### Remote web pages

`git/web.go`: `ParseWeb` turns a remote URL (ssh / https / scp syntax) into a web URL, `KindOf` recognizes GitHub /
GitLab / Gitea / Bitbucket, and self-hosted hosts get their style assigned in `Settings.WebHosts`.

`Web.Commit` / `File` / `Lines` build links to a commit, a file, or some lines of a file; `App.WebOf` (`app/stash.go`)
applies the per-host style from Settings, and `Repo.WebRef` is the revision pinned when viewing working-tree files (HEAD
if it has been pushed, otherwise the remote's default branch).

### Plain folders

Folders that aren't git repositories (`Repo.Plain`) can be opened too:

- The file tree is built by walking with `files.Walk` (at most `WalkCap` 50,000 files), and watching uses `watch.StartPlain`.
- In the Git panel it is a single grayed-out section header reading "Not a Git repository" that can't be expanded; it
  doesn't appear in history. Commands that need git are disabled when they hit `git.NotRepo`.
- Every refresh checks again whether it has been `git init`ed; the watcher still watches for changes to `.git` under the
  root, so it catches up after a `git init`.

`Walk` doesn't descend into `node_modules` or dot-directories (`.venv`, `.pytest_cache`…):

- It lists them as its fourth return value (names ending in `/`, like `LsIgnored`), and `LoadFiles` passes them to
  `Tree.Done` as ignored folders: grayed out and collapsed in the tree, read one level at a time with `ReadLazy` when
  expanded, and their contents aren't in `Paths` (⌘P and full-text search don't go into them).
- This ignores the "Show Ignored Files" toggle; `files.exclude` / "Hidden files" can still hide them. `.git` is neither
  listed nor entered, and `.DS_Store` isn't listed.
- The watcher (`ignorePlain`) still doesn't look inside these folders; adding or removing such a folder under the root is
  a change to the root folder, which a full reread picks up.
