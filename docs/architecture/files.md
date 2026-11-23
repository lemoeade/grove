# File watching and the file tree

[← Architecture overview](../architecture.md)

Per-repository file watching; the file tree's model, interaction and appearance; and Grove's own context menu (first
built for the file tree, now used almost everywhere).

## File watching

The code is in `internal/watch/watch.go`: one `Watcher` per repository, wrapping `mygo.WatchFiles` in HintsOnly mode — it
only wants "something in this directory changed" hints, keeps no snapshot, doesn't stat files and needs no event list.
The app side is in `app/watch.go` (`watchRepo` starts, `watchChanged` receives, `watchStopped` stops).

### What is watched

- The working tree, recursively. `.git` (nested repositories), `node_modules` and git-ignored directories are not
  watched. Ignored directories are read by `git.Ignored` (`ls-files --others --ignored --exclude-standard --directory`,
  listing only the top level) once when watching starts; after that, when a change is reported more than `ignoredTTL`
  (30 seconds) after the last read, it is read again (`.gitignore` may have changed, or something like `dist/` was created).
- In the git directory: `HEAD`, `index`, `packed-refs`, `refs/`, and the `worktrees/<name>` level (each linked worktree's
  own `HEAD` and `index`: creating / removing a worktree or switching branches inside one must update the main
  repository's worktree list); `objects`, `logs` and so on are not watched.
- When `.git` is under the repository root, all of this is in the single working-tree watcher; when the git directory
  isn't under the root (a linked worktree), a second watcher covers the git directory, and if the common directory isn't there either, a third one covers it.

### Coalescing and `Change`

Change hints are coalesced by `Debounce` (300 ms): Grove waits 300 ms after the last change before reporting, and if
changes keep coming, it reports at most `MaxDelay` (1 s) after the first one. What gets reported is a `Change`:

- `Git`: something changed in the git directory (commit, stage, switch branch, pull…).
- `Files`: files in the working tree changed.
- `Dirs`: when `Files` and not `Full`, the directories that changed (repository-relative paths, deduplicated, descendants
  dropped when an ancestor is present), at most `MaxDirs` (200); more than that is treated as `Full`. The file tree uses it
  for incremental refresh (see "Incremental refresh" below).
- `Full`: it's unclear what changed. When the system only reports `.` (files under the root changed, or the system dropped events and everything must be rescanned), `Git`, `Files` and `Full` are all set.

When the app receives it (`watchChanged`), it records it in `Repo.hints` (`fileHints.note`) and calls `RefreshRepo`; for
`Files` it also notifies full-text search, the symbol index and the worktree block (`SearchFilesChanged`, `symFilesChanged`, `wtTouched`).

### Three kinds of watchers

- `Start`: Git repositories.
- `StartPlain`: plain folders; it only watches the working tree and skips `node_modules` and dot-directories, but still
  watches `.git` under the root (only `refs/`), so it notices a `git init` and rereads the folder as a repository.
- `StartTree`: linked worktrees expanded in the Git panel's worktree block (`app/wtwatch.go`, at most `WtWatchMax` 16 per
  repository); it only watches the working tree, not the git directory (its `HEAD`, `index` and refs are all in the main
  repository's `.git`, which the main repository's own watcher already covers).

### When watching stops

If the repository folder (or its git directory) is deleted or moved, `ErrGone` is reported; when a watcher errors or
fails to start, the app logs "File watching stopped" in the Output panel (`watchStopped`), and from then on relies on
switching back to the window (`RefreshOnFocus` rereads repositories whose watcher stopped) or a manual refresh. For the
refresh entry point, see [Git's "Reads and refresh"](git.md#reads-and-refresh).

## File tree

### Model

The model is `internal/files`'s `files.Tree` (`files.go`, one tree per repository); the file list is read by `LoadFiles`
in `app/tree.go` (read in the background, handed back on the main thread via `Tree.Done`).

- Git repositories: built from `git ls-files --cached --others --exclude-standard` (`git.LsFilesModes`, with `-s` to get
  mode bits so symlinks and submodules are recognized; untracked files come from the same call, minus the deleted files listed by `ls-files --deleted`).
- Plain folders: walked by `files.Walk`, at most `WalkCap` (50,000) files; `node_modules` and dot-directories aren't
  entered and are listed as ignored folders.
- After reading, it also: merges in things created inside the app that `ls-files` can't list (`Tree.Extra`: empty
  folders, ignored files, `files.WithExtra`); removes other worktrees nested inside the repository (`Repo.nested`,
  `dropNested`); removes what `files.exclude` hides (`app/exclude.go`, see [Workspace and settings](state.md)).
