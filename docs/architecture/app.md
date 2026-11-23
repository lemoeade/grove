# App, commands and windows

[← Architecture overview](../architecture.md)

The per-window `App` state machine, the command registry and shortcuts, multiple windows, startup and quitting, the settings and
About windows, versions and auto-update.

## App and the command registry

### App

`app.App` holds all the state of one window. The UI reads that state every frame and changes it only through commands
(`a.Exec(id)`) or `App` methods. Background goroutines never access `App` directly; they hand their results to
`a.Update(func())` and the main thread applies them.

#### Focus requests

To give the keyboard to a particular widget, `app` doesn't touch the widget (it has no access to `Focus(c)`). Instead it sets a
one-shot request field. These are always named `FocusXxx`, with a comment along the lines of "give focus to … on the next frame":

- `App.FocusDoc` / `FocusTerm` (`FocusTab` names which tab) / `FocusTree` / `FocusHist`
- `Repo.FocusCommit`
- `SearchState.Focus` / `FocusRepl`
- `TermFind.Focus`

The UI function that draws the widget sees the request, calls `Focus(c)` and clears it. Each request lives next to the thing it
targets (which repository's commit box, which pane's find bar), so they are separate fields rather than one global "who gets
focus". If two are set in the same frame, the widget drawn later gets the keyboard.

#### Switching workspaces

Switching workspaces (`switchTo`, `app/workspaces.go`) rewrites all of `*a`. So a background task saves `up := a.Update` before it
starts, applies its result through `up`, and checks that the original repository still exists (compare repository pointers; for
documents compare `docs.Doc.Gen`: `switchTo` does `Gen++` on every open document).

Fields that belong to the window and are not reset with the workspace (the ones `switchTo` in `workspaces.go` copies back):

- The window itself: `Win`, `Update`, `HasMenu`, `StateDir`, `Windows`, `Slot`, `ownStore`, `LastBar`
- Renderer: `Rend`, `RenderOpen`
- Settings / About windows and menu callbacks: `SettingsWin`, `OnSettings`, `OpenSettings`, `OpenAbout`, `OnRecent`, `OnClearRecent`
- System hooks that tests replace: `Trash`, `OpenURL`, `ShowPath`, `InFinder`, `OpenDialog`, `SaveDialog`, `Preview`
- Logs: `LogDir`, `LastCrash`; the output panel: `Output`, `OutList`, `BottomTab`
- The command palette history `Palette.History`, and alerts not yet handled (`Alert*`)

### Command registry

Every action is defined exactly once in `app/commands.go` (`Command`):

- The id (e.g. `view.sidebar`), the name `Title`, the menu `Group`, and `Sep` for a separator above it
- The shortcut `Mods` / `Key` and the focus context it applies in, `Context`: global / editor / Diff / file tree / terminal /
  commit box / search results / Git sidebar
- `Enabled` (preconditions, such as whether there is a current repository), `Why` (the reason a Git operation can't run right now,
  `app/gitwhy.go`; a non-empty value also counts as disabled, the command palette shows it after the grayed-out command, and
  clicking it from the menu bar shows it as a message), `Run`, `Checked`
- `Section`: the second half of the command palette grouping (the "Fold" in "Editor · Fold"); `Hint`: the key shown in the command
  palette when `Key` is 0 (a default key the editor handles itself; displayed only, not registered)
- `Local`: has a `Run`, but the key isn't registered on the window; a UI element handles it itself (the editor's ⇧⌥F, the editor's
  built-in commands), and `Run` is there for the command palette
- `Custom` / `Shadowed`: used by custom shortcuts, see below

Generated from the registry:

- **The menu bar** (`BuildMenu` in `app/menu.go`). `SyncMenu` compares `menuSignature` every frame and rebuilds only when it
  changes; the signature covers checked states, the repository list, recent workspaces, terminal profiles, the UI language,
  `keyRev` and `KeyRecording`. The menu bar belongs to the whole app (`mygo.App.SetMenu`), and only the frontmost window's `App`
  syncs it; a menu item's Click finds the `App` from the window passed in (`target(win)`). Menu items are never grayed out;
  clicking a command that can't run does nothing.
- **Window shortcuts**. With a menu bar, the menu receives key presses first, so menu shortcuts take precedence over the editor's
  shortcuts for the same key; without a menu bar (tests, `WORKBENCH_PLAIN`), `BindShortcuts` registers the same set on the window
  root. The same key can mean different commands in different contexts (⌥⌘F is Full-File Diff in a Diff and Replace in a file
  tab; ⌘D splits in the terminal and duplicates in the file tree). Both key presses and menu items go through `RunKey(mods, key)`,
  which picks the first command, in registration order, that can currently run. `RunKey` only dispatches commands that have a
  `Run`: the global / Diff / editor ones without `Local` (`windowKey`), plus those for the terminal and file tree.
