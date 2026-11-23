# Architecture

This is an overview of how Grove is structured: its layers, how they depend on each other, what each package is responsible
for, and the conventions to follow when changing code. Details of each subsystem live in topic files under
[`docs/architecture/`](architecture/); see [Subsystems](#subsystems) below. What changed in each release is recorded in
[`CHANGELOG.md`](../CHANGELOG.md) at the repository root.

## Layers

```
cmd/workbench ─► internal/ui ─► internal/app ─► logic packages internal/{git, files, docs, workspace, search, globs, fuzzy,
(three windows,     (builds      (state machine,     watch, graph, outline, history, askpass, termd,
 single instance,    widgets,     command registry,  tasks, snippets, render, fonts, highlight}
 logging, subcommand draws,       UI interaction     (build no widgets; docs, render, etc. use mygo types)
 entry points,       handles      state)
 auto-update wiring) events)
      │                  ├─► widget packages internal/{ged, gmd} (do their own layout, drawing and input; gmd uses css)
      │                  └─► types and pure functions of the logic packages (no IO), internal/{mdprep, appicon}
      └─► internal/{workspace, render, fonts, askpass, termd, version, crash, appicon}, mygo's ui (wiring)
                                internal/i18n (string catalog, used by every layer)
```

Dependencies point left to right, with no cycles (Go wouldn't allow them anyway).

### `ui` and `app`

- `ui` reads `app`'s state and calls its methods and commands; `app` does not import `ui`.
- `ui` also imports `docs`, `git`, `workspace`, `files`, `search`, `outline`, `graph`, `render`, `snippets`, `highlight`,
  `history`, `askpass`, `fonts`, `version`, `appicon` and `mdprep` directly, but only for types, constants and pure functions
  (`git.Change`, the allowed values of `workspace.Settings`, `files.JoinPath`, the prompt kinds in `askpass`, ...). Reading and
  writing files and running git always go through `app` (there is no `os.*` / `exec` in `ui`).
- The boundary between them is not "logic vs. UI" but "who builds widgets":
  - `app` builds no widgets and draws nothing. Apart from `BindShortcuts` registering shortcuts on the `ui.Context` passed to
    it, it never touches `wui.Context`.
  - For the sake of headless testing, UI interaction state machines live in `app`:
    - the custom-drawn context menus (`popmenu.go`: items, position, hit testing, submenu timing);
    - the feel of sidebar scroll-wheel scrolling (`sidescroll.go`);
    - the delay and warm start of file tree hover tips (`treetip.go`);
    - the drop-target geometry for dragging tabs between editor groups (`DropSide` / `DropRect` in `groups.go`);
    - the "this frame / last frame" fields on `App` that `ui` writes back every frame (`GitHover`, `TreeSpans`, `TreeDragging`,
      `MenuKeyAt`, `TermFocused`, ...).
  - That is why `app` also imports mygo's `ui` and `editor`, for types such as key codes, `ListState` and `Rect`.

### Logic packages and widget packages

- The logic packages depend on each other only a little: `search` uses `docs` / `files` / `git` / `globs`; `docs` uses `git` /
  `highlight` / `outline` / `snippets` (plus the state types of the widget packages `ged` / `gmd`); `files` uses `fuzzy` / `git`;
  `watch` uses `git`; `mdprep` uses `emoji`. None of them depend on `app` or `ui`.
- "Logic package" means it builds no widgets, not that it never touches mygo:
  - `docs.Doc` holds the mygo editor's `editor.State` / `editor.Buffer` (in the cases where mygo's editor is still used) and the
    Markdown preview options;
  - `highlight` copies mygo's language definitions;
  - `render` uses an invisible `mygo.WebView`;
  - `watch` wraps `mygo.WatchFiles`;
  - `termd` uses mygo's terminal plugin;
  - `fonts` registers fonts;
  - `outline` uses mygo's Markdown parser.
- The widget packages `ged` (editor and Diff) and `gmd` (Markdown preview) use only mygo's public text layout, drawing and input
  APIs and do their own layout, drawing and input handling. They know nothing of Grove's theme and import neither `app` nor `ui`.
  Styles are passed in by `ui`; `app` / `docs` only hold their state. `gmd` uses `css`, and `css` depends on no other package.

### `cmd/workbench`

Besides `app` / `ui`, it uses directly:

- `askpass` and `termd`: the same executable is also the entry point for these two subcommands;
- `render`: `render.Register` in `main.go` registers the WebView backend;
- `workspace`: callbacks from the settings window, materials;
- `fonts`, `crash` (logging), `version`, `appicon` (the icon in the update window);
- mygo's `ui` (`wui.View` mounts the UI of the three windows) and the updater plugin `plugins/updater` (see
  [auto-update in app.md](architecture/app.md#auto-update)).

### Threads and state

- `app.App` holds all the state of one window. The UI reads that state every frame and changes it only through commands
  (`a.Exec(id)`) or `App` methods.
- Background goroutines never touch `App`; they hand results to `a.Update(func())`, which applies them on the main thread.
  Grab `up := a.Update` before starting: switching workspaces (`switchTo`) rewrites all of `*a`, so on the way back you must also
  check that the original repository and document still exist (compare repository pointers; for documents compare `Doc.Gen`).
- To give the keyboard to a particular widget, `app` sets a one-shot `FocusXxx` request field; the UI function that draws that
  widget sees it, calls `Focus(c)` and clears it.

For details (which fields `switchTo` keeps, the full list of `FocusXxx` fields, the command registry, multiple windows), see
[App, commands and windows](architecture/app.md).

## Subsystems

| Document | Contents |
|---|---|
| [App, commands and windows](architecture/app.md) | `App`, `FocusXxx`, `switchTo`; the command registry and the menus, shortcuts and palette generated from it; custom shortcuts; multiple windows, startup, closing windows and quitting, the settings and About windows, versions, auto-update |
| [Workspaces, settings, sessions and the state file](architecture/state.md) | `.code-workspace` files and plain folders, `Settings`, per-workspace `Session`, the state folder, the state.json format and migrations |
| [Git](architecture/git.md) | All repositories visible at once, the Git sidebar, merged history; reads and refreshing, the write-command queue, credentials, conflicts, operations in progress, branches / tags / remotes, history, the commit graph, worktrees, submodules, interactive rebase, cloning, remote web pages, plain folders |
| [Diff and staging](architecture/staging.md) | Staging / unstaging / discarding by hunk and by line and how patches are assembled; the two code paths of the Diff tab (ged / mygo), the model and view of the ged Diff, inline editing; the editor change bar and inline cards |
| [File watching and the file tree](architecture/files.md) | One watcher per repository and its two kinds of hints; the file tree model (`git ls-files`, incremental updates, lazily read ignored files, compact folders), interaction and appearance |
| [Context menus and button menus](architecture/menus.md) | Grove's own menus: where they are used, the `app.PopMenu` state machine, events and drawing in `ui/gmenu.go`, frosted glass, the few places that still use mygo menus |
| [Editor, search and local history](architecture/editor.md) | Documents and tabs, large files, encodings, saving, formatting, outline and sticky scroll, navigation, editor groups; Grove's own editor widget `ged` (the single-pane editor and shared parts); cross-repository search and replace; local history |
| [Markdown rendering](architecture/markdown.md) | Preprocessing in `mdprep`, layout and drawing in `gmd`, cascading in `css`, emoji shortcodes, Mermaid and math (`render`), side-by-side preview and scroll sync |
| [Terminal](architecture/terminal.md) | Tabs and splits, tasks, agent status, saving scrollback, the termd background process |
| [Appearance, language and shortcuts](architecture/look.md) | Window backdrop, materials and colors, the app icon, Chinese and English, shortcut dispatch domains |

Other documents: [testing](testing.md), [releasing](release.md), [shortcut table](shortcuts.md), [terms](terms.md),
[pitfalls](mygo-pitfalls.md).

## Directories

| Directory | Responsibility |
|---|---|
| `cmd/workbench/` | Entry point. When the first argument is `askpass` / `termd` it hands off to `askpass.Main` / `termd.Main`; otherwise it parses arguments (the workspace path), moves old folders (`migrate.go`), loads fonts, wires up auto-update (`setupUpdates`), creates `app.Windows` and connects mygo's window callbacks to it (`NewWindow`, `OnClose` → `CanClose`, `OnClosed` → `Closed`, `OnFocus`, `OnBeforeQuit` → `RequestQuit`); also the single-instance lock and `deliver`, logs and crash logs, the settings and About windows, and the `WORKBENCH_*` environment variables |
| `internal/app/` | The state machine: workspaces and repositories, panel visibility, editor groups and tabs, the file tree, the Git sidebar, history, search, terminals, the git write-command queue, dialog state, the command registry and the menu bar generated from it, reading and writing state.json, the window registry, auto-update hooks (`updates.go`), and UI interaction state (custom-drawn menus, scroll wheel, drag-and-drop geometry; see above). One `App` per window |
| `internal/ui/` | Pure UI, one file per area: `view.go` (overall layout), `titlebar.go`, `sidebar.go` (file tree), `gitbar.go`, `search.go`, `outline.go`, `openeditors.go`, `tabs.go` (editor groups and tab contents), `history.go` / `graph.go` (right panel), `bottom.go` (terminal / output / problems), `statusbar.go` / `statusdoc.go` (status bar; the latter holds the items for the current tab and the language / indentation / line ending / encoding menus), `palette.go`, `dialogs.go` / `dialogframe.go`, `menus.go` (context menus), `settings.go` / `about.go` (the two small windows), `widgets.go` / `icons.go` / `fileicons.go` / `accent.go` |
| `internal/workspace/` | `.code-workspace` parsing, the state.json schema and migration of old formats, `Settings`, `Session` (per-workspace tabs, expanded folders, terminals, layout), the `Store` shared by all windows, the JSON form of settings (`settingsjson.go`), reading `files.exclude` |
| `internal/git/` | Invoking git. Reads: status (porcelain v2), both sides of a diff, log, branches / tags / remotes, blobs, ls-files, blame, worktrees, submodules. Writes: `RunRW` / `RunOp` (non-interactive environment, timeouts, process groups). Hunk- and line-level patches (`hunk.go`), turning remote URLs into web pages (`web.go`), explanations of common errors (`explain.go`), detecting commands that need a terminal (`interactive.go`) |
| `internal/askpass/` | Both ends of credential prompts: `Main` / `Run` handle the `Grove askpass <prompt>` child process (connect to the socket, send one line of JSON, wait for the answer); `Listen` / `Server` / `Close` handle the app side; `CleanStale` cleans up folders left over from last time; `Classify` recognizes username, password and passphrase prompts |
| `internal/files/` | The file tree model: built from `git ls-files`, or with `Walk` for plain folders; expand / collapse, filtering, change markers, lazily read ignored files, symlinks, submodule rows, compact folders, incremental `Diff` / `Apply`, empty folders |
| `internal/docs/` | Open documents: the various `Kind`s, reading and writing, detecting external changes, encodings, large files, historical versions, both sides of a Diff, parsing conflict blocks, cleanup on save, line diffs (`LineEdits`), interactive rebase plans, cursor info |
| `internal/ged/` | Grove's own editor widget: the Diff (side by side, unified, inline editing) and the single-pane editor for file tabs, both used by default (`WORKBENCH_GED=off` / `WORKBENCH_GED_EDITOR=off` fall back to mygo's); uses only mygo's public text layout, drawing and input APIs and knows nothing of Grove's theme or `internal/ui`. See [editor.md](architecture/editor.md) |
| `internal/highlight/` | Background highlighting for large files: `For(name)` copies mygo's language definition and replaces only the `Highlighter`; one goroutine tokenizes from the start and records the state at the start of every line, which the main thread uses to handle visible lines |
| `internal/search/` | The computation behind cross-repository full-text search and replace: candidates, workers, literal / whole word / regex, globs, encodings, the regexp2 fallback in `advanced.go`, `replace.go` |
| `internal/globs/` | Path globs (`**` crosses folders, `*` and `?` don't, `{a,b}`, case-insensitive): `Set.Match` for search include / exclude, `Set.MatchRoot` for `files.exclude`, and a `Matcher` with a folder cache |
| `internal/fuzzy/` | Fuzzy matching for Quick Open and the command palette: matches the file name first and includes folders only when that fails; consecutive, word-start and full-file-name matches score higher, and match positions are returned to the UI for highlighting |
| `internal/watch/` | File watching: one `Watcher` per repository wrapping the HintsOnly mode of `mygo.WatchFiles`, reporting two kinds of hints, "the working tree changed" and "git changed" |
| `internal/graph/` | The lane algorithm for the commit graph; pure algorithm, touches neither git nor the UI |
| `internal/outline/` | Outline: parses Markdown headings and Go (`go/parser`); other languages are recognized by per-line regexes or brace nesting; computes the pinned lines for sticky scroll (`sticky.go`) |
| `internal/history/` | Local history storage: one folder per file, containing `index.json` and content files; when snapshots are taken and how restoring works lives in `app/localhist.go` |
| `internal/termd/` | The terminal background process: frame protocol (`proto.go`), `Server`, sessions (pty + shell + headless terminal), the app-side `client.go`, a home-grown pty (`pty_darwin.go`), `Spawn` |
| `internal/tasks/` | Reads task definitions at repository roots: package.json scripts, Makefile / justfile targets, common commands for Cargo.toml / go.mod; only reads files |
| `internal/snippets/` | Code snippets: the built-in ones (embedded) plus `snippets/*.json` in the state folder; `Expand` expands a VS Code snippet body into text and tab stops, `Session` handles jumping between tab stops with Tab / ⇧Tab |
| `internal/mdprep/` | Cleanup before Markdown goes to the preview: front matter; HTML containers that span blocks (`<details>`, `<div align>`, ...) become fenced code blocks with the info string `grove-region N`, with their content recorded in `Regions`; emoji shortcodes become characters (`emoji`). Only the opening and closing lines are replaced, so line numbers don't change |
| `internal/emoji/` | GitHub's emoji shortcode table (`gemoji.txt`, from github/gemoji): turns `:tada:` and `:+1:` into emoji characters, but not inside code blocks or inline code |
| `internal/gmd/` | Grove's own Markdown preview (parsing uses mygo's parser): block and inline layout, selection and copy, code blocks, tables, images; styled HTML blocks are drawn as grid / flex / boxes from the results of `internal/css` (`htmlrender.go`); `WORKBENCH_GMD=off` falls back to mygo's |
| `internal/css/` | CSS: parsing colors, lengths, grid tracks, declarations and style sheets; selector matching, specificity-based cascading, inheritance, `var()`, the UA style sheet; callers wrap elements as `css.Element`, and it never touches the UI |
| `internal/render/` | Rendering Mermaid diagrams and math in the Markdown preview: `Renderer` (cache, queue, one goroutine) and `Backend`; `web.go` uses an invisible `mygo.WebView` as the backend |
| `internal/fonts/` | Choosing the monospace font for the editor / terminal and registering bundled fonts; `app` also uses it when rendering Markdown, so it doesn't live in `ui` |
| `internal/i18n/` | String catalog: `T` / `Tf` / `Tn` / `List`, `zh-CN.json` (collected from code by a test), `en.json`; `Set` / `Lang` / `Locale` |
| `internal/version/` | Version numbers: `App` (compiled in via ldflags by `mygo build`; `dev` under `go run` and in tests), `Mygo()`, `Go()`, `Platform()`, `String()` |
| `internal/crash/` | The log folder: `workbench.log` (rotated at 1 MB) and crash logs |
| `internal/appicon/` | Draws the app icon with a signed distance field (a steel-blue G-shaped git branch on a light rounded square); `-update` generates `assets/icon.png`, and the About and update windows use the same drawing code |
| `internal/testutil/` | Test helpers: temporary repositories (`TempRepo` / `NewRepo` / `ThreeRepos` / `SubmoduleRepo`), the background callback queue `Queue`, waiting by pumping frames, `Budget` / `Race`, `WaitTermReady` |
| `tools/measure/` | Scripts for measuring memory and startup time, not shipped in the app: `measure.sh`, `report.py`, `longrun.sh`, `seed/`; on the Go side there is only `TestMeasureScripts` |
| `.github/workflows/` | `ci.yml`: vet, test and build the .app on pushes to main and on PRs (`-race` runs on manual trigger or weekly); `release.yml`: on a `v*` tag, build, sign and upload the update packages to a draft Release (see [release.md](release.md)); `claude.yml`: runs the Claude Agent SDK when `@claude` is mentioned in an issue / PR comment (`.github/claude/`) |

## Development conventions

New code must follow these conventions:

- The UI only reads state; state changes go through commands (`a.Exec(id)`) or `App` methods. Background goroutines return to
  the main thread only through `a.Update`; grab `up := a.Update` before starting (`switchTo` rewrites `*a`).
- Register new actions in `app/commands.go` first; menus, shortcuts, the palette, tooltips and the shortcut docs all come from
  there. After changing keys, run `TestShortcutDoc -update`.
- Git read commands run with `GIT_OPTIONAL_LOCKS=0 LC_ALL=C`; write commands are non-interactive, have timeouts and go through the
  per-repository queue (`RunOp`).
- Write to disk or change a repository only on an explicit user action: refreshing, watching, opening and switching never write.
  File writes go only through `WriteDoc` → `diskWrite`.
- Dangerous operations (discarding changes, force pushing, hard resetting, deleting branches, and so on) are confirmed via
  `AskDanger` with a red button. Overwriting the user's configuration (restoring default settings, importing settings, resetting
  all shortcuts) or unsaved changes (restoring from local history), and anything that can't be undone, count too. The full list is
  in `app/danger_test.go`. Confirmations that act on a whole repository (discard all, force push, reset, delete branch, drop
  stash, ...) wrap their title in `a.inRepo(r, …)`, which prefixes the repository name when the workspace has more than one
  repository (`TestConfirmInRepo` counts these in the source).