- Each repository is a section in the tree, with its repository row sticky; whether a repository is expanded is recorded
  in `Repo.Expanded` (the session's `open`), and expanded folders in the session's `dirs` (`workspace.Session.Dirs`, repository root → folders).

#### Incremental refresh

The code is in `app/treediff.go`:

1. The watcher's `Change.Dirs` (and saved files) are recorded in `fileHints`; if `.git` changed, on `Full`, or when the
   total exceeds `MaxDirs` (200), the whole tree is read.
2. When `RefreshRepo` starts, `startTreeDiff` builds a `treeDiff` from the tree as it is at that moment and `run`s it in
   the background: `Tree.Diff` compares only those directories (and files) between disk and the tree (no `ls-files`), and
   asks git once (`git.Listed`, a single process) whether the new files and new directories should be listed (ignored ones
   aren't). If a new directory contains files to list, more than 200 entries need asking, a directory in the tree is gone,
   `files.exclude` changed, or git errors, it falls back to reading the whole tree.
3. Back on the main thread, `applyTreeDiff`: `Tree.Apply` applies the additions and removals. If the whole tree was reread
   during the background computation (`Gen` changed) or is being read, the result is thrown away.

Plain folders, "Show Ignored Files" being on, or a tree that hasn't finished loading or failed to load skip the incremental path and read the whole tree.

#### Special rows

- Symlinks: tracked ones use the mode from the index; only untracked ones and ones added incrementally look at the disk
  (`ReadLinks`), producing `Link{Target, Broken, Dir, Up, Deep}`. A link to a directory can be expanded (read lazily one
  level at a time when expanded, `lazyLinks`); links that have already passed through `MaxLinkDepth` (8) linked
  directories (`Deep`), point at an ancestor (`Up`) or are broken don't expand and are drawn as file rows. Rows inside a link (`Row.Linked`) have no git status.
- Ignored files: hidden by default; when `ShowIgnored` is on, ignored directories are listed by `git.LsIgnored` and grayed
  out (plain folders ignore the toggle; theirs are the directories `Walk` doesn't enter). They are read one level at a
  time when expanded (`Lazy` / `ReadLazy`, at most `FilterCap` 500 entries per level, with the rest shown as one row "N
  more not listed"). They aren't in `Paths`, so neither Quick Open nor full-text search includes them.
- Empty folders: git doesn't track empty directories, so they're found separately. The tree is drawn first, then a
  background `find` (`FindEmptySkip`: doesn't enter ignored directories, skipping at most `MaxEmptySkip` 500 of them)
  fills them in (`SetEmpty`). When a repository has more than `MaxEmptyScan` (20,000) files, the whole tree isn't searched;
  each level is read as it gets expanded (`lazyEmpty` in `app/emptylazy.go`). Empty folders that turn up during incremental refresh are computed by `emptyIn`.
- Submodules: one row that doesn't expand (`Row.Sub`).
- A folder with a single subfolder and no files is merged with that subfolder into one row (`CompactRows`, `Row.Chain`,
  shown as "internal / ui"), and each segment can be clicked on its own; the `TreeNoCompact` setting turns this off.

### Interaction

#### Filtering, following, sticky rows

- Filtering: typing in the input filters by fuzzy match (`files.Tree.Match`, `fuzzy.Pattern.ScorePath`); hits from all
  repositories are scored together and only the best `FilterCap` (500) are kept (`topHits` in `app/tree.go`, using a heap).
  Filter results are flat: grouped by repository (each group starts with its repository row), sorted by score within the
  group, one file per row, with the matched characters highlighted (`HitRow`).
- Following: switching tabs expands the tree down to that file, selects it and scrolls to it (`FollowCurrent`; the
  `TreeNoFollow` setting turns it off); ⌥⇧⌘E reveals it manually (`RevealCurrent`, command `file.revealCurrent`).
- Sticky rows: while scrolling, the repository row and ancestor folders stick to the top (`stickyStack`, at most
  `maxSticky` 3 levels), and the bottom level casts a shadow downward (`stickyShadow`: in light mode the same as the
  editor's sticky scroll, darker in dark mode), with no border line. Nothing sticks while filtering.
  - The sticky stack reads `TreeSpans` (the rows' positions in the previous frame), and after the list is built shifts
    them to this frame by `sideShift` (the list's offset when last drawn minus this frame's), staying in the same frame as the rows.
  - When the cursor row is covered, the stack yields (that level and the ones below aren't drawn), but only for the row
    the cursor just moved to: `stickyYields` checks `App.TreeYield`; after the cursor moves, that row lands in the list,
    and once the list scrolls again the stack stops yielding (otherwise the whole stack would vanish while scrolling past
    the selected row). Rows entirely above the list (the two extra rows the list builds) don't make it yield.
  - When `TreeRootH` hasn't been measured, it is estimated from `treeRowH()`, and measured once the pinned level is drawn.
- Indent guides are drawn by `drawGuides` (`el.Draw`); the `TreeNoGuides` setting turns them off.

#### Scrolling

`sideList` in `ui/widgets.go` is shared by the file tree, Git panel and search results; the state machine is in `app/sidescroll.go`.

- The list `TrackScroll`s one `app.SideScroll` (`App.SideScrolls`, keyed by `*ListState`).
- Trackpad (precise) scrolling is left to mygo; mouse-wheel scrolling is handled by `sideWheel` (not with ⇧ / ⌃ held):
  one notch (mygo's 40 DIP, `WheelLine`) is one row height, n rows after system acceleration move n rows, up to
  `WheelMaxRows` 6 rows; `WheelGlide` glides to the accumulated target over 160 ms with a cubic ease-out, further notches in
  the same direction keep accumulating, and the opposite direction restarts from the current position.
- The glide writes `ScrollState.Y` itself every frame before the list is built (`SideScroll.Step`) instead of using
  `ScrollState.ScrollTo`: the latter drags the offset back onto the glide path before layout every frame, swallowing any
  `ListState.ScrollTo` / `ScrollIntoView` issued midway. Writing it ourselves means that if `S.Y` isn't what we wrote last
  frame, something else moved it: a difference of more than one row means something else scrolled away, so the glide stops
  and yields; less than one row means the list corrected the offset after measuring new rows, so the whole glide shifts by that amount and continues.
- When gliding up from the bottom, each frame stays at least 1 DIP away from the bottom; otherwise a first frame that moves too little gets pulled back to the bottom by `ListState`.
- The file tree list's `HandleInput` is replaced by `menuPress`, so it calls `sideWheel` first in its own callback.

#### Mouse clicks (issue #46)

- A single click without ⌘ ⇧ ⌃ on an unselected file / folder row is handled by the row itself (`ui.treePress`;
  `HandleInput` takes the press, move and release instead of passing them to the list):
  - The row is selected on the frame of the press (`PressTreeRow`), with no separate "pressed" background (if it were one
    shade darker than the selection, a click would go dark then light, which looks like a flash).
  - Holding and dragging onto other rows selects while dragging (`DragTreePress`, finding rows via `TreeSpans`; it doesn't auto-scroll at the list's edges).
  - The file opens / folder expands only on release (`ReleaseTreePress`); mygo still reports that release as a click on
    the row on the next frame, which `TreeRowClicked` swallows via `eat`. `eat` only covers the frame right after the
    release (`ApplyTreeClick` clears it at the end of every frame), not a time window: however long you hold before releasing, it acts only once (issue #50).
- Double-click: only file rows recognize it (open pinned; the second click on the same row within `treeDoubleClick`
  500 ms). Folder / repository rows don't recognize double-clicks; every single click expands / collapses (like VS Code and
  Zed). The row's own double-click count (`DoubleClicked`, reported on the second press) doesn't count for folders; only the release's `Clicked` does.
- ⌘ / ⇧ clicks, double-clicks and presses on the arrow still go to the list / the arrow; an already selected row is still handled by itself (so it can be dragged). The arrow doesn't switch to a pointing-hand cursor.
- Right-click doesn't change the selection (an existing multi-selection stays); the menu applies only to that row, which
  gets a 1 pt accent-colored outline (`menuRing`: while the menu is open and `App.Menu.Owner` is this row, `menuOnRow` / `menuOwned`).

#### Hover tooltips

- Row tooltips don't use mygo's `Tooltip` (0.6 s, hard-coded in mygo): `app.TreeTip` (`app/treetip.go`) records since when
  the pointer has rested on which row, and only after `TreeTipDelay` (0.5 s) does `ui/treetip.go` draw it in the Overlay,
  attached below the row (`AttachTo`, flipping above when there's no room below).
- Warm start: once one tooltip has shown, moving to another row shows its tooltip immediately; after the pointer leaves all
  rows, or after a press / right-click, it waits again. After clicking a row (`Dismiss`) its tooltip doesn't show again until the pointer leaves.
- Small icons inside the row that have their own tooltips (the change dot, the link icon) take over when hovered. When the
  name is truncated (`cutName` measures text width), the first line of the tooltip is the full name.
- Folder rows' tooltips include size and most recent change (`app/dirtip.go`): it only `Lstat`s files listed in the tree,
  computed in the background on first hover, counting at most `dirInfoCap` (50,000) files per folder.

#### Multi-select and drag and drop

- Multi-select (`FileChosen`): ⌘-click, ⇧-click.
- Dragging within the tree (`app/treedrag.go`; the dragged item is a `TreeDrag`, and `DropTree` decides move vs. copy):
  within the same repository it's a move (`MoveTree`), from another repository it's a copy (`CopyTree`, asks first), and
  holding ⌥ copies (without asking); a selection spanning repositories doesn't move. When moving, folders, multiple items
  or tabs with unsaved changes ask first. Nothing is written to disk until you let go.
- Hovering over a collapsed folder for a moment expands it (`DwellTree`).
- Dragging from Finder onto a folder row copies into it (asks first; items already in this repository are moved, `DropFiles`); dropping anywhere else in the window opens them.
- Copying is implemented by `copyIn` / `copyDir` / `copyNode`: `os.CopyFS` errors out on symlinks, so here links are copied
  as links, regular files and folders keep their permission bits, and pipes and the like aren't copied and report an error.
- Dragging a tree row onto an editor group opens it in that group (`OpenDropped`), and onto a terminal pastes the absolute path; neither writes to disk or asks.
- Drops from other apps (`app/datadrop.go`): onto an editor, inserts the text or opens the file (`DropOnDoc`); onto a terminal, inserts the paths (`DropOnTerm` / `ShellFiles`).
- Repository rows can be dragged to reorder (`RepoDrag`; Move Up / Move Down in the repository menu do the same), landing in
  `MoveRepo`: the repositories from the workspace file have their order written back to the `.code-workspace`'s `folders`
  (`workspace.Reorder`, swapping only those entries and leaving everything else as is; if it can't be written, the change
  is rolled back); those added with "Add Repository" are recorded in the session; when a folder is open as the workspace, only
  memory changes. In the UI, dragging and removing repositories only post requests (`MoveReq` / `RemoveReq`), which
  `pending()` in `ui/view.go` handles together after the list has been drawn, to avoid modifying the list in the middle of drawing a frame.

#### File operations

- Copy / paste / Duplicate (`app/treeclip.go`): on name collisions, `freeName` picks "name copy", "name copy 2" (a file's
  extension stays at the end, e.g. "main copy.go").
- New, rename and Move to Trash are in `app/fileops.go`.
- ⌘Z in the file tree (`app/fileundo.go`): renames / moves are reversed, things created / copied in are moved to the Trash;
  moves to the Trash aren't recorded (put them back with Finder), and there is no redo; `undoStale` refuses to undo if the disk changed since.
- Quick Look: Space (command `tree.quickLook`, `QuickLookChosen`) shows the cursor row (file, folder or repository row;
  with several rows selected, still only the cursor row, since the panel shows one item at a time) in the system Quick
  Look panel; it's also under More in the context menu.
  - It goes through `App.Preview` (by default `mygo.Shell.PreviewItem`, available since mygo v0.9.0; replaced in tests),
    view-only, never writing to disk; calling it again while the panel is open switches the item, and closing it is up to the panel.
  - The panel is a system window, and while it's open it has the keyboard: ↑↓ don't move the file tree's selection; to
    switch, click back into the file tree and press Space again.
  - The file tree supports type-to-select (mygo's type-select): Space is registered as a shortcut, so when typing a name
    that contains a space, the space triggers Quick Look instead of becoming part of what you typed.

### Context menu

The file tree's context menu (like most context menus and button menus in the app) is drawn by Grove itself, with the
state machine in `app/popmenu.go` and the UI in `ui/gmenu.go`; see [Context menus and button menus](menus.md).

### Appearance

#### Font size and row height

Font sizes in the sidebar (file tree, Git panel, search, outline, Open Editors) are unified as constants that follow the
style (`lookSize` in `ui/look.go`; for the default VS Code style and the Grove style, see [Appearance](look.md)):

| | Grove style | VS Code style |
|---|---|---|
| Body text `sideFont()` (file names, repository names, changes, results) | 14 | 13 |
| Repository name in the Git panel's section header `repoFont()` | 14.5 | 13 |
| File tree folder / file rows `treeRowH()` | 24 | 22 |

- Secondary text `sideSub` 12 (branches, counts, folders, group titles, notes) and search-result code lines `sideMono`
  12.5 are the same in both styles; the constants are in `ui/sidebar.go`.
- Sticky levels and Open Editors rows align with `treeRowH()`.
- The Git panel's section header is 34; group titles, stash rows and the worktree group header 26; worktree rows 28; the expanded worktree's summary row 36.
- Overall UI size goes through Zoom (⌘= / ⌘-); there is no separate sidebar font size setting.

#### Colors and icons

- Files are colored by status (modified, added, conflicted, ignored); `TreeNoStatusColors` turns this off.
- File icons are by default the colored squares from `ui/fileicons.go` (by extension); the file tree and Open Editors, at
  the `iconTool` size, use `quietGlyphOf`: no background square, with the color mixed toward the secondary text color by
  `quietMix`, so dozens of them on a screen aren't as heavy. `TreeIconsPlain` switches to line icons.
- Folder icons are colored by name and carry a purpose symbol (`folderKinds`).

UI icons are SVGs in `ui/icons/` (Phosphor 2.1.1, MIT; the file name is the icon name), read by `icons.go` with
`go:embed`; `icon("git-branch")` fetches one. For a new icon, drop the Phosphor Regular SVG into that directory.

- `lang-*.svg` are monochrome language icons (Simple Icons; each logo has its own license, see
  `resources/licenses/simple-icons.txt`; Go, C#, Objective-C, Vue, SQL and proto are drawn in-house).
- Folders are drawn in-house too: `folderKinds` in `fileicons.go` assigns colors and purpose symbols by folder name, and the folder and symbol are composed into an SVG there.
- `TestIconFilesUsed` checks that the directory has no unused files.

#### Open Editors

An "Open Editors" section can be added at the top of the sidebar (`ShowOpenEditors`, off by default, `app/openeditors.go`,
UI in `ui/openeditors.go`): it lists all tabs by group (with a small title before each group when split), at most
`OpenEditorsMax` 9 rows tall, scrolling beyond that; its collapsed state is in the workspace layout
(`Panels.OpenEditorsFolded`). The section keeps no separate list; it is computed every frame by `OpenEditorRows()` (from `Groups()` / `GroupDocs`).