- **Tooltips**: `Tip(text, id)` appends the key in parentheses at the end of a sentence; `KeyText` / `KeyHint` / `KeyList` give
  keys for use mid-sentence. They're computed every frame, so they follow shortcut changes immediately.
- **`docs/shortcuts.md`** (generated by `TestShortcutDoc -update`).
- **The conflict check** `TestShortcutConflicts`: ids are unique, and within one dispatch domain a key belongs to at most one command.
- **The command palette** (`>`).

#### The editor's built-in commands

The editor's built-in actions (copy line, multiple cursors, folding, ...) are registered as commands too, with ids of the form
`editor.<mygo's English name>`, generated by `editorCommands` from `editorTable`. They leave `Key` at 0 and are `Local`: they
don't go into the menu bar or window shortcuts, and the editor handles their keys itself. The registry only uses them to generate
the command palette and the shortcut docs. `Run` is wrapped in `inEditorChecked`, which re-checks `Enabled` before running
(mygo's commands ignore read-only) and hands focus back to the editor afterwards. Commands that scan the whole text (Fold All and
the like) are disabled when the file exceeds `MaxWholeTextBytes` (1 MB).

#### Key-only commands

Commands without a `Run` only register a key; they go into neither the menu bar nor the command palette (empty `Group`). The UI
element in question handles the key by id itself, because the state it needs exists only in that frame's UI (the conflict block
under the cursor, the text in the commit box, the selected row of a list). Currently these are:

- `merge.*` for merge conflicts
- `commit` for the commit box
- `search.replaceOne` / `search.dismiss` for search results
- `tree.*` for the file tree list (except `tree.duplicate`, see below)

These file tree keys must not get a `Run`: `RunKey` dispatches file tree commands that have a `Run` on the window, and the key
would no longer reach the list. `tree.duplicate` (⌘D) has a `Run` on purpose: in the menu bar ⌘D is the terminal's split, so once
the menu receives the key press `RunKey` dispatches it, and when the keyboard is in the file tree it lands on `tree.duplicate`.
`tree.undoKey` (⌘Z) only goes into the docs; the key is handled by `treeCommands`.

The Git sidebar's `changes.toggleStage` / `changes.discard` have a `Run` (for the command palette), but when the list has focus
the list handles the keys itself (`ui/gitbar.go`).

`TestShortcutConflicts` checks that everything with a `Group` has a `Run`; `TestRunlessCommandsHandled` checks that everything
without a `Run` has a key, isn't global, and has its id handled somewhere in `internal/ui`.

#### Plain folders

When the current repository is a plain folder (not a Git repository), all `git.*` commands are disabled, except those that don't
look at the current repository (refresh, cancel, Fetch All / Pull All / stop) and "Run Git Command…" (which can `git init`).

### Custom shortcuts

`app/keybind.go`. Keys the user has changed are stored in `keybindings.json` in the state folder:

- Only keys that differ from the defaults are recorded; an empty `key` removes the default key.
- Key notation is read by `ParseKeys` (both `cmd+shift+p` and `⌘⇧P` work). The file is parsed as JSON first, and if that fails,
  as JSONC with comments and trailing commas stripped (`workspace.StripJSONC`).
- Every `applyKeybindings` recomputes from the default keys (`keyDefaults`), so "reset" just means deleting that entry from the
  file. A command's `Mods` / `Key` are its effective keys, and menus, shortcuts and tooltips keep reading them from the registry.
  The command table is shared by the whole process.

**Conflicts** are computed per dispatch domain (`keyDomain` / `clash`): keys of the editor and the Diff tab both belong to the
global domain, and two commands conflict only if they are in the same domain, in the same context (or one of them is global), and
on the same key. On a conflict the user's configuration wins: the original command loses its key and `Shadowed` records what
displaced it. When the key of an editor built-in command (`editor.*`) is changed, `editorKeys` generates an `editor.Binding` for
the editor (`EditorBindings` / `DiffBindings` / `DiffEditBindings`) and removes mygo's original default key. Chords and `when`
conditions are not supported.

**Where keys are changed**:

- The "Shortcuts" page of the settings window. While recording a key, `SetKeyRecording` keeps the menu from taking keys.
- Editing `keybindings.json` directly in the editor: it is reread on save (`keybindingsSaved`). Entries that can't be parsed are
  reported once (`keyToasted`) and listed on the settings page (`KeyErrors`); when the whole file can't be read (`keyBroken`), the
  settings page stops writing to it so as not to overwrite the original.
- The file is written atomically (`writeFileAtomic`).

## Multiple windows and lifecycle

### Multiple windows

`app/windows.go`. Each window has one workspace and one `App`; a workspace is open in at most one window, and opening it again
brings that window to the front.

- `Windows` is the registry of open workbench windows (the settings and About windows don't count).
- New windows are 1440×880, at least 960×600, offset 24 pt from the previous one.
- Each window has a slot, `Slot`, which determines its `StateKey` (1 is `main`, 2 is `main-2`, ...; new windows reuse closed
  slots first). mygo remembers window positions by `StateKey`.
- All windows share one `workspace.Store`; when something tied to the process (settings, recent workspaces) changes, `syncShared`
  propagates it to the other windows.
- When the same repository is open in two windows, each has its own watcher and terminals.

#### Paths handed in from outside

Command-line arguments, paths forwarded by a second instance, `open -a Grove`, and items dropped on the Dock icon (`OnOpenFile`
can't tell these apart; those collected during a cold start are dispatched together in `Restored`) all go to `Route`:

- Workspace files: open a new window (or bring it to the front if already open).
- Folders go to `routeDir`: if it's already some window's workspace, bring that window forward; if it is (or its git root is) a
  repository in some open window, bring that window forward and jump to that repository; if the frontmost window has an empty
  workspace, open it there; otherwise open one new window per folder.
- Files: handed to the frontmost window's `OpenArgs`.

Items dropped into a window (`OnFileDrop`) skip `Route` and go straight to that window's `OpenArgs`. How `OpenArg` treats folders:
a repository already in the workspace jumps to its section; one that isn't asks whether to add it (recorded only in the session,
the `.code-workspace` is not changed); in a window with an empty workspace, the folder is opened as the workspace.

### Startup

`main` in `cmd/workbench/main.go`, in order:

1. Record `app.Launched` (for startup timing). If the first argument is `termd` / `askpass`, branch off as the terminal background
   process / credential helper and go no further.
2. Version: the version number compiled in by `mygo build` is written to `version.App`.
3. Single instance: `mygo.App.RequestSingleInstanceLock`. If it can't get the lock, it hands its arguments to the instance already
   running and exits. Instances with `WORKBENCH_STATE` set don't take the lock by default (tests and running several copies); set
   `WORKBENCH_SINGLE=1` to make them take it.
4. `migrateDirs` in `cmd/workbench/migrate.go`: moves the state and log folders from the old name (Workbench) before the first call
   to `mygo.App.Path`.
5. Logging (`Setup` in `internal/crash`): the log folder is `WORKBENCH_LOGS`, or mygo's `PathLogs` (`~/Library/Logs/Grove`) if
   unset.
   - The standard library `log` writes to `workbench.log`; at startup, if it's over 1 MB it is renamed to `workbench.old.log` and a
     fresh one is started.
   - `debug.SetCrashOutput` points to `crash-pending.log`; on the next startup, if it isn't empty it is renamed to
     `crash-<date>-<time>.log` and kept (at most 10), and `LastCrash` remembers it so it can be opened from the About window.
   - The terminal background process logs to `termd.log` in the same folder.
6. State folder: `WORKBENCH_STATE`, or mygo's user data folder if unset. `SetWindowStateStore(app.WindowStates(dir))` reads
   `window-state.json`; it must be set before `mygo.App.Run`.
7. `render.Register` registers the protocol for the Mermaid / math rendering page.
8. `OnOpenFile` / `OnSecondInstance` receive paths handed in from outside: `deliver` deduplicates them (the same paths within 2
   seconds count once), holds them while the window isn't ready yet (or is still being restored), and otherwise hands them to
   `Route`.
9. `setupUpdates` (see [Auto-update](#auto-update)), then `SetLanguages(i18n.Locale())`.
10. `WhenReady`: load fonts, `NewWindows`, the settings callbacks (theme, material, language, ...), `Windows.Restore` to reopen the
    windows that were open at the last quit (`windows`), then `Greet`.

`Restore` opens windows one at a time from a queue: each window allocates several window-sized frame buffers when it starts, and
opening them all at once would stack up a memory peak. So the next window opens `RestoreGap` (2.5 seconds, adjustable with
`WORKBENCH_RESTORE_GAP`) after the previous one has drawn its first frame (`OnReadyToShow` → `Ready`). There is no queue when only
one window is opened.

`Greet` (`app/openarg.go`) shows a one-line message in the status bar: if the app crashed last time, where the crash log is; on
first use, "Welcome to Grove …"; if the version changed (the last version is recorded in `appVersion`), "Updated to Grove …".

#### Startup time and memory

Startup time is logged (`app/ready.go`): once all repositories of the first window have been read, there is one `就绪` ("ready")
line per repository (with separate timings for status, tree, empty folders and history), and a final `全部就绪` ("all ready") line
measured from process start (`Launched`).

`WORKBENCH_ALLOPEN=<search term>` (`app/allopen.go`) is the "all open" mode used for memory measurements: after the repositories
are read it opens two Diffs, runs one search, starts the current repository's terminal, then writes a log line.

After large releases of memory, such as a search or closing a window, `debug.FreeOSMemory()` is called once to return memory to
the system.

### Closing windows and quitting

#### Closing a window

`OnClose` → `CanClose`: if there are unsaved files or a git write command is running, it asks first (`RequestLeave`). Programs
still running in terminals don't trigger a question when closing a window or quitting; the terminals are simply closed. Only
switching workspaces asks about them (see the workspace section of [state.md](state.md)).

Once that's settled the window closes, and `OnClosed` → `Closed`:

1. `freeze` records the open windows (see [Quitting](#quitting) below).
2. Record terminal folders (`NoteTermDirs`: read with `lsof` for shells that didn't report OSC 7) and terminal scrollback.
3. Save the session, close the window, and shut down this window's renderer.
4. `debug.FreeOSMemory()`.

#### Quitting

⌘Q, the Dock, shutdown: mygo first sends `OnBeforeQuit`, and if nothing blocks it, closes the windows one by one; if a window
blocks partway through, the windows already closed don't come back. So every quit confirmation is intercepted and handled in
`OnBeforeQuit` (`RequestQuit`): ask window by window, closing each one before asking the next, and quit once more when all are
closed. Canceling partway stops there, and windows already closed are not reopened.

- `freeze` records the open windows when quitting or closing the last window (including, during a restore, those not opened
  yet); as they are then closed one by one, `windows` is saved according to that record.
- `quitting` is the window currently being asked; `ending` means asking is done and the app is quitting, so nothing is
  intercepted any more, and closing the last window doesn't trigger another quit.

### Settings and About windows

Both are opened in `cmd/workbench/main.go`:

- The settings window is 640×480, with `StateKey` `settings`.
- The About window is 460×640, with `StateKey` `about`.

There is only one of each, belonging to the workbench window that opened it; when that workbench window closes, it closes too
(`closeAux`). The settings window shares the `App` of its workbench window (`SettingsWin` is kept across workspace switches), so
changes in the settings window also go through that `App`'s methods. UI zoom, accent color and style are applied to both windows
every frame as well.

### Versions

`internal/version`:

- `App` is the version from `mygo.json`, compiled in via ldflags by `mygo build`; it is `dev` under `go run` and in tests.
- The first log line is `version.String()` (app version, mygo version, Go version, OS).
- `TestVersions` checks that the version in `mygo.json` has a `## v<version> ` section in the CHANGELOG, and that the mygo version
  in go.mod matches `MygoPinned`. `TestAppName` checks the name and bundle id in `mygo.json`.

### Auto-update

mygo's updater plugin (`plugins/updater/native`, with a native update window) is installed by `setupUpdates` in
`cmd/workbench/main.go` (before `SetLanguages`, with the icon from `appicon.Draw(256)`). The update feed and public key are under
`updates` in `mygo.json` (GitHub Releases of `lemoeade/grove`, Ed25519).

The UI doesn't reference the plugin directly: `app.Updates` in `internal/app/updates.go` (`UpdateHooks`: whether updating is
possible, check, the two switches, last check, the version found) is filled in by main, and is nil in tests and under `go run`.

- "Check for Updates…" in the menu / command palette is the command `app.checkUpdates` (grayed out in the command palette when
  `Updates` is nil; the menu bar item is never grayed out but does nothing when clicked), which opens the plugin's update window.
- The "Update" page of the settings window has two switches and a "Version" row; when the plugin's state changes
  (`updater.OnChange`), the settings window is redrawn.
- The switches are stored in the plugin's own `updater.json` (in the user data folder), not in state.json.
- Instances with `WORKBENCH_STATE` set turn off background checks (`DisableAutomaticChecks`), so no update window pops up and grabs
  the foreground during memory measurements and smoke tests.

For releasing, signing and keys, see [release.md](../release.md).