- Git commands in the menu bar and command palette act on the current repository: the menu bar has no other context, so the
  current repository follows the repository last clicked (`a.UseRepo`), and the confirmation names it (previous item). Buttons and
  context menus in the Git sidebar, file tree and history act on the repository of that row and switch the current repository to it.
- The main thread never runs git or walks folders. Synchronous disk reads are limited to three cases, each reading a single file
  with a size cap:
  - hover (`app/hover.go`; files that aren't open must be within `hoverMaxFile`, 1 MB, because the editor needs the result
    immediately);
  - "Mark as Resolved" on a conflict row (counts the conflict markers on disk);
  - replace and replace preview (capped at the search file-size limit; replacing is a write the user clicked, and open files must
    have their buffers changed on the main thread).

  All other disk reads go in a background goroutine.
- Dialogs use only `dialogFrame` from `ui/dialogframe.go` and its `dlgButton` / `dlgField` / `dlgInput` and friends, so they look
  the same. State for a new dialog follows the "New Branch" pattern: an `XxxDialog` struct on `App` with an `Open` field, and an
  `a.OpenXxx(…)` that fills it in and opens it in one go (`OpenNewBranch`). The few other existing patterns (`ConfirmOpen` +
  `Confirm`, a standalone `bool`, and so on) are historical; don't add more of them.
