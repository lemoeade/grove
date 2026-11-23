# Testing and acceptance

## Before committing

Run from the repo root:

```sh
GOTOOLCHAIN=auto go vet ./...
GOTOOLCHAIN=auto go test -count=1 ./...   # the full suite takes tens of seconds, mostly internal/ui
GOTOOLCHAIN=auto go test -race ./...      # UI tests also run under -race; a minute or two
WORKBENCH_TEST_WS=<.code-workspace or directory> GOTOOLCHAIN=auto go test ./...   # adds the read-only tests against a real workspace (they skip when unset)
mkdir -p <dir> && WORKBENCH_SHOT=<dir> GOTOOLCHAIN=auto go test -run 'TestScreenshot|TestAboutView' -v ./internal/ui/   # screenshots; copy the ones to update into docs/screenshots/
GOTOOLCHAIN=auto go test -run TestCatalog ./internal/i18n/   # after changing UI text, first collect zh-CN.json with -update, then add translations to en.json
GOTOOLCHAIN=auto go test -run '^$' -bench . -benchtime 20x ./internal/search/ ./internal/fuzzy/   # search / fuzzy-match speed
GOTOOLCHAIN=auto go tool mygo build -skip-dmg -skip-notarize -sign -
WORKBENCH_STATE=<dir> WORKBENCH_LOGS=<dir> GOTOOLCHAIN=auto go tool mygo dev -background -sign -   # live reload: saving Go code rebuilds and relaunches "Grove Dev" (in .mygo/dev); -background makes it start in the background each time instead of coming to the front; Ctrl+C stops it. Always set WORKBENCH_STATE so your everyday state is left alone
open -n -g -a build/darwin-arm64/Grove.app --env WORKBENCH_STATE=<dir> --env WORKBENCH_LOGS=<dir>   # after 30 seconds, footprint -p <pid> --noCategories -f bytes, then kill only that pid
open -n -g -a build/darwin-arm64/Grove.app --env WORKBENCH_STATE=<dir> --env "WORKBENCH_ALLOPEN=async def"   # "everything open" memory (two diffs, search, the current repo's terminals all started)
open -n -a build/darwin-arm64/Grove.app --env WORKBENCH_STATE=<dir> --env WORKBENCH_FOCUS=editor   # memory with focus (editor or terminal; check with lsappinfo front that it is frontmost); add -g for without focus; --env WORKBENCH_CARET_REST=off to compare with a caret that never rests
```

For routine memory measurement use `tools/measure/measure.sh`; the method and rules are in the
[Single instance, real windows and measuring memory](../AGENTS.md#single-instance-real-windows-and-measuring-memory)
section of AGENTS.md.

Anything screenshots can't verify (system menus, native dialogs, vibrancy, input methods, native drag and drop,
shortcuts in a real window) needs manual testing; say in the PR what you tested and how.

After a behavior change, update README.md and README.zh-CN.md (features, shortcuts), the "## Unreleased" section of
`CHANGELOG.md`, `docs/architecture.md` or the matching page under `docs/architecture/`, and `docs/shortcuts.md`
(`go test -run TestShortcutDoc ./internal/app/ -update`). Record newly hit problems in
[mygo-pitfalls.md](mygo-pitfalls.md).

When the full suite runs in parallel on a heavily loaded machine, some timing tests occasionally time out (the watch
tests in `internal/watch`). Rerun them on their own first to see whether load is the cause; only dig further if they
still fail when run alone.

## How testing is split up

### Logic packages

`internal/app`, `git`, `files`, `docs`, `workspace`, `search`, `fuzzy`, `watch`, `i18n`, `testutil` and the other logic
packages are unit tested with temporary directories and temporary repos, and run under `-race`.

- The search acceptance criterion (100,000 lines < 1 s) is checked by `TestHundredThousandLines` in `internal/search`.
  Performance tests are `BenchmarkHundredThousandLines`, `BenchmarkWorkspace` (on the real workspace
  `WORKBENCH_TEST_WS` points to, read-only, skipped when unset) and `BenchmarkMatch` in `internal/fuzzy`.
