# Editor, search and local history

[← Architecture overview](../architecture.md)

Open documents and tabs, editor groups, formatting, navigation, Grove's own editor component (`internal/ged`), cross-repository search and replace, and local history.

## Editor

Documents, tabs, editor groups and the features around the editor (formatting, linting, outline, navigation, completion, hover…) are Grove's own (`internal/docs`, `internal/app`, `internal/ui`).

There are two editor components that do the drawing:

- File tabs default to Grove's own single-pane editor (`ged.NewEditor` from `internal/ged`, `ui/gedfile.go`), and diffs default to ged's diff (`ui/gedview.go`); see [Editor component (internal/ged)](#editor-component-internalged) below.
- Everything else still uses mygo's `editor`: hex view, read-only large files, files that are read-only or whose encoding can't be written back, files with conflict markers, files with the line-by-line blame column open, files over 50,000 lines, and tabs that aren't editable files at all — history versions, settings documents, search editors and terminal output text. With `WORKBENCH_GED_EDITOR=off` / `WORKBENCH_GED=off` everything falls back to mygo's editor.

Whichever component draws it, a `docs.Doc` always holds a mygo `editor.State` (`Doc.St`) and a buffer. When ged is in use, `St` is still there, with the caret, selected text and buffer kept in sync with ged, so App code keeps querying `St` as before (see [Keeping the single-pane editor in sync with `St`](#keeping-the-single-pane-editor-in-sync-with-st) below).

### Documents and tabs

A `docs.Doc` represents one open document. Its `Kind` is one of: file, history version (read-only), diff, interactive rebase, replace preview, search editor, file compare, terminal in the editor area, settings document, terminal output text, or side-by-side Markdown preview (`MDPreview`, see [markdown.md](markdown.md)); there are also two display modes, image and hex. Everything except files lives only in memory.

Tab rules (`app/tabs.go`):

- A single click opens a preview tab (italic), which the next preview replaces; double-click, Return or an edit pins it. The file being replaced records its caret and top visible line (`noteView`), and reopening it resumes from there (`resumeView`, only within the current run); opens that specify a line via `jumpIn` skip this.
- Reading files, history versions and diffs happens in the background; writing to disk happens only when the user saves, or chooses Save in the confirmation dialog when closing a tab or quitting.
- ⇧⌘T reopens closed tabs; ‹ › go back and forward (`Back` / `Forward`, ⌃- / ⌃⇧-), and jumping to another line in the same file counts as a step.
- Switching tabs in the current group (`app/tabswitch.go`):
  - ⇧⌘[ / ⇧⌘] and ⌥⌘← / ⌥⌘→ walk the tab strip in order, wrapping around at the ends (`StepTab`).
  - ⌃Tab / ⌃⇧Tab walk in most-recently-used order (`StepTabByUse`), using the `tabMRU` recorded in `visit` (memory only, not saved to state.json). Each ⌃Tab press switches to the previously used tab (repeated presses toggle between two tabs); there is no tab-list overlay shown while ⌃ is held yet.
  - When the keyboard is in a terminal, these keys go to the terminal tabs first (`term.prevTab` and friends come first in the registry).
- When a file changes on disk: unmodified tabs simply reload; modified tabs are flagged as conflicting, and the user chooses whether to keep their version or reload.

### File size

| Size | How it opens |
|---|---|
| Under `MaxDiffBaseBytes` 2 MB | Normally, with change bars (compared to HEAD) |
| Up to `MaxEditBytes` 8 MB | Editable, but not compared to HEAD (`NoBase`, with a notice on the tab), because the change-bar diff is computed by mygo's `DiffState` on the main thread |
| Up to `MaxViewBytes` 64 MB | Opened read-only (`Doc.Big`, using mygo's editor), highlighted in the background |
| Larger | Not opened; an explanation is shown instead |

#### Reading files

Files are read in the background (`docs.Loaded`) and swapped in with `setText` once ready.

- Reading happens in two steps on the same background goroutine: first `ReadFileText` / `ReadVersionText` reads the text and `a.Update` puts it in place (at this point `Doc.BasePending` is set), then `Loaded.LoadBase` reads the HEAD copy (for history versions: the parent commit's) and `ApplyBase` puts it in place.
- `Gen` tells whether a result still belongs to the current version: if `Gen` has changed (the file was reloaded), the result and base are discarded, and during a reload the slot keeps the old base in the meantime. The `fresh` parameter of `readDoc` is a separate matter: when true, a new buffer is built in the background instead of `Reset`-ing the old one on the main thread.
- For newly opened file / history version / diff tabs (`Loading`), the main thread waits in `readDoc` for up to `quickLoad` (40 ms): when the background read finishes, it first tries to hand the "put in place" step over an unbuffered channel to the waiting main thread and apply it synchronously (so the editor area doesn't go blank for a frame when the preview tab switches files, issue #50); if nobody is waiting any more (timed out), it goes through `a.Update` as usual.
- The base is not waited for in this step; it keeps loading in the background. Until it arrives — and for files with no base in the repository (untracked) — the text itself serves as `St.DiffBase`, reserving the change-bar strip in the gutter (`holdGutter`) so the code doesn't shift sideways when the base arrives.
- `Doc.SlowLoad` is set and "Loading…" drawn only if a tab is still loading after `loadingDelay` (150 ms); before that, only the editor background shows.
- When a read-only large file changes on disk, it is reloaded (`reloadBig`) after 2 seconds (`bigReloadGap`), to avoid constant reloads while a log file is being written.

#### Highlighting large files

In `internal/highlight`: mygo's built-in highlighter catches up 4 ms per frame on the main thread, so 64 MB takes two or three seconds. Here a goroutine tokenizes from the start and records the state at the beginning of each line, while the main thread only tokenizes the few visible lines from the recorded states; lines not yet reached are guessed from state 0 (`final` false). `Highlight` always returns true; on edits the old goroutine is stopped and computation resumes from the edited line.

### Encodings

`docs/encoding.go`. Encodings are detected on read (`Decode`):

- UTF-8 / UTF-16 with a BOM have the BOM stripped; UTF-16 without a BOM is detected from NULs at odd or even positions (`sniffUTF16`).
- Anything containing NULs is binary; valid UTF-8 is taken as is.
- Otherwise one of GB18030, Big5, Shift-JIS and EUC-KR is picked (`pickCJK`, thresholds tuned against the samples in `encoding_test.go`); if none fits, the encoding is unknown and the file is treated as read-only UTF-8.

Legacy encodings (GB18030, Big5, Shift-JIS, EUC-KR, EUC-JP, Windows-1252, ISO-8859-1) are editable only if encoding the text back reproduces the original byte for byte (`Writable`; otherwise the file is marked `Lossy`). Saving writes in the original encoding, asking first if some characters can't be encoded. A manual "Reopen with Another Encoding" (`ReopenWithEncoding`) that fails to decode marks the file `Broken` and read-only; the chosen encoding is recorded in the session as `Tab.Enc`. "Convert to UTF-8…" (`ConvertUTF8`) switches the encoding so the next save writes UTF-8.

Line endings in the buffer are always `\n`; on write they are converted back to CRLF if that's what the file had. The status bar can change the line endings of the whole file (`Doc.SetEOL`).

### Saving

Tidying before save (`docs/tidy.go`, two settings toggles `TrimTrailing` / `FinalNewline`): trailing whitespace is removed and a final newline added, as a single undo step. The caret's line is left alone, and in Markdown a line ending in exactly two spaces (a hard line break) is kept.

Auto save has three modes: Off, When the window loses focus (`blur`), and After 1 second of inactivity (`delay`). All file writes go through `WriteDoc` → `diskWrite`, which is also where local history is recorded (see [Local history](#local-history)).

### Formatting and linting

Both formatting and linting call external commands; there are no built-in tools and no LSP. Commands run in a login shell (the `Shell` setting or `$SHELL`, with `-lc`) so that Homebrew-installed tools are on PATH. The working directory is the repository root, or for files outside a repository, the file's folder. On timeout the whole process group is killed.

**Formatting** (`app/format.go`): ⇧⌥F, the context menu, or on save (`FormatOnSave`).

- The buffer's UTF-8 text is piped to the command's stdin, stdout is read and compared with the original line by line (`docs.LineEdits`, with each change narrowed down to the characters that actually differ), and only the changed parts are applied, as one undoable operation that preserves the caret, folds and scroll position.
- If the text changes again while the command is running, the result is not applied.
- Default commands are in `DefaultFormatters` (keyed by `editor.Language.Name`); `Settings.Formatters` can override them, and `"-"` turns one off. `{file}` is replaced with the quoted absolute path.

**Linting** (`app/lint.go`, UI in `ui/problems.go`): on save ("Lint on save" on the Formatting page of Settings, off by default) or manually with "Lint Current File".

- Default commands are in `DefaultLinters` (go vet, tsc, eslint, ruff, cargo check, shellcheck); `LinterOf` checks `Linters` in Settings first. There are two placeholders, `{file}` / `{dir}`, and `LintTimeout` is 60 seconds.
- Linters read the file on disk, so the buffer version is recorded at the moment of saving (`snapOf`).
- Output is parsed as `path:line:col: message` (`ParseProblems`) and set as `St`'s diagnostics (`SyncProblems`: clear first, then set against the recorded version; if the version no longer matches, positions are recomputed against the current text). mygo's editor draws them itself; ged's editor takes them from `St.Diagnostics()` and draws squiggles (`edSquiggles`), also listing them in the hover.
- The third tab of the bottom panel, "Problems", lists results from all repositories (sorted by repository, file, line), and the status bar shows the error and warning counts.

### Outline and sticky scroll

`internal/outline`:

- Markdown recognizes headings; Go uses `go/parser`; Python / JS / TS / Rust / Shell / YAML use per-line regular expressions; C / C++ / Java / Kotlin / Swift / C# use brace nesting (`brace.go`).
- `Parse` → `Build` flattens into a `Tree`; `At(line)` finds the symbol at the caret, and `Symbol.End` is the line where it ends.
- Files over `MaxBytes` 1 MB aren't parsed, and there are at most `MaxSymbols` 2000 entries.
- The sidebar's outline mode is in `ui/outline.go`.

Sticky scroll overlays, at the top of the editor, the first line of each not-yet-ended symbol that contains the topmost visible lines, up to `StickyMax` 5 lines (`Tree.Sticky` decides which). The `NoStickyScroll` setting turns it off (`stickyOn` in `ui/sticky.go`).

- mygo's editor: `stickyEditor` in `ui/sticky.go` adds an overlay layer on top of the editor.
- ged's editor: `gedFile` passes the result of `Doc.StickyOutline` through `Tree.Sticky` to `ged.Style.Sticky`, and ged draws it itself (see the ged section below).
- For files over `StickyQuick` 256 KB, or whose last parse took more than `StickySlow` 1 ms, the old outline is used while typing and recomputed only after a `StickyPause` of 500 ms, to avoid dropping frames while typing.

### Navigation and the Command Palette

The ⌘P palette (`app/palette.go`) picks a mode by prefix:

| Input | Contents |
|---|---|
| Empty | Recently used |
| Text | Files in all repositories (`fuzzy`) |
| `name@symbol` | Symbols in a given file (`app/palettesym.go`) |
| `name:line[:col]` | A given file, opened at that line |
| `@` | Symbols in the current file |
| `#` | Workspace symbols (`app/symindex.go`) |
| `>` | Commands |
| `>git ` | Run a git command |
| `>term ` / `>snip ` | Open a new terminal with a given terminal profile; insert a snippet |
| `>restore ` | Restore a deleted file from local history |
| `!` | Tasks |
| `:line` | Go to line |

The workspace symbol index is built on first use: a few background workers read and parse files and hand results back to the main thread in batches. After that, directories reported by the watcher are marked dirty, and the next use re-stats only dirty ones and reparses only files whose version changed. Open files use the tab's outline (unsaved edits count). The index isn't kept resident or saved to disk; there is one per window, capped at 200,000 symbols (`maxIndexSymbols`).

### Editor groups

`app/groups.go`. The main area can be split horizontally or vertically into several groups, nested, each with its own tab strip. The split tree is `EdNode` (a leaf is a group; otherwise it's two halves, side by side or stacked, plus a ratio), saved in the session's `layout`.

#### Tab strip

- The tab strip (`tabStrip` in `ui/tabs.go`) and the bottom panel's terminal tabs (`ui/bottom.go`) share `scrollStrip`: horizontal scrolling, with its own 2 pt thin bar drawn along the bottom edge.
- Scroll positions are kept in `Group.TabStrip` and `Repo.TermStrip` respectively; when the current tab changes it is scrolled into view (`Group.TabShown` / `Repo.TermShown`).
- Each tab is a `panelTab` (`ui/widgets.go`): full-bleed tiles of equal height `headerHeight`, no rounded corners, a `cardLine` on the right; the current tab covers the line under the card header with the card body color. The first tab sits flush with the card's left edge (card header left padding 0). In the VS Code style, a 1 px line is also drawn on top of the current tab (`vsActiveLine`).

#### The current group and other groups

The tabs of the focused group live in `a.Docs` / `a.DocSel`, so code dealing with "the current tab" doesn't need to care about groups. Other groups keep their tabs to themselves, and switching groups swaps the two (`swapTo`). Use `AllDocs()` to iterate over every tab.

#### Splitting and dragging

- A tab lives in only one group at a time: dragging a tab, ⌘\ (split right) and ⌥⌘\ (split down) all move the current tab over (`SplitEditor`).
- When dragging a tab, dropping it in the middle of a group moves it into that group; dropping within 1/4 of an edge splits off a new group on that side (`DropSide` / `DropRect`). A group smaller than twice the minimum size (160×120) can't be split.
- When the last tab leaves a group, the group collapses (`collapse`) and its neighbors fill the space.

#### Mirror tabs

To view the same file in two groups at once, use a mirror tab (`app/mirror.go`, ⇧⌘\): it shares the buffer and diff base with the original, while the caret, scrolling, folds and find state are independent. Reading from disk, saving, switching encodings and the like are done on one document only and then synced to the other mirrors via `relink`. Opening a file always finds the original (`Copy` is 0).

#### Maximized bottom panel and Zen Mode

Maximize Bottom Panel (`app/bottommax.go`, ⇧⌘J): `App.BottomMax`, per window and memory-only. When `BottomMaxed()`, `center` in `ui/view.go` doesn't draw the main area or the drag handle beside the bottom panel, and the bottom panel takes over that space with `Grow(1)`; `St.BottomH` / `St.TermW` stay unchanged. It is restored when the bottom panel is closed (`ToggleArea` / `ToggleProblems`), when entering Zen Mode, and when switching to or opening an editor tab (`showEditor` in `SelectDoc` / `FocusGroup` / `MoveDocTo`).

Zen Mode (`app/zen.go`, ⌃⌘Z): all three panels close and the editor groups are centered. It is per window and not persisted; when saving state.json, the panel state from before entering is written. Press Esc twice to exit.

#### Start page

When an editor group has no tabs, the start page is drawn (`ui/startpage.go`):

- Contents: the workspace name and `reposSummary`; changed files (up to 5 per repository, clicking runs `OpenDiff`); files in `St.Recent` that are still in the workspace (`RepoOfPath`); and worktrees other than the main one (status is shown only for those `WtInfo` has computed).
- Rows are `ButtonBase` with `FocusBind(&a.StartFocus, rowIndex)`; the page's `OnShortcut` moves through them with ↑↓ via `FocusedValue`. Row keys are supplied by the caller (the same file can be both staged and unstaged, so names can repeat).

### Go to definition, completion, hover, find references

None of these use LSP. When a file tab uses mygo's editor, they come in through mygo's `State.OnDefinition` / `OnCompletion` / `OnHover` callbacks; with ged's editor, `ui/gedfile.go` handles them (⌘-click goes through `Style.Definition`, and Grove draws the completion list and hover itself). What gets looked up and how it's ranked is the same App code either way. `Doc.ShowCompletions` / `ShowHover` / `Run` and friends dispatch to ged or `St` based on `UsingEd()` (`docs/edit.go`).

#### Go to definition

`app/definition.go`. ⌘-click, F12 and the context menu call `GoToDefinition`:

- `identAt` takes the identifier under the caret and its `pkg.` qualifier.
- Declarations with the same name are looked up in the `Doc.Outline()` of open files and in the symbol index (`app/symindex.go`), excluding headings and groups; `narrowDefs` narrows by qualifier = directory name, same file, same directory.
- A single match goes straight there with `OpenAt` (pinned tab, recorded in the ‹ › history); several open the palette at `#name`, and while `Palette.defs` is set only same-name entries are listed (`definitionItems`).
- If the file list isn't loaded yet or the index is being built, the request is recorded in `App.defWait` and rechecked when `LoadFiles` finishes (`defFilesLoaded`) and when `mergeSymbols` finishes the last batch (`defIndexed`); the jump proceeds only if the tab, content version and caret are all unchanged.

#### Completion

`app/wordcomplete.go`. `Complete` passes snippets (`snippetCompletions`) and words (`wordItems`) together to `ShowCompletions`.

- Words come from: 128 KB before and after the caret in this file (nearer first; those in the outline carry a kind, taken from `Doc.LastOutline()` rather than reparsing the whole file on every keystroke); open files of the same language family (`langFamily`; cached in `App.words` by tab and content version, files over 1 MB not scanned); and names in the same repository's index (only if the index has already been built — completion doesn't build it).
- Prefix matches come first, then subsequence matches; an entry exactly equal to the prefix is not offered.
- `Settings.NoWordComplete` turns off words while typing (⌃Space still offers them); in Markdown / plain text (`docs.IsProse`) words are offered only on ⌃Space.
- mygo's editor: mygo's Return accepts a completion, so `CompleteTick` (run every frame before drawing the editor) removes and re-offers the same-name entry from the list once a whole word has been typed; mygo has no API to read the completion state, so nothing is re-offered on Backspace.
- ged's editor: the list lives in `Doc.EdCompl` and is drawn by `ui/gedfile.go`; it is triggered by `Style.Changed` after an alphanumeric character is typed, ↑↓ selects, Return / Tab accepts, Esc closes.

#### Hover

`app/hover.go` + `ui/hover.go`. Declarations are found the same way as for go to definition (`definitions` + `narrowDefs`; nothing is shown when resting on the declaration line itself). If the index isn't ready, nothing is shown this time, and `EnsureFiles` / `refreshSymbols` are kicked off to build it.

- Trigger: for mygo's editor it is `State.OnHover` (pointer resting 600 ms, or the `editor.showHover` command; mygo's default ⌘K ⌘I is taken first by the menu bar's ⌘K, see [docs/shortcuts.md](../shortcuts.md)), which calls `HoverAt` synchronously — mygo only accepts an immediate answer, and a late `ShowHover` would attach to a hover that's already closed. For ged's editor it is `Style.Hover` (pointer resting 500 ms); `edHover` calls `HoverAt`, adds the lint problems at that position, and `edTipCard` draws it.
- Declaration source: open files are read from the buffer, others from disk (files over 1 MB or binary aren't read; only the location is given).
- `declText` takes text from the declaration line until brackets balance, cut before the first `{` outside brackets, at most 6 lines. `commentAbove` collects the line or block comments immediately above (skipping `@annotations` and `#[attributes]`, stopping at blank lines); for Python, a docstring after the declaration is checked first.
- The hover shows the declaration (in the editor font), followed by the comment, a link button for the location, and "…and N more" for same-name matches (clicking goes through `GoToDefinition` and opens the palette).
- `Settings.NoHoverDef` turns off declarations (mygo's editor doesn't install `OnHover`); lint problems still show on hover.

#### Find references

`app/references.go`. `editor.findReferences` (⇧F12; not in the editor's keymap, so it falls through to the window's command) and the editor context menu call `FindReferences`: the name under the caret becomes a whole-word, case-sensitive `search.Query`, "files to include" is the extensions of the same language family (`referenceGlobs`; files without an extension aren't restricted), `Search.Scope` is cleared to search the whole workspace, and the results are simply the sidebar search.

### Other

- The status bar's language, indentation, line endings and encoding are all clickable (`app/docstatus.go`); a changed language or indentation applies only to that file and is recorded in the session.
- Snippets (`internal/snippets`): built-in ones plus those in the state folder; Tab expands, Tab / ⇧Tab move between placeholders.
- Images: preview, and image diff against HEAD. Markdown and SVG toggle between source and preview with ⇧⌘V (`TogglePreview`); `ToggleHex` in `app/image.go` toggles the hex view, and files can also be opened in the default app.
- Diffs come in side-by-side and unified, and full file and changes only (`unified` / `fullDiff`); ged's diff can also ignore leading and trailing whitespace (`diffTrim`).
- Comparing two files on disk, or a file with the clipboard: `app/filecompare.go` (first "Select for Compare", then "Compare with <file>").
- Code colors (`codeTheme` / `softTheme` in `ui/occur.go`, set into `St.Theme` every frame): the VS Code style swaps in `fullCodeTheme` wholesale; the Grove style changes only two things and otherwise follows mygo's defaults — whitespace symbols drawn inside selections (dots, arrows) use `TextMuted` at 0.22 (mygo's default is 0.45), and operators and punctuation use `TextMuted` (`softTokens`).

#### Only on mygo's editor

The following are add-ons for mygo's editor; ged has its own approach (see the next section):

- After the caret moves via the keyboard, the editor keeps min(5 lines, 1/5 of the view height) above and below the caret (`caretMargin` in `ui/caret.go`). mygo itself keeps only 3 lines; clicks and the scroll wheel are unaffected. ged scrolls only enough to make the caret visible (`revealCaret`).
- When the caret rests on an identifier (no selection, a single caret) for 250 ms (`ui.OccurRest`), other occurrences of the same name in the file (whole word, case-sensitive) get mygo's `Occurrence` background (`occurMarks` in `ui/occur.go`, written into `St.TextBackgrounds`, with state in `Doc.Occur`); typing or moving to another word clears them immediately.
  - mygo itself only marks other occurrences of the selected text when there is a selection, and has no "word under caret" option, so Grove does this on its side with `TextBackgrounds`.
  - It is computed once per content version and word; for files larger than `StickyQuick` only the visible lines are searched, and scrolling triggers a new search. Search editors (whose background marks the matches) and the hex view are not marked.
  - ged doesn't draw `St.TextBackgrounds`; it marks other occurrences of the selected text instead (see the next section).
- The caret in the editor and in diffs stops blinking after a period of inactivity (`restingEditor` / `restingDiff` in `ui/caret.go`; `WORKBENCH_CARET_REST=off` disables this). mygo's editor schedules a frame every 500 ms to blink the caret while it has the keyboard, and mygo only releases the window-sized frame memory after 2 seconds without a new frame, so constant blinking costs an extra one or two hundred MB of memory and burns CPU continuously. ged's caret doesn't blink in the first place.

## Editor component (internal/ged)

`internal/ged` is Grove's own editor component; a single type, `ged.Diff`, serves two uses:

- **Diff** (`ged.NewDiff`): side-by-side, unified, and in-place editing of the new side. Diff tabs, replace previews and file compares use it by default (`gedDiff` in `ui/gedview.go`).
- **Single-pane editor** (`ged.NewEditor`): `plain` mode, where every row is "unchanged" and old and new are the same text, so only one pane is drawn. File tabs use it by default (`gedFile` in `ui/gedfile.go`).

It uses only mygo's public text shaping, painting and input APIs (`ui.ShapeRichText`, `Painter`, `HandleInput`) and imports only `mygo/ui` and `mygo/ui/editor` (buffer, selection, language tokenizers, keymaps). It knows nothing about Grove's theme and doesn't touch `internal/ui`: colors, fonts, keymaps and any extra things to draw are supplied by the caller through `ged.Style`. The diff internals (alignment, folding, word-level highlighting, hunks, line selection, unified view, in-place editing, wrapping, find) are covered in [staging.md](staging.md); this section covers the single-pane editor and the parts shared by both. Known pitfalls are in the "Editor" section of [../mygo-pitfalls.md](../mygo-pitfalls.md).

### Files

| File | Responsible for |
|---|---|
| `diff.go` | The diff model: `Align` / `AlignTrim` alignment, `Folded` folding, `Words` word-level highlighting, `Hunks`, `Unified`; types such as `Row` / `Span` / `Hunk` |
| `view.go` | `Style` (all configuration supplied by the caller), `View(c, key, d, s)` for drawing and taking input, the per-line shaping cache and tokenization, scrolling, the caret |
| `edit.go` | In-place editing: buffer changes, undo grouping, `Caret` / `PlaceCaret`, `revealCaret` |
| `editor.go` | The single-pane editor: `NewEditor`, `SetText`, folding, selection, `VisibleLines` / `ScrollToLine` / `TopLine` and other queries for the App, `Command` / `Lookup` |
| `keys.go` | Keymap and editing commands |
| `find.go` | Find and replace, other occurrences of the selected text |
| `wrap.go` | Word wrap: prefix sums of screen rows per line, with estimates for lines not yet shaped |
| `sticky.go` | Sticky scroll |
| `minimap.go` | Minimap |
| `guides.go` | Indent guides |
| `marks.go` | Gutter change bars, squiggles, end-of-line notes (inline blame) |
| `glide.go` | Smooth scrolling for the mouse wheel |
| `menu.go` | Positions and actions the context menu needs: `Spot` / `MenuSpot` / `KeySpot`, `Run`, `CanEdit`, `HasSelection` |

### How the UI configures it

ged has no notion of themes; every frame `internal/ui` assembles a `ged.Style` and hands it to `ged.View`:

- `gedStyleBase` (`ui/gedview.go`): the base shared by both uses — code colors (`fullCodeTheme`), the editor font, the Grove style's added/removed colors, the fold bar text ("%d unchanged lines"), and so on.
- `gedStyle`: for diffs, adding diff backgrounds and added/removed colors on top of the base (the VS Code style has its own set of word / line colors, blank filler and bracket coloring).
- `gedEditorStyle` (`ui/gedfile.go`): for the single-pane editor; in the VS Code style, line numbers use VS Code's colors, brackets are colored by depth (`bracketColors`) and the minimap is on.
- `gedFile` / `gedDiff` then fill in behavior per tab: the buffer (`Buf`), clipboard (`Copy` / `Paste`), keymap (`Bindings`: `app.EditorBindings()` / `app.DiffEditBindings()`), indentation (`a.DocIndent`), whitespace rendering, guides, wrapping (`a.WrapOf`), sticky scroll, the find bar (`gedFindBar`) and the context menu (`Menu`).
- The single-pane editor additionally has: `Intercept` (⌥-click to add a caret, etc.), `Changed` (sync back to `St` after edits, trigger completion), `Fallback` (commands ged doesn't know), `Hover`, `Definition` (⌘-click), `Changes` (change bars, from `app.DocHunks`), `GutterClick` (clicking a change bar opens the peek, `a.TogglePeek`), `Squiggles` (`St.Diagnostics()`), `Notes` (inline blame, `d.BlameInlay`) and `Extra` (`St`'s other carets).

### When mygo's components are still used

- Diffs: `useGed(d)` (`ui/gedview.go`) holds when `WORKBENCH_GED` isn't `off` and there is a `DiffSt`; otherwise mygo's diff is drawn (`restingDiff`).
- File tabs: `useGedEditor` (`ui/gedfile.go`) requires that `WORKBENCH_GED_EDITOR` isn't `off`, the document is a file, has a buffer, isn't hex, isn't a read-only large file (`Big`), `St` isn't read-only, `Doc.Editable()` holds (not `Lossy` / `Broken`, encoding writable, not a local history snapshot), there are no conflict markers, the line-by-line blame column isn't open, and the file doesn't exceed `gedEditorMaxLines` (50,000 lines). Otherwise it goes the old way: `blameEditor` when the blame column is open, `stickyEditor` when sticky scroll is on, else `peekEditor` (`ui/tabs.go`).
- Every other `Kind` (history versions, settings documents, search editors, terminal output text) uses mygo's editor.

### Keeping the single-pane editor in sync with `St`

When ged is in use, `Doc.Ed` is that `*ged.Diff`, and `Doc.EdOn` means it was what got drawn last frame (`UsingEd()`).

mygo's `editor.State` (`d.St`) is still there and works without a view (`Selections` / `GoTo` / `Insert` / `Run` touch only the buffer), so the buffer, carets and St are kept in sync, and the App code that queries the caret, `St.GoTo` / `St.Select`, snippets, word completion, formatting and dropped-in text all work unchanged. Two rules keep them in sync:

- Before drawing each frame, `edPull`: if St's selection differs from the last sync (`Doc.EdSel`), the App has moved the caret; the primary one is passed to ged with `SetSelection` (scrolled into view) and the other carets are drawn as `Style.Extra`.
- After ged handles an event, `Style.Changed` → `edPush`: if ged's selection changed, `St.SetSelections` (leaving just one).

Beyond that:

- Multiple carets are handled by St: when St has more than one selection, `Style.Intercept` → `edMulti` hands typing (`St.Insert`) and keys (looked up as commands via `Diff.Lookup`, then `edRunMulti` runs them on St with `Run`, with the clipboard going through Grove's) to St; Esc leaves only one. ⌥-click adds one via `PointAt`.
- Commands ged doesn't implement go through `Style.Fallback` (`edFallback`): go to line opens ":", and completion, go to definition and show hover are handled directly; everything else goes to `St.Run` followed by `edPull`.
- Visible lines, scrolling to a line, running editor commands, and opening completions / hover go through `Doc.VisibleLines` / `ScrollTo` / `Run` / `ShowCompletions` / `ShowHover` (`docs/edit.go`), which dispatch to ged or St;
  every place in the App that used to touch the St view directly now goes through them. `findGed` routes find, replace and go to line to ged as well when `UsingEd`.
- ⌘-click is `Style.Definition` (go to definition).

### Completion and hover

- The completion list (`edCompletionList`) and hover (`edTipCard`) are placed by `CharBox` in the Row at the `gedFile` level.
- Typing an identifier character triggers, inside `Changed`, `edComplete` → `a.Complete` → `Doc.ShowCompletions` (`Doc.EdCompl`); `edCompletions` filters by prefix first, then by subsequence.
  ↑ ↓ Return Tab Esc are handled in Intercept; accepting is `St.Select` + `St.Insert` (snippets are still recognized by `SnipTick`).
- When the pointer rests (`HoverDelay`), `Style.Hover` → `edHover`: the declaration from `a.HoverAt` plus the diagnostics covering that spot (`edDiagnosticsAt`); the card stays open while the pointer is on it (`Doc.EdOut`).

### Drawing and scrolling

- Each line has a cached shaped `TextLayout`; tokenization uses mygo's `lang.Tokenizer`, with the tokenizer state and bracket depth recorded per line (for bracket coloring). Only the first part of a line longer than `lineMax` (4000 bytes) is shaped.
- The caret is 2 pt wide, doesn't blink, and has no timer scheduling frames. The caret's line gets a background highlight (VS Code's current line highlight), drawn even without keyboard focus.
- Moving the caret via the keyboard scrolls only enough to make it visible (`revealCaret`).
- You can scroll past the last line (`tail()`): when the content is taller than the view, the single-pane editor leaves max(3 lines, view height minus one line) extra, so the last line can be scrolled to the top (VS Code's scrollBeyondLastLine); diffs leave 3 extra lines.
- Mouse wheel (`glide.go`): a non-trackpad notch glides over with ease-out in `glideTime` 150 ms; consecutive notches chain together and the opposite direction turns around immediately. Anything else that moves the scroll position (trackpad, keys, dragging the scrollbar, being carried along by a preview) stops the glide. The trackpad still tracks your fingers directly; with "Reduce motion" on in System Settings, each notch jumps at once.
- If in the same frame there is both a restored top (`wantTop`) and a caret to reveal (`want`), and the caret is already within the scrolled view, it doesn't scroll further for the caret.
- Word wrap is in `wrap.go`, shared with the diff.
- Tests of mygo editor features (its FindBar, `LineTop` / `ScrollY`, `TextBackgrounds`, the sticky scroll lines it draws) are pinned to `gedEditor(t, false)`.

### Folding

Single-pane editor only (`editor.go`):

- Rule: a line ending in an opening bracket folds up to the line before the matching closing bracket; otherwise the following lines with deeper indentation are folded.
- A folded line shows "⋯" after it; fold arrows appear only when the pointer moves over the gutter (`foldGutterW` 16); a caret entering a folded range unfolds it automatically.
- Commands: `Fold` / `Unfold` / `ToggleFold` / `FoldAll` / `UnfoldAll` / `FoldRecursive` / `UnfoldRecursive` / `FoldAt` / `UnfoldAt`; `FoldImports` folds the first foldable run of imports in the first 200 lines (lines starting with import / use / from).
- The diff's "fold unchanged lines" is a separate mechanism (`Folded`, see [staging.md](staging.md)).

### Sticky scroll

`sticky.go`: at most `stickyMax` 5 lines, and no more than 1/3 of the view height when the view is short. Which lines stick to the top is supplied by the caller through `Style.Sticky` (the single-pane editor uses the outline; the diff uses `DiffOutlineOf`, one per side; the unified view uses the new side). ged only draws them and jumps there on click.

### Minimap

`minimap.go`. Drawn only on the single-pane editor, when `Style.Minimap` is true (`gedEditorStyle` turns it on only in the VS Code style) and the editor is at least `minimapMinW` (480) wide: `minimapW` 100 wide on the right, each line 2 points tall and each character 1 point wide. The slider for the current screen is always drawn faintly (following VS Code's `minimap.showSlider: always`), one shade darker under the pointer and another shade darker while dragging; pressing outside the slider moves it under the pointer, ready to drag. Thin bars are drawn in token colors (cached per line by text and tokenizer state).

When it's taller than the editor, it scrolls proportionally (when the editor reaches the bottom, so does the minimap). `geom.right` is the width reserved on the right, and the widths of the text, sticky scroll, horizontal scrolling and soft wrap are all computed from it. Tested by `TestMinimap` in the `ged` package.

### Guides, brackets, whitespace

- Indent guides (`guides.go`, `Style.Guides`): the block containing the caret is one shade darker.
- Bracket colorization by depth (`Style.Brackets`, supplied only in the VS Code style, with the three Light+ / Dark+ colors; ged's diff gets it in the VS Code style too): the bracket depth at the start of each line is cached together with the tokenizer state (`depths` / `stateAt`), and `walkBrackets` skips brackets inside strings and comments.
- Whitespace rendering (`Style.Whitespace`) follows the setting.

### Find and replace

`find.go`:

- The find bar is drawn by the caller through `Style.FindBar` (`gedFindBar`); ged handles matching, jumping to the previous / next match, and replacing one / all. Case-insensitive matching uses ASCII folding.
- When the selection is within one line, isn't all whitespace and is no longer than `occurMax` (200 bytes), identical text elsewhere (case-sensitive) gets a background highlight (VS Code's selection highlight).

### Gutter and line marks

`marks.go` (drawing) and `ui/gedfile.go` (data), for the single-pane editor:

- The change bars to the right of the line numbers are `Style.Changes` (`edChanges` from `app.DocHunks`: green for added, blue for modified, and for deleted a red triangle at the top of the next line, `ged/marks.go`).
- When there are changes, clicking that line's number triggers `Style.GutterClick` → `a.TogglePeek`; the card is `peekCardAt`, shared with mygo's editor (`edPeekCard` computes the card's top from `CharBox`); see the section on editor change bars and the peek card in [staging.md](staging.md).
- Diagnostics (`St.Diagnostics`, still set by `SyncProblems`) are drawn as `Style.Squiggles` squiggles; inline blame (`Doc.BlameInlay`) is the faint end-of-line text of `Style.Notes`.

### Keys, chords and the context menu

- In single-pane mode, ged, like mygo's editor, first passes keys, text and presses to `Style.Keys` (`docKeys`: Tab for snippet placeholders, ⇧⌥F and the ⌥ character that comes along with it) before handling them itself.
- Chords (like ⌘K ⌘0) are recognized by ged's `command` from the keymap (the first key returns `chordPending`; single-pane editor only). Those starting with ⌘K are taken first by the menu's "Run Git Command…", same as with mygo's editor.
- The context menu is drawn by Grove (`gedFileMenu`; tests use `shownMenu` / `chooseMenu`) and follows VS Code's editor context menu down to the Command Palette: Go to Definition, Find References; Change All Occurrences, Format Document; Cut, Copy, Paste; Command Palette….
  As in VS Code, split, local history, Finder and Copy Path are in the tab's context menu (`tabMenu`), while save, lint and blame are in the menu bar and Command Palette; mygo's editor menu still uses `docFileItems`.
- The editor's own commands (Go to Definition, Change All Occurrences) aren't in the menu bar registry; their keys are looked up by `edBindingOf` in `app.EditorBindings()` and then `editor.DefaultBindings()` (single keystrokes only, no chords).
  When a command has several keys, the one shown in VS Code's menu is picked from `edMenuKeys` (Change All Occurrences is ⌘F2, `TestEdMenuKeys`).
- `menu.go` reports which line and column was clicked (`Spot` / `MenuSpot`, or `KeySpot` when the menu is opened from the keyboard) and whether editing is possible and there is a selection; the menu itself is drawn by the caller. The diff's is `gedMenu` (see [staging.md](staging.md)).

## Search and replace

### Search

`internal/search` only does the search computation. Candidates come from `git ls-files` (or the list the file tree has already loaded), minus `files.exclude` and the include / exclude globs; there is one worker per CPU. Literal / whole word / regex and case sensitivity are all handled by one `Matcher`.

- By default, binaries and files over `MaxFileSize` 2 MB are skipped; checking "Also search files over 2 MB" raises the limit to `MaxBigFileSize` 32 MB, and checking "Also search ignored files (.gitignore)" adds ignored files (`Query.Big` / `Ignored`).
- At most `MaxMatches` 5000 matches; "Load 5000 more" raises it, up to `MaxLimit` 50,000.
- Non-UTF-8 files have their encoding detected with `docs.Sniff` and are decoded to text before searching, so positions are positions in the editor buffer (`Match.Enc`).
- Multi-line literal terms are converted to regular expressions, with line breaks written as `\r?\n`. `Match.Len` extends only to the end of the line, `Match.Span` is the length of the whole match, and `CR` / `BufLen` convert byte positions in CRLF files to buffer positions.
- Regular expressions use Go's RE2 first; if that fails to compile and the pattern uses backreferences or lookaround, `regexp2` takes over (`advanced.go`), with a 2-second limit per match (`advTimeout`). Files that time out are treated as skipped and flagged in the results (`Result.TimedOut`).

App side (`app/search.go`): searching starts 150 ms after typing stops, and a new search cancels the previous one; results are grouped by file. After a batch of searching completes, `debug.FreeOSMemory()` is called once to return the memory of the candidate list and the file contents read to the system.

- Recent searches: `SearchHistory` (up to 20 entries, including toggles and globs), browsed with ↑↓ in the box.
- Scope: right-click a folder and choose "Search Only in This Folder" (`SearchScope`, `Root.Sub`).
- Dismissing: click × in the results to remove one match or a whole file (`DismissMatch` / `DismissFile`); dismissed items are skipped when replacing.
- Following: when a file in the results changes, after `FollowDelay` 300 ms only those files are reread and searched again (`app/searchfollow.go`), without redoing the whole search.
- ⇧⏎ turns the search box into a multi-line one (`multiLine`).
- Search editor (`app/searchdoc.go`): opens the results as a read-only text tab with context lines; Return jumps to a match.

### Replace

`search.Plan` first verifies that every match is still in place (the file may have just changed), then computes the edits; `$1` in regex replacements is expanded by `Expand`.

`ReplaceMatches` in `app/replace.go`:

- Open files have their buffer modified (undoable, not saved); files that aren't open are written straight to disk (through local history).
- After replacing, the search isn't redone wholesale: only the replaced files are rematched in memory against their new contents and spliced into the results (`splice`), with dismissed items moved along (`moveDismissed`).
- Replace operations are logged in the Output panel.

Replace preview (`app/replacepreview.go`): opens a diff-style tab with the original on the left and the replaced content on the right. If a file changes again while the preview is open, the preview is flagged as out of date and can be regenerated.

"Undo last replace" (`app/replaceundo.go`) handles only files written directly to disk:

- The pre-replace contents are recorded (at most `MaxUndoReplace` 20 MB per replace; beyond that, that replace can't be undone), and undo restores only files that haven't changed again since the replace.
- Only the most recent operation is recorded, and it isn't persisted; open files are undone with the editor's ⌘Z.

## Local history

`internal/history` only does storage: `Store{Dir, Now}`, one directory per file at `<state folder>/history/<first 12 hex digits of the path's sha256>/`, containing `index.json` plus content files (named by the millisecond timestamp `T`). Each entry has a `Why`: Save, On disk before save, Before restore, or Renamed from.

- A new "Save" entry less than `MergeWindow` 10 seconds after the previous one replaces it, so auto save doesn't produce piles of entries.
- Files over `MaxSnapshot` 256 KB aren't recorded; each file has at most `MaxEntries` 50 entries, and the total is capped at `MaxTotal` 200 MB.
- `Prune(days)` deletes expired entries (30 days by default, `historyDays`) and, when over the total cap, deletes oldest first; it runs once in the background `PruneDelay` 30 seconds after startup. `noLocalHistory` turns the whole thing off.

`app/localhist.go` decides when to record:

- All file writes go through `diskWrite` (save, save all, save after formatting, auto save, convert to UTF-8 and replace all use it).
- Before writing, if the copy on disk hasn't been recorded yet, an "On disk before save" entry is recorded first (on the first save, or when another program has changed the file); after a successful write, the written copy is recorded.
- Files in the state folder, under `.git`, images, and files hidden by `files.exclude` aren't recorded (`keepsHistory`).
- Renames and moves inside the app carry the history along (`localMoved` → `Store.Move`).

In the right panel's file history, local history entries and commits are interleaved by time (`Hist.Local`). Clicking an entry opens a compare tab (`OpenLocalCompare`, with that entry on the left and the current file on the right; `docs.Doc.Snapshot` records which entry it is), from which it can be restored (`RestoreSnapshot`, which first records a "Before restore" entry). Accidentally deleted files can be recovered with "Restore File from Local History…" (`>restore ` in the Command Palette) (`RestoreDeleted`).
