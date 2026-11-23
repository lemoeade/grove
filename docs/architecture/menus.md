# Context menus and button menus

[← Architecture overview](../architecture.md) · [File tree](files.md)

Most context menus and button menus are drawn by Grove itself (the file tree since issue #48, most other places since #58). System
menus aren't used: a system menu popped up by mygo has its top-left corner pinned to the click point, and near the bottom of the
window it doesn't flip upward but collapses into a menu with scroll arrows (see [mygo-pitfalls.md](../mygo-pitfalls.md)). mygo's
`PopoverMenu` isn't used either: you can't hold the right button, drag onto an item and release, submenus have no delay or safety
triangle, disabled items can't explain why, and the click outside falls through to whatever is underneath.

## Where they are used

- File tree: `treeMenu` (empty space) / `rowMenu` (rows) in `ui/sidebar.go`, repository rows, rows pinned at the top, and each
  segment of a compact folder, installed by `menuPress`.
- Outside the file tree, `ctxMenu` / `ctxMenuRow` (right-click) and `menuButton` / `openMenuBelow` (buttons) are used:
  - Git sidebar: the section header context menu and branch menu, change rows / file rows in a stash / untracked folder rows (the
    owner is `app.ChangeKey`, so the staged and unstaged entries of the same file are two separate owners; multiple selection is
    still computed by `MenuChanges`), stashes, worktree rows and worktree actions, right-clicking the push button, the ⋯ Git
    actions menu, the git queue, and "More" on commit cards.
  - Title bar: the branch pill and the branch on section headers, the workspace button.
  - Editor and history: editor tabs, Open Editors, commit / change / repository rows in history, the title row and compare row of
    the history panel, local history, blame, the action of each rebase step, and "Change Encoding…" on the read-only bar.
  - Other: terminal tabs, terminal profiles from the bottom panel's "+", search results and summary rows, the context line count of
    the search editor, the language / indentation / line ending / encoding and zoom in the status bar, and right-clicking in the
    clone dialog.
- Widgets that receive their own events (the ged Diff) pass right-button press, drag and release to `menuPointer`.
- Still drawn by mygo (`ContextMenu` / `Menu`):
  - "More" in the settings window (the settings and shortcuts pages): the settings window is a separate window with no
    `gmenuView`, and `App.Menu` exists only for the main window.
  - The title bar breadcrumbs (`dirMenu`, two levels of submenus, at most `crumbMenuMax` 80 items per level): Grove's menu builds
    submenus along with everything else in `menuCollect` every frame, which in the worst case is hundreds of thousands of items;
    submenus would first have to be built on demand.
  - The editor's context menu (`d.St.ContextMenu`, opened by mygo's editor itself, which Grove can't take over).
- `root` calls `c.SetMenuPresentation(wui.PopoverMenus)` every frame: the remaining mygo menus above, and the edit menus of text
  fields and selectable text, are all drawn by mygo inside the window (in the style of mygo's `PopoverMenu`, not Grove's menus); the
  menu bar and Dock menu are still the system's.

## Where a menu opens

- Right-clicking a row opens the menu with `PopMenu.OpenRow`: at the pointer's x coordinate, below that row; if it doesn't fit
  below, it flips above the row so as not to cover it. For targets taller than `MenuRowMax` (64) (empty space in a list), it opens
  at the pointer. It scrolls only when the window can't fit it either way.
- Button menus (`openMenuBelow`) also open with `OpenRow`, treating the button as "the row": if they don't fit below, they flip
  above the button without covering it (status bar).
- Right-clicking another registered place while a menu is open closes the old menu and opens a new one there with `OpenRow` (see
  [Feeding events and drawing](#feeding-events-and-drawing-uigmenugo) below).

## When the chosen item runs

- The chosen item runs at the start of the next frame, before the tree is built (`gmenuRunChoice` in `root`, much like a system
  menu item running when the next frame builds that element). Actions that replace list state, such as switching workspaces or
  re-running a search, would leave this frame's layout holding the old list if they ran after the tree was built (a nil pointer in
  mygo's `layoutList`).
- Where a menu-building function has a `do` that means "wait until the list is built" (search results), it is passed `menuDo`
  (run directly); actions handed to a `do` captured in the frame the menu opened would never be run.
- Menu-building functions rerun every frame (`PopMenu.Refresh`), and the thing a menu points at may disappear while it's open:
  functions that index slices by the index captured at open time check first (`searchRowLive` for search results, the rebase step,
  the compare section, clone), and add nothing if it's gone; when there are no items at all, `Refresh` closes the menu and the
  chosen item doesn't run.

## The state machine: `app.PopMenu`

`app/popmenu.go`, one `App.Menu` per window, never touches the UI.

- Items are collected from the menu-building function through `app.MenuSink` (recollected every frame, so enabled state follows
  the current state). After an item is chosen, the menu-building function runs once more, and the chosen item's `Chosen()` reports
  true (the same as with mygo's menus).
- Position, hit testing, submenu delay (`MenuSubDelay` 0.2 s), the safety triangle, tooltips on disabled items (`MenuTipDelay`
  0.5 s), press-drag-release (the pointer must move more than 4 pt to count as moved; releasing without moving keeps the menu
  open), not closing on zero scroll (`dy` below 0.01 doesn't count as scrolling), and the keyboard are all handled there.
- `popmenu_test.go` tests it by feeding events directly (the Tester can't hold down the right button).
- Menus with many items get a filter field (`PopMenu.Filterable`, with `filterMenuButton` as the UI; currently only the status
  bar's language menu):
  - The field is pinned to the top of the panel (`MenuLevel.Head`, drawn by `gmenuFilter`; it doesn't take the keyboard, the
    receiving layer passes typed text to `PopMenu.Key`), and the panel shows at most `MenuFilterRows` (12) items.
  - Typed text filters level 0 with `internal/fuzzy` (`shown`: better matches first; no matches shows a line of gray text); the
    first item is highlighted and Return chooses it. Backspace deletes a character; Esc clears the text first, then closes.
  - The panel's position and width are computed from all items; when fewer items match, it shrinks from the bottom and the field
    stays put.
- A menu that doesn't fit in the window and has to scroll scrolls the checked item into view as soon as it opens
  (`revealChecked`), and follows the keyboard when it moves to an item outside the panel (`reveal`); scrolling the wheel over a menu
  that doesn't need to scroll no longer closes it.
- Menu items are pressable in the accessibility tree (`Clicked()` on the `gmenuItem` row): when assistive technology presses one it
  goes through `PopMenu.Press` and is chosen just like pressing Return (items with a submenu open the submenu). Mouse presses and
  releases don't take this path; they still go through the receiving layer to `PopMenu`.

## Feeding events and drawing: `ui/gmenu.go`

- `menuPress` attaches `HandleInput` to elements in the tree (outside the file tree, `ctxMenu` is used; button menus open below
  the button via `openMenuBelow`): a right-button press (⌃-click counts as a right click) opens the menu, and movement while held
  and the release (which belong to the element that was pressed) are forwarded to the menu.
- `gmenuRunChoice` (at the start of `root`) runs the chosen item; `gmenuView` (in `root`, before `pending`) draws the panel of each
  level and, while a menu is open, lays a full-window receiving layer in the Overlay: clicking outside closes the menu and swallows
  the click; scrolling, Esc and losing focus close it; and it also takes the keyboard.
- The receiving layer covers the whole window, so events can't reach the elements underneath. That's why `menuPress` registers
  every element with a menu (its position, the stacking order computed by `menuZ`, and its menu-building function) with
  `PopMenu.AddTarget` every frame. Right-clicking elsewhere while a menu is open (button 1 of `Down`) finds the target among the
  registered ones (empty space in the file tree list doubles as the clip region; outside the file tree, Z starts at `MenuZFree` and
  isn't clipped), closes the old menu and opens the new one; the left button and other buttons only close and are swallowed.
- Opening a menu from the keyboard (⇧F10 / the Menu key):
  - When the file tree has the keyboard, the list's `HandleInput` handles it (`openTreeMenuByKey`, using the position of the
    cursor row from `TreeSpans`); `PopMenu.OpenByKey` opens the menu with the first item highlighted. If nothing is chosen and it
    closes, `App.MenuByKey` returns the keyboard to the file tree.
  - Outside the file tree: when a `ctxMenu` element has the keyboard, or a `ctxMenuRow` row is the selected row of a list that has
    the keyboard, it is registered as `App.MenuKeyAt` every frame; `gmenuView` accepts ⇧F10 / the Menu key (`c.Shortcut`) only
    when that is set, and calls `OpenByKey` just below the element's bottom-left corner.
- The receiving layer doesn't take the keyboard on its first frame and calls `Focus()` only on the second: mygo records the element
  that had the keyboard in the frame the Overlay appears (after the tree is built) and gives it back when the layer disappears
  (`restoreFocus`), so after the menu closes (chosen, Esc, clicked outside) the keyboard returns to the element it was on before.
- Look: font size 13, item height 24, panel corner radius 8, padding 4, item corner radius 5, minimum width 180; space for check
  marks is reserved only when this level has checkable items (`MenuLevel.TextL`), otherwise the text gets a left inset of 10;
  the highlighted item gets a faint fill of the text color (8% in light, 10% in dark) and keeps its text color (`gmenuHot`). The
  panel is `RoleMenu` and the items are `RoleMenuItem`, which the test helpers `shownMenu` / `chooseMenu` / `closeMenu` still
  recognize.

## Frosted glass

- Menu panels are frosted (`gmenuPanel`): `Material(frostMaterial{gmenuTint(t)})` (`ui/frost.go`) blurs whatever the window has
  drawn underneath (`glass.Blur` from mygo's `plugins/glass`, in the same module as mygo, with blur radius `frostBlur` 12), then lays
  a semi-transparent tint over it within the rounded corners.
- `gmenuTint`: in the Grove style it is the color of `gmenuBg`, 44% opaque in light and 60% in dark; in the VS Code style it is
  `vsOverlayBg` at `frostAlpha()`. The two Grove-style opacities are as transparent as it can go: with the theme's own background
  underneath, text (on the panel and on the highlighted item) reaches 4.5:1, and with the worst possible color underneath (solid
  black under a light theme, solid white under a dark one) it still reaches 3:1; any more transparent and it wouldn't
  (`TestGroveMenuTintContrast`).
- When the system's Reduce Transparency / Increase Contrast is on (`gmenuFrosted`), there is no blur and the opaque background
  color is used directly (`TestGroveMenuFrosted`).
- Other overlays are frosted too, following VS Code's frosted glass: `frostFill(c, el, bg)` gives `el` a `frostMaterial` when
  `canFrost` holds (Reduce Transparency and Increase Contrast are off and the opacity isn't 100): `glass.Blur{Radius: frostBlur}`
  blurs what's underneath, then `FillRadii` lays `bg` at `frostAlpha()` on top, all in one Material with no extra child element (a
  child filling the whole area would take up `Gap`). When it can't, it's a solid `bg`.
  - Used for the command palette (`paletteBg`), `dialogFrame` (`dialogBg`), the editor's hover tip `edTipCard` and completion list
    `edCompletionList` (`widgetBg`).
  - In the VS Code style the background is VS Code 2026's menu / quickInput / editorWidget background (`vsOverlayBg`; `vsQuickBg`
    and `vsWidgetBg` are both it, #FAFAFD / #202122).
  - `TestFrostFill` measures on a black-and-white boundary whether it blurred and whether it fell back to a solid color.
- How much tint is applied is the "Overlay opacity" setting (`Settings.Frost`, json `frostOpacity`, 50–100; 0 means `DefaultFrost`,
  80, matching VS Code's default; the earlier 65 let blobs of colored, blurred code show through). The color functions can't reach
  the App, so `setLook` stores it in the package-level `frostPct` every frame; at 100, `canFrost` reports false and a solid color is
  used. At the default opacity, VS Code text colors still reach 2.5:1 on the worst background (`TestFrostContrast`).

## Menu-building functions: `ui/menus.go`

- They all take the `menu` interface (`app.MenuSink`); system menus (`ContextMenu` / `Menu`) are adapted via `nativeMenu(m)`, so
  both kinds share the same code.
- Disabled items explain why with `whyItem` / `whySub`: system menus put the reason in parentheses after the label (`withWhy`),
  Grove-drawn ones use a tooltip.
- Destructive items (Move to Trash, discard, delete, force push, abort, hard reset, Remove from List) chain `Danger()`: Grove-drawn
  menus use `t.Danger`, including for the hover fill; system menus can't draw it, so it does nothing there.
- File and folder menus have at most 14 top-level items (`fileMenu`), with less common ones under "More" (`fileMoreItems`); group
  order: open and view → stage → Finder and paths → copy and paste → rename, Move to Trash → More. `changeMenu` follows the same
  order.