- The watch acceptance criterion (updates within 1 s of an external file change or commit) is checked by
  `TestWatchRepo` in `internal/watch` and `TestWatchFollowsExternalChanges` in `internal/app`. They cover external
  edits, new files, commits, switching branches and switching back, and measure the time from the change to the UI
  state being updated. The watch tests all use temporary repos in `t.TempDir()`; point `TMPDIR` at an external volume
  to run them again on another file system.
- Budgets for timing assertions are written as `testutil.Budget(…)` and relaxed automatically under `-race`.
- Table tests for the command registry are in `internal/app`: `TestShortcutConflicts` (shortcut conflicts, computed
  per dispatch context), `TestShortcutDoc` (`docs/shortcuts.md` matches the registry; after changing shortcuts, run it
  with `-update` to regenerate) and `TestMenuStructure`.
- `TestCatalog` in `internal/i18n` checks that `zh-CN.json` contains every Chinese string in the code, that `en.json`
  is complete, and that placeholders match.

### UI

`internal/ui` uses `ui.NewTester` to drive the whole window in memory without opening a real window; the queue and
waiting helpers come from `internal/testutil`.

- UI tests also run under `-race`. For example, renaming changes document fields on the main thread, so a background
  file read has to copy those fields first; only UI tests cover races like this.
- Every entry point of a new feature (shortcut, title bar button, sidebar page, panel item, context menu, ⌘-click in
  the terminal) must actually be triggered by a Tester test.
- The Settings window uses a second Tester (`ui.SettingsView(a)`) that shares one `App` with the main window.
- `newUI` draws the editor area's start page empty (`bareStartPage = true`): the start page lists file names and words
  like "Worktrees" that would collide with those in the sidebar and right panel (the Tester finds elements by text, and
  `HasText` matches substrings). Tests that need the start page set it back to `false` after `newUI` and restore it with
  `t.Cleanup`.
- For the English UI use `i18n.Set("en")` + `Tester.SetLocale(i18n.Locale())`; `TestEnglishUI` checks that the main
  window, context menus and the Command Palette contain no Chinese.
- Things to watch out for when writing Tester tests are in the "Tests and Tester" section of
  [mygo-pitfalls.md](mygo-pitfalls.md).

### Multiple repos

Behavior that involves "every repo visible at once" is tested with `testutil.ThreeRepos`: three temporary repos in one
workspace, with interleaved commit times, two with changes and one clean. Tests check that every repo is listed at
the same time, that a write operation in the second repo affects only that repo (comparing the other repos' HEAD and
status), that ⌘1–9 jumps to and switches the current repo, and the order and pills of the merged history.
New features must pass multi-repo tests.

### Real workspaces

Write operations are tested only on temporary repos created with `testutil.TempRepo` / `testutil.NewRepo`. Tests that
read a real workspace (`TestStatusMatchesPorcelain`, `TestWorkbenchView`, `TestTerminalChineseAndVim`,
`TestRepoMenuAddRemove`, `TestScreenshot`, `BenchmarkWorkspace`) are read-only and only run when
`WORKBENCH_TEST_WS=<.code-workspace or directory>` is set; otherwise they skip. Before and after a read-only smoke test,
`git status --porcelain=v2` / `git submodule status` / `HEAD` / `git stash list` must be identical.

### Caret and memory

- Advance simulated time with `Tester.Advance(CaretRest)` to confirm that no new frames are produced once the caret
  stops blinking (`TestCaretRestEditor` / `TestCaretRestDiff`), and that typing, input methods and ↑ still work.
- Measure real-window memory the way AGENTS.md describes: with a separate `WORKBENCH_STATE`, and with local clones of
  the workspace's repos as the test workspace. To rule out the shell's influence, use `/bin/cat` as the terminal shell.
  Measure without focus / with focus for 30 s / with focus for 90 s separately, recording `footprint` + `ps -o time`
  every 5 seconds and frame counts with `MYGO_FRAME_STATS=all`.
