# Appearance, language and shortcuts

[← Architecture overview](../architecture.md)

Styles, window backdrop and colors, the app icon, and the UI language.

## Appearance, language and shortcuts

### Window backdrop and colors

#### Two styles

The style is `Settings.Look`, implemented in `ui/look.go`:

- The default `""` is **VS Code**: modeled on the Light 2026 / Dark 2026 themes from VS Code 1.147 on, with opaque flat surfaces.
  The three surface colors are `vsSide` (title bar, sidebar, status bar, terminal, right panel), `vsEditor` (editor groups) and
  `vsLine` (lines); `vsActiveLine` is the line on the current tab and bottom panel title (black in light, #BFBFBF in dark), and
  `vsActivityLine` is the line on the view switcher. The accent color defaults to "Black"; "Blue" is VS Code's #0069CC / #297AA0.
- `grove` is **Grove**: rounded cards on the window backdrop; see [Grove style: window backdrop and cards](#grove-style-window-backdrop-and-cards) below.

**How switching works**: the small functions that pick colors (`cardBg` / `windowBg` / `headerBg` / `cardLine` / `diffBase`) only
get the theme, not the `App`, so every frame `applyAccent` (for the main window's `root`, the settings window and the About window)
first does:

- `setLook`: stores the style in the package-level `vsLook` (read by `vscode()`), stores the overlay opacity from the settings in
  `frostPct`, and calls `highlight.SetDocComments` (see [VS Code style: code colors](#vs-code-style-code-colors) below).
- `applyLook` (VS Code style only): swaps this frame's theme text colors, line colors, `Surface` (the white of text fields, which
  mygo's text fields and dropdowns use as their background) and `ScrollbarStyle`, and replaces `Inverse` / `InverseText` with the
  overlay background and body text color.

#### VS Code style: layout

- `card` has no rounded corners or shadow; `panelCard` (bottom panel, right panel) has the sidebar color as its background and
  no border.
- The gap in `center` is 0 (the drag handle is still `cardGap` wide, and its background is just the sidebar color); there is a
  `vsLine` across the top of the root's middle row and across the top of the status bar.
- Editor tabs still use `panelTab`, with a `vsActiveLine` across the top of the current one and no `focusBar`; bottom panel tabs
  use `panelTitle` (no cells, with a line under the current one).
- `segTabsOf` with icons only (the sidebar view switcher) is flat, with a line under the current one.
- The title bar's `branchButton` is flat (transparent background, weight 400, `toolFill` on hover); Quick Open is modeled on the
  command center (`quickOpen`).
- Secondary dialog buttons use `vsSecondary`.
- The window is opaque: `vibrancy` in `cmd/workbench` never uses a system material in the VS Code style.

#### VS Code style: scrollbars

`applyLook` replaces the theme's `ScrollbarStyle` with `vsScrollbar`: 10 wide, square, flush with both ends, with a transparent
track and a 25% gray thumb (40% on hover, 55% while dragging; lighter than the 40% of VS Code Light+ / Dark+, since the 75% thumb of
the 2026 themes is too solid). The ged Diff overview ruler is 14 wide.

#### VS Code style: sidebar and lists

- Sidebar lists have no horizontal padding (`sidePadX()`) and square selection blocks (`sideRowRadius()`; with solid selection,
  `newListLook` swaps in a theme with radius 0).
- The three tint levels go through the VS Code branch of `softFills`: all three are neutral grays, and the active selection is just
  one shade darker. `listLook.rowAt` draws no outline (VS Code's blue list.focusOutline is not copied) and no bar, and the theme the
  list is built with has `Radius` 0 (mygo uses it to draw the hover block).
- The file tree's indent guides (`drawGuides`) use `vsGuide`, with other levels at 40% of it; the semi-transparent lines lie over
  the selection and hover backgrounds without gaps (`TestVSTreeHoverGuides`).
- The sticky stack's background is `vsSide`.
- Matches in `hitText` are only bolded, not recolored.
- Sidebar view titles use the font size `sideTitle()` (11 in VS Code style; in Grove style, the same as `sideSub`).
- Symbol glyphs (`symbolGlyph`) are colored by `symbolColor`, following VS Code's symbolIcon.* defaults: classes #D67E00,
  functions #652D90, variables #007ACC, in dark #EE9D28 / #B180D7 / #75BEFF, everything else gray.

#### VS Code style: code colors

- Code colors use `vsCodeTheme` (token colors are the classic Light+ / Dark+, everything else follows 2026), swapped in within
  `fullCodeTheme` / `softTheme`.
- ged Diff colors use the VS Code branch in `gedStyle` (`PlainNumbers` keeps the line numbers of changed lines from changing color).
- The editor's selection, matching brackets and find matches are overlaid with `accentTint`: normally the accent color, but in dark
  mode, when the accent color is gray, light gray #D4D4D4 instead. `vsSelection` is 25% of it (33% in dark).
- Python docstrings are strings in the VS Code style and comments in the Grove style: `setLook` calls
  `highlight.SetDocComments(Look != "")`, which changes only the token class, not the tokenizer state, so open editors switch on
  the next frame.

#### VS Code style: overlays, menus and tooltips

- Tooltips are light, as in VS Code: the `Inverse` / `InverseText` swapped by `applyLook` turn mygo's Tooltip and notification bars
  into the overlay background and body text color, and `tipStyle` adds a `vsTipLine` border.
- Overlays (Grove-drawn menus, the command palette, dialogs, editor hover tips and completion) have `vsOverlayBg` as their
  background. Frosted glass (`ui/frost.go`, modeled on VS Code's frosted glass) exists in both styles: it blurs the content
  underneath and lays a tint over it at the "Overlay opacity" from the settings; see [menus.md](menus.md#frosted-glass).
- Grove-drawn menus (`gmenuPanel`) use a `vsMenuLine` border and `vsMenuSep` separators; the highlighted item gets only a
  semi-transparent gray `vsMenuHot` fill, with no outline and no text color change. There is no outline because mygo's Border takes
  up layout space: with an outline the text would shift inward by 1 pt and appear to jump on hover (`TestGroveMenuHotOnlyFill`
  compares the text pixels before and after highlighting).

#### Sizes that follow the style

| | Grove | VS Code |
|---|---|---|
| Sidebar body font size `sideFont()` | 14 | 13 |
| Repository name font size `repoFont()` | 14.5 | 13 |
| File tree row height `treeRowH()` | 24 | 22 |
| Right panel history row height `histRowH()` | 30 | 22 |

Commit graph nodes are vertically centered in the row (`graphNodeY()`).

#### Grove style: window backdrop and cards

The window backdrop (`Settings.Backdrop`, used only in the Grove style) has three options:

- Soft (default `""`): a few very faint patches of color drawn with `Draw` on a solid background (`paintBackdrop`, blurred with
  Shadow; the color of the top-left patch is derived from the accent color, `blobColor`), independent of the desktop wallpaper.
- Flat (`flat`).
- Translucent (`vibrancy`, with a system material chosen in `material`; `WORKBENCH_MATERIAL` overrides the setting). Translucent
  looks flat on the default wallpaper, and HUD-type materials wash out the text, so it isn't the default.

Each area is a rounded card (`card` in `ui/widgets.go`), and the 6 pt gaps between cards (`cardGap`) are the drag handles
(`resizer`). The title bar is 38 pt tall (`barHeight`), the window is `TitleBarHidden`, and the buttons in the title bar are laid
out around the traffic lights (`TrafficLights`).

#### Accent color and list selection

The accent color lives in `ui/accent.go`:

- Choices (`workspace.AccentKeys`, in the order of the settings dropdown): Follow system `system`, Black `black` (the default,
  `DefaultAccent`, stored as `""`; #333333 in light and #5C5C61 in dark, both with white text), Blue `blue` (VS Code 2026's
  #0069CC / #297AA0), plus `forest` / `steel` / `ink` / `graphite` / `teal` / `indigo`.
- `applyAccent` swaps a full set on the theme every frame: `Accent`, hover / pressed 8% / 16% darker (lighter in dark mode), and the
  opacity of selected text and the focus ring. With Follow system it changes nothing.

List selection is a light tint by default (`listLook` / `softFills`):

- A faint layer of the accent color plus a bar on the left; when the list doesn't have the keyboard, it becomes a neutral gray with
  no bar; hover is fainter still.
- In the theme the list is built with, Accent is transparent, and each row draws its own selection background in `Draw` based on
  `ListFocused`. When the row above or below is also selected, that side has no rounded corner and the bar runs to the row edge, so
  consecutive selected rows join into one block.
- `SolidSelection` switches to mygo's original solid block; `onAccent` decides whether a row should be drawn "on the accent color"
  (when selected and solid, text and icons switch to `AccentText`).

Keep decoration minimal: banners, hover and the like use very faint fills (`quietFill` / `bannerFill`) and no borders.

#### UI zoom

`Settings.Zoom` steps through `ZoomSteps`, applied by `c.SetZoom` on every window every frame, settings and About windows
included. The terminal's own font size keys (⌘= ⇧⌘= ⌘- ⌘0) are given over to UI zoom (`termZoomKeyFor` in `app/terminals.go`); the
terminal font size can only be changed in the settings.

### App icon

`internal/appicon` draws the icon on the fly with a signed distance field (`Draw(size)`), anti-aliased at any size; the small icon
in the About window and the icon in the update window (`Draw(256)`) are drawn by it too. `assets/icon.png` (the 1024×1024 image
used by `mygo build`) is generated from it: after changing the drawing, run `go test ./internal/appicon -update`; `TestIconFile`
checks that the two match.

### Language

All UI strings go through `internal/i18n`:

- `T` / `Tf` / `Tn` (with a count; English picks singular or plural by count) / `List` (conjunctions for enumerations), plus
  language-dependent punctuation such as `Colon` / `Comma` / `Paren`.
- The source strings are the Chinese in the code; `zh-CN.json` is collected from the code by `TestCatalog -update` (so strings must
  be literals, not concatenations), and `en.json` holds the English.
- English plurals are written as variants like `key|1`; with two counts they are `|1|` (the first is 1), `||1` (the second is 1)
  and `|1|1` (both are 1), and `PluralKeys` works out which ones to look up.
- The language is chosen in the settings (`Settings.Lang`: empty is Chinese, `en` is English) and switched by `i18n.Set`. mygo's own
  strings (the editor's find bar, system menus, dialog buttons) follow the language via `SetLanguages(i18n.Locale())`: set once at
  startup and again after the settings are read.

Accessibility: every icon button needs a name (`Label` / `Tooltip`). `TestAccessLabels` walks through the common states of the
main window and settings window to check; new areas, overlays and windows need a step added there to draw them (see
[AGENTS.md](../../AGENTS.md)).

### Shortcuts

See the custom shortcuts section of [app.md](app.md#custom-shortcuts).
