# Terminal

[← Architecture overview](../architecture.md)

Terminal tabs and splits, tasks, agent status, saving scrollback, and the termd background process.

## Terminal

### Tabs and splits

The model and operations live in `app/terminals.go`; the bottom panel UI is in `ui/bottom.go`.

- Each repository has its own row of terminal tabs (`TermTab`), and the bottom panel shows the current repository's row
  (`App.TermRepo`). In an empty workspace the terminal opens in the home folder `~` and isn't recorded in the session.
- Each tab is a single pane or several split panes (the `TermNode` split tree: a leaf is a pane, otherwise it splits into two
  halves left / right or top / bottom), at most `MaxPanes` 8 panes (session validation uses `workspace.MaxTermPanes`, also 8). Each
  pane (`TermPane`) runs one shell.
- Shells start on demand: only when first needed for display (the bottom panel is open, its repository is current, and it is the
  current tab), via `EnsurePane` called from `ui.termPane`. Switching to another repository and back keeps the same shell.
- Recorded in the session: tab names, order, splits, each pane's shell folder (as reported by OSC 7), and the terminal profile used.

The bottom panel UI (`ui/bottom.go`): `bottomBar` draws the tab strip (`termTab`, renamable, with a context menu) and the Output and
Problems pages; `termNode` draws panes and drag handles following the split tree; `termPane` draws one pane (dragging file tree rows
into it pastes their quoted absolute paths); `termFind` is the find bar; `exitedBar` is the bar shown when the shell has exited /
the connection to termd is lost; `newTermButton` is the "+", and right-clicking or ⌥-clicking it creates a terminal from a profile.

#### Process ids, foreground commands and folders

Whether a program is running in a terminal (whether to ask before closing a tab or window) is all handled in `app/termjobs.go`:

- Claiming process ids: mygo's terminal doesn't expose process ids, so `pollPids` / `claimPids` pick the app's child processes that
  have a terminal (tty) (`ps -A -o pid=,ppid=,tty=`) and assign them, in ascending pid order, to panes queued in the order they
  were started.
- Foreground commands: `ps -o pid,pgid,tpgid,ucomm,args` gets the process group, the terminal's foreground process group, the
  executable name and the command line in one go. Asking about one pane before closing it (`TermPane.Job()`) uses two `ps -p` calls;
  polling for agents uses one `ps -A` (`foregroundJobs`).
- Names are always computed by `jobName`: when the foreground process is an interpreter (node, bun, deno, python, python3, ruby), the
  script name is used (or argv[0] for processes that renamed themselves); when the path contains spaces, argv[0] gets cut off, so
  ucomm is used.