- Before a release, measure four scenarios: idle, everything open, editor focused for 90 s, terminal focused for 90 s.
  Check the launch peak separately with categorized footprint samples every 0.3 s. Don't turn on `WORKBENCH_CAPTURE`
  while measuring.

### Git

Tests involving write operations all use temporary repos; real repos are only used for read-only tests.

- **Hunk operations**: in `internal/git`, patches from `git.HunkPatch` are actually fed to `git apply` and checked
  against the index / working tree (start and end of file, no trailing newline, CRLF, special file names, touching only
  one of two hunks, the file changing again after the diff was read); in the UI, `TestHunkOps` is the hunk part of
  `TestGitOps` (stage / unstage / discard a hunk, J / K, ⌥J / ⌥K).
- **Line staging**: `TestLinesPatch` feeds patches from `LinesPatch` to `git apply --cached` / `git apply` (paired
  lines, extra lines, only the old side selected, no trailing newline), and `TestLinesEdits` checks the pairing rules;
  the UI's `TestLineStaging` selects lines with `DiffState.LineSelection`, stages / unstages / discards once each, and
  compares against `git diff --cached`.
- **blame**: `TestBlameParse` in `internal/git` runs a real `git blame --porcelain` (`--author` / `--date` pin the
  author and time zone, a rename with `git mv`, an uncommitted change to the last line); the UI's `TestBlameGutter`
  checks that the gutter updates on scroll, clicking selects the commit in the history, unsaved changes are shown
  faded, it rereads after save / commit, and the history version tab and closing behavior.
- **Submodules**: `testutil.SubmoduleRepo(t, name)` creates a library repo (two commits) and a main repo; in the main
  repo, `vendor/lib` is initialized and `vendor/lib 2` (a path with a space) is deinitialized.
  Cloning from a local path needs `protocol.file.allow=always`, enabled only in the test environment: `SubmoduleRepo`
  uses `-c` itself; UI tests initialize through the app's `RunOp`, so the test first sets
  `t.Setenv("GIT_CONFIG_COUNT", "1")`, `GIT_CONFIG_KEY_0=protocol.file.allow` and `GIT_CONFIG_VALUE_0=always`. The app
  itself never adds this config.
  `TestReadSubmodules` / `TestStatusSubField` check status reading and the diff description; `TestSubmodules` covers
  tree rows, adding as a repo, the repo menu, the pill, discarding and initializing.
- **Read-only behavior**: startup, refresh and opening tabs never run write commands.

### Web pages and clipboard

Tests replace `App.OpenURL` with a function that only records the address without opening a browser, and use made-up
remote URLs, so nothing touches the network. Clipboard operations go through `app.CopyText`, which tests replace with
a function that only records the content, leaving the system clipboard alone.

### Terminal

- In UI tests the shell is `/bin/sh` (`setBar` writes it into state.json so the user's zsh config doesn't run); the
  initial state (bottom panel open or closed, `terms` in the session) has to be written before `newUI`.
- `readyPane` waits for the shell prompt; `typeLine` types a line and waits for output (`testutil.WaitTerm` /
  `WaitTermGone`).
- Starting on demand is checked with `App.StartedPanes()` (the number of shells started and still running); sessions
  are checked through `sessions[…].terms` after `SaveState`, calling `EvalSymlinks` before comparing directories.
- ⌘-click in the terminal uses `Tester.HoldModifiers(Cmd)` + `Move` vertically, finds link positions with
  `Terminal.HoveredLink()`, and checks opened URLs with `Tester.OpenedURLs()`.
- Measure memory the way AGENTS.md describes: with a real zsh, recording child processes' footprint separately; close
  the bottom panel of every repo, compare before and after starting on demand, and with `WORKBENCH_TERMS=all` measure
  3 × 2 without focus / with focus for 90 s.

