# Keyboard shortcuts

This file is generated from the command registry (`internal/app/commands.go`) by `go test ./internal/app -run TestShortcutDoc -update`. Do not edit it by hand.

⌘P (Quick Open) and ⇧⌘P (Command Palette) are the same overlay: input starting with `>` runs a command, `>git ` runs a git command in the current repository (⌘K opens the palette with it already typed, even when the editor has focus, because the menu gets ⌘K first), `:line` jumps to a line in the current file, and `name:line` opens a file at that line; ⌥↩ on a file item opens it to the side. Esc closes dialogs and panels.

The same key can mean different commands in different contexts, dispatched by the current tab: ⌥⌘F is the full-file diff in a Diff tab and Replace in a file tab. The editor's built-in ⇧⌘G (Find Previous) shares its key with the Git sidebar: when there is a menu bar the menu gets it first, so it opens the Git sidebar; use ⇧↩ in the find bar for Find Previous. The editor's built-in ⇧⌘H (Replace) likewise yields to the sidebar's search and replace (all repositories); to replace only in the current file use ⌥⌘F.

⌘-click in the terminal: a file path (`path:line[:col]`, relative to the shell's current directory; `~/` and `file://` also work) opens pinned in the editor at that line, a directory inside a repository is revealed in the file tree, and a URL opens in the system browser.

While the keyboard is in a terminal, the keys in the Terminal section below belong to the terminal and override global and editor commands on the same key: ⌘W closes the focused pane (with a single pane it closes the tab, asking first if a command is running), ⌘K clears the screen (instead of opening the Command Palette with `>git `), and ⌘F / ⌘G / ⇧⌘G search the terminal's scrollback. ⌃⇧` (new terminal tab) works wherever the keyboard is.

⌘= / ⌘- / ⌘0 zoom the interface (View menu; applies to all windows, and the status bar shows the zoom when it is not 100%). This also holds in the terminal: its own font-size keys (⌘= ⇧⌘= ⌘- ⌘0) are given up for interface zoom; change the terminal font size on the Terminal page of Settings. In the Settings window ⌘F focuses the search field, and Esc clears it when it has text.

## Global

| Shortcut | Command |
| --- | --- |
| ⌘N | File › New File… |
| ⇧⌘N | File › New Folder… |
| ⌘O | File › Open Folder… |
| ⌥⌘N | File › New Window |
| ⌥⌘O | File › Open Folder in New Window… |
| ⌘P | File › Quick Open… |
| ⇧⌘P | File › Command Palette… |
| ⌘T | File › Go to Symbol in Workspace… |
| ⌘S | File › Save |
| ⌥⌘S | File › Save All |
| ⌘W | File › Close Tab |
| ⇧⌘W | File › Close All Tabs |
| ⌥⌘W | File › Close Saved Tabs |
| ⇧⌘T | File › Reopen Closed Tab |
| ⌘R | File › Refresh Git Status |
| ⌘K | File › Run Git Command… |
| ⌘B | View › Sidebar |
| ⌘J | View › Bottom Panel (Terminal / Output) |
| ⇧⌘J | View › Maximize Bottom Panel |
| ⇧⌘M | View › Problems Panel |
| ⌥⌘B | View › Right Panel (History) |
| ⌃⌘Z | View › Zen Mode |
| ⌘= | View › Zoom In |
| ⌘- | View › Zoom Out |
| ⌘0 | View › Actual Size |
| ⇧⌘E | View › File |
| ⌥⇧⌘E | View › Reveal Current File in File Tree |
| ⇧⌘F | View › Search |
| ⇧⌘H | View › Search and Replace |
| ⇧⌘G | View › Git |
| ⌃- | View › Back |
| ⌃⇧- | View › Forward |
| ⇧⌘[ | View › Previous Tab |
| ⇧⌘] | View › Next Tab |
| ⌃⇥ | View › Previously Used Tab |
| ⌃⇧⇥ | View › Least Recently Used Tab |
| ⌘\ | View › Split Editor Right |
| ⌥⌘\ | View › Split Editor Down |
| ⇧⌘\ | View › Open Another Copy in the Other Group |
| ⌥Z | View › Soft Wrap |
| ⇧⌘V | View › Markdown / SVG Preview |
| ⌥⇧⌘V | View › Preview Markdown to the Side |
| ⌃⌘B | View › Show Blame Gutter |
| ⌃⇧` | Terminal › New Terminal |
| ⌃⇧⌘` | Terminal › New Terminal in Editor Area |
| ⇧⌘B | Terminal › Run Task… |
| ⇧⌘U | Git › Pull |
| ⌥⌘P | Git › Push |
| ⇧⌘R | Git › Fetch All |
| ⌥⇧⌘U | Git › Pull All |
| ⌥⇧⌘B | Git › New Branch… |
| ⌘1 | Repository › Jump to Repository 1 |
| ⌘2 | Repository › Jump to Repository 2 |
| ⌘3 | Repository › Jump to Repository 3 |
| ⌘4 | Repository › Jump to Repository 4 |
| ⌘5 | Repository › Jump to Repository 5 |
| ⌘6 | Repository › Jump to Repository 6 |
| ⌘7 | Repository › Jump to Repository 7 |
| ⌘8 | Repository › Jump to Repository 8 |
| ⌘9 | Repository › Jump to Repository 9 |
| ⇧⌘O | Repository › Add Repository… |
| ⌘, | App › Settings… |

## Editor

| Shortcut | Command |
| --- | --- |
| ⌥⌘← | Previous Tab (Alternate Keys) |
| ⌥⌘→ | Next Tab (Alternate Keys) |
| ⌘F | Find… |
| ⌥⌘F | Replace… |
| ⌘G | Find Next |
| ⌃G | Go to Line… |
| ⌃⇧O | Go to Symbol… |
| ⇧F12 | Find References |
| ⌥⇧F | Format Document |
| ⌥F7 | Next Conflict |
| ⌥⇧F7 | Previous Conflict |
| ⌃⌥↓ | Next Conflict |
| ⌃⌥↑ | Previous Conflict |
| ⌃⌥1 | Conflict: Accept Ours |
| ⌃⌥2 | Conflict: Accept Theirs |
| ⌃⌥3 | Conflict: Accept Both |

## Diff

| Shortcut | Command |
| --- | --- |
| ⌥⌘L | View › Unified Diff |
| ⌥⌘F | View › Full-File Diff |
| ⌥J | View › Next File |
| ⌥K | View › Previous File |
| J | Next Change |
| K | Previous Change |

## File Tree

| Shortcut | Command |
| --- | --- |
| → | Expand Folder / Repository |
| ← | Collapse Folder / Repository, or Go to Parent |
| ↩ | Open File (Pinned) |
| ⌥↩ | Open to the Side |
| Space | Quick Look |
| F2 | Rename… |
| ⌘⌫ | Move to Trash… |
| ⌘C | Copy Files |
| ⌘V | Paste Files |
| ⌘D | Duplicate |
| ⌘Z | Undo Last File Operation |

## Terminal

| Shortcut | Command |
| --- | --- |
| ⌘W | Close Terminal |
| ⌘K | Clear |
| ⌘F | Find in Terminal… |
| ⌘G | Find Next |
| ⇧⌘G | Find Previous |
| ⌘D | Split Right |
| ⇧⌘D | Split Down |
| ⇧⌘[ | Previous Terminal Tab |
| ⇧⌘] | Next Terminal Tab |
| ⌥⌘← | Previous Terminal Tab |
| ⌥⌘→ | Next Terminal Tab |
| ⇧⌘C | Open Terminal Output in a Tab |

## Commit Box

| Shortcut | Command |
| --- | --- |
| ⌘↩ | Commit |

## Search Results

| Shortcut | Command |
| --- | --- |
| ⌘↩ | Replace Selected Match / File |
| ⌫ | Dismiss Selected Match / File |

## Git Sidebar

| Shortcut | Command |
| --- | --- |
| Space | Stage / Unstage Selected Changes |
| ⌘⌫ | Discard Selected Changes… |

## Built into the editor (mygo default keys)

The editor's own (mygo) commands are all in the ⇧⌘P Command Palette, searchable by their display name or their English id (such as `foldAll`), grouped like "Editor · Folding". While the editor has focus you can also press the keys in the table below (the editor handles them itself; they are not in the menu bar). Rows marked "—" are mygo default keys whose first stroke is taken by the menu bar, so they can only be run from the Command Palette: ⌘K is "Run Git Command…" and the menu gets it first, so mygo's ⌘K chords (⌘K ⌘0 fold all, ⌘K ⌘L toggle fold, ⌘K ⌘/ block comment…) do not work in the window; ⇧⌘U (redo after undoing a selection change) is Pull, ⌥⌘L (find in selection) is the unified diff, and ⇧⌘G (Find Previous) is the Git sidebar.

Commands that change text are unavailable in read-only tabs (history versions, big files); jumping to and reverting changes needs a HEAD comparison; Fold All, Fold Level N, Fold Recursively, Select All Occurrences and Split Selection into Lines scan the whole text and are unavailable on files over 1 MB.

| Shortcut | Group | Command |
| --- | --- | --- |
| ⌥⌘[ | Fold | Fold |
| ⌥⌘] | Fold | Expand |
| — | Fold | Toggle Fold |
| — | Fold | Fold All |
| — | Fold | Unfold All |
| — | Fold | Fold Recursively |
| — | Fold | Unfold Recursively |
| — | Fold | Fold Selected Lines |
| — | Fold | Fold Level 1 |
| — | Fold | Fold Level 2 |
| — | Fold | Fold Level 3 |
| — | Fold | Fold Level 4 |
| — | Fold | Fold Level 5 |
| — | Fold | Fold Level 6 |
| — | Fold | Fold Level 7 |
| — | Fold | Fold Imports |
| ⌥⌘↑ | Multiple Cursors and Selection | Add Cursor Above |
| ⌥⌘↓ | Multiple Cursors and Selection | Add Cursor Below |
| ⌘D | Multiple Cursors and Selection | Add Next Occurrence |
| ⌃⌘D | Multiple Cursors and Selection | Add Previous Occurrence |
| ⇧⌘L | Multiple Cursors and Selection | Select All Occurrences |
| — | Multiple Cursors and Selection | Split Selection into Lines |
| ⌘L | Multiple Cursors and Selection | Select Line |
| ⌃⇧→ | Multiple Cursors and Selection | Expand Selection |
| ⌃⇧← | Multiple Cursors and Selection | Shrink Selection |
| ⌘U | Multiple Cursors and Selection | Undo Selection Change |
| — | Multiple Cursors and Selection | Redo Selection Change |
| ⌃M | Multiple Cursors and Selection | Go to Matching Bracket |
| — | Multiple Cursors and Selection | Select to Matching Bracket |
| ⌥↑ | Lines | Move Line Up |
| ⌥↓ | Lines | Move Line Down |
| ⌥⇧↑ | Lines | Copy Line Up |
| ⌥⇧↓ | Lines | Copy Line Down |
| — | Lines | Duplicate Selection |
| ⇧⌘K | Lines | Delete Line |
| ⌃J | Lines | Join Lines |
| ⌃T | Lines | Transpose Characters |
| ⌘] | Lines | Indent Line |
| ⌘[ | Lines | Outdent Line |
| ⌘/ | Lines | Toggle Line Comment |
| — | Lines | Toggle Block Comment |
| — | Lines | Sort Lines Ascending |
| — | Lines | Sort Lines Ascending (Case Insensitive) |
| — | Lines | Reverse Lines |
| — | Lines | Delete Duplicate Lines |
| — | Lines | Trim Trailing Whitespace |
| — | Case | Transform to Uppercase |
| — | Case | Transform to Lowercase |
| — | Case | Transform to Title Case |
| — | Case | Transform to snake_case |
| — | Case | Transform to kebab-case |
| — | Case | Transform to camelCase |
| — | Case | Transform to PascalCase |
| — | Find | Find Previous |
| ⌘E | Find | Use Selection for Find |
| — | Find | Find in Selection |
| F12 | Code | Go to Definition |
| — | Code | Show Hover |
| ⌃Space | Code | Trigger Suggest |
| ⌃L | Other | Scroll Cursor to Center |
| ⌘F8 | Other | Next Change |
| ⇧⌘F8 | Other | Previous Change |
| ⌥⌘Z | Other | Discard Hunk |

## Customizing

The keys above are the defaults and can be changed: in Settings → Keyboard Shortcuts select a command, click "Change…" and press the new key (⌫ removes the shortcut, Esc cancels), or edit `<state dir>/keybindings.json` directly (normally `~/Library/Application Support/Grove/keybindings.json`; on the shortcuts page, ⋯ at the top right → Open keybindings.json). The file is an array, one entry per command:

```json
[
  {"command": "palette.commands", "key": "cmd+k"},
  {"command": "git.command", "key": ""}
]
```

- `command` is the command id: hover a command name on the shortcuts page to see it, and that page's search field also matches ids. An empty `key` removes the command's shortcut; deleting the entry (or clicking "Reset" on the shortcuts page) restores the default.
- `key` is modifiers plus a key joined by `+`, case-insensitive: modifiers `cmd` (`command`, `meta`, `⌘`), `ctrl` (`control`, `⌃`), `alt` (`option`, `opt`, `⌥`), `shift` (`⇧`); symbols written at the front need no `+` (`⇧⌘P`). Keys are `a`–`z`, `0`–`9`, `f1`–`f12`, `enter`, `escape` (`esc`), `backspace`, `tab`, `space`, `delete`, `insert`, `home`, `end`, `pageup`, `pagedown`, `left`, `right`, `up`, `down` and `` - = , . / ; ' [ ] \ ` `` (or `minus`, `equal`, `comma`, `period`, `slash`, `semicolon`, `quote`, `bracketleft`, `bracketright`, `backslash`, `backquote`).
- Conflicts are counted per receiver: the window (global, Diff tab, editor), the terminal, the file tree, the commit box and the search results are each one receiver. Within one receiver the same key conflicts only if the contexts match or one of them is global (⌥⌘F being one command in Diff tabs and another in the editor is fine): the changed binding wins, the displaced command loses its key, and its row on the shortcuts page says what took it; if two entries in the file use the same key, the later one wins.
- The editor's built-in commands (`editor.*`, the previous table) can be rebound too; once changed, mygo's default key stops working.
- There are no chords (two-stroke keys like ⌘K ⌘0) and no `when` conditions.
- When the shortcuts page writes this file it rewrites it whole, one entry per line, so comments in the file are lost; entries it cannot understand are kept as they are and listed at the top of the shortcuts page, while the rest take effect. If the whole file cannot be read (it is not a JSON array) all default keys are used and the shortcuts page does not write to it, so nothing is overwritten; after you fix it in the editor and save, it takes effect immediately.
