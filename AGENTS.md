# Contributing guide (for humans and coding agents)

Grove is a native macOS multi-repo workbench written in pure Go on [mygo](https://github.com/egoist/mygo): file tree +
editor + per-repo Git review + terminals. A workspace can be a `.code-workspace` file or a plain directory (the
directories in it don't have to be Git repos); the first launch starts with an empty workspace.

Language conventions:

- Docs are written in English. [README.md](README.md) is English and [README.zh-CN.md](README.zh-CN.md) is the Chinese
  copy: whenever README.md changes, update README.zh-CN.md to match.
- Code comments stay in Chinese (the existing convention).
- UI strings in code are written as Chinese source strings via `i18n.T("中文")` / `i18n.Tf(…)`; the English text lives in
  `internal/i18n/en.json` (keys are the Chinese source strings). The UI defaults to English: an unset `lang` setting resolves
  through `i18n.Default`, which `cmd/workbench` sets to `"en"`. Tests never call it, so inside tests `i18n.Set("")` is still
  Chinese and assertions stay in Chinese.

Docs:

- [README.md](README.md) / [README.zh-CN.md](README.zh-CN.md): user-facing overview (English / Chinese)
- [docs/architecture.md](docs/architecture.md): architecture overview (layers, packages, conventions); per-subsystem
  details are in [docs/architecture/](docs/architecture/)
- [docs/testing.md](docs/testing.md): how testing is split up and how to measure
- [docs/release.md](docs/release.md): versioning, builds, DMG, signing and notarization, CI
- [docs/shortcuts.md](docs/shortcuts.md): shortcut table (generated from the command registry; don't edit by hand)
- [docs/mygo-pitfalls.md](docs/mygo-pitfalls.md): pitfalls we've hit (mygo, git, macOS); read the relevant section
  before touching UI, lists, keyboard handling or terminals
- [docs/terms.md](docs/terms.md): glossary of UI terms (English ↔ Chinese)
- [CHANGELOG.md](CHANGELOG.md): what changed in each version

## Must follow

0. **Every repo visible at once** (the first principle): the file tree and the Git panel always list every repo in the
   workspace, one section / one root node per repo; no "current repo + switcher" views. The "current repo" only
   decides the terminal, the right history panel (its "current repo" page) and the branch pill in the title bar;
   whichever repo you click something in, or jump to with ⌘1–9, becomes the current repo. Git commands in the menu bar
   / Command Palette act on the current repo, and confirmation titles for actions on a whole repo name it via
   `a.inRepo`. A new feature tested on only one repo doesn't count (use `testutil.ThreeRepos`).
1. **The app writes only on explicit user action**: write operations such as commit, stage, discard, save, push and
   adding / removing worktrees run only after the user clicks a button / menu item or presses a shortcut. Anything that
   loses data, changes a remote or rewrites history asks for confirmation first (`a.AskDanger`; the list is counted
   from source by `app/danger_test.go`). Startup, refresh, opening tabs and file watching never run write commands.
2. **Tests never write to real repos**:
   - Tests of write operations always run on temporary repos created with `git init` in `t.TempDir()`:
     `TempRepo` / `NewRepo` / `ThreeRepos` / `SubmoduleRepo` in `internal/testutil`.
   - Tests that read a real workspace (`TestStatusMatchesPorcelain`, `TestWorkbenchView`, `TestScreenshot`,
     `BenchmarkWorkspace`, etc.) are read-only (status / diff / log / screenshots) and only run when
     `WORKBENCH_TEST_WS=<.code-workspace or directory>` is set; otherwise they skip.
   - During development, only run read commands against your own real repos too. For worktrees, only run
     `worktree list` / `status` / `rev-list` / `log`: never create, remove, lock or `prune`. To measure a repo with many
     worktrees, `git clone file://…` it into a temporary directory and run `worktree add` on the copy.
   - Tests don't touch anything else on the dev machine either: the clipboard (`app.CopyText` is replaced with a stub),
     the browser and Finder (`a.OpenURL` / `a.ShowPath` are replaced with versions that record instead of opening),
     `~/Library/Application Support/Grove/`, `~/Library/Logs/Grove/`, and a running Grove instance and its termd (see
     "Single instance" below).
3. **mygo is pinned to the version in go.mod**. Before upgrading, run the full test suite (including `-race`).
   `TestAccessLabels` uses reflect to read unexported Tester fields; if those fields change, `TestAccessProbe` fails
   first. `TestVersions` checks that the compiled-in mygo version matches go.mod. Never run `go get -u` in CI.
4. **Look at the UI through screenshots**: coding agents usually lack Screen Recording / Accessibility permissions, so
   they inspect the UI through headless Tester screenshots (`WORKBENCH_SHOT`) and `WORKBENCH_CAPTURE`. Things that can't
   be captured (system menus, native dialogs, vibrancy, input methods, native drag and drop) must be called out in the
   PR as needing manual testing. Real windows steal focus and the keyboard: on a machine someone else is using, prefer
   headless tests, and if you really need a window, launch it in the background with `open -g -n --env …`.
5. **Quality bar**:
   - `gofmt` clean; `go vet` clean; `go test ./...` and `go test -race ./...` all pass (UI tests also run under
     `-race`).
   - Every icon button has a `Label` / `Tooltip`. `TestAccessLabels` in `internal/ui` checks this automatically; new
     regions, overlays and windows need a step in it that draws them, or they go unchecked.
   - No shortcut conflicts (`TestShortcutConflicts`, computed per dispatch context).
   - Idle memory ≤ 100 MB, hard cap 300 MB.
   - All UI text goes through `i18n.T("中文")` / `i18n.Tf(…)`; after changing UI text, run `TestCatalog -update` and
     then fill in `en.json`.
   - Terminology follows the UI: the terms are the English UI strings in `internal/i18n/en.json`, and README, CHANGELOG
     and docs use them. [docs/terms.md](docs/terms.md) is the glossary (English ↔ Chinese): Staged / Unstaged /
     Untracked (the last two share the "Changes" group), Show only the lines around changes / Full-File Diff, Side by
     side / Unified, Fetch / Pull / Push, Stash, Discard Changes, cherry-pick, preview tab / pinned tab, Working tree
     clean…
6. **state.json stays backward compatible**: `~/Library/Application Support/Grove/state.json` carries a format version
   (`workspace.Version`); files in older formats are read as usual and migrated step by step to the current format (see
   [docs/architecture/state.md](docs/architecture/state.md)). When adding a field, old files must still load (missing
   fields take defaults); extend `TestStateMigration` / `TestStateSessions` accordingly, and `TestStateDocKeys` requires
   the example in state.md to show every new key.
7. **Behavior changes update the docs**: README.md and README.zh-CN.md (features, shortcuts), the "## Unreleased"
   section of `CHANGELOG.md`, `docs/architecture.md` or the matching page under `docs/architecture/`, and
   `docs/shortcuts.md` (generated, see the commands below); new pitfalls go into `docs/mygo-pitfalls.md`.

## Commands

Run from the repo root. go.mod may require a newer Go than the one installed locally, so every command carries
`GOTOOLCHAIN=auto`.

```sh
GOTOOLCHAIN=auto go vet ./...
GOTOOLCHAIN=auto go test ./...
GOTOOLCHAIN=auto go test -race ./...                                   # the full suite takes a minute or two, mostly internal/ui
WORKBENCH_TEST_WS=<.code-workspace or directory> GOTOOLCHAIN=auto go test ./...   # also runs the read-only tests against a real workspace
mkdir -p <dir> && WORKBENCH_SHOT=<dir> GOTOOLCHAIN=auto go test -run 'TestScreenshot|TestAboutView' -v ./internal/ui/
GOTOOLCHAIN=auto go test -run '^$' -bench . -benchtime 20x ./internal/search/ ./internal/fuzzy/   # search / fuzzy-match speed
GOTOOLCHAIN=auto go test -run TestShortcutDoc ./internal/app/ -update   # regenerate docs/shortcuts.md after changing commands / shortcuts
GOTOOLCHAIN=auto go test -run TestCatalog ./internal/i18n/ -update      # re-collect zh-CN.json after changing UI text;
                                                                   # then run it again without -update: it lists what en.json is missing / has extra; add translations by hand
GOTOOLCHAIN=auto go test ./internal/appicon -update                     # regenerate assets/icon.png after changing how the icon is drawn
GOTOOLCHAIN=auto go tool mygo build -skip-dmg -skip-notarize -sign -    # produces build/darwin-arm64/Grove.app (ad hoc signed)
open -g -n build/darwin-arm64/Grove.app --env WORKBENCH_STATE=<temp dir> --env WORKBENCH_LOGS=<temp dir>
WORKBENCH_STATE=<temp dir> WORKBENCH_LOGS=<temp dir> GOTOOLCHAIN=auto go tool mygo dev -background -sign -   # live reload: rebuilds and relaunches on Go changes;
                                                                   # "Grove Dev" (in .mygo/dev) starts in the background without coming to the front; Ctrl+C stops it
footprint -p <pid> --noCategories -f bytes                         # measure after 30 seconds; afterwards kill only the pid you started
```

### Environment variables

All of these are for development, testing and memory measurement; users don't need them.

```sh
WORKBENCH_TEST_WS=<.code-workspace or directory>   # real workspace for read-only tests and screenshots; those tests skip when unset
WORKBENCH_SHOT=<dir>                         # screenshot tests write PNGs here (create the directory first)
WORKBENCH_SHOT_WS=<workspace>                # take screenshots of a different workspace (when your real workspaces are all clean, use a clone with changes)
WORKBENCH_SHOT_RENDER=<dir>                  # replay Mermaid / math in screenshots from real WebKit results recorded in this directory
WORKBENCH_MEASURE=1                          # run the large-file timing tests (TestMeasureBigFile, etc.), skipped normally
WORKBENCH_WORKSPACE=<path>                   # open this workspace at launch
WORKBENCH_STATE=<dir>                        # use state.json / window-state.json / termd from that directory instead of the default; also skips the single-instance lock and background update checks
WORKBENCH_LOGS=<dir>                         # write logs and crash logs here instead of ~/Library/Logs/Grove
WORKBENCH_SINGLE=1                           # take the single-instance lock even with WORKBENCH_STATE set (only for testing single instance itself)
WORKBENCH_PANIC=1                            # panic in a background goroutine 3 seconds after launch (tests crash logs)
WORKBENCH_CAPTURE=<file.png>                 # save the window contents as a PNG 5 seconds after launch (don't use while measuring memory)
WORKBENCH_CAPTURE_AFTER=2s                   # change how long after launch to capture
WORKBENCH_FOCUS=editor|terminal              # give keyboard focus to the editor / terminal at launch (to measure memory with focus)
WORKBENCH_CARET_REST=off                     # editor caret blinks forever (never rests); only for comparing memory
WORKBENCH_TERMS=all                          # start every terminal at launch (to measure n repos × m terminals)
WORKBENCH_TERM_CMD='echo MARK-$$'            # type this line once the first terminal of the first repo shows a prompt (termd smoke test)
WORKBENCH_ALLOPEN='async def'                # "everything open": once repos are loaded, open two diffs, search the sidebar for this text, start the current repo's terminals
WORKBENCH_MATERIAL=hud                       # material to use when the window background is translucent (a mygo.Vibrancy value); overrides the Material setting
WORKBENCH_PLAIN=1                            # plain title bar, no vibrancy; for comparing memory
WORKBENCH_GED=off                            # diffs fall back to mygo's (the default is Grove's own internal/ged)
WORKBENCH_GED_EDITOR=off                     # the editor in file tabs falls back to mygo's (also internal/ged by default)
WORKBENCH_GMD=off                            # Markdown preview falls back to mygo's (the default is Grove's own internal/gmd)
WORKBENCH_FONT=<font family>                 # monospace font for terminals / editors ("monospace" is the system default)
WORKBENCH_SECOND=<workspace>  WORKBENCH_SECOND_CLOSE=30s   # open a second window at launch; close it after this long (to check memory is returned)
WORKBENCH_RESTORE_GAP=600ms                  # when restoring several windows, wait this long after one draws its first frame before opening the next (default 2.5s)
MYGO_FRAME_STATS=all                         # write mygo frame stats to the log (count frames while measuring memory)
```

`WORKBENCH_TERMD_HELPER` and `WORKBENCH_ASKPASS_SOCK` / `WORKBENCH_ASKPASS_ID` are passed internally between processes;
don't set them by hand.

**Why "workbench" is everywhere**: the project used to be called Workbench. The rename to Grove only changed the
user-visible names (the name and bundle id in `mygo.json`, the window title, About, `TERM_PROGRAM`, temp file
prefixes). `cmd/workbench`, the `WORKBENCH_*` environment variables, the log file name `workbench.log` (read by smoke
tests and `tools/measure`) and the `"workbench"` key in exported settings JSON were deliberately left alone; don't
rename them in passing. `cmd/workbench` matters most: the directory name becomes the executable name under `go run`,
and mygo uses it as the state directory name. Renamed to `grove`, it would share
`~/Library/Application Support/Grove/` with the installed Grove on case-insensitive APFS. On first launch,
`cmd/workbench/migrate.go` renames the old Workbench state and log directories to Grove's.

## Commits

Commit messages follow [Conventional Commits 1.0](https://www.conventionalcommits.org/en/v1.0.0/), in English:

```text
type(scope): summary

Body explaining why.
```

- Types: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `build`, `ci`, `chore`, `style`, `revert`.
- Scopes name the area touched, e.g. `ui`, `editor`, `diff`, `git`, `terminal`, `markdown`, `i18n`, `release`, `deps`,
  `app`, `tree`, `search`, `settings`…
- Summary in the imperative mood ("add", not "added"), at most 72 characters; the body explains why the change was made.
- Breaking changes are marked with `!` after the type/scope (`feat(app)!: …`) or a `BREAKING CHANGE:` footer.
- Release commits are `chore(release): vX.Y.Z`.

## Conventions

**Timed acceptance tests** (watching catches up within 1 s, searching 100,000 lines < 1 s) write their budgets as
`testutil.Budget(time.Second)`: under `-race` the budget is relaxed 3× (`testutil.Race` is set by the build tags in
`race.go` / `norace.go`), otherwise it stays as written. Write new timing assertions the same way.

**Icons and icon buttons**: icon sizes only use the three constants in `widgets.go`: `iconInline` (11, small marks that
follow text), `iconRow` (13, list rows, tabs, input fields) and `iconTool` (16, icon buttons); large icons in the
middle of empty states use `iconEmpty`. `TestIconSizes` scans the source and fails if `Icon(...).FontSize` is given a
number. Icon buttons use `toolButton` / `iconButton` (`toolSize`, 28 square), `rowButton` (20) and `toggleButton`;
hand-built buttons get their look from `toolStyle`, and their hover fill is `toolFill`, not `SurfaceHover`. Button hit
areas (side length of the square) only come from the sizes in `widgets.go`: new buttons use `toolSize` (28; title bar,
card headers, sidebar headers) or `rowToolSize` (20; inside list rows). The other sizes are reserved for a few existing
spots: `headToolSize` 26 (right history panel header), `midToolSize` 24 (Pull / Push in Git section headers, the
expanded commit card, Review / Merge on worktree rows), `smallToolSize` 22 (worktree rows, filter toggles), `closeSize`
18 (the × on tabs) and `tinyToolSize` 16 (the stop button after the status bar progress); use them only to line up with
what's next to them. `TestButtonSizes` fails if `squareButton` / `iconActionSize` or the `Size` of `ButtonBase` /
`ToggleBase` is given a number. Font sizes 12 / 12.5 / 14.5 and corner radius 5 also have constants (`sideSub` /
`barFont` / `noteFont`, `bodyFont` / `sideMono`, `repoFont`, `rowRadius` / `ctlRadius`); `TestVisualConsts` fails on
literal numbers. Shadows use `shadowCard` / `shadowLow` / `shadowFloat` / `shadowDialog` … from `widgets.go`
(`el.Shadow(shadowDialog())`), and tooltips get their look from `tipStyle`.

**New actions** go into the registry in `internal/app/commands.go` first (menus, shortcuts, hover tooltips and
`docs/shortcuts.md` all come from it), then the UI calls them.

## Single instance, real windows and measuring memory

**Single instance**: a Grove launched without `WORKBENCH_STATE` takes the single-instance lock; if one is already
running (say, the one you use every day), it hands its arguments over and exits. So manual testing and memory
measurement always use `WORKBENCH_STATE=<temp dir>` (such an instance skips the lock, leaves the default state.json
alone and has its own termd), and `WORKBENCH_LOGS=<temp dir>` for logs. Add `WORKBENCH_SINGLE=1` only when testing
single instance itself, and first check with `pgrep -fl Grove` that no other instance is running. Only kill pids you
started; never kill by name.

**Measure memory** with `tools/measure/`: `measure.sh <label> <Grove.app>` builds a `WORKBENCH_STATE` from a seed
(`seed/idle.json` / `allopen.json`), copies `seed/window-state.json` (fixed 1440×880), and launches once via
`open -g -n --env …` with `WORKBENCH_LOGS` and `MYGO_FRAME_STATS=all`. It records the footprint and child shells every
few seconds, sends TERM once it has run long enough, and `report.py` prints the summary. Don't run the binary directly:
the app would come to the front and steal the keyboard. `longrun.sh` is the 8 h long run, best started yourself in a
terminal.

Rules built into the scripts; don't work around them:

- Start only once the machine's input idle time (`HIDIdleTime`) is ≥ 60 s; if not, check again every 20 s for up to
  45 min, then write `SKIPPED`.
- Once launched, send no events to the window ("everything open" is set up with `WORKBENCH_ALLOPEN`, not by
  clicking).
- Record `git status --porcelain=v2 --branch` for every repo before and after measuring; if they differ, report
  `REPO CHANGED`.
- Changes of the frontmost app go into `front.txt`; stretches where the measured Grove (by pid) was frontmost are
  listed separately in the summary and not counted. If another smoke test / measurement is running, wait for it to
  finish first (it competes for CPU, and when it quits the system hands the front to some other window).
- The workspace points at a copy on the internal disk (a freshly built ad hoc-signed app hangs reading removable
  volumes, see `docs/mygo-pitfalls.md`), and the copy includes ignored directories (whether `node_modules` is there
  changes empty directories and search counts).
- Machine load affects launch time: `meta.txt` has `uptime` from before launch and after measuring; check the load
  before comparing times.

Follow the same rules when measuring by hand:

- Don't turn on `WORKBENCH_CAPTURE` (it adds an extra window bitmap).
- The measured instance may become frontmost midway (someone clicked its window, or the frontmost app was killed and
  the system activated the next one). Once frontmost, the caret blinks, frame counts rise and memory jumps into the
  two hundreds. Check `front=` and frame counts in the log, and only use samples with `front=other` and unchanged frame
  counts.
- Measure both with and without focus: each caret blink is a frame, and mygo only releases window-sized frame memory
  after 2 seconds without a new frame, so measuring only without focus misses one or two hundred MB.
- Window position (`window-state.json`) lives in the `WORKBENCH_STATE` directory. A new directory doesn't have one, so
  the window opens at the default position; measured in the background, it may be fully covered by the frontmost app's
  window, mygo then stops drawing frames, and idle memory drops to about 63 MiB, which isn't comparable with other
  numbers. Use `tools/measure/seed/window-state.json`.

## Directory layout

Details are in [docs/architecture.md](docs/architecture.md) (overview and package table) and
[docs/architecture/](docs/architecture/).

- `cmd/workbench`: entry point, opens windows; the very first line forks off the termd background process
- `internal/app`: state machine and command registry (`commands.go`); one `App` per window, `app.Windows`
  (`windows.go`) is the registry of open windows, and the windows share one `workspace.Store` (state.json)
- `internal/ui`: pure UI. UI functions only read state and change it through `a.Exec(command id)` or `App` methods;
  background goroutines get back to the main thread only via `a.Update`
- Logic that doesn't touch the UI: `internal/git`, `files`, `docs`, `search`, `globs`, `fuzzy`, `watch`, `workspace`,
  `graph`, `outline`, `highlight`, `history` (local history), `termd` (terminal background process), `askpass`,
  `tasks`, `snippets`, `render` (Mermaid and math)
- `internal/ged`: Grove's own editor widgets (diffs: side by side, unified, edit in place; and the single-pane editor
  in file tabs, both used by default; `WORKBENCH_GED=off` / `WORKBENCH_GED_EDITOR=off` fall back to mygo's). It only
  uses mygo's public text layout, drawing and input APIs, and knows nothing of Grove's theme or `internal/ui`
- `internal/gmd`: Grove's own Markdown preview (parsing comes from mygo; layout, drawing, selection and copy are its
  own; styled HTML blocks are drawn as grid / flex / boxes using styles computed by `internal/css`);
  `WORKBENCH_GMD=off` falls back to mygo's. `internal/css`: CSS parsing and cascade (selectors, specificity,
  inheritance, `var()`), no UI
- `internal/i18n`: string tables; `internal/version`, `crash`, `appicon`: version number, logs and crash logs, the
  icon drawn in code
- `internal/testutil`: test helpers (temporary repos, waiting, timing budgets)
- `tools/measure`: memory measurement scripts
- `resources/`: fonts and third-party licenses copied verbatim into the .app; `assets/icon.png` is the app icon