### Terminal background process (termd)

- Tests in `internal/termd` start a `Server` inside the test process, with the socket at `SockPath(t.TempDir())`
  (when `t.TempDir()` is too long, it switches to a short path under `$TMPDIR` named after the directory), and
  `t.Cleanup` calls `Stop` and removes the socket and `.lock`.
- Only `TestSpawnDetached` actually starts a child process (the test binary re-enters `Main` with
  `WORKBENCH_TERMD_HELPER=1` and the `termd` argument), and shuts it down at the end.
- UI tests replace `app.TermdStart` with an in-process `Server`, and the `termDaemon` setting is written into
  state.json before `newUI`. `termKey` goes through `a.Update`, so after typing, use `wait(...)` until the queue has
  been processed; to type into the terminal again after ⇧⌘C, first do `a.FocusTerm = true; tt.Frame()`.
- **Smoke test** (real app and termd): use a separate `WORKBENCH_STATE` / `WORKBENCH_LOGS` so the instance has its own
  termd.
  1. Launch the first time with `WORKBENCH_TERM_CMD='echo MARK-$$'`, wait for the prompt, then end the app with TERM;
     use `pgrep -f 'Grove.app/Contents/MacOS/Grove termd'` to confirm termd is still running and the shell is still its
     child.
  2. Launch a second time without that variable and confirm the screen still shows the same `MARK-<pid>` (take a look
     with a `WORKBENCH_CAPTURE` shot), and that termd.log has only 1 session.
  3. Finally `kill -TERM` this termd (it hangs up the shell and removes the socket); only end processes you started.
- **Measuring termd memory**: record the memory of the app, termd and its child shells separately (the shells are
  children of termd, not of the app); keep the window in the background and only use samples taken with front=other:

  ```bash
  A=…/Grove.app; P=$(pgrep -nf "$A/Contents/MacOS/Grove\$"); D=$(pgrep -f "$A/Contents/MacOS/Grove termd")
  footprint -p $P --noCategories -f bytes | grep phys_footprint; footprint -p $D --noCategories -f bytes | grep phys_footprint
  for k in $(pgrep -P $D); do footprint -p $k --noCategories -f bytes | grep phys_footprint; done; ps -o pid,ppid,rss,command -p $D
  ```

### Pre-release checks

- **Accessibility labels**: `TestAccessLabels` walks through a dozen or so states of the main window, every page of
  Settings and the About window, checking that buttons have names a screen reader can use. New regions / overlays /
  windows need drawing steps added to the test before they are covered. `TestAccessProbe` checks that the fields read
  via reflect still match; when upgrading mygo, it is the first to fail.
- **Opening paths**: `TestOpenArgs` tests on temporary repos (in the workspace, outside the workspace, a plain
  directory, not found, another `.code-workspace`), and confirms after "Add Repository" that the contents of the
  `.code-workspace` are completely unchanged.
  Real single-instance and `open -a` testing needs two processes: use a separate `WORKBENCH_STATE` / `WORKBENCH_LOGS`
  plus `WORKBENCH_SINGLE=1`, check first with `pgrep -fl Grove` that no other instance is running (otherwise the
  arguments go to the running instance), and end only the processes you started, by pid.
- **Crash logs**: tests in `internal/crash` run in a temporary directory; on a real machine use `WORKBENCH_PANIC=1` +
  `WORKBENCH_LOGS=<temp dir>`, leaving `~/Library/Logs/Grove/` alone.
- **Versions**: `TestVersions` checks that CHANGELOG has a section for the version in `mygo.json` and that the mygo
  version matches the pinned one; the release steps are in [release.md](release.md).

## Acceptance criteria

The acceptance criteria are checked by the timing, memory and behavior tests above. Acceptance is complete only when
all tests pass and the necessary manual testing is done. Memory requirements: idle ≤ 100 MB, hard cap 300 MB.