- When a folder hasn't been reported via OSC 7, `lsof` is used to look up the cwd (`procCwd`).
- Whether a session in termd is busy is checked by termd itself with `TIOCGPGRP` (some sessions aren't in any window, and the app
  doesn't know their process ids).

#### Keys and settings

- Terminal commands go through the registry: those in the Terminal context (split ⌘D / ⇧⌘D, find ⌘F, clear ⌘K, close pane ⌘W, ...)
  and the global New Terminal ⌃⇧`; all other keys go to the terminal (`Options.OnKey` → `termKey`).
- The settings include "Use ⌥ as Meta key", "Copy on select", "Cursor blink", and colors (follow appearance, light / dark, or a
  Ghostty theme name).
- Terminal profiles (`TermProfiles`, at most `workspace.MaxTermProfiles` 10) are a shell plus arguments; when the first pane uses a
  profile, the tab is named after it (`Repo.TermLabel`).

#### ⌘-click

⌘-clicking in the terminal (`OpenLink` in `app/links.go`):

- File paths (absolute, `~/`, relative to the terminal's current folder, and `file://` too) open in the editor, jumping to
  `:line[:col]` if present;
- folders inside a repository are revealed in the file tree;
- other URLs go to the system browser.

The folder reported by the shell may have symlinks resolved (`/var` → `/private/var`); when they don't match, both sides are
resolved before comparing (`linkRepo`).

#### Opening in the editor area

Terminal tabs can also be opened in the editor area (`app/termeditor.go`, ⌃⇧⌘`; the bottom panel tab's context menu can move them
there too): a `TermTab` is either in the bottom panel (`Repo.Terms`) or in the editor area (`Repo.EdTerms`, matching a `docs.Term`
tab). Moving it doesn't restart the shell, only changes where it's shown; it's drawn the same way as in the bottom panel
(`ui.termNode`).

#### Opening terminal output in a tab

The terminal has no keyboard selection, hence "Open Terminal Output in a Tab" (⇧⌘C, `app/termtext.go`): it opens a pane's screen and
scrollback as a read-only tab, where you can select, search and copy in the editor. The content is a snapshot taken when it was
opened and can be refreshed from a banner (`RefreshTermText`). The same file also has "Copy All Output".

### Tasks

`internal/tasks` recognizes tasks at repository roots; it only reads files and never runs anything:

- package.json scripts (the lock file decides which package manager to use), Makefile / justfile targets, and common commands for
  Cargo.toml / go.mod;
- at most 200 (`tasks.Max`); `Stamp` decides whether to rescan based on the names, sizes and modification times of those files.

Running happens in `app/tasks.go`:

- ⇧⌘B (`task.run`) or `!` in the palette picks one (tasks from all repositories are fuzzy-matched together); the context menu on
  package.json and similar files in the file tree can run them too.
- `RunTask` runs it in that repository: it reuses a terminal tab named after the task with no foreground command running, or else
  opens a new one named after the task; then it switches to that repository, opens the bottom panel and gives it the keyboard.
- Recently run tasks are recorded in `TaskHistory` (at most `workspace.MaxTaskHistory` 20) and listed first in the palette.

### Agent status

When an agent such as claude or codex is running in a terminal, its status is computed per pane and then aggregated per worktree
(`app/agents.go`). It's all computed locally and only looks at Grove's own terminals; it doesn't read agent logs or talk to their
APIs.

#### Three states

- Waiting for you (`AgentWaiting`): the terminal rang the bell (BEL) or requested a notification (OSC 9 / OSC 777) while the
  keyboard wasn't in that pane (`paneLooked`).
  - Recorded in `TermPane.bellAt`, and cleared when the user switches into that pane (`FocusPane`, `GoToPane`).
  - Sequences are recognized via the mygo terminal's `OnBell` / `OnNotify` (a BEL used as the terminator in OSC 0/2/7/52 doesn't
    count).
  - termd sessions hook these two callbacks on the headless terminal and send `frameBell` to the window (recorded when no window is
    attached, with one frame sent on attach); on the window side it's `Conn.OnBell`.
- Running / idle (`AgentRunning` / `AgentIdle`): the foreground command is on the agent list (`DefaultAgents`: claude, codex,
  gemini, aider, opencode, plus the "Agent commands" setting `AgentNames`); output within the last 10 seconds (`agentQuiet`) counts
  as running, otherwise idle.

#### Background polling

- Foreground commands are read by a background goroutine (`agentPoller`) every 2 seconds (`agentPoll`) with a single `ps -A` for all
  open panes (`foregroundJobs` in `termjobs.go`).
- If the foreground commands and running / idle states are the same as last round, it doesn't return to the main thread, to avoid
  an extra frame every 2 seconds while idle.
- Output time: termd panes record it on every screen frame received (`Conn.OnOutput`, atomic); local panes have no output callback,
  so when the foreground process is an agent, a fingerprint of the screen text is checked for changes.

#### Terminal → worktree

- Which worktree the pane's current folder (`paneDir`: OSC 7, or the last lsof reading if absent) falls in (`git.Under` /
  `SamePath`; a folder inside a worktree nested within another counts toward the nested one).
- `WtAgent(r, w)` takes the pane that most needs you (waiting for you > running > idle), along with when it started running /
  waiting and which terminal it is.
- `GoToPane` switches to that pane (for the bottom panel: open the bottom panel, switch repository and tab; for the editor area:
  switch to that tab).

#### UI

After the icon of a terminal tab (bottom panel and editor area), waiting for you is drawn as a 7px amber dot and running as a 13px
(`iconRow`) gray spinner (`agentMark` in `ui/agents.go`); idle and no agent draw nothing. Worktree rows in the Git sidebar use the
same symbols (`wtMark` in `ui/wtrows.go`).

### Saving scrollback

`app/termhist.go`. Without the background process, shells end when the window closes; so that the previous output can still be
seen after reopening:

- Before closing a window, quitting or switching workspaces, every started pane running in the app process writes its scrollback
  (`Terminal.History`, as VT sequences) to `<state folder>/termhist/<workspace hash>/<milliseconds>-<pane ID>.vt`, and the file name
  is saved in the session in the corresponding split's `hist` (`workspace.TermSplit.Hist`). There are no periodic writes; while a
  full-screen program is open (`History` is empty), the previously written file is kept.
- The next time the pane starts (`takeHist`), the content is first fed into the new terminal, preceded by a faint separator line;
  the file is deleted after it is read.
- Each pane keeps at most `TermHistCap` 256 KB (cut at a line start, keeping the end), and the whole folder is capped at
  `TermHistMax` 20 MB, with the oldest files deleted beyond that; files not referenced by any session are cleaned up by
  `gcTermHist`.
- Panes in termd don't write: their sessions are still around.

### termd

When "Keep terminals running after the window closes" (`TermDaemon`) is turned on in the settings, the shells of newly created
panes run in the terminal background process termd, and terminals in the window are just mirrors. The package is `internal/termd`
(`server.go`, `session.go`, `client.go`, `proto.go`, `spawn.go`, `pty_darwin.go`); the app side is in `app/termd.go`.

- Process: it's Grove itself with the `termd` argument (`Grove termd <sock> <log>`), branched off on the first line of `main`,
  without starting `mygo.App`. `termd.Spawn` starts it with `Setsid` and waits until it can connect.
- Socket: `termd.sock` in the state folder, mode 0600, with no other authentication (only this user can connect). When the path is
  too long (macOS's `sun_path` is only 104 bytes), `$TMPDIR/grove-termd-<first 6 bytes of sha1>.sock` is used instead (`SockPath`).
  If two are started at once, a flock on `<sock>.lock` keeps only one. There is one termd per state folder, so `WORKBENCH_STATE`
  instances have their own.
- Session: one pty (the home-grown `pty_darwin.go`, using `/dev/ptmx` + `ioctl`), one shell and one headless `terminal.Terminal`.
  Shell output feeds the headless terminal, and the terminal in the window receives mirror frames through `terminal.Options.Conn`
  (`ResizeByConn`: the screen size follows the termd side), keeping screen and scrollback in sync.
- A session allows only one connection at a time: a new connection replaces the old one, which receives `frameGone "kicked"`, and
  that window's pane shows "This terminal was attached in another window".
- Detaching and closing: closing a window, ⌘Q and switching workspaces detach; the session stays in termd and its sid is recorded
  in the Grove session, so reopening the same workspace reattaches (if the session is gone, a new one is opened with a notice).
  Closing a pane or tab, removing a repository and restarting close the session (the shell receives SIGHUP).
- Bell: when a session's headless terminal rings the bell or requests a notification, it sends `frameBell` (mirror frames don't
  carry it); see [Agent status](#agent-status) above.
- Exiting: sessions whose shell has exited and that have no attached window are removed after `Linger` (`DefaultLinger`, 10
  seconds); with no sessions and no connections for 60 seconds (`DefaultIdle`), termd exits on its own and deletes the socket.
  SIGTERM / SIGINT hang up all shells and then exit.
- The terminal page of the settings window shows whether termd is running and how many sessions it has (`TermdStatus`, rechecked in
  the background if not checked for 2 seconds), and can stop it (`StopTermDaemon`; in the menu it's "Stop Terminal Background
  Process…"): it asks first, then hangs up every shell in termd; open panes show as exited, and termd exits on its own once it has
  no sessions and no connections.
- Turning the setting off affects only newly opened panes.