- Switches that are on by default are stored inverted in settings (`noHoverDefinition`, `noStickyScroll`, `treeNoCompact`, ...;
  the zero value is the default, and `omitempty` keeps it out of the file). These are the key names users see in the settings
  document; flipping them to positive would break users' settings files and exported JSON, so they stay. Store new default-on
  switches the same way.
- Every clickable element's label (Label) must be unique within its window, for tests and accessibility: add the repository or
  position to duplicate names, following the existing patterns (`name (repo)`, `Tab x`, `Edit x`, `Branch x`, `Location repo/path`,
  `Open …`, `Command …`; the Chinese UI uses `名（仓库）`, `标签 x`, and so on).
- All UI strings go through `i18n.T` / `Tf` / `Tn` as literals; run `TestCatalog -update` to collect them into `zh-CN.json`, then
  fill in `en.json`.
- All repositories are visible at once: don't add a "current repository + switcher" style list.
- Keep memory in check: compute large lists on demand and read them in the background; call `debug.FreeOSMemory()` after bulk
  operations; shut down the render backend, termd and similar components when idle.
- When behavior changes, update the docs: features and shortcuts in the README, the unreleased section of `CHANGELOG.md`, and this
  file or the matching one under `docs/architecture/`.

For the full development conventions see [`AGENTS.md`](../AGENTS.md); for notes on using mygo see
[mygo-pitfalls.md](mygo-pitfalls.md); for how to test see [testing.md](testing.md).
