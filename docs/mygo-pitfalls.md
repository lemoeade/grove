# Pitfalls

This document records problems we ran into while building Grove and the conventions we settled on. Most are about mygo's behavior
(as of the version pinned in go.mod); some concern git, macOS and the Go standard library. Each entry first describes the symptom or
convention, then how we handle it. After a mygo upgrade some entries may no longer apply, so confirm on the new version before changing code.

Add new problems to the matching section. If a similar entry already exists, update it instead of adding a duplicate.

## Layout and drawing

- `Grow(1)` has a flex basis of 0: put `Spacer().Grow(1)` next to text that also has Grow and the two split the space evenly, so the text gets cut off with an ellipsis. When the text should fill the space, give Grow to the text only.
- `TextArea` needs `.MinHeight(0)`, otherwise it stretches the panel.
- `PassThrough()` applies only to the element itself, not to its children: when a tip you draw yourself in an Overlay covers other elements, the `Text` inside it needs `PassThrough()` too,
  otherwise a click on the row under the tip lands on the text (the file tree's row tip covers the next row, and ⌘-clicking that row failed to add it to the selection; see `TestTreeRowTip`). mygo's own `Tooltip` dismisses itself on press, so it never hits this.
- `wui.SplitPercent` takes 0–100; `wui.Split` / `SplitVertical` take DIPs.
- `Row` centers its children vertically by default. To make children fill the row height (dividers, a whole-row click area), add `.AlignItems(wui.Stretch)` to the Row.
- A child of a Row that uses `Fill()` without a width overflows the row, pushing elements out of the window where they can't be clicked. For fixed-width things like the sidebar use `Width(w).Shrink(0)`.
- Don't use `Fill()` in a Column that has other children: it takes 100% of the parent's width and height, so together with the other children it exceeds the available height and the bottom content is pushed under the status bar (the Git sidebar's commit box
  and the Files page's filter box were both hidden this way). Use `Grow(1).MinWidth(0).MinHeight(0)`; `TestSidebarFits` checks this.
- Children of a scroll container shrink by default. To make one taller than the container (for example to push the horizontal scrollbar into a clipped-off area), use `Shrink(0)`.
- Several shrinkable pieces on one line (such as breadcrumbs):
  - Left alone, all the pieces shrink to ellipses together. Text with `Ellipsis` shows "…" as soon as it is squeezed by a fraction of a DIP.
  - Making them all `Shrink(0)` overflows instead and draws over the neighboring elements.
  - You have to compute the fit from the available width yourself (`crumbFit`: `c.MeasureText` measures the text, `Element.LayoutBounds()` gives last frame's width, `c.Size()` is this frame's window size),
    then use `Shrink(0)` to stop the pieces from shrinking, `MaxWidth` to cap their width, and `ClipX` to clip what overflows.
- `LayoutBounds` / `Bounds` are from the previous frame, and a new element's bounds are empty. In that case request another frame with `c.AnimationFrame()` (`Context` has no `Invalidate`).
- The scrollbar width is the theme's `ScrollbarWidth`, one value for the whole window; it can't be set per element. The horizontal scrollbar (6 pt wide, on a 12 pt tall track) is drawn inside the bottom edge of the scroll container.
  The tab strip's workaround: the scroll container is `stripHidden` taller than the outer `Clip` box and has the same amount of bottom `Padding`, so the scrollbar lands in the clipped-off strip;
  it then draws its own 2 pt thin bar from the `ScrollState` (`TrackScroll`; `s.Bounds(c)` is the viewport).
- Colors of a selected list row:
  - A text field inside a selected row (new file / rename in the file tree) needs an explicit `.TextColor(t.Text)`, otherwise it inherits the selected row's color and the text becomes hard to read.
  - The default selection style is a faint background with the text color unchanged; only with solid selection (`SolidSelection`) turned on in the settings is it an accent-colored background. To decide whether something is "on the accent color"
    (text and icons switch to `AccentText`), always use `onAccent(a, row.Selected())`, never `row.Selected()` directly.
    Accent-colored text buttons in a row use `textButtonOn(…, onAccent(a, sel), …)`, otherwise with solid selection the text is the same color as the background.
  - Tests that check "selected turns `AccentText`" must set `a.St.Settings.SolidSelection = true` first.
- The corner radius of a selected `List` row comes from the theme's `Radius`; there is no separate parameter. Sidebar lists (radius 5) are built under a copy of the theme with a modified `Radius` (`sideList`).
- `c.SetTheme` resets the root node's background color and text style. Switching to a theme copy with `withTheme` inside a card resets the root's background. Therefore:
  - `root()` sets the root background from `c.Vibrancy()` (transparent / `windowBg`) only at the very end.
  - List rows may be built during layout (after `root()` has finished), and switching the theme inside a row still clobbers the root background, so for that frame the whole window background turns card-colored:
    while scrolling (especially scrolling up, when the pinned section header changes) the whole window flickers. So after `withTheme` switches back to the original theme it sets the root background back to
    `rootBg` itself. Always switch themes through `withTheme`, never call `c.SetTheme` directly. `TestHistScrollKeepsWindowBg` measures the window background on every frame:
    a single Tester input pushes several frames until no more are requested, so looking only at the last frame shows nothing; sample the previous frame's pixels in `beforeFrame` (ui_test.go).
  - New lists follow the same pattern: `l := newListLook(a, c, radius)`, `l.build(c, func(){ … wui.List(…).Rows(l.rows(fn)) })`.
  - The default accent color is `workspace.DefaultAccent`; `applyAccent` must be called at the start of every window's root function (main window, settings, about).
- Each frame builds a fresh theme copy for cards, so the pointer is new every frame. The editor (`DefaultTheme(t)` reads colors by value) and the terminal (looks only at `Dark`) don't rebuild every frame because of this;
  watch out if a future component caches data keyed by the theme pointer.
- Keep visual decoration down (see the notes at the top of `ui/accent.go`): the accent color is reserved for "current / focused / can drop here" and for category palettes. New badges, pills, banners and text buttons use
  `quietFill` / `bannerFill` with the body or secondary text color.
- mygo's `Split` divider is fixed at 1 px and can't be made wider. The gaps between panels are our own drag bars (`resizer`, and `splitDivider` for split groups), which draw `gapLine` on hover.
- `Border` is drawn inside the element and takes up layout space: content moves in by the border width. A border added only on hover or highlight makes the text jump by 1 pt, which looks like a font change. For a border that exists only in some states, draw it with `Draw` (no layout space), or just change the background instead (that is what the highlighted item in Grove's menus does). A `Clip` element's stroke is drawn on top of its children (a card's outline isn't covered by the editor); `Shadow` is drawn only outside, and the gaps are only 6 pt, so keep the blur small.
- Draw large blurry color blobs with `Painter.Shadow`, not `Element.Blur` or `Canvas`:
  - `Element.Blur` is a whole offscreen layer bitmap, several MB for the full window.
  - `Shadow(r, …)` normally draws only outside r, but when r has zero width or height the whole blurred shape is drawn. Give a line segment (width w−h, height 0) a spread of h/2 plus some margin
    and you get a rounded ellipse blob; sigma is half the blur.
  - The root node can `Draw` (after the background, before the children; `Draw(nil)` clears it). Opaque elements covering the window background (the Git sidebar's sticky section headers) must `underBackdrop`:
    draw the same blobs again, `Clip`ped to themselves, using whole-window coordinates (`Draw` inside a scrolling list also gets window coordinates). `TestBackdrop` measures the seams.
- Title bar and traffic lights:
  - `TrafficLightPosition` is an integer Point (the top-left corner of the close button), and the traffic lights are 14 pt tall: the title bar height must be even (38) for them to sit exactly centered.
  - With `TitleBarHidden`, mygo's `c.TitleBar()` reports a height of 38 with 72 reserved on the left (`TitleBarHiddenInset` is 66 × 79).
  - When text is vertically centered in a Row, the center of its ink sits 0.5–0.75 pt lower than an icon's. Text on the same line as icons, such as in the title bar and tab strip, is shifted up with `textNudge` (`TestTitleBarAlign` checks the error stays within ±0.5 pt).
- `Tooltip` defaults to above the element and goes up whenever there is room, so tips for elements right below the title bar cover the traffic lights. Use `tooltip(c, el, s)`:
  when last frame's `Bounds().Y` is below `tipBelowY` it pops downward with `TooltipBase`, looking the same as `Tooltip`.
- Draw dashed lines with `Path.Dash(0, segment length)`. `Path` line caps are round by default: a 1.5-wide line extends 0.75 past each end, leaving only 0.5 of a 2-wide gap. Pair it with `Cap(wui.ButtCap)`.
- mygo ignores trailing spaces when measuring text width: `Text(c, " / ")` renders as "a /b". To leave space on both sides, use `"/"` plus the Row's `Gap` (or `PaddingX`), not spaces in the text.
- `wui.Color.Alpha` multiplies the existing alpha instead of setting it: `t.Text.Alpha(0.62).Alpha(0.16)` has an alpha of 0.1. When a background block's color is derived from a shape's color and the shape already has alpha, the block gets fainter too.
- UI zoom is per window: `c.SetZoom` only affects its own window, so the workbench's `root`, `SettingsView` and `AboutView` each call `c.SetZoom(a.ZoomFactor())` every frame.
  A new kind of window needs this line too, otherwise it stays at 100%. The settings and about windows aren't part of the workbench's frame, so when the settings change, main must `Invalidate` them to redraw.
  The terminal view gets keys first (it has its own ⇧⌘= to change the terminal font size); `termZoomKeyFor` in `termKey` intercepts these keys and hands them to UI zoom.
- `Painter.Scale()` doesn't include UI zoom. To render a bitmap at screen pixels (`ui/mdrender.go`), the factor is `p.Scale() * a.ZoomFactor()`; multiply by only one and the image goes blurry after ⌘=,
  or isn't sharp enough on a Retina display. Before the first draw the screen isn't known yet, so use 2 for now; `SeenScale` records the actual value when drawing.
- Dialogs use only `dialogFrame` (`ui/dialogframe.go`), never `wui.AlertDialog` / `wui.Modal`:
  - Buttons use `dlgButton` (normal / primary / danger, all the same height), fields use `dlgField` + `dlgInput`, and command previews use `dlgCommand`.
  - Confirmations that lose data, change remotes or rewrite history use `a.AskDanger` (red); `app/danger_test.go` scans the source for the call sites, so add the function name there for each new one. Other confirmations use `a.Ask`.
  - The bottom row of buttons is built first (`Reverse()`): the hint "⏎ Delete Branch" uses the same words as the button, so finding by name (`Find` / `mustClick`) must hit the button first.

- Widgets we draw ourselves (`internal/ged`):
  - `Painter.Line` only draws horizontal and vertical lines. When the two points differ in both x and y it draws the top or left edge of their bounding box; for diagonals (hatching) use `StrokePolyline`, one segment per line.
  - `HandleInput` has no "pointer left" event: ask `Element.Hovered()` every frame for hover state and clear it once the pointer is gone.
  - The colors in a `TextLayout` from `ShapeRichText` are fixed at shaping time. If you keep layouts for reuse, invalidate all of them when switching between light and dark (the theme) or changing fonts, or dark mode still shows light-mode text.
  - `TextLayout.SelectionRects` / `Caret` indices are UTF-16 units, not bytes and not runes: convert byte ranges first (`ged.lineLayout.u16`).
  - Finding our own widgets by accessibility name in tests (`Tester.Find`) may hit something else with the same name first (the file's row in the sidebar): have the widget record its position from the last frame (`ged.Diff.Bounds`) for tests.
  - `Element.Bounds()` is last frame's position: in a frame that scrolled, using it to place children is off (clicking "Stage Hunk" hit the row below). Absolutely positioned children compute their position from this frame's
    scroll themselves and use `Bounds()` only for size. Likewise, before using the scroll position in `View`, apply where this frame should scroll to (`want`) first.
  - A parent's `HandleInput` receives a press before a child button's `Clicked()`: if the parent always takes presses (for drag selection), the child buttons can't be clicked.
    The parent must remember the area its children occupy (in local coordinates) and not take presses inside it.
  - `Tester` has no double click: two `ClickAt`s back to back are a double click (the interval is measured by its own clock), three are a triple click.
  - `ShapeRichText` wraps once given a width: the number of `Lines()` is how many lines it wrapped into, and line k's top is k × (the shaped line spacing). That spacing isn't necessarily Grove's own line height `lh`,
    so for drawing, hit testing and caret placement convert with `round(y / spacing)` to a line index and multiply by `lh` (`ged.lineLayout.rowY` / `caretRow`), otherwise the error grows with every wrapped line.
  - A widget that handles its own events and wants Grove's context menu must not attach a second `HandleInput` (an element accepts only one); forward right-button press, drag and release to `menuPointer`.
    While the menu is open its catcher layer covers the widget, so the widget never gets right-clicks elsewhere; it therefore doesn't register `AddTarget` (if it did, the menu would open at the last press position).
  - ⇧⌘G is the global "Git Sidebar" command (`view.git`, in the menu), so it never reaches "Find Previous" in the Diff: ged's find bar uses ⇧↩, and the menu has no Find Previous item.

## Lists, clicks and dragging

- While a list's `TrackScroll` `ScrollState.ScrollTo` animation is running, `ListState.ScrollTo` / `ScrollIntoView` / `ScrollToEnd` have no effect: every frame of the animation
  writes the offset back onto the animation's path in `adoptScroll`, overriding whatever the list moved during layout. The sidebar's wheel animation therefore writes `ScrollState.Y` itself every frame (`app.SideScroll`).
- For a mouse wheel (not precise), mygo counts one line as 40 DIP, and macOS acceleration sends several lines per event. The easing is front-loaded (one 40 notch moves 11.5 in the first frame, 400 moves 115),
  so a list with 24-high rows looks jerky. `Tester.Wheel` sends this kind of wheel scroll; `Tester.Scroll` sends trackpad scrolls.
- An element's `Bounds()` is last frame's layout. Using it as a row's position to draw something layered over the list (the sticky header stack) lags the row by one frame; shift it by the difference between the list's offsets in the two frames.
- Inside input callbacks `Context.Now()` is not the event's time; use `InputEvent.Time`.
- `c.Root().Bounds()` read inside an input callback (`HandleInput`) is 0×0: that `c` is the one from building the tree, and the root hasn't been laid out when the event arrives. A context menu's `PopMenu.OpenRow`
  then lays out against a 0×0 window, so the menu shrinks to a tiny piece on top of the pointer and looks like it "flashes". Capture the window size and the element's own last-frame position **while building the element**
  (`b := el.Bounds()`, `root := c.Root().Bounds()`) and use only those two snapshots in the closure (`menuPress`, `openTreeMenuByKey`).
  The headless Tester does have the window size during event handling, so tests can't catch this; it rests on convention (found via logs in a real window on mygo v0.8.x; not rechecked on v0.9.0, but the snapshot approach is correct on both).
- macOS right-clicks and trackpad phase events deliver `InputScroll` with a `DY` of 0. Logic like "scrolling outside the menu closes it" must filter these out first
  (`PopMenu.Scroll` treats `dy` below 0.01 as no scroll), or the menu closes in the very frame it opens.
- To tell whether the window lost focus use `c.WindowActive()`, not some element's `Focused()`: other elements in the window grab the keyboard on press (lists),
  and the window stays active while a system overlay (Quick Look, an input method) has the keyboard. In tests the equivalent is `tt.SetWindowActive(false)`; `tt.SetFocused(false)` only moves the keyboard away.
- Under `SetMenuPresentation(PopoverMenus)`, `ContextMenu` / `Menu` menus are rebuilt every frame (a system menu is a snapshot taken when it opens): if state changes while it's open, items gray out accordingly;
  the test scenario "choose an item that was enabled when the menu opened but no longer is" no longer exists, and `Tester.Menu` / `ChooseMenuItem` still work.
  Grove's own menus (`app.PopMenu`) are also rebuilt every frame: a menu built by indexing a slice with the index from the frame it opened (search results' `s.Res.Matches[sr.Match]`)
  goes out of range if the results shrink while it's open. Check that the item is still there and still the same one first; if not, add nothing (the menu closes).
- Swapping out a list's state after the tree is built (at the end of `root`, e.g. switching workspaces) makes this frame's layout hit a nil pointer in `layoutList`: do the action before building the tree (the chosen item of a Grove menu runs at the start of `root`).
- Once a row has `menuPress` (`HandleInput`), input goes to the row rather than the list cell: `tt.Click(name)` fails with "is not the input target"; use `clickRow`.
- Double clicks: List rows use `RowDoubleClicked`; ordinary elements use `DoubleClicked`.
- As soon as a child element in a row uses `Clicked()` / `Pressed()` / `Drag`, it consumes the event and the list no longer gets it. Don't add `Clicked()` to a row just to read the modifier keys.
  The file tree's approach: only selected rows act as drag sources (pressing an unselected row is still the list's drag selection); clicks on selected rows are recorded by `TreeRowClicked`, and after the list is built `ApplyTreeClick`
  applies the list's rules on top (⇧-click, ⌘-click, double click). Outline rows' contents also use `Clicked()`, so clicking an outline row moves focus to the editor; to focus the outline, click the empty area below the list.
- A List doesn't fire `Changed` when you click a row that is already the only selected one, with or without modifiers. Consequently:
  - "Click again to go back" can't rely on `Changed`. A single click / ↑↓ on a worktree row shows its history (`viewWorktree`, without switching); Return / `RowDoubleClicked` opens it in the workspace;
    going back is the "Show Branch History" menu item or the pill's × (`ClearRefHistory` also releases `gitWt`).
  - ⌥-clicking a file row opens it to the side by checking `c.Modifiers()&wui.Alt` in the frame where `Changed()` fires, so ⌥-clicking a row that is already the only selected one does nothing (use ⌥Return or the context menu).
    The list's own Return submit is `Shortcut(0, KeyEnter)`, which matches modifiers exactly, so ⌥Return isn't treated as a plain Return; it needs its own `FileList.OnShortcut(c, wui.Alt, wui.KeyEnter, …)`.
  - The title row of a compare section (`compareRow` in `ui/history.go`) toggles expansion with its own `Clicked()`. Both a double click and Return make the list report `Submitted`, and the first click of a double click has already toggled once,
    so on `Submitted` it doesn't toggle again when the mouse is over the title (`compareRow` returns `Hovered()`); the cost is that Return doesn't toggle while the mouse rests on the title.
- To tell a mouse click from keyboard selection: record the row's `Hovered()`; on `Changed()`, `hover == sel` means it was a mouse click. The Git sidebar resets `a.GitHover` to -1 every frame before drawing rows.
- `ListState.Header` rows stick to the top and are never selected by clicks or arrow keys; give them a background yourself (otherwise rows below show through while scrolling).
  `ListState.ScrollTo(row, wui.Start)` only scrolls at the next layout. With `Key` set, the selection follows the key when row indices change.
- When mygo counts a double click on an already selected row itself, the second press reports `DoubleClicked()` and the release reports `Clicked()` again. `ApplyTreeClick` records the double-clicked row and time (`dbl` / `dblAt`)
  and drops a single click on the same row that follows within 500 ms; otherwise double-clicking a folder would expand it and then collapse it.
- mygo's list has no API to set the ⇧ selection anchor. The list knows nothing about the clicks / ⌘-clicks that `ApplyTreeClick` applies, or about `chooseOnly` and `ClearTreeSel`; its anchor is still the row it last handled itself
  (which may be collapsed out of sight, so a ⇧-click selects just one row). So these places record `treeClick.anchor` / `own`, and on `list.Changed()` call `TreeListChanged(mods)`:
  without ⇧ it clears `own`; with ⇧ and `own` set it reselects from the app's anchor via `extendFrom`. Other places should also change the selection through `chooseOnly`.
- The file tree's list has `MenuKeepsSelection` on: without it, right-clicking a row first acts as a click (changes the selection and previews that row). Multi-selection menus use `a.MenuRows(row)` to decide whether to act on the whole selection or only that row;
  right-clicking an unselected row selects it via `a.MenuSelect(i)` in the menu callback (without previewing).
- The Git sidebar's cursor `GitSel` and its selection set `GitChosen` must be updated together. The List has `Selection = &a.GitChosen`, so on clicks and arrow keys mygo changes both;
  when the app moves the row itself (following the current tab, skipping group headers, Esc collapsing a multi-selection) it goes through `a.chooseGit(rows, i)`. If only `GitSel` changes, the highlight, which Diffs Return opens (`ChosenChanges`)
  and whether a right-click counts as multi-selection (`MenuChanges`) all still follow the old row. Tests select rows programmatically with `pickGit(a, i)`.
- `OnShortcut` on a list receives keys before a text field inside a row: a key goes to the innermost element on the focus chain that registered it, so Esc and ⌫ on a list get there before the commit box's or search box's own handlers.
  Register list-level keys only when the list itself has focus (`Focused(c)`, not `FocusWithin`) and actually needs them: the Git sidebar's Esc requires `GitChosen.Len() > 1`; the search results' ⌘↩ / ⌫ require `s.List.Focused(c)`.
- When a button or context menu in a List row changes the rows, wait until the list has been built. `rowButton` runs its callback synchronously when pressed during the build, but mygo's `List` then keeps calling
  `Key` / `Label` with the row count from the start of the build, and if `Rows` has shrunk it crashes out of range. The search results' "Replace This Match" / "Dismiss" are handed to `do` and run after `sideList` returns; `Key` / `Label` also bounds-check the index.
  Actions that don't change rows, like opening a tab or copying, don't need to wait.
- Don't modify the repository list while building a list or menu. Menu builders and `Drop` both run while the frame is being built; moving or removing a repository then makes the list keep building with the old row count (duplicate Keys crash).
  Record them as `a.MoveReq` / `a.RemoveReq`, and `pending()` in `ui/view.go` performs them after the frame is drawn. `TreeRows()` rebuilds in place: in a row callback, don't `tree.ver++` and then keep using the rows you're holding.
- Two identical `c.Key`s under the same parent panic (in the Tester too). When an item may be drawn once in each of several loops (settings search results listed by page), wrap each page in
  `wui.Column(c.Key(p))` and use `c.Key(i)` inside. Every table drawn by index follows this.
- Elements without a key are identified by their order within the parent. When a sibling drawn only on hover (the Git sidebar section header's "Stage All") is inserted before a button, moving the pointer over it changes the following button's identity,
  so press and release land on different elements and `Clicked()` never fires. That is why the section header's Pull / Push buttons carry `c.Key("pull")` / `c.Key("push")`.
- A grayed-out button inside a clickable parent (a Git sidebar section header collapses / expands on click): if it doesn't call `Clicked()`, the click falls through to the parent. Call `Clicked()` even when grayed out to catch it, just don't run the callback (`syncButton`).
- A list's `Label` is used both for keyboard type-to-find and as the name the screen reader reads. Type-to-find strips the leading dot (`Row.FindLabel`), so VoiceOver doesn't read the dot either; making the two differ has to wait until mygo separates them.
  mygo's type-to-find only recognizes A–Z, 0–9 and space, so Chinese names can't be matched; don't work around it in the app.
- `repoMenu(a, m, r, tree)`: only when `tree` is true does it include the file-tree-only items (Collapse Other Repositories, Expand Changed Folders, Paste); the repository menus of the Git sidebar and the history right pane pass false.
- In-app value drag and drop (`wui.DragOver[T]` / `wui.Drop[T]`): an element has only one `accepts`, and a type registered later overrides an earlier one; along the hit chain, the innermost element that accepts the value handles it.
  To accept two types, nest two layers: the repository row is wrapped in a `slot` Column that accepts `RepoDrag`, and the row inside accepts `TreeDrag`; an editor group's card accepts `TreeDrag` on its outer layer and `*docs.Doc` on the content area inside.
  If the inner layer also accepts the outer layer's type, the event never reaches the outer layer.
- In native drag and drop (`DropData`), format-based targets also consume file drops. The hit chain goes inside out, and a target matching a pasteboard format consumes the whole drop, which no longer falls back to `DroppedFiles` / the window's `OnFileDrop`.
  Files dragged from Finder may also carry the file names as text on the pasteboard, and a target that only accepts `Text` takes them as a piece of text. So:
  - The editor's and terminal's `DataDrop` also accept `FileList`, and handle drops containing files as files (`OpenArgs` / `ShellFiles`).
  - Values (`Drop[T]`) and formats (`DropData`) are two separate fields; one element can register both.
  - `transfer.Drop` has no drop location, so the editor can only insert at the cursor (`offsetAt` in `ui/editor` isn't exported).
- mygo's `terminal.View` registers its own `DroppedFiles`, which pastes paths backslash-escaped without a trailing space. Grove registers a format target (`DataDrop`, including `FileList`) on the pane's outer frame to catch the drop first;
  `ShellFiles` pastes single-quoted paths, the same format as dragging rows from the file tree (`app.PastePaths`). A split pane whose shell has exited registers `ExitedDrop`, which accepts only files (opened by the window's rules);
  without it, files from Finder would land in the view's own `DroppedFiles` and be written into a terminal with no shell, with nothing to show for it.
- Expanding on drag hover must wait until all rows are drawn: in a row's drop target `DwellTree` only records whether the dwell fired; `EndTreeDrop` (after the list is built) does the `SetOpen` / `ToggleRepo` and requests the next frame.
  mygo produces no new frames while the pointer is still, so use `c.After(time remaining)`. The Tester's `Release` carries no modifiers, so ⌥-release-to-copy can only be tested in the app package by calling `DropTree(…, alt)`.
- In a real window, a file tree row has to be clicked to select it before dragging (pressing and dragging an unselected row is drag selection). Synthesize drags with CGEvent: press → several drags → release (a small Swift program is enough;
  coordinates are pt from the window's top-left corner, and before each event confirm the frontmost process is Grove); pause 1 second between two actions so they aren't taken as a double click.

- Don't use mygo's `Disabled` for a grayed-out icon-only button: it shows no tooltip, and with no text on the button the user can't tell why it doesn't work. The commit button (`commitRow` in `ui/gitbar.go`)
  is drawn gray with `toolStyle(c, b, false, gray)`, acts only on `canClick && b.Clicked()`, and explains why in its tooltip with `withWhy` (`app.CommitWhy`). To make it keyboard reachable use `wui.ButtonBase`;
  `squareButton`'s `Box` can't be reached with Tab.

## Keyboard and shortcuts

- Since mygo v0.8 every key is delivered to `HandleInput` / `ObserveInput` / the editor's `OnInput`: modifier keys pressed alone, Caps Lock, F13 and other unnamed keys also arrive as `InputKeyDown`,
  with both `Key` and `PhysicalKey` set to `KeyUnknown` and only `Code` (`CodeShiftLeft` etc.) filled in. Code that picks keys with a `switch` is unaffected; places that react to "any key pressed"
  (the key-recording card, `restingEditor` waking the caret, `docKeys` clearing `EatText`) must filter these out first with `unnamedKey` from `ui/keybind.go`, or when you press ⌘K the ⌘ that arrives first marks the recording card "This key can't be used".
  In the Tester, simulate a lone modifier press with `CodeDown(mods, wui.CodeSuperLeft)`. The terminal's `OnKey` never receives these keys (the plugin filters them out itself).
- Menu shortcuts receive keys before the editor (mygo doesn't override performKeyEquivalent on darwin): with the editor focused, ⌘K still opens the `>git ` palette,
  and none of the mygo editor's ⌘K chords (⌘K ⌘0 etc.) reach the editor. Only where there is no menu bar (tests, `OnShortcut`) does the editor get keys first, so ⌘K chords working in tests doesn't mean they work in a real window.
- Window shortcuts override the mygo editor's built-in binding for the same key: ⇧⌘G (Find Previous) goes to the Git sidebar, ⇧⌘H (Replace) goes to the search page's replace (replace within the current file is ⌥⌘F),
  and ⇧⌘\ goes to `view.openInOtherGroup` (the editor's `MoveToBracket` still has ⌃M). `TestShortcutConflicts` only knows the commands in the registry, not the mygo editor's keymap,
  so before adding a window-level shortcut check the keymap in mygo's `ui/editor`, and for any binding that gets overridden, change `docs/shortcuts.md` to the remaining key.
- The same key can be different commands on different tabs (⌥⌘F: full-file diff on a Diff tab, replace on a file tab). Keys and menu items both go through `a.RunKey(mods, key)`, which finds the first currently runnable command in registration order;
  don't `Exec` a specific id directly from a menu item or `OnShortcut`.
- Window shortcuts take keys before the terminal. For a key that is both a window shortcut and needed by the terminal (⌘W, ⌘K), the only way is to add a command with `Context: Terminal` to the registry, ordered before the global one,
  so `RunKey` handles the terminal command first. Keys that only make sense in the terminal (⌘D, ⇧⌘[) shouldn't be registered as window shortcuts; handle them in the terminal's `Options.OnKey`.
- Shortcut conflicts are computed per dispatch domain (`keyDomain` / `clash` in `app/keybind.go`): the window (global, Diff tab, editor), the terminal, the file tree, the commit box and the search results are each a dispatch domain,
  and the same key being a different command in two of them isn't a conflict; within the window it's a conflict only if the contexts are the same or one of them is global. A new place that handles its own keys (a list, a text field) needs its own `Context` and an entry in `keyDomain`.
- In this app ⌘1–9 jump to the nth repository's section in the Git sidebar (`TestJumpRepo`), not to the nth tab. Tests switch tabs with `a.SelectDoc(i)`.
- In a real window ⌘C / ⌘V / ⌘A never reach `OnShortcut`: the menu bar's Edit menu takes these keys and sends them as `InputCommand` (`copy` / `paste` / `selectAll`) to the focused element's `HandleInput`.
  - The file tree handles them with `HandleInput(treeCommands(a))` (returning false for anything else). Edit menu items are only enabled when the focused element can perform them; what `HandleInput` doesn't take is sent on to shortcuts as a key press, so the two never both run.
  - `tt.Command("copy")` tests this path; `tt.Key(Cmd, KeyC)` tests `OnShortcut`, which only matters in tests.
  - Keys that have a menu bar item (⌘D is the terminal's "Split Right") are received by the menu and dispatched by `RunKey`; `RunKey` also considers commands with `Context: FileTree` and a `Run` (when `a.TreeFocused`).
- An ⌥ shortcut in the editor may type an extra character. mygo's `keyDown:` sends KeyPressed first, and as long as the keyboard is in text input and neither ⌘ nor ⌃ is held, it goes on to interpretKeyEvents no matter who handled the key,
  so the character ⌥ produces (⇧⌥F is "Ï") then arrives in the editor as InputText; keys registered on the window (`OnShortcut`) additionally keep the editor's OnInput from even seeing the KeyDown.
  So for ⌥-only keys in the editor, the command is marked `Local` (not registered on the window), and the editor's OnInput (`ui.docKeys`) takes the KeyDown and swallows the InputText that immediately follows.
  The Tester doesn't send that text on its own; simulate it in tests with a following `tt.Type("Ï")`.
- The text field inside `wui.SearchField` is unreachable: `OnTextKey` and `TextSelection` / `SetTextSelection` don't work on it (they attach to the outer layer). ⇧⏎ also sends `Submitted`,
  and `c.Modifiers()` is the keys held when the frame was built, not during that Return, so in a real window releasing ⇧ first counts as ⏎ (tests can't tell). For a box that must tell ⏎ from ⇧⏎ or needs the caret, use `TextInput` / `TextArea` + `OnTextKey`
  (`k.Mods` is from that key press); that's why the search bar's box is our own `searchBox`. In the frame where the box switches from single-line to `TextArea`, focus can't be given (the new box isn't built yet); `Focus` / `SetTextSelection` in the next frame (`placeCaret`).
- An element has only one `OnTextKey`; a handler set later replaces the earlier one. The single-line search box distinguishes ⏎ / ⇧⏎ / ⌥⏎ all in one handler, `shiftEnter`; the multi-line `TextArea` also has just one (⏎ and ⌥⏎ search, ⇧⏎ is left to the box as a newline).
  Don't attach another one to the same box elsewhere.
- Window-level Esc uses `OverlayShortcut`: it only takes keys nobody on the focus chain uses, so the editor clearing its selection, the find bar and dialogs all get Esc first. Registering it as a normal shortcut overrides the editor's own Esc handling.
- After clicking a toolbar button, keyboard focus moves to the button: buttons are `Focusable`, so after the click `tt.Key` or real `=` `-` `0` key presses land on the button and the canvas never sees them. The image toolbar sets `imgFocus` after a click,
  and the canvas takes focus back the next frame; UI tests must run one `tt.Frame()` after the click before pressing keys. The same goes for any other place where "keys should follow a button click".
- `menuSignature` must include `keyRev` (`app/menu.go`): the menu bar is rebuilt only when the signature changes. If changed shortcuts (`applyKeybindings` increments it each time) or active key recording (`KeyRecording`; the menu bar shows no keys while recording)
  aren't part of the signature, the menu keeps showing the old shortcuts.
- Read keys from the registry, don't hard-code them: context menu items use `shortcut(it, id)`, keys an element handles itself use `onKey` / `app.KeyOf(id)` (`ui/keybind.go`), and keys in help text use `app.Find(id).Keys()`.
  With hard-coded `wui.Modifiers` + key, once the user rebinds a key the menu labels no longer match the shortcut that actually responds. After changing commands run `go test -run TestShortcutDoc ./internal/app/ -update`.
- Don't hard-code keys in tooltip text either (`app/commands.go`):
  - For keys in parentheses at the end of a sentence use `app.Tip(text, id)` (no parentheses when there's no key; full-width parentheses in Chinese, half-width in English).
  - For keys mid-sentence use `app.KeyHint(id, sentence with key, sentence without key, args…)`, writing both as complete literals (the i18n catalog only collects literals); the keyless sentence names the command instead, never leaving an empty pair of parentheses.
  - To list the keys of several commands use `app.KeyList`. `KeyText` panics on an id that doesn't exist, so a mistyped id fails the tests.
  - Leave as literals: text that names only modifiers ("⌥-click", "⇧Return"), keys that lists and text fields handle themselves, and the editor's own undo.
- Changing a global shortcut means also updating: the places in tests that press that key (e.g. `ui/terms_test.go`), keys missing from `keyNames` (⌃⇧` needs `KeyBackquote`),
  `docs/shortcuts.md` (`TestShortcutDoc -update`), and the keys written in README.md / README.zh-CN.md. Search for every reference before making the change.

## Editor

- For a self-drawn editing widget (`internal/ged`) to receive `InputText`, it must call `el.TextCaret(rect)` while focused: without it the input method doesn't engage, and a KeyDown that isn't taken won't be followed by text either.
  On macOS KeyDown arrives first and text only comes if the KeyDown wasn't taken, so `HandleInput` must return false for a KeyDown for typing to work (keys like J and space, which ged takes in view mode, must be let through while editing).
  `InputCompose`'s `Caret` is a rune index, and an empty `Text` means composition ended; the Edit menu's (`RoleEditMenu`) cut, paste, undo and redo arrive at the focused element as `InputCommand`.
- Every `editor.Buffer.Edit` is one undo step: mygo's logic for merging consecutive typing into one step (`editTyping`) isn't exported, and only its own editor view can use it.
  ged records how many `Edit`s each group merged and calls `Undo` that many times on undo (`ged/edit.go`); if the buffer was changed elsewhere (version mismatch) it stops undoing by group.
- `editor.State.Label` registers the editor as a text field label, so in the Tester `tt.Click("编辑 a.txt")` focuses it.
- A focused editor keeps drawing: as long as `Focused()` is true when it's built, mygo's `ui/editor` schedules a frame every 500 ms to blink the caret, with no switch to turn it off; frame memory is only released after 2 seconds without new frames,
  so this costs an extra 100–200 MB and continuous CPU. New editor / Diff views must go through `restingEditor` / `restingDiff` in `ui/caret.go` (stops blinking after 30 s idle); don't use `editor.View` / `editor.DiffView` directly.
- After the caret moves, the mygo editor auto-scrolls leaving only min(3 lines, half a screen) above and below the caret, with no option to change it (it has nothing like VS Code's cursorSurroundingLines).
  `restingEditor` lets mygo finish scrolling, then `caretMargin` tops it up to min(5 lines, 1/5 of the view height). It only tops up when the keyboard moved the caret, via `Rest.Keyed`: true when `OnInput` receives a key / text / command, false on pointer press, and cleared after each frame (otherwise an Esc or ⌘S that didn't move the caret would linger, and a later external content change would yank a scrolled-away view back to the caret).
  This leaves clicks, the wheel and scrollbar dragging alone, and programmatic `GoTo` already scrolls to the center, which is margin enough. It doesn't top up when the caret's line is soft-wrapped into several rows, because the public API doesn't say which wrapped row the caret is on.
- `DiffState.GoToHunk(i)` puts the caret at the start of that hunk and scrolls only if it's out of view, to the center (after the view has been laid out). mygo has no public scrolling API, so "first change" lands in the center of the view, not the upper third.
  `ExpandAll` / `CollapseAll` (around changes ↔ full file) don't preserve the position of the change; call `GoToHunk` again afterwards (`Doc.ToHunk`).
- `editor.State.LineBackgrounds` / `GutterMarks` store line numbers as of when they were set and don't move with edits: after inserting a line the backgrounds stay on the old lines. Conflict block coloring (`conflictMarks` in `ui/merge.go`)
  therefore resets them every frame from the current blocks, setting nil when there are none; the blocks themselves are cached by `Buf.Version()` (`Doc.ConflictBlocks`), so resetting every frame doesn't add much work. Anything else that colors by content should do the same, not set it once on open.
- `editor.Buffer.Modified()`, whenever the current text and the saved text aren't the same rope (edited, even if undone back to identical), concatenates both into whole strings to compare them, a several-MB allocation each time for big files.
  Anything that asks every frame (tab dot, status bar, menus) goes through `Doc.Modified`, which caches by buffer pointer, `Version()` and save count. `Buffer.MarkSaved()` doesn't change `Version()`,
  so Grove calls it only through `Doc.markSaved` (which also invalidates the cache); never call `d.Buf.MarkSaved()` directly.
- `GutterMarks` belong to conflict blocks; diagnostics go only through `SetDiagnostics` (`app/lint.go`): `conflictMarks` resets all of `GutterMarks` every frame, so anything else put there disappears the next frame.
  Problems reported by checks are handed to the mygo editor's built-in diagnostics (`st.SetDiagnostics`: squiggles, hover, scrollbar marks); no dots are drawn in the gutter.
- The version in `SetDiagnostics(version, ds)` is the text version ds was computed against; the editor moves the positions to the present along the edit history after that version. If the buffer doesn't have that history (reloaded, version mismatch) it sets nothing and reports no error.
  So (`SyncProblems`): first clear with `SetDiagnostics(cur, nil)`, then set with the recorded version; if `Diagnostics()` is still empty afterwards, recompute against the current text and set with the current version.
  Positions are computed against the text and version at the moment the command ran (`snapOf` takes the snapshot on save / manual check), not against the current text when the result comes back.
- The editor's `OnInput` is captured at `View` time, and a handler set later replaces the earlier one. `restingEditor` attaches its own `OnInput`, so extra keys must be chained in through its `keys` parameter (e.g. `mergeKeys`), not with another `.OnInput(...)` outside.
  Returning true consumes the key event (⌃⌥1 doesn't also insert a character); handle only `InputKeyDown`, and compare keys against the command registry (`ev.Mods == cmd.Mods`, and `Key`, or `PhysicalKey` when `Key` is `KeyUnknown`).
  These local commands (`Run` is nil) aren't window shortcuts and don't appear in the palette. In the Tester `tt.Key(wui.Ctrl|wui.Alt, wui.Key2)` reaches them; whether an input method swallows ⌃⌥ keys in a real window needs manual testing.
- `editor.NewBuffer` can be built in the background: it only builds the rope and doesn't touch the UI, and `Buffer` methods are safe to call from any goroutine. Buffers for big files are built in `ReadFile`, and the main thread just swaps the pointer.
  When the `St.Buffer` pointer is swapped, mygo resets selections, folds, find and scroll; save what should survive (caret, `ScrollY`) first and restore it after the swap (`docs.setText`).
- `Buffer.Reset` records the entire old text in the undo log (mygo keeps 256 entries), so every reload of a several-MB file keeps another copy of the old text. Big-file reloads don't use Reset; they swap in a new buffer built in the background.
- A plain editor (`editor.State`) can't reserve a block of height between two lines for the caller to draw in: mygo has such blocks internally (`blockAnnotation`, which pushes the lines below down during layout), but only exposes them to the Diff viewer through `DiffState.Annotations` + `RenderAnnotation`.
  So the change peek card (`ui/peek.go`) can only be layered over the code, covering a few lines below the hunk, rather than inserted between lines as in VS Code. Doing that needs a similar API on `State` from mygo (line number, height, Key, with the line number moving with edits); it's noted as a mygo feature request.
- The background of changed lines in a Diff is mygo blending `Theme.Added` / `Deleted` into the background at fixed ratios (light: 12%, gutter 9%, words 15% opacity; dark: 20 / 15 / 20; still the case in v0.10.0). The ratios can't be changed, and no choice of color reaches VS Code's intensity;
  `editor.State` has `LineBackgrounds` / `TextBackgrounds`, but `DiffState` doesn't pass them to the editors on either side. Grove's approach (`ui/difftint.go`) is to draw underneath itself:
  - `NoBackground = true` turns off the changed-line backgrounds, and `Theme.Background` is set to the editor background color with `A` at 0 (don't use the all-zero `Color{}`: mygo treats it as unset and fills in the opaque default background; the RGB is also used to decide light vs. dark).
  - Wrap it in a `Column` painted with the editor background, and compute in `Draw` from `DiffState.Hunks` / `LineTop(side, line) - ScrollY()`. `Draw` runs every frame, so scrolling needs no separate sync.
    In side by side, the old side is `(W-1) × SplitPercent` wide (0 counts as 50%) and the new side starts 1 pt to its right; line height is rounded to screen pixels the same way mygo does (`round(font size × line height factor × scale) / scale`).
  - mygo also blends selected lines into `Background`, so with a transparent background they become invisible and have to be drawn ourselves.
  - Replacing the Diff editor with a WebView (Monaco, CodeMirror) is not an option: one WKWebView with its WebContent and GPU processes takes 200–300 MB (see `render.IdleClose`), over the memory limit.
- In mygo v0.10.0 `Buffer` records each line's own line ending, and `Modified` compares line endings too; `SetLineEnding` doesn't bump the content version, so anything caching `Modified` by version (`docs.textModified`) must invalidate the cache itself when the line ending changes.
- Changed words within a line: only when `Indicators` is `DiffBars` does mygo paint them at 22% of the bar color (`lineLook.word`); under `DiffClassic` (+ / -) and `DiffNoIndicators` the bar color is empty and changed words fall back to `Theme.Occurrence` (gray), the same color on both sides, which looks muddy. For red / green words the bars must be on.
- mygo draws change marks in the Diff's scrollbar track itself (one color each for added, deleted, modified); don't add an overview ruler next to it: we tried once and it drew two identical strips side by side.
- `DiffSeparator` exports only the line count, `Step` and `First` / `Last`, not the line where the gap starts (the field `a` is unexported). Showing the enclosing function on the fold bar needs that number, so `sepStart` in `ui/diffsep.go` reads it via reflect
  and shows nothing if it can't; `TestSepStart` fails first if a mygo upgrade changes the field. The first visible line after the gap is `a + context kept above + Lines`, and the first changed line adds one more context's worth (the separator at the top of the file has no context above it; when `ContextLines` is 0 it's 3). Don't look up the enclosing function from the first visible line: that is often the previous function's `}`.
- `Loading` isn't set on reload: reloading a tab that already has a buffer doesn't clear it. In tests, wait for a reload by checking the content (`strings.HasSuffix(d.Buf.String(), …)`) or `Gen`, not `!d.Loading`.
- `unsafe.String` is only for byte data the caller owns exclusively: `decode(data, …, owned=true, …)` borrows the UTF-8 `data` as a string without copying, and the caller must never modify `data` afterwards
  (fine for what `ReadFile` read or what `git.Blob` returned). For bytes passed in by others use `Decode` (which copies); `TestSameBaseHash` checks this convention.
- A mygo `Highlighter` that reports "not ready yet" makes the editor redraw every frame: if `Highlight` returns false, or `Tokens` returns final as false, the editor keeps requesting frames (the latter re-asks for visible, unsettled lines every frame).
  The background highlighter's `Highlight` therefore always returns true, and `invalidate` is called only twice: once when it passes the requested lines and once when it finishes. Don't call it on every batch, and don't leave unfinished lines with final false for long.
- mygo doesn't call Close when it replaces a highlighter (the interface has none): switching language, switching buffer, or rebuilding after an undo log mismatch all just drop the old one. A highlighter that starts its own goroutines must exit on its own when done and stop the old work on `Edit`;
  it can't stay resident waiting for new jobs. Tests use `highlight.Running()` to wait for them all to exit.
- `editor.State.Run` ignores ReadOnly (mygo only checks it on the keyboard input path): on read-only tabs (historical versions, big files) `St.Run(editor.DeleteLine)` still changes the text.
  Editor commands in the command palette must check `Enabled` themselves inside `Run` (`inEditorChecked`), not rely on the palette's disabled state alone; `Exec`, custom shortcuts and menus all go through `Run`.
- mygo editor commands that process the whole text compute on the main thread: Fold All / Fold Level N / Fold Recursively compute every fold range, and Select All Occurrences and splitting into one cursor per line scan the entire document.
  On a 64 MB file Fold All hadn't finished after five minutes and splitting cursors took 18.7 s, so the command table marks these `whole`, disabled above `MaxWholeTextBytes` (1 MB). Before adding a command to the table, try it once on a 64 MB file.
- Layers over the editor cover the editor's own find bar (`ui/sticky.go`): the find bar and Go to Line bar are built by the mygo editor inside itself, so an `Absolute` overlay built later draws on top of them and gets clicks first.
  To know where they are, replace `State.RenderFindBar` / `RenderGoToLine` (record `bar.Bounds()`, then call `editor.DefaultFindBar` / `DefaultGoToLine`) and stop the overlay's right edge at their left; the position you get is last frame's.
  mygo also has no API for the gutter width; `stickyGutter` is an estimate copied from the editor's `gutterWidth`, and must follow if mygo changes the gutter.
- The blame gutter must re-check the scroll position at draw time: the editor still scrolls itself inside `Draw` (caret jumping out of view, drag-selecting at an edge, soft wrap measuring new line heights), so `ScrollY` / `LineTop` read while building the UI
  may have changed by the time it draws. The column's `Draw` compares them and, if they changed, calls `Invalidate` to build another frame; otherwise the gutter is one frame out of step with the code.
  The gutter's width is reserved with the editor's left `Margin` (not as a column outside the editor), so the wheel still lands on the editor.
- Outline names kept long-term need `strings.Clone` (`app/symindex.go`): `Name` / `Detail` found by per-line regexes are substrings of the whole document, and storing them in the index without copying keeps the entire file's text in memory.
  The index detects file list changes by the slice identity of `files.Tree.Paths` (`samePaths`): `Paths` is only ever replaced wholesale; don't `append` to or modify it in place.
- Tabs sharing one buffer need `relink` after reading from disk (`app/mirror.go`): mirrors and the original share an `*editor.Buffer`, but when `Doc.ApplyFile` reads a new file it swaps the buffer pointer.
  After reading from disk, saving, the conflict banner, Keep Mine, and changing encoding / language / indentation / line endings, call `a.relink(d)` so the other copies `Share` the new one; otherwise editing one tab doesn't update the others.
  Any new path that swaps `Buf`, the comparison base or the encoding must hook in too. The unsaved count, the quit list and watcher reloads only consider the original (`Copy == 0`); don't count twice.
- File tabs write to disk only via `WriteDoc` → `diskWrite` (`app/localhist.go`): Save, Save All, save on quit, auto save, save after formatting and Convert to UTF-8 all go through it,
  and local history (recording "On disk before save" before writing and "Save" after) and reloading keybindings.json depend on it. When adding a new save entry point for file tabs, don't call `os.WriteFile` directly, or it bypasses local history;
  other disk writes in the app (rename / move, restore, `>restore`) call `LocalHist()` themselves to record.
- Saving the settings document doesn't write to disk: `docs.SettingsDoc` is an editable in-memory document, and `Save` / `WriteDoc` hand it to `applySettingsDoc`. When adding a save entry point, keep it out of the file write path (it has no `Abs`).
  When settings change elsewhere (the settings window, ⌘=, import), `syncSettingsDocs` in `SettingsChanged` replaces settings documents that haven't been edited and leaves edited ones alone.
- The `editor.State` under ged's file editor has no view: `Selections` / `GoTo` / `Select` / `Insert` / `Run` still work (they touch only the buffer, and undo is the buffer's too),
  but layout-based values like `ScrollY` / `LineTop` / `VisibleLines` are meaningless, and `Run(Copy / Cut / Paste)` can't reach the window's clipboard (go through `app.CopyText` / `ReadClipboard`).
  To know which lines are visible or to scroll, use `Doc.VisibleLines` / `ScrollTo` (`docs/edit.go`) rather than asking St; before adding new code that touches St's view, check `Doc.UsingEd`.
- When testing ged's file editor the keyboard doesn't land on the editor by itself: after `OpenFile` set `a.FocusDoc = true` and wait for `d.Ed.Focused()` before typing (`openGedFile`).

## Terminal and termd

- The first line of `main` in `cmd/workbench` dispatches termd: `Grove termd <sock> <log>` is the terminal background process, and `if len(os.Args) > 1 && os.Args[1] == "termd"`
  must come before argument parsing, the single-instance lock, font loading and opening windows. Don't put anything in front of it: the background process must not open windows or grab the single-instance lock.
- The terminal's `Options.OpenLink` is called on the main thread outside the terminal lock, so it may call `Terminal.Dir()`; when it returns false, URLs go to the system browser and paths do nothing.
- The terminal's cursor blink (`terminal.Options.NoBlink`) is fixed at `terminal.New` and can't be changed while running (only font, theme, copy on select and minimum contrast can), so the "Cursor blink" setting only applies to new terminals.
- mygo's terminal doesn't expose the shell's process ID, and on macOS `ps -E` can't see other processes' environments. The only way to get the PID is to find the app's child processes and claim them in launch order;
  when closing a pane that hasn't claimed one yet, wait for `Term.Done()` (up to 300 ms), or its shell may be claimed by the next split pane.
- While a just-started shell hasn't claimed its PID, `TermPane.Job()` is `"?"` (treated as "a command may be running"). Tests must wait for `Job() == ""` before switching workspaces or closing the terminal.
  In an empty workspace, an open bottom panel starts a shell in `~`, and switching workspaces then asks "Close terminals?": UI tests of the empty workspace call `termState(t, false)` before `newUI` (the first frame decides whether to start a shell).
- The terminal's `Text()` strips trailing spaces from each line: when waiting for a prompt don't wait for `"$ "`; wait for something without trailing space, like `"\nname $"`.
- A pane attached to termd (`ResizeByConn`) calls `Term.Resize` only when it receives `resized`: resizing just sends the new size to termd (`Conn.ResizePixels`), and the screen is laid out at the size termd reports back.
  If the local side calls `Resize` first, the next mirror frame is still laid out at termd's old size and lines come out garbled. `attachPane` sets `conn.OnResized = term.Resize`; don't call it anywhere else.
- Mirror frames don't carry OSC 7: a `terminal.Mirror` frame contains only the screen and scrollback, termd sends the directory reported by the shell separately (`OnDir`), and the window's `Term.Dir()` is empty for background panes.
  To get the directory use `shellDir()` (`Term.Dir()` first, then `rdir` sent by termd); for a shell that never reported one, termd reads the cwd by PID.
- Don't `ClearScreen` locally in a background pane: ⌘K is handed to termd to clear (`conn.Clear`, which `OnClear` also routes to); a local clear gets drawn back by the next mirror frame. In full-screen programs termd replies `N` (don't clear).
- An instance with `WORKBENCH_STATE` set has its own termd: the socket follows the state directory (`termd.SockPath(StateDir)`), so it never attaches to the sessions of the Grove you use every day.
  When done, `kill -TERM` only your own termd (`pgrep -f '<your own .app>/Contents/MacOS/Grove termd'`); don't kill any other.
  The socket is 0600 and the protocol is unauthenticated: any process that can connect to the socket can send input to the shells, so never put it in a directory others can read.
- Restoring the previous output must happen before the shell's first output (`app/termhist.go`): the sequence from `Terminal.History()` ends with the cursor on the line below it, and `startLocal` `Feed`s it right after creating the terminal.
  Too late (the shell has read its rc files and printed the prompt) and it lands on top of the prompt or between two lines. Split panes attached to termd don't restore this output, because mirror frames carry their own scrollback; if the connection fails and falls back to local, it is still restored.
- Exited sessions in termd are managed from both ends: on the termd side, a session whose shell exited and that no window is attached to is removed after `Server.Linger` (reattaching cancels that; the `session.linger` generation won't match);
  on the window side, closing the window or switching workspaces `Kill`s exited panes directly (panes taken over by another window, `lost`, are only disconnected), and `sessionTerms` records only sessions that haven't exited.
  When changing one side, check the other: rely only on termd and the next window may first connect to a session about to be removed; rely only on the window and sessions left behind by a crashed window are never cleaned up.

## Windows, tabs and lifecycle

- `mygo.Dialog.Open` blocks; run it in a goroutine and send the result back to the main thread via `update`. Each call can pick either files or folders, not both, which is why "Open Folder…" and "Open Workspace File…" are two items.
- `Ask` can show only one confirmation at a time: when several things need confirming at once, combine them into one question (several repositories not in the list, in `OpenArgs`); don't call `Ask` back to back, or the later one replaces the earlier one.
- Document fields a background goroutine needs (`Abs`, `Rel`, `Root`, `Change`…) are copied to local variables before `go func`: a rename (`Doc.Moved`) changes them on the main thread,
  and reading `d.Abs` directly in the goroutine triggers a `-race` data race report.
- A `Run` in the `Commands` registry that references a function which uses `Commands` causes an initialization cycle; fill those in `init()`.
- Switching workspaces overwrites all of `*a` (`*a = *n` in `switchTo`):
  - Reading `a.Update` / `a.Win` in a goroutine, `time.AfterFunc` or terminal callback (`OnTitle` / `OnExit`…) races with the overwrite and `-race` reports it.
    Take them out on the main thread first and pass them in: `up := a.Update` / `up, win := a.Update, a.Win`, then `go func() { …; up(func() { … }) }()`.
    Timers in `cmd/workbench` use `win.Update`. Menu Clicks and key callbacks already run on the main thread and don't need this.
  - New per-window fields on `App` must be carried over in `switchTo`; anything not carried over is wiped. Currently carried: `Windows`, `Slot`, `LastBar`, `ownStore`, `Win`, `Update` and the callbacks;
    if `Windows` were left out, the registry could no longer manage that window correctly after a workspace switch (menus and saving would both go wrong).
- Always save through `a.store()` (the registry's `Store`), never `workspace.SaveState` directly, or you overwrite other windows' sessions.
- Don't keep UI state in package-level variables: several windows draw on the main thread, so package-level variables get changed by each in turn (e.g. with one window full screen and one not, a log line every frame). Put it on `App` (`a.LastBar`);
  only process-wide things (settings, language, theme) are global.
- The loop that draws the tab strip may see things that modify the slice, such as `Drop` or the context menu's "Close": iterate over `slices.Clone(a.Docs)` / `slices.Clone(a.TermTabs(r))`,
  not `range` over the original slice (modifying while iterating shifts items and duplicates Keys).
- With split editor groups, `a.Docs` holds only the tabs of the currently focused group; other groups' tabs are in `Group.docs`, and `swapTo` exchanges them when switching groups. Code that changes "the current tab" keeps using `a.Docs` / `a.DocSel`;
  anything that needs all tabs (sessions, asking about unsaved changes before closing a window, finding an already open file, updating Diff tabs when Git status changes) uses `a.AllDocs()`, with `GroupOf` / `locate` to find which group.
  Tests inspect groups with `a.Groups()` + `a.GroupDocs(g)`.
- Which group has the keyboard with split groups: call `FocusGroup` in the frame where `groupCard`'s `FocusWithin` goes from false to true; don't set the current group from `FocusWithin` every frame (in the frame right after the program switched groups, before the keyboard has moved, it would switch back).
  Programmatic group switches go through `toGroup` (sets `a.FocusDoc`; next frame that group's card `Focus()`es first, then the editor takes it); before switching, set `Rest.Woke = true` on the old editor,
  or a resting editor grabs focus back every frame and other groups can't be focused.
- `FocusDoc` is a global "next frame, give the keyboard to the current tab of the group that has the keyboard", and it keeps waiting while that tab is still loading (`Loading`). If a test ⌥-clicks to open a file (setting `FocusDoc`)
  and, before it finishes loading, switches groups programmatically + sets `a.FocusTree = true`, the loaded tab grabs the keyboard back and the file tree gets it for only one frame. In tests, switch groups before giving the file tree the keyboard:
  first wait for that file's `Buf != nil`, or do what a user would and click that group's editor (`mustClick(t, tt, "编辑 x")`).
- Releasing a dragged tab is handled while drawing one of the groups, by which time `collapse` / `splitOff` have already modified the tree in place (`cut` is `*n = *sibling`). `edNode` copies `n.A` and `n.B`
  into local variables before drawing the two halves, otherwise it may get nil when drawing the second half. The new tree is drawn the next frame.
- A session's `Tabs` are stored in tree order (top-left to bottom-right, left to right within a group), not opening order; `Layout` leaves record indices into this order.
- Tabs in a session that aren't files must also write an absolute `path`: `State.clamp` deletes tabs whose `path` isn't absolute, which shifts every later `active` and `layout` index.
  Diff / historical version tabs write the repository root as `path`; `restoreTabs` sees a directory, skips it and leaves an empty slot so the indices still line up. Any new kind added later should do the same.
- Read-only generated tabs (search editor, replace preview) aren't saved in sessions and aren't in ⇧⌘T: `sessionTab` is an allowlist, so new Kinds aren't recorded by default; `closed` in `RemoveDoc` is a denylist, so new Kinds must be added to it explicitly.
  Content generated in the background checks `d.Gen` before going back to the main thread (the tab may have been rewritten or closed), and closing a tab cancels its running search with `searchSrcOf(d).stop()`.
- mygo has one menu bar for the whole app (`mygo.App.SetMenu`): with several windows, only the frontmost `App` rebuilds it in `SyncMenu` (`Windows.Focus` clears `menuSig`), and the other windows never touch it.
  A menu item's `Click(item, win)` goes to `a.target(win)` (the workbench window in front at click time); don't use the `a` that built the menu directly inside Click.
  The Tester has no menu bar (`HasMenu` is false), so this can only be tested by `TestWindowsRouting` calling Click directly.
- Quit confirmation belongs in `OnBeforeQuit`: mygo sends `OnBeforeQuit` first, then closes windows one by one, stopping at whichever window's `OnClose` blocks it, but windows already closed don't come back.
  So everything that needs asking on ⌘Q lives in `Windows.RequestQuit`: block the quit, ask one by one, then `Quit()` itself when done (`ending` set to true lets this one through); another ⌘Q while asking (`quitting` is not nil)
  only brings the window being confirmed to the front. There is no `OnWindowAllClosed`; `Windows.Closed` quits by itself when the last window closes.
- Switching workspaces is `*a = *n` (`app/workspaces.go`): pointers to fields of `n` still point into `n` after being copied. `Repo.set` (`&a.St.Settings`, read by the Git sidebar's switches for showing worktrees / stashes)
  is therefore re-pointed for every repository after the copy; any future pointer in `Repo` to an `App` field needs the same treatment.
- `Window.OnReadyToShow` fires only once (when the window finishes its first frame); handlers registered after that never fire. The restore queue advances on it: in main's `NewWindow` hook, register
  `win.OnReadyToShow(func() { wins.Ready(a) })` right away. A window closed before finishing its first frame never gets it, so `Windows.Closed` calls `Ready` on its behalf. Tests have no real windows and call `w.Ready(a)` directly.
- The `RestoreGap` timer must go back to the main thread: `time.AfterFunc` runs on another goroutine, so when it fires it only does `mygo.RunOnMain(fn)`, and `fn` (`Windows.next`) reads and writes the registry on the main thread.
  `Windows.Later` is this hook; tests replace it with a stub that records the call to run it manually.
- `mygo.App.SetWindowStateStore` must be called before Run: mygo reads the window position once when it creates the first window with a `StateKey` (that's the only read); setting it later only changes where it writes.
  So main decides `stateDir` (`WORKBENCH_STATE` or PathUserData) and sets `app.WindowStates` before `WhenReady`; `App.Path` is usable before Run.
- `mygo.App.Path` creates an empty directory the first time it's called, so `migrateDirs` (`cmd/workbench/migrate.go`, which renames the old Workbench state and log directories to Grove) must run before it.
  Nothing is moved when the executable name isn't Grove (`go run`) or when `WORKBENCH_STATE` / `WORKBENCH_LOGS` is set. Migration is tried only in a test's temporary HOME (`TestMigrateDirs` sets `HOME`),
  never against the real `~/Library/Application Support/`. For why `cmd/workbench` isn't renamed, see `AGENTS.md`.

## Git

- `git.Run` can return output together with an error. Check `err` to decide whether it failed; don't judge by whether the output is empty.
- Resolve symlinks when comparing paths: `git worktree list`, `git rev-parse --show-toplevel` and `AddRepoDir` return real paths (the `/var/…` of `t.TempDir()` comes back as `/private/var/…`),
  while paths stored in the workspace or built by hand may not be. To decide "already open", nesting inside a repo, where a branch is checked out, or whether a submodule has been added, always use `git.SamePath` / `git.Under`, never `==` or `strings.HasPrefix`.
- `git log --follow` combined with `--grep` / `--author` drops the commits before a rename: when searching a file's history, read the whole range (`git.MaxFollowSearch`) and filter in Go.
- The commit graph needs `git log --date-order`: the default chronological order puts a parent before its child when commit times are out of order (rebases, a skewed clock), and since `internal/graph` only computes forward, the lines break.
  `--date-order` guarantees every parent comes after all its children while other commits stay sorted by time; on a 5-repo workspace, reading 200 commits costs an extra 0.02–0.05 s.
  With a search term, or when viewing a file's history, only some commits are read, so no graph is computed (`History.Graph` stays empty).
- When lanes merge, `graph.Lanes.Next` goes back and modifies the `Row.Edges` it returned last time (same backing array): `History.Graph` appends the returned Row directly. Don't copy or sort Edges before passing in the next commit,
  or the previous row loses its curved lines. When paging, keep feeding commits into the same `Lanes` object (`TestPaging` checks that paged results match processing everything at once).
- `graph.Lanes.Reserve` only has an effect before the first `Next` (it does nothing once `active` is non-empty); don't Reserve again on every page when paging. `History.lanes` is reset to its zero value in `reset` and immediately Reserved;
  the HEAD hash must be read by the background job's own `rev-parse`, not taken from `Extra.Head` (status and history are re-read concurrently, so it may be stale).
- Clicking a commit that is already expanded does not collapse it (`OnHistSelected` only expands): to collapse in a test, click a different row.
- The watcher must let `.git/worktrees/<name>` through: each linked worktree's `HEAD`, `index` and `locked` live in that directory. Without it, an external `worktree add / remove` or a branch switch inside a worktree
  never reaches the main repository as a change notification (`TestWatchWorktreeAdded`). Anything one level deeper (`logs/` and so on) is still ignored.
- `git worktree prune --dry-run -v` writes what it would prune to stderr: `git.Run` only takes stdout, which is empty, so `PruneList` uses `RunOp` (combined output).
  Always pass `--porcelain -z` to `worktree list` (paths can contain spaces and newlines), and skip unknown keys when parsing (future git versions will add more).
- Don't run commands unconditionally when reading worktrees: for a regular repository, first check whether `<common dir>/worktrees` exists (`Extra.CommonDir`); if it doesn't, there are no worktrees. Change counts and ahead / behind are only computed while the Worktrees block is expanded
  (a repository with 45 linked worktrees means 90 git commands, about 1 s).
- When a conflict is resolved to exactly what HEAD has, nothing is staged after `git add`: tests must not assume Staged == 1 (only taking "theirs" gives you that). When a merge is stopped halfway and every conflict has been marked resolved,
  `Repo.MergeReady()` keeps the commit box able to commit (git allows such merge commits); cherry-pick and rebase cannot.
- Git write commands that can stop (merge, rebase, cherry-pick, revert, `rebase --continue` / `--skip`) don't necessarily fail when they exit non-zero: finishing one step and stopping at the next conflict exits with 1.
  - The banner's continue / skip goes through `app.runStep` (with its own `Op.done`); `CONFLICT (` in the output means "stopped at the next conflict". Don't use `RunOpThen`'s then — it doesn't run on non-zero exit.
  - Starting such an operation goes through `stopOp` (`app/gitdaily.go`): if the output contains `CONFLICT (`, or the repository is still in an ongoing operation (cherry-picking an empty commit), hand it to the conflict banner with a one-line notice instead of the default failure dialog.
  - "Already in HEAD" (`inHead`) only walks parents through the history the right panel has loaded; anything not loaded counts as not in HEAD.
  - In git 2.54, cherry-pick / revert sequences leave no `sequencer/done`, only `sequencer/todo` (whose first line is the stopped commit), so the banner only says how many more are queued.
  - `git merge --skip` does not exist; `cherry-pick --continue` fails when the resolution is empty ("is now empty"), and `rebase --continue` silently drops an empty commit.
- `git rebase -i` returns 0 when it stops at an edit: `err == nil` in `o.done` after an interactive rebase does not mean the rebase is over. Check `git.ReadOngoing(gitDir) == git.Rebasing` first (on conflicts it exits non-zero; check this first anyway).
  - Commands on `exec` lines in the todo go through `sh`, so wrap paths with `shellQuote` (single quotes; an embedded single quote is written as "close quote, `\'`, reopen quote").
  - `GIT_SEQUENCE_EDITOR` is passed to that one command only, through `Op.env` / `git.RunOpEnv` (appended after the default environment; a later value with the same name overrides the default). Never set it in the process environment.
  - The todo and message files can't be deleted while the rebase is stopped (later exec lines still read them). Delete them only when the rebase ends (`followRebase` sees `Ongoing` leave `Rebasing`, then confirms once more with `ReadOngoing`).
- Renames / copies in `--numstat -z` are not in one segment: a normal entry is `added\tdeleted\tpath\0`; for a rename the path field is empty and is followed by two more segments, `oldpath\0newpath\0`. Binary files show `-` / `-` for added / deleted.
  `git.addNumstat` matches entries back to the files listed by `diff-tree` by new path and skips anything that doesn't match (if numstat fails, files just lack numbers and are still listed). Don't split the whole output on `\t`, and don't assume one entry per segment.
- Untracked files from `stash -u` live in the stash's third parent: comparing the stash commit itself (the working tree) with `^1` (HEAD at stash time) gives the changed files, but untracked ones aren't there.
  When `^3` exists, compare it with the empty tree (`diff-tree --root`); those files' `Change.Commit` is the hash of `^3` (the old side is empty when opening the Diff).
  Read the parents with `log -1 --format=%P`; don't build `stash@{n}^3` (a stash made without `-u` has no third parent and that errors).
- `git restore` does nothing to submodules: discarding a submodule entry (gitlink) is `git submodule update -- <path>` (check out the commit the parent repository records),
  and there is only something to discard when the checked-out commit changed (`Change.Sub` contains `C`). Changes or untracked files inside belong to the submodule itself; tell the user to handle them inside the submodule. `DiscardAll` skips submodules.
  The commit a submodule records is not in the parent repository, so `cat-file` / `show <commit>:<path>` can't read it: to tell whether something is a submodule, check for mode `160000` in `ls-tree -z <commit> -- <path>`.
- Line-level staging always applies patches forward: unstaging / discarding doesn't use `git apply -R`; instead it writes a "change it back" patch against the new side and applies that forward (the pairing rules of `LinesEdits` are in [architecture/staging.md](architecture/staging.md)).
  For a file ending in a newline, the Diff view shows one extra empty line at the end; selecting it doesn't count as a changed line.
- `git ls-files -s` prints tracked and untracked files differently: with `--others`, tracked files are `mode sha stage\tpath` and untracked files are just the path (git records nothing about them).
  Mode `120000` is a symlink; a link to a directory is also listed as a file. When reading the whole tree, don't `Lstat` each entry to detect links (50,000 files); call `Readlink` only on the much smaller `Links` + `Others` (`files.ReadLinks`).
- `git ls-files --others --ignored --exclude-standard --directory` lists an untracked directory containing only ignored files as `sub/` (the directory itself isn't ignored):
  `files.Tree.Done` treats everything ending in `/` as a wholly ignored directory, and expanding it reads the disk one level at a time with `ReadLazy`.
- Exit code 1 from `git check-ignore` means "none of them is ignored", not an error (`git.CheckIgnore`); `--no-index` makes tracked paths follow the rules too. It understands parent directories and `dir/` rules,
  so asking about the bare empty-directory path is enough; no need to append `/`. When it can't answer (not a repository, canceled), show none of the empty directories — better to show too few.
- Untracked directory rows don't go into `OpenDiffs` (`app/gitchosen.go`): a `Change.Dir` row is only a collapsed directory, and opening it shows a single explanatory line. Skip it in a multi-selection; to pick the files inside, expand it and select them.
- `Status.DirFiles` is filled in later in the background (`app/untracked.go`): after reading status, the same goroutine runs `git.ListUntrackedDirs` and waits at most 50 ms; if that's not enough, it posts the status first and fills in the listing when done.
  A directory not yet listed has no key in `DirFiles` (draw "Loading…"); don't treat "no key" as "the directory is empty". The same directory reuses the previous listing first (`keepDirs`).
- Git dialogs of the "one name + one or two fields" kind go into `app.RefDialog` (`app/gitrefs.go` + `ui/gitrefs.go`): for a new kind, add a `RefDlgKind` and give its title / description / button /
  command preview / `run` in `refDialog`. Duplicate names and characters git doesn't allow are checked by `refNameErr`; don't add another `Open` field. To split a remote branch name into remote and branch, use `splitRemote` (it matches existing remotes; don't `strings.Cut` at the first `/`).
  Push / delete tags with `TagRemote`.
- askpass (the dialog shown when a write command needs credentials):
  - The `GIT_ASKPASS` script for write commands execs `os.Executable()`, which in tests is the test binary. Test packages that run write commands (`RunOp` / clone) must branch into askpass mode in `TestMain`,
    with the three lines from `app/askpass_test.go`; otherwise, as soon as git asks for credentials, the whole test suite runs again and hangs.
  - The script and socket live in `os.TempDir()`, and their paths must be quoted: git starts askpass with `sh -c`, so a path with a space breaks it; macOS unix socket paths can't exceed 104 bytes.
    `~/Library/Application Support` violates both, so don't move them into the state directory; quote the exec path in the script with `shellQuote`.
  - Add the credential variables only to the write command's `cmd.Env`, never with `os.Setenv`: terminals (termd, in-process shells) inherit the app's environment, and git in the terminal would start popping the dialog too.
- Wrap user-facing error explanations you write yourself in `humanErr`: `Fail` runs `git.Explain` over the raw error text, and a message of your own such as "A lock file was left behind after canceling" contains `index.lock`, so unwrapped it would be "explained" as something else.

## File tree and file system

- Temp directories live under `/var`, but `lsof` reports the resolved directory (`/private/var/folders/…`): `EvalSymlinks` both sides when comparing with the repository root.
- Copied-in directories (dragged from Finder) are copied recursively by our own code (`copyIn` / `copyDir` / `copyNode`: `Lstat`, links rewritten as-is with `Readlink` + `Symlink`, relative links stay relative). Don't switch back to `os.CopyFS`:
  it errors on symlinks, and it gives files 0o666 plus only the source's executable bits, dropping every other permission bit. Single-file copies (`copyIn`) keep permission bits as they are.
- Incremental file tree refreshes only modify the tree on the main thread (`app/treediff.go`):
  - The background `treeDiff.run` only reads the tree root captured at `startTreeDiff`; changes go back to the main thread in `applyTreeDiff` and then `Tree.Apply`. If `Gen` has changed by then (the whole tree was re-read or is being re-read), they are dropped in favor of the full read.
  - The background job touches nothing owned by the main thread: the snapshot's `Links` is a `maps.Clone` copy (`Apply` modifies `t.Links` on the main thread), and `Diff` must not call `filePaths()` (it writes each directory's path cache on the main thread).
  - `Tree.Paths` may only be replaced wholesale with a new slice; ⌘P / search may be holding the old one.
  - Deleting the last file in a directory makes `Tree.Apply` remove the directory too. To keep the now-empty directory, record it in `Extra` with `AddEmpty` before `Apply`.
  - To count `git ls-files` calls in tests, swap the `lsFiles` variable. When a test replaces a file with a link, `os.Remove` it first: `os.WriteFile` onto a link writes to the file it points to.
- Ignored directory rows sort after regular directories and before files; follow this when writing expected orders in tests.
- Finding empty directories (`files.FindEmpty`): `find -empty` descends into submodules and nested repositories, and `-name .git -prune` alone isn't enough; empty directories inside nested repositories must be filtered out by repository path (`dropNested`),
  or they show up in the outer repository's file tree. Only leaf empty directories are listed (a parent with an empty subdirectory isn't empty); a directory containing only `.DS_Store` doesn't count as empty. Repositories above `files.MaxEmptyScan` (20,000 files) aren't scanned.
- The file tree header buttons are only built while the pointer is in the sidebar or the sidebar has keyboard focus. The Tester's pointer isn't in the sidebar by default, so before clicking "New File…", "Add Repository…" and the like, a test must `tt.Move` into the sidebar
  (or set `a.FocusTree = true`) and then `Frame()` twice (`Hovered` looks at the previous frame's pointer).
- Filtering:
  - Score against paths with their original case. `fuzzy` lowercases when comparing, but word starts (uppercase letters before lowercasing) need the original; don't `ToLower` before matching (`TreeSide` would lose its bonus).
  - Files at the repository root have `name` 0: the file tree uses `MatchPath` / `ScorePath` (which treat 0 as "the whole string is the name"); `Match` treats 0 as "no name" and is meant for the command palette.
  - Don't keep every match: with 50,000 files, typing `t` matches nearly all of them, and building a `Row` for each, recording matched characters and sorting costs several MB and tens of milliseconds. `Tree.Match` yields only path and score;
    the caller keeps the best in a 500-entry heap (`topHits`) and computes matched characters with `HitRow` only for the rows displayed. Scoring uses the non-allocating `ScorePath`.
- Outline: Go uses `go/parser.ParseFile`, which on syntax errors still returns the part of the AST it parsed (not nil); list it anyway rather than discarding it because `err != nil`.
  The outline is only computed when the sidebar is in outline mode or the command palette is in `@` mode (`Doc.Outline` reuses the last result while the buffer version is unchanged); don't compute it for every tab on every frame.

## Encoding and search

- `search.Match`'s `Len` only reaches the end of the line; `Span` is the whole match: for a multi-line regex (`a\nb`), `Len` is shorter than `Span`. Use `Len` for selecting and highlighting, and `Span` for replacing and checking the match is still in place;
  don't slice content with `Col + Len`.
- The GB18030 decoder accepts many double-byte combinations: Shift-JIS and EUC-KR files also "decode" as GB18030 (no replacement characters, just mojibake), so `pickCJK` can't rely on decoding success alone; it must also choose by character proportions.
  The thresholds are in `internal/docs/encoding.go`, tuned on a corpus: samples are the first 64 KB; Shift-JIS looks at the share of full-width kana, EUC-KR at the share of Hangul and common syllables, Big5 at the share of second bytes falling in 40–7E,
  and BOM-less UTF-16 at one side having many NULs while the other has almost none. Conversely, Big5 codes that land in GB18030's user-defined area decode to replacement characters, so a Big5 file decoded as GB18030 is `Broken`.
  - Shift-JIS is counted by full-width kana only (`isKana` only accepts U+3040–U+30FF): half-width katakana (single bytes A1–DF) are lead bytes of double-byte sequences in GB18030, and decoding them that way doesn't yield kana either.
  - Run `TestDetectCJK` / `TestDetectCorpus` / `TestDetectUTF16NoBOM` before changing thresholds. Kanji-heavy Japanese and very short Big5 being detected as GB18030 is known; don't add rules for them that would misdetect Chinese.
  - Detection skips candidates that can't win: in `pickFrom(data, true)`, once GB18030 decodes, `cannotWin` looks only at bytes (`big5Low`; `kanaMost` / `hangulMost` are upper bounds on `share` when decoding succeeds)
    to know which candidates can't win and skips decoding them (10 KB of Chinese went from 99 µs to 34 µs). Run `TestPickSkip` and `TestKanaHangulMost` before changing thresholds, `score`, or the order of `cjk`.
- Not every GB18030 byte sequence round-trips: a lone 0x80 decodes to €, and € encodes back as A2 E3. So whether a GB18030 file is editable is decided in the background by `Encode`-ing it back and comparing byte by byte (`decode`'s `check`);
  files that differ are `Doc.Lossy` and read-only. In tests, use 0x80 to build a file that "can't be saved back as-is". `Encode` returning nil means the encoding can't represent some character; `write` reports an error and doesn't write.
- Whether a legacy encoding is editable is decided by `Lossy` / `Broken`, not by the encoding name (`Writable`): GB18030 / Big5 / Shift-JIS / EUC-KR / EUC-JP / Windows-1252 / ISO-8859-1 are editable if they round-trip;
  BOM-less UTF-16 stays read-only. To test the read-only bar, use a sample that doesn't round-trip (a lone 0x80 at the end); don't treat ordinary Big5 as read-only.
- `Settings` contains a map (`Formatters`), so compare two settings values in tests with `reflect.DeepEqual`, not `==`.
- Search works on decoded text, so coordinates are buffer coordinates:
  - `search.readFile` first checks `utf8.Valid` (no extra allocation for UTF-8); only if that fails does it call `docs.Sniff` (the same detection used when opening files). The decoded string is borrowed as a read-only `[]byte` via `unsafe` and dropped after searching;
    `Match.Enc` records the encoding. Unrecognized files are searched as raw bytes, with U+FFFD in the preview.
  - Replace: for open documents, `Plan` on `Buf.String()` → `Buf.Edit` (clean documents are encoded back with `d.Enc` and written to disk); for unopened files, replace only if `docs.Decode` says editable, then write back with `Encode` (nil reports "can't encode back to %s" and doesn't write).
  - If a tab's encoding was changed manually and differs from what search detected, `Plan`'s check fails and the result counts as stale. Details in "Search and replace" in [architecture/editor.md](architecture/editor.md).
- When results follow changes, only "files that changed" are replaced (`app/searchfollow.go`): don't re-run the whole search to follow the disk, and don't touch files whose `ver` hasn't changed. While "Replace all?" is being asked, `ver` decides whether the results are stale;
  bumping versions for no reason turns the user's click on "Replace" into "the search results changed, so nothing was replaced". Any new path that changes results (replace, undo replace, following updates) goes through `splice`, preserving (repository order, path) order.
- regexp2 (`search/advanced.go`):
  - Get positions through the string API: `FindRunesMatch` gives rune indexes; only `FindStringMatch` gives byte positions (`bytesString` borrows the `[]byte` without copying; don't modify it until you're done).
  - `MatchTimeout` applies to a single match call, not the whole file: a timeout skips that file (`Result.TimedOut`).
  - Group numbers only follow order of appearance, like RE2, with `OptionMaintainCaptureOrder()`; `$1` is expanded with `expandTemplate` (`TestExpandTemplate` checks it against `regexp.Expand`).
  - Only switch to regexp2 when RE2 can't compile the pattern and `advancedSyntax` recognizes one of the supported constructs; don't send ordinary regexes to the backtracking engine (slow, and it can time out).
- External commands that rewrite text go through `formatDoc` (`app/format.go`): they run in the background, and the result is applied back on the main thread only if `Buf` and `Version()` are both unchanged, in one step with `docs.LineEdits`.
  Empty output is treated as failure so the file isn't wiped; on timeout the whole process group is killed (`Setpgid` + `Kill(-pid)`).

## Copy and i18n

- The app defaults to English, but only in the shipped binary: the settings `lang` value `""` means "not chosen" and resolves through `i18n.Default`, which `cmd/workbench` sets to `"en"`; `"zh"` / `"en"` are explicit choices. State files written before format version 5 get `"zh"` on load so existing users stay on Chinese. Inside tests nobody calls `i18n.Default`, so `i18n.Set("")` is still Chinese (the source language) and test assertions stay in Chinese.
- All UI copy goes through `i18n.T("中文")` / `i18n.Tf("…%s…", x)`; never concatenate translated fragments (English word order differs).
- `TestCatalog` only collects literals containing Chinese characters. Format strings without Chinese, like `"%s：%s"` or `"，"`, don't go into `zh-CN.json`, so the English UI keeps the full-width punctuation.
  To join pieces, use a format with Chinese in it (`"已暂存：%s"`), or a separator that works in both languages such as `" · "`. "Git", shortcuts and the like never go into the table.
- `TestCatalog` forbids indexed placeholders (`%[2]s`): the English translation must keep the same argument order as the Chinese, and its sentence is shaped to fit.
- Each Chinese string maps to exactly one English translation. "变基", "还原", "拣选" and "合并" are operation names in the copy table (lowercase rebase / revert / cherry-pick / merge in English, used in banners and queue labels);
  buttons that need a capitalized label get their own Chinese string ("开始变基" → "Start Rebase", "还原提交" → "Revert Commit"). Don't change the English of the operation names.
- Reasons for disabled menu items use `ui.withWhy` (full-width Chinese parentheses, or ` (…)` in English, by `i18n.Lang()`); the reason itself goes through `i18n.T`. Shortcut parentheses in tooltips use `app.Tip`; don't build them yourself.
- To get mygo's built-in strings (find bar, go to line, DiffView folded lines, SearchField) in Chinese, `import _ "github.com/egoist/mygo/locales/zh-Hans"`
  and call `mygo.App.SetLanguages(i18n.Locale())` (in `cmd/workbench`; `OnSettings` calls it again when the language changes). Tests use `Tester.SetLocale(i18n.Locale())` (already set in `newUI`),
  so mygo text that tests look for follows the current language, which is Chinese by default in tests.
- When `wui.SearchField` has no `Placeholder`, the placeholder is a localized "Search". Still put what the field is for in its Tooltip / Label.
- Tests that change the language must `t.Cleanup(func() { i18n.Set("") })`, or later tests keep running in English.
- For copy whose format depends on "today", compute test expectations with the same function: `LocalStamp` (`app/localhist.go`) gives `14:32` for today, `11-01 14:32` for other days this year, and includes the year for earlier years.
  A test that hard-codes a timestamp on some day and also hard-codes `"14:32"` fails once the clock passes midnight. For all copy that depends on the actual time (`LocalStamp`, relative times), always build the expected value with the same function.

## Memory and performance

- The Go heap doesn't shrink right away once the app goes idle: after a burst of short-lived allocations (search reading tens of thousands of files), an idle app has no new allocations to trip the GC threshold, so the garbage waits for Go's forced GC every 2 minutes,
  and meanwhile it all counts toward the footprint (~23 MiB extra in "all open"). Background jobs that read a lot at once and release it when done should call `debug.FreeOSMemory()` once after they have fully finished
  (as `SearchNow` and `loadWtInfo` do). Don't call it every frame or on every keystroke.
- The startup peak (two hundred-plus MiB in the first few seconds) is mygo's Metal renderer drawables and has nothing to do with app data (at the peak the Go heap is the same as when idle); don't attribute it to app data when investigating.
  Broken down by category with `footprint -p PID --sample 0.1 -f bytes`: right after a maximized window opens there are 3 IOSurface drawables totaling ~87 MiB, plus ~55 of graphics unmapped, and it takes about 2.5 s after the first frame to drop back to 1 drawable.
  Every window opened adds another set of both, so windows opened closer together than ~2.5 s stack their peaks. That's why `RestoreGap` defaults to 2.5 s; don't just shorten it. Before changing it, re-measure the startup peak of two windows this way
  (`TestRestoreGapDefault` checks the default).
- Frame memory: mygo only releases window-sized frame memory after 2 seconds without a new frame. A blinking caret, animations, or any component that keeps requesting frames in the background holds on to it, so measure memory both focused and unfocused.
- When a window is completely covered by other windows, mygo stops producing frames (`holdFrame`) and only draws the first one, so idle memory is abnormally low (around 63 MiB). Numbers measured this way can't be compared directly with others;
  measure with a fixed window position (see the single instance / real windows / memory measurement section of `AGENTS.md`).

## Markdown preview rendering

- A WebView can only be created in a Content window, and it isn't visible unless an element is placed (`internal/render`): a `win.NewWebView` with no `ui.WebView` element attached is just a page running in the background, which is exactly what rendering needs.
  The page's protocol (`wbrender://`) must be registered with `mygo.Protocol.Handle` before any window is created (`render.Register()` in main); register it later and WebKit won't recognize it.
  `Page.EvalContext` can be called from a background goroutine, and the renderer goroutine relies on that; don't wait on it from the UI thread.
- A tainted canvas can't export PNG: when an SVG containing `foreignObject` (which mermaid's default `htmlLabels: true` produces) is drawn to a canvas, `toDataURL` throws a SecurityError.
  Mermaid in `page.html` must use `htmlLabels: false` (including `flowchart.htmlLabels`); don't turn it back on when changing the mermaid config.
- MathJax errors are `TexError` (not `Error`, so `EvalError` only shows `[object Object]`); wrap them in an `Error` before throwing (MathJax 4 uses `tex.formatError`). Its SVG bbox hugs the glyphs, so leave padding around it, or the result gets clipped.
- MathJax 4 switches like `options.enableEnrichment` / `enableSpeech` are overridden by the menu's default settings: unless `enrich`, `speech` and `braille` are turned off in `options.menuOptions.settings`,
  it still loads `sre/speech-worker.js`, and since the render page doesn't have that file, it waits until it times out.
- MathJax 4 line-breaks inline formulas by default, so a long inline formula is split into several `<svg>`s and `querySelector("svg")` only gets the first piece (wrong width). Use `svg.linebreaks.inline: false`.
- MathJax 4's fonts aren't all in `tex-svg.js`: bold, calligraphic, double-struck and so on are loaded on demand from `[fonts]/mathjax-newcm-font/svg/dynamic/*.js`, by default from a CDN;
  `\ce{}` needs mhchem's font extension. `loader.paths.fonts` points to the local `mathjax/fonts`, and everything used must be bundled (`TestAssetsPresent` checks a few).
- mygo's `ui.ParseSVG` doesn't draw `<text>`, `<image>`, `<marker>`, `<pattern>` or `filter`; it doesn't error on them, it just leaves them blank. In MathJax's SVG, characters missing from the font are `<text>`,
  so the render page checks first (`vectorOK`) and renders anything containing these as PNG. mygo draws path-only SVGs very well, and `currentColor` is the text color at draw time.
- In mermaid.js 11 without HTML labels (`htmlLabels: false`), text in mindmap circle nodes (`((root))`) is laid out from the center to the right, so half of it spills outside the circle (white text on a light background disappears):
  the render page's `polish` adds `text-anchor="middle"` to that text. Mermaid's config has no node corner radius either, so `polish` also changes `rx` / `ry`.
- The rendering scripts are stored as `.gz` in the embed (16 MB → 5 MB). `mygo.FileServer` doesn't decompress them, so `web.go`'s `assetHandler` decompresses them itself and calls `http.ServeContent` (the type comes from the name without `.gz`).
- mygo paragraphs only ask `InlineMath` again when their content changes: if an inline formula isn't ready the first time, the paragraph is laid out with the source text and won't re-lay itself out once the formula is ready. `mdRender` returns `Renderer.Gen()`,
  and the preview uses it as part of the key, so the key changes whenever a batch is ready. `parser.ExtDollarMath` isn't in `parser.Default`; OR it in yourself to recognize `$…$`.
- `ScrollIntoView` takes effect after this frame's layout, and it lays out the tree built this frame. A link's `Clicked` is only known midway through building the tree, so if a jump set then places its marker in the same frame,
  the marker often lands before blocks already built, or never gets placed, and the next frame clears it as "already handled". The preview only honors the `d.Anchor` / `d.AnchorLine` present at the start of a frame (`mdEnv.anchor` / `line`).
- In a `ui.List`, calling `ListState.ScrollTo` and then `ScrollIntoView` on a newly laid-out row in the same frame wastes the second call: the list lays out rows from the `ScrollTo` position, overriding the row's reveal.
  The segmented preview waits for the frame after `ScrollTo` before placing the marker (`mdView.seek >= 2`).
- To scroll a block to the top, use `ScrollIntoView` (default `ScrollNearest`) with a transparent marker taller than the view: anything taller than the view is revealed from its start, which puts it at the top.
  But there must be enough content after it: the last section can't scroll to the top, only to the bottom of the scroll range. Tests must pad more than a screenful of text after the target, or it looks like "it didn't scroll".
- `MeasureText(width, …)` can't measure min-content: when a word doesn't fit, mygo's text layout breaks it mid-word, so measuring at 1pt width gives the width of a single character, not the longest word.
  Computing a table column's minimum width (`minContent` in `gmd/table.go`) splits at spaces, hyphens and every CJK character, and measures only the longest few pieces.
- Children of `ScrollHorizontal` don't shrink to the viewport: `Shrink(1)`, `WidthPercent(100)` and `Grow(1)` are all computed against the content (infinite width). For "as wide as the viewport when it fits, scroll only when it doesn't",
  don't wrap it in a scroll container when it fits (in a normal column, `FillWidth().MaxWidth(…).MinWidth(…)` clamps it in between); wrap it only when it doesn't. Decide from the previous frame's width,
  and if `Draw` finds this frame's layout crossed over to the other case, call `c.Invalidate()` to request another frame (`Invalidate` can be called from anywhere).
- After a side-by-side preview is dragged into the source file's group, the source editor stops drawing and ged's `TopLine` stays at the last drawn frame; once the preview pushes a line to it (`ScrollToFrac` only takes effect when drawn),
  if the next frame arrives after the quiet period, the preview thinks the editor scrolled and pulls itself back. Check `a.EditorShown` before following the editor's scrolling.
- In segmented Markdown, each segment is an independent `parser.Document`: the last block of a segment has `b.Next == doc.Len()`, and computing a line range "to the end of the document" sweeps in every later line;
  clamp it to the first line of the next segment.

## Testing and Tester

How testing is divided up and how to measure is in [testing.md](testing.md). This section lists things to watch for when writing tests.

Driving and frames:

- `ui.NewTester` draws the first frame as soon as it's created, using the values already in state.json (or `Default`, with all three areas open, if there are none). For a test to start from a state like "bottom bar closed",
  write state.json before `newUI` (`termState` / `setBar` in `terms_test.go`); changing `a.St` after creation is too late, since the first frame has already started the terminal.
- The Tester advances frame by frame: `tt.Advance(time.Second/60)` draws at most one frame, and `settle` (`Frame()`) draws up to 20 frames in a row. Results returned from the background through `a.Update` need `c.Invalidate()` on a retained `*wui.Context`
  to schedule the next frame (`measureUI` in `TestMeasureBigFile`).
- `OnTextKey` callbacks run between frames: state changes inside them go through `a.Update`, so tests must `wait` for the queue to drain before they can see them; calling `tt.Frame()` alone isn't enough.
- An `Element` is only valid in the frame that built it; calling `.Bounds()` on it in the next frame panics ("element from pass N used in pass N") and aborts the whole package's tests.
  To measure positions across frames use `wui.Handle`: `.Bind(&h)`, then outside the frame `h.Bounds(c)` (`c` is the `Context` the frame was built with).
- Between two clicks on the same row, `tt.Advance(time.Second)`, or they're recognized as a double-click.
- After `a.Ask`, call `tt.Frame()` before looking for the confirmation text: `Confirm` is an overlay drawn on the next frame.
- An open dialog blocks the Tester's keys: while `AlertOpen` / a confirmation card is open, `tt.Key` doesn't reach what's underneath. After closing it (`a.AlertOpen = false` or clicking a button), `tt.Frame()` once before sending keys.
- Toasts are cleared by the next frame (`a.Toast` is emptied): UI tests check notices with `tt.HasText`; only tests in the app package compare `a.Toast` directly.
- `app.New` only reads state and doesn't reopen the last tabs; in a real window `Start` is called by `Windows.Open` (`app/windows.go`). When testing session round-trips, call `a.Start()` yourself after recreating the App,
  or the tab list is empty and looks like an unsaved session.
- The settings window is a second `mygo.Window` sharing one `App` with the main window; tests use a second `wui.NewTester(SettingsView(a), …)`, and after changing settings the main window's Tester needs one `Frame()` to see it.
- `view.files` (⇧⌘E) toggles the sidebar, so calling it while the sidebar already shows Files closes the sidebar. To make sure Files is open in a test, use `a.ShowSidebar(workspace.ModeFiles)` + `a.EnsureFiles(r)`.
- `mygo.App.SetMenu` can't be called under `go test`. The Tester's `Menu()` doesn't include the disabled state; to check whether a menu item is disabled, select it with `ChooseMenuItem`, confirm the error contains "disabled", then call `CloseMenu()`.

Finding elements:

- Tester `Label`s / text must not be duplicated, or `tt.Click` may hit the wrong one: tab rows use `"标签 "+name` ("Tab %s"), editors use `"编辑 "+name` ("Edit %s").
- `Find` returns the first match in tree order (title bar → sidebar → main area → bottom bar → right panel → status bar → dialogs); when the same text appears in two places, locate it by a unique Label.
- `tt.Click` / `tt.RightClick(text)` click the center of the element found. When a Markdown paragraph contains only a link, `Find("link text")` matches the whole paragraph spanning the column,
  whose center falls on the blank space right of the link, so the click misses (and the context menu is the selection's, not the link's). In tests, put the link in the middle of a sentence (`去看[另一篇](other.md)吧`).
- The Tester's clock doesn't run: `c.Now()` only changes on `tt.Advance(d)`; `tt.Frame()` doesn't move it. Tests of time-based debouncing (the preview re-lays out 150 ms after typing stops) must `Advance` past that interval.
- `Find` requires the whole text to be exactly equal; `HasText` is a substring match. Use `Find` for composed text like "2 stashes" or short text like "demo/docs" on the scope chip;
  result rows carry the path in their Label ("Result file demo/docs/b.md"), and `HasText` would match those.
- Once a `Text` has a `.Label`, `Find` / `Texts` see the Label, not the displayed text. To find by displayed text, don't give the text a Label.
- For rows with a Label (tree rows, history rows), `Find` returns the whole row's rectangle (tree rows start at x=4), not the span of the name. To measure positions, pin X with text that appears only once (a short hash); `Find(title).Y` is the row's top;
  to measure names and icons, scan pixels with `inkRuns`. `Find` rectangles are in DIPs; `Image()` pixels must be multiplied by the `SetScale` factor. `TreeSpans` holds the previous frame's positions, so after changing scroll / rows, `Frame()` a few more times before checking the sticky stack.
- `wui.Toggle` (the pill switch) is a toggle button in the accessibility tree (`roleToggleButton` 31, pressed = on), not a Switch (6): when searching `accessTree` for switches by role, use 31.
- `TestAccessLabels` reads unexported Tester fields via reflect, which is why mygo is pinned. When upgrading mygo, `TestAccessProbe` fails first; update the field names according to its error.

Pixel measurement:

- The soft background is a gradient, so two pixels a few pt apart naturally differ by 1 or 2. All color comparisons (guide lines, hover fills, text ink) use `inkDist` with a tolerance, never `==`.
- Colored fills don't count as ink: a fill differs from the background by only 30–80, so `inkRuns` (threshold 90) only picks up the letters / icons on top. To measure fills, use `inkRunsOver(…, 20)` and scan only the stretch before the name (with too low a threshold, indent guides also register as fills).
  To measure the file tree's colored badges, first set `a.St.Settings.TreeIconsPlain = true` + `SettingsChanged()` + `Frame()` twice.
- Pixel-measuring tests (`TestCardGeometry`) need `Backdrop: "flat"` and dark mode (solid background, no shadows) to be clean; the soft color blobs bleed into gaps. The Tester isn't translucent by default, so the root is the window background color, and the default soft backdrop is drawn in every UI test.
- With Tester `SetVibrancy(true)`, the material area is transparent pixels (same as `WORKBENCH_CAPTURE`). Screenshot tests use `writeMaterialShot` to lay down a synthetic frosted glass, which is only an approximation; how a real window looks through it must be tested by hand.
- Native menus (context menus, a button's `Menu`) aren't in `tt.Image()` / `WORKBENCH_CAPTURE`; tests use `tt.Menu()`, `tt.ChooseMenuItem(…)`, `tt.CloseMenu()`.
  `WORKBENCH_CAPTURE` images don't include the traffic lights either.
- The system context menu (`ContextMenu`) gets truncated near the bottom of the window (issue #6): mygo's darwin backend pops it with `popUpMenuPositioningItem:nil atLocation:inView:` (true from v0.7.4 through v0.8.3; the first argument in `internal/darwin/menu.go` is still 0),
  pinning the top-left corner at the click point. When there isn't room below (down to the screen bottom / Dock), AppKit doesn't flip it upward; it truncates the menu and shows scroll arrows at the bottom. Grove can't reach that call (nor change the position),
  so the file tree's context menus (`treeMenu`, `rowMenu`, repository rows, rows pinned at the top, each segment of a compacted directory) are drawn inside the window (first as `PopoverMenu`, and since issue #48 by Grove's own
  `ui/gmenu.go`), opening upward when there's no room below and scrolling only when even the window can't fit them. Their look follows the theme, and the frosting is our own (next item). Later (issue #58) most context menus and button menus switched to it too; the only ones still drawn by mygo (`SetMenuPresentation(PopoverMenus)`) are the editor's context menu, the settings window's "More", and the title bar breadcrumbs. The list is in the menus section of [architecture/files.md](architecture/files.md).
- Panels drawn inside the window get frosting from `Element.Material` + `plugins/glass` (v0.9.0, same module as mygo, no extra dependency): `glass.Blur{Radius}` only blurs and adds no tint,
  and `Material` replaces `Background`, so the tint must either be a separate semi-transparent child layer or (the current approach, `frostMaterial` in `ui/frost.go`) a custom `Material`:
  in `PaintMaterial`, call `glass.Blur.PaintMaterial` first, then `p.FillRadii` to lay the tint over it. The child-layer approach gets squeezed in panels with `Gap` / `Padding` (dialogs, the command palette), taking up a gap's worth of space; `glass.Glass{}` is the macOS 26 style glass with a bright edge.
  It blurs the elements drawn beneath it in this frame, not the desktop behind the window (that's the window's `Vibrancy`). With "Reduce transparency" on, `glass.Blur` draws itself as the theme's `Background`,
  which isn't the menu's background color, so Grove checks `c.Preferences()` itself and falls back to `Background(gmenuBg)`. The headless Tester can draw it (glass has a CPU implementation),
  and it has been compared against Metal offscreen rendering: corner radii and colors match. mygo's `PopoverMenu` / `ContextMenu` have no hook for changing the panel's look (only `PopoverMenuButtonOptions.Panel`),
  so their menus can't be frosted.
- mygo v0.9.0's system menus still use `popUpMenuPositioningItem:nil` (`internal/darwin/menu.go`); `PopoverMenu` still can't do press-right-drag-onto-item-and-release,
  still opens submenus instantly on hover, and the click outside still falls through to what's underneath (from reading the source, not retested in a real window), so the file tree menu is still drawn by Grove.
  The Tester's `Menu()` / `ChooseMenuItem` can't see drawn menus: tests uniformly use `shownMenu` / `chooseMenu` / `closeMenu` from `popmenu_test.go`
  (they handle both kinds; drawn menus are found in the accessibility tree as RoleMenu / RoleMenuItem, so `gmenu.go`'s panel and items must set `Role`, `Label`, and `Disabled` on grayed items). `mustRightClick` already uses them.
- `PopoverMenu` (v0.7.4; main `6121009b`'s `SetMenuPresentation(PopoverMenus)` is also it, so these points still hold) opens on right-button press on macOS (`menuPress`), and since no element is "pressed" when it opens, item hover highlights follow the pointer while the right button is held;
  but releasing the right button doesn't select the item under the pointer (items only respond to left-button `Clicked`), submenus open instantly on hover with no safe triangle, grayed items have no tooltip, and the click that closes it by clicking outside still lands on the element underneath.
  Fixing these requires drawing our own menus (issue #46), which is why the file tree switched to Grove's own (issue #48, `app/popmenu.go` + `ui/gmenu.go`).
  The Tester only has left-button `Press` / `Release`, and for the right button only `RightClickAt` (press and release together), so right-drag can't be tested: the menu's state machine lives in `app.PopMenu` and is tested by feeding it events directly.
- Notes on drawing our own menus (issue #48): the right-button press is taken by the row's `HandleInput`; once taken, the moves and release while held are all delivered to that row (wherever the pointer is), so press-drag-release needs nothing else.
  An element has only one `HandleInput`, so the ones already attached (`treePress`, the list's `treeCommands`) must be wrapped inside it, forwarding everything that isn't a right-button event as before. A press that a `HandleInput` declines
  is not passed on to an outer element's `HandleInput` (`rt.handler` only takes the innermost one), so the list's blank-area menu only opens where no `HandleInput` is attached.
  Swallowing the click outside relies on a receiver layer in the Overlay covering the whole window (`Absolute` + window size + `HandleInput` taking the press); it calls `Focus()` every frame to keep keyboard focus,
  and `Element.Focused()` reports false when the window loses focus (mygo's `windowFocus`), which serves as "close on losing focus". Callback coordinates are element-relative; to convert them to window coordinates, use the `Bounds()` recorded while building the UI
  (from the previous frame); callbacks must not touch elements.
  When an Overlay closes, mygo returns the keyboard to the element that had it in the frame the Overlay appeared (`openOverlay` / `restoreFocus`), but it records the focus after the Overlay's `fn` has run:
  if an element inside calls `Focus()` in that first frame, what gets recorded is that element itself, and after closing the keyboard has nowhere to go. The receiver layer must wait one frame before calling `Focus()`.
- mygo main (from `6842ad5e`) validates in `Tester.Click` / `RightClick`: the element holding the label (the first input-accepting ancestor) must be the same one that receives input at the click point;
  if not, it reports `"xxx" is not the input target`. A list row's name (`ListOptions.Label`, used for type-to-jump and screen readers) is recorded on the List's cell, but clicks are received by the row inside the cell
  (file tree rows' `menuPress`, Git panel section headers, command palette result rows, history rows); the history right panel's title row has its name "History" on the whole row, with the search box in the middle. In a real window, clicking that cell is clicking the row,
  so it's not a regression. In tests, click such rows with `clickRow` / `rightClickRow` / `mustClickRow` / `mustRightClickRow` from `clickrow_test.go` (they find the cell by name and click its center);
  use `Click` for everything else, keeping its validation.
- From the same version, all key presses go to `HandleInput` (`d3288030`): Grove's callbacks (`treeCommands` only accepts `InputCommand`, the menu receiver layer lets ⌘ / ⌃ through, the image viewer only accepts unmodified = - 0,
  and the shortcut-recording card is meant to take everything) all filter by kind and key, so they don't take extra events or swallow keys.
  A receiver-layer callback returning false doesn't pass the press on to the element beneath it either (`hitChain` only walks this layer's ancestors and never leaves the Overlay), so "right-click elsewhere while open opens a new menu"
  has to find the target itself from registered positions (`PopMenu.AddTarget` / `TargetAt`); right-clicking outside the file tree only closes the menu.
  ⇧F10 / the menu key: mygo only opens menus for elements with a `ContextMenu` / `PopoverMenu` attached (`menuKey`), so once `PopoverMenu` is removed those keys do nothing;
  handle them yourself in the focused element's `HandleInput` (`InputKeyDown` goes to it first, and if it takes the key mygo stops looking for a menu).
- List row presses (issue #46): `ListState.DragSelecting()` only reports true after the pointer has been dragged some distance while held, so it can't be used to mean "still pressed". The list selects an unselected row on press,
  so the app can't tell press and release apart. To select on press and open on release, have the row take `InputPointerDown` itself through `HandleInput` (after that, moves and the release belong to it),
  and keep attaching `HandleInput` every frame until the release (if it isn't attached, the callback on the state is gone and the release can't be delivered). Callbacks can't use the `wui.Element` from building the UI (it panics with
  "element from pass … used in pass …"); for positions, use the rectangle recorded in the previous frame. For an element with `HandleInput` attached, mygo counts double-clicks against it even if the callback declines the press
  (`countPress` takes whichever of handler and target is innermost), so a plain click on the same row right after a ⇧-click is counted as a double-click; in tests, `Advance` between the two clicks.
- The headless Tester software-renders the whole window every frame, with a base cost of about 1.8 ms per frame, so measured frame times are higher than in a real window. A background highlighting catch-up frame takes 0.1 ms, and since the Tester draws frames back to back, 1800 frames finish in under 1 s
  while the background work hasn't finished yet. So don't count frames when measuring time: `TestMeasureBigFile` measures wall-clock time (it stops only after both 1800 frames and 30 s have passed); "done in N frames" is only meaningful against a real 60 Hz window.

Terminal, search, watching:

- Calling `tt.Type` right after the terminal starts makes the shell echo it as a line of "typeahead" before the prompt appears: wait for the prompt first (`testutil.WaitTermReady`), then type.
- Testing ⌘-click on links in the terminal: `tt.HoldModifiers(wui.Cmd)` + `tt.Move` sweeping vertically; `Terminal.HoveredLink()` gives the link and its rectangle relative to the view; then `tt.ClickAtWith(wui.Cmd, x, y)`.
  Check URLs with `tt.OpenedURLs()` (recorded only, not actually opened).
- In tests the termd socket lives in `t.TempDir()`: `termd.Server` runs in the test process with `Listen` / `Serve`, with `t.Cleanup(srv.Stop)`. macOS unix socket paths are limited to 104 bytes,
  and `t.TempDir()` is long, so use `SockPath(t.TempDir())` (which switches to a short path under `$TMPDIR` when too long), and delete the `.lock` too in `t.Cleanup`.
  UI tests replace `app.TermdStart` (don't actually start a child process); only `TestSpawnDetached` starts a real child process, and it shuts it down at the end.
- After typing, wait for search content, not just `Done`: typing is debounced by 150 ms, during which `Done && !Running` still reflects the previous search, so `wait` passes immediately.
  Wait until the result count and `Preview` contain the term (`searched` in `searchbits_test.go`).
- Before testing "should not fire", wait for a probe to fire first (`probe` in the `internal/watch` tests): FSEvents delivers in order, but events from creating a repository, committing or switching branches can arrive hundreds of milliseconds late under load,
  and simply waiting a while lets them fall into the "should not fire" window. Change a file that will fire, wait until it does (everything before it has arrived), then wait quietly.
- Assertions like "didn't do an extra pass" must guard against other watchers butting in: a `git status` run inside a worktree incidentally rewrites `.git/worktrees/<name>/index`, and a late Git change on the main repository's watcher triggers a full recomputation.
  Before asserting "only that one was recomputed / the count didn't change", wait until things settle (all repositories idle, no full recomputation in progress) before recording the baseline; if you hit interference, retry a few times with fresh input and assert that the path was taken at least once (`app/wtwatch_test.go`).

Don't disturb the development machine:

- Tests never call `copyText` or any clipboard action; those change the test machine's system clipboard. Web pages go through `a.OpenURL` and Finder / opening folders through `a.ShowPath`; replace them with functions that only record
  (`newUI` sets both to no-ops by default).
- `git log` / `git diff` run in tests and screenshots need `--no-pager`: git in the terminal opens less, and the screen sticks at `(END)`.
- Submodules from local paths in tests need `protocol.file.allow=always` (since git 2.38, cloning submodules from local paths is disallowed by default). `testutil.SubmoduleRepo` passes its own `-c`;
  initialization that goes through the app's `RunOp` must first `t.Setenv("GIT_CONFIG_COUNT", "1")` plus `GIT_CONFIG_KEY_0` / `GIT_CONFIG_VALUE_0` in tests. The app itself doesn't add this config.
- Tests don't run real lint / format tools: go vet, tsc and cargo write GOCACHE / `target/`, are slow, and may not be installed. Always use a one-line shell command (`printf '…'; exit 1`)
  or a fake script written into `t.TempDir()` (when a screenshot needs a realistic command name, write `#!/bin/sh` scripts named go / tsc), set `Settings.Shell` to `/bin/sh`, and shorten `LintTimeout` when testing timeouts.

Real windows and native drag and drop:

- The Tester can't do native drag and drop: `DropData` only exists inside an AppKit dragging session, so tests call `App.DropOnDoc` / `DropOnTerm` directly. Try the UI layer in a built .app:
  a small Swift `NSDraggingSource` window as the drag source (text / file URL / file URL + file-name text, the last mimicking Finder), with CGEvent press → drag → release. These scripts aren't in the repository.
  Don't drive Finder with AppleScript: it times out (-1712) and may pop up authorization dialogs.
- With unsaved tabs, TERM doesn't make the app quit: the app is asking whether to save, the process stays alive, and the next real-window test may hand its arguments to that instance. For an instance you started, first confirm `WORKBENCH_STATE` is a temp directory, then `kill -9` its own pid;
  never kill by process name, or you may hit the Grove in daily use.
- A freshly ad hoc–signed build hangs when reading a removable volume: macOS requires an authorization the first time each newly signed app reads a removable volume (an external drive and the like), and with nobody to approve it, the app blocks in `open()` (visible with `sample`)
  and the window never appears. When measuring memory or running throwaway builds, point the workspace at a clone on the internal disk, not at repositories on an external volume.

Miscellaneous:

- Create `TestScreenshot`'s output directory first.
- Don't write two consecutive single quotes in comments: gofmt turns `''` into a curly quote (`'\''` becomes `'\”`).
- Before creating a test file, check whether one with the same name exists: shell redirection (`cat > …_test.go`) silently overwrites an existing test. Write new files with the editor tool's "create" (which errors if the file exists), or `ls` first.
- In zsh, `echo ====` errors (a leading `=` triggers filename expansion); quote it.
