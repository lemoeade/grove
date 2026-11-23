# Changelog

What changed in each version, newest first.

## Unreleased

- The interface now defaults to English. Installs that already have a state file from an earlier version keep Chinese
  (the state format moves to version 5 and fills in `"lang": "zh"` for them); Settings → Interface language switches
  between English and 中文 at any time.
- Documentation is now written in English: README.md is English with a Chinese copy in README.zh-CN.md, and AGENTS.md,
  CHANGELOG.md and everything under docs/ (architecture, pitfalls, testing, release, terms) were rewritten in English.
  docs/shortcuts.md is generated in English from the command registry.
- Commit messages follow Conventional Commits; the existing history was reworded accordingly.
- English interface fixes: the commit button says "Commit 1 file" instead of "Commit 1 files", and a repository's
  header shows a one-word badge (merging, rebasing, cherry-picking, reverting) instead of "merge in progress", which
  squeezed the repository name down to an ellipsis.
- The README screenshots were retaken with the English interface and English sample content.

## v0.3.0 — 2026-11-23

- Automatic updates: once a day Grove checks GitHub Releases in the background for a new version. If there is one, a window lists what
  changed in that release; choose “Install” to download it (using a delta update when available), verify its signature, install it and
  relaunch. You can also skip that version or be reminded later. Grove → “Check for Updates…” (also in the Command Palette) checks by hand at
  any time; Settings has a new “Update” page: check automatically, download and install automatically (takes effect on the next launch),
  the current version, and when it last checked. `go run` and development builds don't check.
- Installing: `curl -fsSL https://github.com/lemoeade/grove/releases/latest/download/install.sh | sh` downloads the latest version,
  verifies its signature and installs it into Applications; you can also download the DMG. Grove isn't notarized by Apple yet, so the
  first time you open the DMG you need to click “Open Anyway” in System Settings → Privacy & Security (not needed when installed with
  install.sh). Pushing a `v*` tag has GitHub Actions build and sign the app and upload the DMG, update packages, update manifest and
  install.sh to a draft Release, which is published after review.

- Grove now draws the Markdown preview itself (`internal/gmd`, the same approach as replacing the editor with `internal/ged`: parsing still
  uses mygo, while layout, drawing, selection and copying are Grove's). Styled HTML in a document renders the way VS Code's preview renders it:
  classes from `<style>` and `style=` attributes cascade by selector and specificity (the new `internal/css`), with grid / flex layout,
  background colors and gradients, borders (solid, dashed, single-side colors), rounded corners, shadows, padding and margins,
  `::before` / `::after`, `display: none`, and styled tables and lists. Architecture diagrams like these used to come out as a run of text.
  When an author's light background ends up under the dark theme's light text, the text switches to a readable color automatically.
  Unstyled README HTML (centering, `<details>`, badges) works as before. `WORKBENCH_GMD=off` goes back to mygo's preview.
- The Markdown preview fills the width of the tab (like VS Code) instead of a centered 860-pt column. Tables use the browser's automatic
  table layout: if they fit they take their content width; if not, the whole table is one column wide and the long columns wrap onto more
  lines. Columns no longer each stretch to the maximum, the whole table no longer scrolls off sideways, and the last column is no longer
  squeezed to one character per line. Only unbreakable long strings that don't fit scroll horizontally.
- After dragging a side-by-side preview into the source file's group (so the source file's tab is hidden), scrolling the preview and then
  pausing no longer snaps it back to where it was.

- One mouse-wheel notch in the editor and the Diff no longer jumps 40 pt at once: like scrolling elsewhere in Grove (and in browsers) it eases
  over 150 ms; several notches in a row join up, and scrolling back reverses immediately. A side-by-side Markdown preview glides along with it.
  The trackpad still tracks your fingers directly; with “Reduce motion” on in System Settings, each notch still jumps.
- Command Palette: typing `>` no longer swaps the magnifier on the left for › (the field already shows `>`, so it used to read “› >”); when a
  scroll bar appears, rows leave room for it on the right so shortcuts no longer touch the thumb. The mouse and the keyboard share one
  selection (like Spotlight): the selection follows the pointer only when the pointer moves within the palette, and there's no separate gray
  hover highlight anymore. Previously, with the keyboard on one row and the pointer resting on another, it looked as if two rows were
  selected. Opening the palette under a still pointer, the arrow keys and the scroll wheel never drag the selection to the pointer's row.
- The side-by-side Markdown preview catches up with VS Code: the editor and preview scroll each other, mapped to the line within a block
  (previously it only aligned by block, and only from editor to preview), without tugging back and forth while you drag one side. It aligns by
  the editor's screen lines (a wrapped line takes as many lines as it wraps to, folded lines take none); after a trackpad flick the preview
  slows down smoothly with it instead of juddering fast-slow frame to frame; large files (whose preview is laid out in chunks) no longer
  flash back to the start of the chunk at each step while following the editor; wrapped lines are always counted by their estimated line
  count, so the preview doesn't jump when a line scrolls in and gets laid out.
  The block containing the source cursor gets a vertical bar on its left; double-click the preview to go back to that source line; switching
  to another Markdown tab with the keyboard makes the preview follow it, unless “Lock Markdown Preview” (also in the tab's context menu) is on,
  in which case the title reads “[Preview] x”.
  Each direction has its own switch (Settings → Editor: Scroll Preview with Editor / Scroll Editor with Preview). GitHub emoji shortcodes such
  as `:tada:` in the preview become emoji (not inside code); hovering to the left of a heading shows 🔗, which copies a link to that heading.
  The VS Code style preview uses 14-pt text and 1.6 line height, like VS Code. Like VS Code, the Markdown tab bar has only the “Open Preview to
  the Side” button (left and right), no more eye button (in-place preview is still ⇧⌘V, and the in-place preview keeps a button back to the
  source).
  When the source file is a preview tab and another file replaces it, the side-by-side preview no longer gets stuck on “The source file's tab is
  closed”: if the replacement is Markdown it follows it, otherwise it waits for the next Markdown file. VS Code's ⌘K V wasn't added: ⌘K is
  already “Run Git Command…”, and shortcuts don't support chords.
- File tabs now also use Grove's own editor by default (`internal/ged`): the same text layout, colors, indent guides, sticky scroll,
  find and replace, folding, soft wrap and editing keys as the Diff. The completion list, hovers (declaration + diagnostics), ⌘-click to go to
  definition, and ⌥-click to add cursors still work. Like VS Code, change bars are drawn to the right of the line numbers (green for added, blue
  for modified, a red triangle for deleted); clicking one opens that hunk's inline card. Diagnostics get squiggly underlines; inline blame sits
  at the end of the line. Hex, large, read-only files, files with conflict markers, files with the blame column open and files over 50,000
  lines still use mygo's editor. When a file is longer than a screen, its last line can scroll to the top (like VS Code).
  `WORKBENCH_GED_EDITOR=off` goes back to mygo's everywhere (for comparison and troubleshooting).
  A few more VS Code touches: the context menu matches VS Code's editor menu and goes only as far as the Command Palette (Go to Definition F12,
  Find References, Change All Occurrences, Format Document, Cut / Copy / Paste, Command Palette); Split, local history, Reveal in Finder and
  Copy Path are in the tab's context menu. Brackets cycle through three colors by nesting level (VS Code style); the cursor line is
  highlighted even without keyboard focus.
  A minimap on the right (VS Code style): the whole file shrunk into thin strips in token colors, with a slider for the current screen always
  drawn faintly (one shade darker under the pointer); click or drag to scroll.
  Menu shortcuts are written the VS Code way: Change All Occurrences shows ⌘F2 (⇧⌘L still works).
- Frosted overlays, following VS Code's frosted glass: Grove's menus, the Command Palette / Quick Open, dialogs, and the editor's hovers and
  completion list blur the window content beneath them so it shows through, under a layer of 80% of their original background (VS Code's
  default; more transparent than that lets blurred code bleed through in colored blotches and looks dirty). Settings → Appearance →
  Overlay Opacity changes it (50–100; 100 is opaque).
  The VS Code style is frosted too now (it used to be opaque); menus, the Command Palette, dialogs, hovers and completions use VS Code 2026's
  overlay background (light #FAFAFD, dark #202122).
  With “Reduce transparency” or “Increase contrast” on in System Settings, they're still solid and opaque.
- The Diff now uses Grove's own renderer by default (`internal/ged`, the “experimental” entry below): Diff tabs, compare tabs, replace
  previews and inline editing all go through it; in full-file mode it scrolls to the first change. `WORKBENCH_GED=off` goes back to mygo's
  Diff (for comparison and troubleshooting).
- The VS Code style is now the default (Settings → Appearance → Style), following the default themes from VS Code 1.147 on, Light 2026 /
  Dark 2026: the title bar, sidebar, tab bar, terminal, right panel and status bar are opaque #FAFAFD (dark #191A1B), the editor is white
  (dark #121314), and areas are separated by a thin #F0F1F2 (#2A2B2C) line, with no rounded cards, shadows, gaps, color patches or frosted
  glass. Text is #202020 / secondary text #606060 (dark #BFBFBF / #8C8C8C), and popup backgrounds change to match. The interface is mostly
  neutral gray: a black line on top of the active tab and under the active title in the bottom panel (light gray #BFBFBF in dark, not VS Code
  dark's blue).
  Code uses VS Code's classic Light+ / Dark+ colors (the ones Light Modern / Dark Modern use: keywords blue, types teal, functions yellow-brown,
  variables dark blue, strings red-brown, numbers and comments green; not the 2026 themes' GitHub colors). Line numbers, the cursor, the
  selection (25% of the accent color, 33% in dark; in dark, a gray accent color gets a light-gray overlay), find matches, matching brackets,
  indent guides and whitespace markers follow it too. The Diff's reds and greens and changed characters follow it (the dark reds and greens
  changed as well), the diagonal-hatch filler uses VS Code's default, and line numbers no longer change color. Scroll bars are VS Code's
  square, edge-hugging kind, with a transparent track and a faint gray thumb (25%, one shade darker on hover and while dragging; not 2026's
  nearly opaque thumb). The sidebar uses VS Code's 13-pt text and 22-pt rows.
  The right panel's history shows one commit per 22-pt row. The view switcher at the top of the sidebar follows VS Code's activity bar: no
  pill, icons laid flat, with the active one darker and underlined in black.
  Sidebar list rows are full width and square; hover, selection without keyboard focus and selection with keyboard focus are all gray
  (black 8% / #DADADA 60% / black 15%; white in dark). With keyboard focus the selection is just one shade darker gray (no VS Code blue
  outline, and no longer a bar on the left); the hover block is also full row and square (it used to be rounded).
  The file tree's indent guides follow VS Code's tree.indentGuidesStroke (#A9A9A9 for the selected row's parent level, others faded to 40%;
  previously that guide was near-black and the other opaque light lines cut the selected row's gray background into strips). The file tree's
  sticky stack matches the sidebar color. Quick Open and filter matches are only bold (VS Code uses bold blue; no blue here), with no
  background.
  Menus are #FAFAFD (dark #202122) with a #E4E5E6 border and #EEEEF1 separators; the hovered or keyboard-selected item gets only a gray
  translucent background (black 8%, white 10% in dark), with no outline and unchanged text.
  The title bar's Quick Open looks like the Command Center, white with a thin border, and input fields are white; secondary buttons in
  dialogs are light gray; the bottom panel's Terminal / Output / Problems tabs aren't boxed or filled, and the active one is underlined.
  Hovers are light, like VS Code (#FAFAFD background in light, #202122 in dark, body text color, a thin #E2E2E5 / #2A2B2C border), no longer
  inverted dark with white text. The title bar's branch pill is flat (no background, text not bold, a faint gray background only on hover),
  like the branch in VS Code's status bar.
  The Git view's commit box follows VS Code's Source Control: a white input field with a thin border, and below it a full-width
  “✓ Commit N Files” primary button (with a ▾ split off on the right). It's there even without a message, grayed out, and clicking it gives
  the keyboard to the input field.
  Python docstrings use the string color like VS Code (the Grove style still uses the comment color); the small symbol-kind labels in the
  Command Palette, Outline and Diff fold bars are colored like VS Code's symbol icons (classes orange, functions and methods purple, variables
  blue; headings, types and constants gray). Sidebar view titles (Repository, Changes, Outline, Open Editors) are 11-pt semibold like VS Code.
  The old look stays as “Style: Grove”; window background and material apply only in the Grove style.
- The default accent color is now “Black” (#333333 with white text in light, mid gray #5C5C61 with white text in dark): buttons, switches,
  selected text, and the editor's selection and find matches are all a neutral gray-black, with no blue left in the interface.
  “Blue” was added too (VS Code 2026's #0069CC, dark #297AA0) for anyone who wants VS Code's original blue. Settings saved with the old default
  (Forest) read back as Black; pick Forest again if you want it.
  The “modified” bar next to Diff line numbers and “info” in diagnostics are still blue (they're colors with meaning, unrelated to the accent).

- Diff fold bars follow VS Code's collapsed unchanged regions: a faint full-width bar with “N unchanged lines” on the left, followed by the
  line of the function / class / heading containing the change below it (like the part after `@@` in a GitHub hunk header, preceded by the
  small kind label from the outline); rest the pointer on it to see the full text. It's matched by the changed line and only appears for
  languages with outline support; it's omitted when the change falls between two functions (comments, blank lines) and on the bar at the end
  of the file.
- Diff reds and greens are redrawn at VS Code's intensity: changed lines get a full-line red / green background like VS Code's default theme
  (it used to be so faint you could barely see it), changed words within a line get a deeper layer (they used to be gray, so you couldn't
  tell which characters changed), and each line starts with a red / green bar (dashed on the deleted side). It stands out equally in light and
  dark; in light, changed line numbers use a darker green so they're readable on the green background.
- An experimental Diff of Grove's own (`internal/ged`, using only mygo's public text layout, drawing and input APIs, without changing mygo):
  with `WORKBENCH_GED=1`, read-only Diffs are drawn by it — the two sides line up line for line (when a change deletes d lines and adds a, the
  first lines are paired and the extras get a diagonal-hatch filler on the other side), changed lines get a full red / green background,
  changed characters within a line a deeper layer (when the whole line changed, only from the indentation on), line numbers and + − change
  color with them, unchanged stretches more than three lines from a change are folded (click to expand; the fold bar shows the function
  containing the change below after “N unchanged lines”, as before), and the overview strip on the right marks where the changes are and
  scrolls when dragged. Only visible lines are laid out and drawn. When the pointer moves onto a change (or you jump there with previous /
  next change), “Stage Hunk / Unstage Hunk / Discard Hunk” appears at its top right and sticks to the top of the hunk while scrolling; the
  tab bar's previous / next change, the bottom panel's “Change i of n”, and J / K follow its changes, and switching “Show only the lines
  around changes” to full file no longer folds. Drag to select text (auto-scrolling at the top and bottom edges), double-click to select a
  word, triple-click to select a line, ⇧-click to extend; ⌘C copies, ⌘A selects all of that side, Esc cancels; after clicking, ↑ ↓ ← →, Page
  Up / Down, Space and Home / End scroll.
  Click line numbers to select lines (drag to select several, ⇧-click to extend to that line; selected lines get an accent-colored bar on the
  left); the panel below still stages / unstages / discards the selected lines, and which line pairs with which follows its alignment (as you
  see it). The unified view is drawn by it too: each change lists the deleted lines and then the added ones, with two line-number columns on
  the left (old / new), changed characters still get the deeper layer, and lines on both sides can be selected by line number and staged.
  Inline editing (after clicking the pencil) is drawn by it as well: click on the new side to place the cursor (it doesn't blink, and no extra
  frames are drawn while idle); typing, ⌫ / ⌦ (⌥ by word, ⌘ to line start), Return copying the line's indentation, Tab, ← → ↑ ↓ (⌥ by word,
  ⌘ to line start / end and file start / end, ⇧ to extend the selection), Home going to after the indentation first, and Page Up / Down; IME
  composition is drawn at the cursor and the candidate window follows the cursor; ⌘X / ⌘V / ⌘Z / ⇧⌘Z go through the Edit menu, and a run of
  typing undoes in one step. The diff realigns as you type, without folds, scrolling or the cursor jumping around. It still edits the file
  tab's buffer, and ⌘S works as before.
  ⌥Z soft wrap works too: long lines wrap at the column's width, in side-by-side mode a line's height is whichever side wraps more, and
  clicking, selecting and ↑ ↓ move by wrapped lines.
  ⌘F finds in the Diff (a bar at the top right; if text within one line is selected, that's what it searches for): it searches the visible
  text on both sides (an unchanged line counts as one match), case-insensitively, highlights every match and the current one a shade darker;
  Return / ⌘G goes to the next, ⇧↩ to the previous, wrapping around at the ends and expanding folded stretches; Esc closes.
  There's a context menu now (mygo's Diff had none), on right-click (or ⇧F10), for the line and change under the click: Cut / Copy / Paste (Cut
  and Paste only while editing inline), Stage / Unstage / Discard Hunk, the selected-lines items, Copy Line (“Copy Original Line” on the old
  side), Find, Previous / Next Change, Open Line in Editor, toggles for unified / full file / soft wrap / inline editing, Select All, Reveal in
  Finder and Copy Path; right-clicking outside the selected text moves the selection there.
  Fold bars follow VS Code / GitHub: the line-number cell has arrows; clicking one reveals 20 more lines continuing down from the change above
  or up from the change below (the first stretch can only expand up, the last only down); clicking elsewhere on the bar expands the whole
  stretch; the fold bar's context menu also has “Show All N Lines / Expand 20 Lines Down / Expand 20 Lines Up”.
  “Ignore Leading/Trailing Whitespace” (View menu, Command Palette, context menu; saved in state.json and applied to every Diff tab): lines
  where only indentation or trailing spaces changed don't count as changes, and staging / discarding a hunk touches only the visible change.
  ⌃G go to line works too (it goes to the line on the new side, expanding folds); while editing inline, the context menu has Undo and Redo.
  When text within one line is selected, the same text elsewhere on both sides is faintly highlighted (as in VS Code). In side-by-side mode,
  drag the middle divider to change the widths of the two sides, and double-click it to reset to half and half; while editing inline, the
  cursor line is highlighted and its line number is brighter.
  While editing inline the editor's keys work too (sharing the keymap with file tabs, so remapped keys change together): ⌥↑ ↓ move lines,
  ⇧⌥↑ ↓ copy lines, ⇧⌘K deletes a line, ⌘↩ / ⇧⌘↩ insert a line below / above, ⌘] / ⌘[ and Tab / ⇧Tab indent, ⌘/ comments, ⌃J joins lines,
  ⌃T transposes, ⌃K cuts to end of line, ⌘L selects the line, ⌘D selects the word / the next identical occurrence, ⇧⌘\ jumps to the matching
  bracket, ⌃↑ ↓ move by paragraph. Return keeps the indentation, indents one more level after an opening bracket, and splits into three lines
  between a pair of brackets; typing a bracket or quote adds the other half (not for quotes inside strings), typing the closing one skips over
  it, typing with text selected wraps it, and Backspace deletes an empty pair. When the cursor is next to a bracket, both brackets get a box;
  whitespace is drawn per the “Show Whitespace” setting (dots for spaces, arrows for tabs); ⌥⌘F opens replace below the find bar, ↩ replaces
  one and goes to the next, and “Replace All” undoes in one step (the old side only finds, never replaces).
  Indent guides are drawn (one per indentation level, continuing across blank lines; while editing inline, the guide of the block containing
  the cursor is a bit darker).
  Sticky scroll follows the setting: each side pins the functions / classes enclosing its top lines (from its own outline, at most 5 lines),
  and clicking one jumps there; when scrolled partway, the last line is no longer drawn over the panel below. mygo's Diff is still the default.
- ⌥Z soft wrap now works in Diff tabs too: long lines wrap to the next line instead of scrolling sideways. Like file tabs it follows the
  setting (Markdown and plain text wrap by default) and toggles only that tab.
- Upgraded to mygo v0.10.0 (the following come from mygo's changes): holding ⌘ draws the name under the pointer as a link when it can go to a
  definition. When the cursor moves to the window edge, three lines of margin are kept above and below before scrolling. Large Diffs (over
  10,000 lines or 1 MB combined) are computed in the background, showing the original text first without blocking the interface. In a narrow
  editor the find bar first hides its count, then keeps only the input field. Screen readers can read which lines a fold bar hides.
  Files with mixed line endings (LF / CRLF / CR) keep each line's original ending when saved, and editing a line touches only that line
  (previously the whole file was unified to one kind); choosing a line ending in the status bar converts every line.
- Markdown preview:
  - Front matter (YAML / TOML) at the top of a file is drawn as a collapsible metadata card instead of a rule followed by a paragraph.
  - HTML common in READMEs renders the way GitHub renders it: `<p align="center">` / `<div align>` center, `<details>` collapses (click
    `<summary>` to expand), `<img width>` uses the given size, `<picture>` picks the image for light or dark, and inline tags such as `<b>`,
    `<a>`, `<code>` and `<br>` lay out normally; `<script>`, `<style>` and comments aren't shown.
  - Remote images (https badges, screenshots) are fetched and shown, SVG badges included; only the image itself is fetched, without cookies,
    up to 8 MB per image. Turning off “Show remote images in Markdown preview” in Settings shows just the alt text.
  - The typography was retuned: body text one size larger with more line spacing, and spacing around headings, code blocks, quotes and tables
    close to GitHub's.
  - Links have a context menu: Open Link, Copy Link, plus Copy Path and Show in Finder for links to local files; resting the pointer on a link
    to a local file shows its path in the repository, or says it can't be found.
    Path-like inline code (`docs/a.md`, `src/x.go:12`) can be clicked to open that file.
  - With the preview open, clicking the sidebar Outline scrolls the preview to that section (it used to jump back to the source).
  - ⌥⇧⌘V (View › Preview Markdown to the Side): opens a preview that follows the source file in the group beside it, keeping the keyboard in
    the source file; the preview follows wherever the editor scrolls, and re-lays out only after you pause typing. Press it again, or click the
    button on the preview tab's header, to close it; closing the source file closes the preview too.
  - Previews of large files are drawn in sections in a virtual list, so opening and scrolling don't stutter; `#heading` links and the Outline
    still scroll to parts that haven't been laid out yet. Switching between source and preview doesn't re-parse or re-section.
  - Mermaid upgraded to 12.3: flowcharts, state diagrams, class diagrams and ER diagrams are laid out with ELK by default, with right-angle
    edges with rounded corners and fewer crossings (the layout differs from before; to get the old one, write `layout: dagre` in the front
    matter). The look follows the black-and-white document style common in VS Code's Markdown preview: white nodes (in dark, a gray one shade
    lighter than the background), thin dark borders, a faint shadow offset to the bottom right, edges in the text color, edge labels on a
    light-gray background, and databases drawn as cylinders; pie charts, mind maps, git graphs and Gantt charts use a set of muted blue, green,
    amber, red, purple and olive (light in light mode, dark in dark mode, with text in the body color).
    Colors follow the interface theme's light or dark mode. The mind map root node's text no longer drifts outside the circle. When the
    system WebKit is too old (before Safari 17.4) to render Mermaid 12, the preview says to update macOS instead of showing an
    incomprehensible error.
  - Math switched to MathJax 4: the new New Computer Modern font, and most formulas drawn as vectors (sharp at any size, no redraw when
    switching themes or moving between displays); characters the font lacks, such as Chinese inside `\text{}`, still produce bitmaps.
    `\ce{}` (chemical formulas), `\mathbbm` and `\mathds` are supported. All fonts ship inside the app; nothing is fetched from the network.
  - The rendering scripts ship compressed inside the app (18 MB down to 5.5 MB), about the same as before (Mermaid 11 + MathJax 3, 5.8 MB).
- Go to Definition: ⌘-click, F12 or “Go to Definition” in the editor's context menu looks up a declaration for the name under the cursor in
  the outline: this file, open files (unsaved changes included) and every repository's symbol index.
  Multiple matches with the same name are narrowed first: `util.Parse` prefers the one in a util folder; without a qualifier, this file and
  the same folder come first. If several remain, a palette opens listing just those, nearest first. If the name is itself the definition or
  isn't found anywhere, a notice says so. If the index isn't built yet, the request is remembered and the jump happens once it's built. It
  doesn't use a language server, so it's a guess by name.
- Word completion: while typing in a code file, the completion list includes words from this file (nearest the cursor first, functions and
  types with icons), open files of the same language and the same repository's symbol index, listed together with snippets; ⌃Space
  suggests by hand at any time, and Markdown and plain text offer suggestions only on ⌃Space.
  Once you've typed a whole word, it's dropped from the list, and Return inserts a newline as usual. “Suggest While Typing” on the Settings
  “Editor” page turns it off (on by default).
- Hovers: rest the pointer on a name in code (for the name at the cursor, use “Show Hover” in the Command Palette; mygo's ⌘K ⌘I is taken
  first by the menu bar's ⌘K, but you can assign another key in the shortcut settings). The declaration is found the same way as Go to
  Definition, and the hover shows the declaration line (several lines when parameters wrap, with the function body cut off), the comment
  above it (the docstring for Python), and the repository, file and line it's in; click the location to open it. If the same name exists
  elsewhere it says “N more”, and clicking lists them to pick from.
  Nothing appears on the declaration itself or when nothing is found. “Show Definition on Hover” on the Settings “Editor” page turns it off
  (on by default); problems reported by linters still show on hover.
- Find References: ⇧F12 or “Find References” in the editor's context menu searches the sidebar Search for the name under the cursor as a
  whole word, case-sensitive, only in files of the same language (TypeScript includes .tsx and .js), with results from every repository.
  It's a search by name too, so matches in comments and strings count.

- Menus drawn by Grove (context menus in the file tree and Git view, the status bar's language menu, the branch button's menu) have frosted
  panels: the window content beneath blurs, under a translucent background layer (44% opaque in light, 60% in dark, blur radius 12: over the
  theme's own background, text reaches 4.5:1, and at least 3:1 however bright or dark the content beneath). What blurs is the window's own
  content, not the desktop behind the window.
  With “Reduce transparency” or “Increase contrast” on in System Settings, the panels are the old opaque ones. Menus drawn by mygo (context
  menus in the editor and text fields) are unchanged.
- Rewriting pushed commits always asks with a red confirmation: interactive rebase and rebasing onto a branch when pushed commits are
  involved (previously only amending and undoing the last commit were red), and also the commit at which a rebase stops when you choose
  “Skip” partway through (its changes don't make it into the result).
- Fix: “New Branch…” in a Git view section header and in the branch menu creates the branch in that repository (it used to use the current
  repository, so clicking it in another repository's menu created it in the wrong place).
- Fix: opening a repository's changes from the start page makes it the current repository (terminal, history, the title bar's branch pill)
  (it used to stay on the previous one).
- Terminal tabs recognize agents more accurately: scripts run by node, programs that changed their process name, programs run inside Python
  frameworks, and programs whose path contains spaces are all recognized by name (the same goes for the “what's still running” question when
  closing a tab).
- The home folder itself is shown as “~” in the Recent Workspaces menu and in folder displays (it used to be the full path).
- The English interface no longer has full-width colons, question marks, parentheses or enumeration commas mixed in (in assembled messages
  for the queue, images, blame, replace, drag and drop, submodules and the like).
- The editor command “Revert This Change” is renamed “Discard Hunk” and the inline card's button “Revert” becomes “Discard”, matching the Git view's
  wording.
- When a workspace has more than one repository, red confirmations that act on a whole repository (discard all changes, force push, reset,
  undo / amend a pushed commit, rebase onto a branch, delete a branch / remote branch / remote, drop a stash, files applied from a stash
  overwriting changes) put the repository name in front of the title, so when clicking from the menu bar you know which repository it is.
- Restoring local history over a file with unsaved changes, resetting all shortcuts, and importing settings (replacing all current settings)
  now use red confirmations, like restoring default settings.
- The replace preview checks the size before reading a file that isn't open, and doesn't read files that grew past the search limit after the
  search.
- The green of worktree rows and of the Staged group header's checkbox now uses the theme's success color, the same green as the staged dot and
  +N in the Git view; the editor's peek card shadow matches the floating bar.
- When some folders couldn't be added while adding folders, the notice is now Grove's own notice box (it used to be a system dialog), like
  other error notices.

## v0.2.0 — 2026-11-14

- Context menus no longer flash and disappear (only seen in real windows; four fixes together): the right-click event handler read the
  window size as 0×0, so the menu was laid out as a tiny block for an empty window; it now takes the previous frame's window size when the
  elements are built (the same for menus opened with ⇧F10 / the menu key). macOS sends a `dy=0` scroll on right-click and from the trackpad,
  which used to count as “scrolling the file tree” and closed the menu right away; scrolls under 0.01 no longer count. Something else in the
  window taking the keyboard no longer counts as the window losing focus; only switching to another window / app closes the menu.
  Pressing and releasing the right button without moving keeps the menu open however long you held it (previously, releasing after 0.15 s
  selected whatever item was under the pointer); pressing, dragging onto an item and releasing still selects it.
- Space in the file tree opens the system's “Quick Look” (the panel Space opens in Finder) for the cursor row, files and folders alike, so you
  can glance at images, PDFs, videos and other things Grove doesn't open itself.
  With the panel open, going back to the file tree and pressing Space again switches to the new item; there's also an item under “More” in the
  context menu. It only views; files aren't changed.
- The status bar's language menu can be filtered by typing: there's a filter field at the top, so open it and type (`ts`, `py`…) to keep only
  matching languages, and Return picks the first. Backspace deletes characters; Esc clears first, and closes on the second press. The menu is
  no longer taller than the window: it shows at most 12 items, and the checked current language is visible as soon as it opens.
- Menus drawn by Grove (context menus, button menus): in menus too tall for the window that need scrolling, moving with ↑↓ to items outside
  the panel scrolls along (the highlight used to run off out of sight); scrolling the wheel over a menu that doesn't need scrolling no longer
  closes it.
- Unstaged Diffs can be edited in place: the tab bar has a new pencil toggle; click it and the right side (the working tree side) can be edited
  directly, the diff follows your typing, and ⌘S writes the file back. What you edit is that file's tab (opened in the background), and undo
  and save belong to it. Click the pencil again to go back to read-only. While editing, J / K type instead of jumping between changes, and the
  per-hunk staging buttons don't appear.
  In the Command Palette it's “Edit in Place in Diff”. The tab bar's “Open in Editor” icon changed from a pencil to a file, matching the one on
  Git view rows.
- Items in menus drawn by Grove can now be pressed directly by assistive technologies such as screen readers and Voice Control: previously only
  the mouse and keyboard worked, and pressing them with assistive technology did nothing.
- Upgraded to mygo v0.9.0 (the following come from mygo's release notes; Grove's own code didn't change): scroll bars look like VS Code's
  editor scroll bars. The editor also recognizes Svelte, HCL / Terraform, ERB, Jinja and Nix, and scripts are recognized by the `#!` on their
  first line. Lists, the outline and context menus do less work per frame. In the terminal, Ctrl+I, Ctrl+M and Ctrl+[ send Tab, Return and
  Esc, and URLs end at the first non-ASCII character. The save before quitting also runs on logout and shutdown.
- Soft wrap by file type: by default only Markdown and plain text wrap (plus .rst, .adoc, .org, and extensionless files such as README and
  LICENSE), code doesn't. Settings → Editor → Soft Wrap can be changed to “All files” or “Off”. ⌥Z toggles only the current tab without
  changing the setting. If you had soft wrap on in Settings before, all files still wrap.
- `.vue` files have their own highlighting (the status bar says Vue, no longer HTML): `<script lang="ts">` is tokenized as TypeScript (with
  types colored), and `{{ }}` in templates plus the values of `v-if`, `:prop`, `@click` and `#slot` are colored as code instead of one long
  string; `<style>` is still CSS.
  Formatting defaults to prettier too.
- Softer code colors: operators and punctuation (`=` `:` `=>` and brackets across languages) are toned down to gray, no longer the same red
  as keywords.
- Several Python highlighting changes: the `:`, `=` and `->` in annotations, default values and at line ends are punctuation, no longer red
  operators; attribute chains and import paths (`sqlalchemy.orm`) are no longer a sea of blue; `self` and `cls` are italic, no longer
  function purple; docstrings are drawn as comments.
- Markdown headings, `**bold**` and `*italic*` are bold and italic again in the editor (Grove lost them when it changed the whitespace marker
  color).
- When nothing is open, the editor area shows a start page: the workspace name and an overview of all repositories, then files with changes
  (by repository, up to 5 each; click to open the Diff), recently opened files, and other worktrees (those with computed status show how many
  changes and how far ahead), with Quick Open and Open File at the bottom. After tabbing in, ↑↓ move between rows and ⏎ opens.
- Each repository has a color (gray-blue, gray-green, gray-sand, gray-purple, gray-rose, gray-teal, cycled in workspace order): a small dot
  in front of repository rows in the file tree and Git view and on terminal tabs, and in the “All Repositories” history the repository name at
  the end of each row is a faint pill in the same color.
- The right panel history's date sections are now Today, Yesterday, This week, Last week, This month, and then one section per month
  (“October”, with the year when it isn't this year); the time at the end of each row follows its section: “15:04” for today and yesterday,
  “Thu 14:20” for this week and last week, “10-22” for earlier, with the full date and time on hover.
- Every page of the Settings window is grouped into rounded cards (Theme, Interface, Text, Save…), one setting per row: name and description on
  the left, the control on the right, and clicking the name is the same as clicking the control.
  “Material” appears only when the window backdrop is set to Translucent, indented under the window backdrop. Number settings have their own
  number field (↑↓ step, and − / + gray out at the limits).
- Upgraded to mygo v0.8.3: the accent color turns gray when the window isn't frontmost (like system apps); text layout on macOS uses less CPU
  (about half when dragging dividers or resizing the window); right- and middle-button presses no longer drag; clipping inside rounded cards
  and grow animations no longer create a new mask every frame.

- Several icon-only rows in the Git view now have text: the stash group header and worktree group header say “Stashes 1” and “Worktrees 4”,
  like the Staged / Changes groups; the row under an expanded worktree says “2 commits · 2 files +6 −0”; in the banner of a half-finished
  merge / rebase, Continue, Skip and Abort became small labeled buttons (Continue has the accent background and is grayed out while there are
  conflicts).
- For added and deleted files the Diff draws only the column that has content (even with side by side selected; the side-by-side / unified
  buttons are grayed out), instead of half the width being hatching. The bottom panel says “Staged changes” / “Unstaged changes”, and
  `HEAD ↔ Index` moved into the hover tooltip.
- The left side of the status bar is now an overview of all repositories: “4 repositories · 2 with changes · 1 to push · infra merging”, with
  read failures in red; hover to see one line per repository, click to open the Git view.
  With only one repository it shows that repository's changes. The current repository and branch are shown in the title bar and the right
  panel header; the old click-↑↓-to-sync in the status bar is gone, and “Sync” is in the push button's context menu.
- The first lane of the commit graph and the color blob at the top left of the soft window backdrop follow the accent color (previously they
  stayed steel blue and blue after changing the accent color).
- The README screenshots were retaken (the old ones still showed 0.1.0).
- Folded unchanged lines in the Diff became a thin line with a small pill in the middle saying “N unchanged lines”, with no gray background;
  click the pill to expand (at the start / end of the file it expands 100 lines toward the change; between two changes it expands everything
  at once; with ⇧ held it always expands everything). Bars are no longer drawn next to the line numbers of changed lines (the dashed line on
  the deleted side is gone too); changes are distinguished by background and by red / green line numbers.
- The default accent color is now Forest (Settings → Appearance → Accent color gains “Forest”; Steel is still there); if you never changed the
  accent color, it switches to Forest.
- The commit button shows only a paper-plane icon, with ▾ as before; how many files will be committed and why it can't be clicked are in the
  tooltip when the pointer moves over it.
- When several consecutive rows are selected in the file tree or Git view, the faint background and the bar on the left join into one block
  and one bar, instead of one segment per row.
- The right panel history was laid out again: row titles are one size smaller, the agent's four-pointed star moved into the card, and when
  `main` and `origin/main` are on the same commit they fold into one pill (a cloud after the name); date section headers stay pinned to the top
  while scrolling. The selected commit expands into a card: the full title, then author · agent · time and branches, the body laid out as a
  list (with trailers such as Co-Authored-By removed, at most three items when collapsed, and “Show all” when it doesn't fit), and the short
  hash, file count and added / deleted lines on one line.
  When the working tree is clean there's no more “uncommitted changes” row (ahead / behind are in the Git view). When the right panel is
  narrower than 300, the header omits the branch name and the card keeps only the “More” button, and “More” gains “Compare with HEAD”. The
  search field's placeholder is shorter, and the query syntax is behind the ? on the right.
- The commit graph is drawn differently: thinner lines, narrower lanes (14 → 12), and bends come out vertically, curve to horizontal, then curve
  back to vertical, so they never become diagonals however many lanes they cross; nodes no longer have a background-colored outline, and there's
  no white halo around the dot on row hover or selection. A history with a single line is drawn as a thin gray line with small dots, with HEAD as
  a thin accent-colored ring. Branch pills not on the main line use their lane's color.

- The editor area is steadier when switching files: the first time a file opens, the code no longer shifts 6 px right when the HEAD version
  finishes loading (files in a repository reserve the change-bar column from the start, untracked ones too); clicking files one by one in the
  Git view, the Diff no longer shows an empty frame first (like files, ones that load quickly are in place when they open); when a file opened
  with a single click (preview) is replaced and you click back to it, it returns to its previous scroll position and cursor (remembered only for
  this run), already there in the first frame.
- Folders dropped on the Dock icon (and `open -a`, or launching again from the command line) open in a new window (#58): an already open
  workspace or repository is brought to the front (a repository jumps to its section); if the frontmost window is an empty workspace it opens
  there; dropping several opens several windows. Dropping into a window still asks whether to add it to the current workspace.
- Branch entry points (#58): the branch name in a Git view section header can be clicked to open that repository's branch menu (without
  switching the current repository first or collapsing the section); the repository context menu gains a “Branch” submenu.
- The repository context menu is trimmed to 13 items: Refresh, Restart Terminal, Expand / Collapse, Move Up / Down, Remotes, Submodule,
  Show Ignored Files, Open Main Repository and Open on GitHub moved under “More”.
- Menus drawn by Grove (the file tree context menu, Git view section headers, the branch menus of the branch pill and section headers) got a new
  look: 13-pt text, 8-pt corner radius, a faint fill on hover, no checkmark column when no item is checkable, and fainter separators.
- Mouse-wheel scrolling in the file tree, Git view and search results (#58): one notch moves one line, larger system-accelerated notches move
  more, at most 6 lines at once, gliding over about 160 ms; several notches in a row join into one movement without stuttering, and reversing
  at the end turns around immediately; no glide with “Reduce motion”. The trackpad still tracks your fingers directly. The sticky stack moves
  in the same frame as the rows, so the top levels no longer jump when scrolling fast.
- Context menus drawn by Grove no longer cover the right-clicked row: they open at the pointer's x position below that row, and flip above
  the row near the bottom of the window (#58).
- Context menus drawn by Grove now cover editor tabs, Open Editors, stashes, history (commit rows, change rows, repository rows) and local
  history (#58): like the file tree, press the right button, drag onto an item and release to select it.
- Menus drawn by Grove now cover the context menus of worktrees, blame, terminal tabs and search results, the push button's context menu, the
  status bar's language / indentation / line ending / encoding menus, ⋯ Git actions, worktree actions, the git queue, and rebase action choices
  (#58).
  When there's no room below a button, its menu flips above the button instead of covering it. Replace, Dismiss and Search Only in This Folder
  in the search results menu work as before.
- Git view change rows, files in stashes and untracked folder rows, the history panel's title row, compare row and the commit card's “More”,
  the title bar's workspace button, the status bar's zoom, the terminal profiles under the bottom panel's “+”, the search editor's context
  line count, and the clone dialog's context menu also switched to menus drawn by Grove (#58).
- Search results shrinking (re-search, replace, undo replace) while their context menu is open no longer crashes; the menu closes (#58).
- After a menu drawn by Grove closes (by choosing, Esc or clicking outside), the keyboard goes back to where it was before the menu opened;
  converted context menus can still be opened with ⇧F10 / the menu key when you have keyboard focus, with the first item highlighted (#58).
- The editor's context menu, text fields' edit menu, and button / context menus not yet converted to Grove's menus are all drawn inside the
  window (`SetMenuPresentation(PopoverMenus)`, #58) instead of popping up system menus; if state changes while a menu is open, its items gray
  out and give the reason. The menu bar and Dock menu are unchanged.
- The file tree's sticky stack no longer disappears for no reason: scrolling past the selected row (with the selected row under the stack, or
  just scrolled off the top of the list) keeps the stack, which only makes way when the cursor has just moved to a covered row (↑, following the
  current file) and pins again as soon as you scroll; it also pins when the file tree is first drawn already scrolled to the middle, before the
  repository row has ever been drawn.
- Clicking file tree rows no longer flickers: the press state, one shade darker than selection, is gone; pressing shows the selection
  background directly, and releasing doesn't change it; clicking an already selected row leaves its background alone.
- The whole set of interface icons was replaced with Phosphor (MIT, added to the third-party list in the About window): thinner, rounder
  lines and solid folders; Undo and Refresh no longer look alike, and Pull and Push are arrows with a bar.
- The language letters on file icons were all replaced with monochrome language icons (24 languages), still in the old category colors;
  .jsx / .tsx are React. Go, C#, Objective-C, Vue, SQL and proto are drawn by hand; the rest come from Simple Icons.
- With split editors, the accent bar on top of the current group stops short of the card's rounded corners instead of cutting a slanted
  notch at the corners; the tab bar's top-left corner is clipped to the card's radius, so after scrolling, the tab against the left edge no
  longer pokes a white tip out of the card's corner.
- File tree folders are now hand-drawn solid folders, colored by folder name and carrying a purpose symbol, 26 kinds in all: `<>` for src, a
  checkmark for tests, lines of text for docs, a down arrow for build, a wrench for utils, a lock for auth, a cloud for deploy, and so on.
- File and folder context menus are trimmed: 13 items for files (plus the staging items when there are changes) and 14 for folders, in five
  groups: open and view, staging, Finder and paths, copy and paste, rename and Trash. Local history, Run, Compare, Add to .gitignore, Open
  with Default App, Open on GitHub, Collapse All Folders and folder history moved under “More”; file rows no longer have New.
- Destructive items in menus drawn by Grove (Move to Trash, Discard, Delete, Force Push, Abort, hard reset, Remove from List…) use the danger
  color, including the hover fill.
- Menu wording is unified: `Open` (no more `Open in Editor`), `View Diff`, `Search Here`; the group order of Git view change rows and of
  change rows in commits matches the file tree.
- Mouse-wheel scrolling in sidebar lists no longer gets stuck going up after hitting the bottom (when the first frame moved too little, the
  list pulled it back to the bottom and that notch was lost); when the list measures new rows and corrects its offset, the glide continues,
  and scrolling up no longer moves only a little per notch.
- Cleaner repository sections in the Git view:
  - The section header has only the repository name, branch, Pull and Push; the `+` is gone, and the change count shows only when collapsed.
  - Group titles read “Changes 13” and “Staged 3” instead of being told apart by icons; `↩ + −` appear only on pointer hover, and the context
    menu has the same actions. The “Unstaged” group is renamed “Changes”.
  - One box does it all for committing: normally a one-line input field that grows as you type, with a split button at its bottom right whose
    label says what will happen (“Commit 2 files”). Without a message there's no button, and the row of tools below is gone too.
    `▾` holds the new Commit and Push (regardless of the auto-push setting), plus Amend Last Commit and Undo Last Commit; the box's context
    menu has the same items. ⌘↩, ↑ and Esc are unchanged.
  - When nothing is staged, the button says “Commit all 13 files”: it stages everything at once (untracked files included) and then commits,
    and doesn't commit if staging fails; it used to say there were no staged changes.
- Git view change rows, files in stashes and files in commits are written differently: on the left the same language icon as the file tree,
  following the colored / outline setting; M / A / D are no longer color blocks but small letters at the end of the row, which make way for
  buttons on hover. When space runs out the path shrinks first, so the file name is no longer cut down to `index.…`.
- The commit box got a new texture: normally a translucent background with a faint border, solid only with keyboard focus; corner radius and
  padding are a bit larger, and the buttons inside follow suit.
- mygo upgraded to main (`6121009b`).

- Interface (#56): no more button shapes inside panels, and tidy tab bars.
  - The four icons at the top of the sidebar (Files / Search / Git / Outline) became a compact left-aligned group instead of filling the sidebar's
    width (the right panel's Current Repository | All Repositories still split it evenly).
  - The Git view section header's “Stage All” shows without hovering: it appears whenever there are changes to stage, and not otherwise.
  - “Amend Last Commit” below the commit box changed from a checkbox with text to an icon toggle (a pencil), placed left of “Undo Last
    Commit”, so the row is three icons: [toggle][undo] … [commit].
    On, it's the accent color; while committing it's grayed out and clicking doesn't toggle it; the tooltip is unchanged, and when on, the
    explanation is still the small line of text above.
  - Text buttons inside panels became icons (screen-reader names and tooltips unchanged, still reachable with Tab; grayed out when unavailable,
    with the reason in the tooltip): the conflict bar (Accept Ours / Accept Theirs / Accept Both / Accept All Ours / Accept All Theirs / Mark as
    Resolved / Diff Both Sides, ours in green and theirs in the accent color), the in-progress banner (Continue / Skip / Abort), the replace
    preview bar, the Diff line-selection bar, a hunk's floating buttons, search's Replace All / preview, the rebase wizard, and the terminal's
    exited bar. The worktree row's “Merge into main” lost its solid square background and “Review” its gray background.
    History's “Load N More” became a link style. Buttons in dialogs, Settings, About and empty-state pages are unchanged.
  - Editor and terminal tabs became flush, full-width tiles: the first tab touches the card's left edge, all tabs have the same height with no
    rounded corners, separated by a thin line on the right; the current tab has the card body's background, and others get a faint background
    on hover.
    Text is uniformly 12 / 500, and titles are at most about 200 pt, truncated with an ellipsis.
  - Terminal tabs scroll horizontally (like the editor, with a 2-pt thin bar at the bottom), the selected tab scrolls into view, and with few
    tabs “+” sits right after the last one; “Output” and “Problems” became compact icon-only tabs (followed by a count when there are entries),
    fixed at the end, with the vertical separators removed.

- Git view (#52): untracked files and unstaged changes merged into one “Unstaged” group, in the same order (tracked first, untracked after),
  with a count that's the sum of both.
  Each row is as before (letter, color, discard, stage, expanding untracked folders). The group header's + stages the group's changes and
  untracked files: with no conflicts that's `git add -A` (no paths passed, so even tens of thousands of files can't exceed the command-line
  length limit); with conflicts it stages these files by path, leaving the conflicted ones alone. Discard is still `git restore .`, which
  leaves untracked files alone, and it's hidden when the group is all untracked. The section header count's tooltip and the status bar also
  say just “Unstaged” (including untracked).
  The commit box moved above the changes (below the section header, banner and worktree group header), so with many files you don't have to
  scroll to the bottom of the section; “Amend Last Commit” scrolls the section header to the top.
  The commit button became an icon button that no longer says “Commit N files” (which overflowed the sidebar at its default width); the text
  moved into the tooltip and the screen-reader name, and it's still reachable with Tab.
  The worktree group no longer expands automatically because a worktree is waiting for you / running / mergeable; it lists them only when you
  open it (or jump there from the “Needs you” bar).
  The stash group header is also down to the arrow, icon and count (the tooltip and screen-reader name still say “Stash”).

- Git view (#54): worktrees and stashes became switches in Settings, both hidden by default. “Show worktrees in the Git panel” under Settings →
  Git controls the worktree group, the “Needs you” bar at the top and the section header's worktree count; when off, worktree status isn't
  computed and their folders aren't watched (turning it off stops existing watchers, turning it on reconnects them), and clean repositories
  with worktrees no longer expand automatically.
  “Show stashes in the Git panel” controls the stash group and the section header's stash count; when off, Stash and Pop Stash still work.
  When switched back on, each repository's worktree group keeps its expanded state (what's in the session isn't cleared).
  The commit button became a plain icon like the “Undo Last Commit” next to it (no background, a faint background only on hover), no longer a
  solid accent-colored square; when you can't commit it's grayed out and clicking doesn't commit, and the tooltip gives the reason (no commit
  message yet, nothing staged, already committing).

- File tree clicks (#50): repeatedly clicking the same folder expands / collapses it on every click (the second and third clicks within 500 ms
  used to be swallowed as a double-click, which looked like nothing happened); folder and repository rows ignore double-clicks, so a
  double-click just toggles twice, like VS Code and Zed, and double-clicking a file still pins it open. Holding for more than half a second
  before releasing no longer toggles / opens one extra time.
  Previewing a file by clicking no longer flickers: files that load quickly (within 40 ms) get their content in the same frame, so the editor
  area, outline and breadcrumbs don't show an empty frame first; slow files still load in the background, with “Loading…” after 150 ms.

- The file tree context menu is now drawn by Grove itself (#48, step two of #46): it opens at the pointer on right-button press, and pressing,
  dragging onto an item and releasing selects it (releasing right away without moving keeps the menu open). Items with submenus open after a
  0.2 s pause, and moving diagonally toward a submenu over other items doesn't close it (safety triangle); hovering a grayed-out item for 0.5 s
  shows a tooltip saying why, so labels no longer trail a string of parentheses. Clicking outside the menu only closes it, without the click
  landing on the row beneath; Esc, the window losing focus and scrolling the file tree all close it. Keyboard: ↑↓ skip separators and grayed
  items, → / ← open and close submenus, Return selects, and typing a letter jumps to the next item starting with it. It opens upward when
  there's no room below and leftward when there's no room on the right, and scrolls only when even the window can't fit it (the rule from #6
  is unchanged). Context menus elsewhere (tabs, Git view, history…) are still system menus, unchanged.
  - With keyboard focus in the file tree, ⇧F10 / the menu key still open the menu: it opens at the left of the cursor row, right below it,
    with the first item highlighted so ↑↓ and Return work immediately; closing it without choosing returns the keyboard to the file tree.
  - Right-clicking (or ⌃-clicking) elsewhere in the file tree while the menu is open closes it and opens a new one at the pointer on that row
    (or blank space), like system menus and VS Code; right-clicking a row outside a multi-selection still only outlines it without clearing
    the selection. Left clicks and other buttons outside only close the menu and are swallowed.
  - In the dark theme the highlighted item is a darker solid accent color with white text (at least 4.5:1), no longer a light-blue background.

- File tree mouse feel (#46): an unselected row is selected on the frame it's pressed, drawn with a “pressed” background (one shade darker than
  hover), and the file opens / folder expands only on release; pressing and dragging over several rows selects them without opening anything
  on release. Right-clicking an unselected row no longer clears an existing multi-selection: the menu applies only to it, and the row gets an
  accent-colored outline that goes away when the menu closes. Row tooltips now appear after a 0.5 s pause, and once one has shown, moving
  between rows switches straight to the new row (warm start); they hide on press and right-click. Folder arrows no longer turn the cursor into
  a hand. The context menu is still mygo's `PopoverMenu` (it already opens on press, highlights on hover, handles the keyboard, closes on Esc
  or a click outside, and opens upward / leftward when it doesn't fit); press-drag-release to select, submenu delays and the safety triangle,
  hovering grayed items to see why, and clicks outside closing it without landing beneath are things it can't do, left for the next step of
  drawing our own menus.

- The bottom panel can be maximized (#44): ⇧⌘J, the button at the far right of the bottom panel's title row, or double-clicking blank space in
  the title row makes the bottom panel (Terminal / Output / Problems) take over the editor area's space, and doing it again restores the
  previous height (with the terminal on the right, it fills the space between the sidebar and the right panel). It isn't saved in
  state.json; it restores automatically when you close the bottom panel, open or switch to an editor tab, or enter Zen Mode; with the bottom
  panel closed, ⇧⌘J opens it maximized directly.

- In plain folders (not Git repositories) the file tree lists dot-folders (`.venv`, `.pytest_cache`, `.ruff_cache`…) and `node_modules`
  (#42): grayed out and collapsed by default like ignored folders in Git repositories, reading one level when expanded; they aren't walked,
  so a `.venv` with tens of thousands of files doesn't slow down opening, and ⌘P and full-text search skip them too.
  They're unaffected by “Show Ignored Files”, and “Hidden files” can still hide them; `.git` and `.DS_Store` are still not listed.

- Git view redesign: a worktree is an agent's workstation (#35). Each worktree gets one row with a single status symbol: waiting for you (amber
  dot), running (spinner), Ready to merge (green check), Has changes (folder plus change count), idle (hollow circle), No agent, Merged (merge
  icon, translucent; “prunable” is folded in). The group is sorted in that order, with the main worktree first.
  Branch, path and main / open / locked go into the tooltip or small icons. Hovering shows Terminal / Review / Remove Worktree buttons. Clicking
  a row (or Return) expands it: a summary (commit count, file count, +added −deleted), Review and “Merge into main…” (asks first, then uses the
  existing merge), the files `main...branch` adds (click to open the Diff), and the agent's terminal.
  When a worktree is waiting for you, running or ready to merge, the group is expanded by default, and a collapsed group header shows only the
  level that needs you most. A “Needs you” bar at the top of the sidebar lists worktrees in every repository that are waiting for you or ready to
  merge, and previous / next jump between them. Repository section headers: a green checkmark in a circle when clean, a change-count badge
  when there are changes, ↓ / ↑ in gray, and Pull / Push as 24-pt square icon buttons; the “no uncommitted changes” row is gone. The Unstaged /
  Staged group headers, the worktree group header and the Files / Search / Git / Outline switcher at the top of the sidebar keep only their
  icons (screen-reader names and tooltips unchanged).
- Grayed-out Git actions say why (#32): Fetch, Pull, Push, Force Push, Sync, Stage / Unstage All, Discard, Stash / Pop Stash, merge / rebase onto
  a branch, switch / rename / delete branch, Push Tag, remove worktree, undo commit, cherry-pick / revert / reset. Native menus put the reason in
  parentheses after the label (“Push (in sync with origin)”, “Remove Worktree (the main worktree can't be removed)”, “Merge into Current Branch
  › feature (already merged)”), title bar and Git view buttons put it in the tooltip, and the Command Palette shows it after the grayed-out
  command; clicking an unavailable Git action from the menu bar also shows the reason. All reasons are computed in one place,
  `internal/app/gitwhy.go`, in “most fundamental first” order, and only one is reported.
  The check runs again on click: if state changed while the menu was open (a watcher refresh, another agent just pushed), the action doesn't run
  and the reason is shown.
  Behavior changes: Push is grayed out when in sync with, only behind, or diverged from upstream (use Force Push when diverged); Fetch is
  grayed out with no remote; Pull, Sync and Stash are grayed out during a merge / rebase in progress; “Merge into Current Branch / Rebase
  Current Branch onto” are grayed out when there are conflicts; in the “More Git actions” menu, the item is called “Push and Set Upstream” when
  there's no upstream.
- Agent status in the terminal (#33): when claude, codex, gemini, aider or opencode is running in the foreground of a terminal (more can be added
  in Settings → Terminal → “Agent commands”), Grove works out “waiting for you”, “running” or “idle” per pane and rolls it up per worktree.
  When the program rings the bell (BEL) or posts a notification (OSC 9 / OSC 777) while you aren't looking at that terminal, it's waiting for you,
  and an amber dot appears after the terminal tab's icon, disappearing when you switch to that terminal; output in the last 10 seconds means
  running, shown as a slowly spinning gray circle on the tab; idle isn't marked.
  Everything is computed locally, looking only at Grove's own terminals (the foreground process is read with ps every 2 seconds), without
  reading the agent's logs or hooking into its APIs; terminals running in the background (termd) are recognized too. Showing it in the Git
  view comes later.
- Right panel history redesign (#34): the header row is the repository name and branch, with icon toggles on the right (search, All branches,
  Agent commits only, All Repositories) and close; the “Current Repository | All Repositories” segmented switch, the always-visible search
  field and “N commits” are gone. The search field expands only when you click the magnifier; Esc clears it first, then collapses it. The
  commit graph toggle moved to the header's context menu.
  One row per commit (30 high), with the agent mark, branches / tags and time on the right (`HH:MM`, replaced by the short hash on hover);
  sections by date (Today, Yesterday, November 3).
  The current branch uses the accent color; remote branches are just a cloud icon. “Uncommitted changes” is an orange pencil plus the change
  count. An expanded commit is a card: the body (at most two lines), the short hash (click to copy the full hash), author, agent, and Compare
  with HEAD / Open on the web / More (full hash, email, exact time, parent commits and commit actions), followed by the changed files. The
  commit graph's lane colors are softer and avoid red, lines are 1.8 wide, and nodes have a ring of background color; “uncommitted” is an
  orange dashed circle.
  “All Repositories” no longer draws the commit graph; each row has a repository dot on the left and the repository name in gray on the right.
- History: commits whose message has a `Co-Authored-By:` trailer from a coding agent (Claude, Codex, Copilot, Gemini, Cursor…) carry a
  four-pointed star; the header's “Agent commits only” keeps just those commits, combined with search and All branches (`git.AgentOf`).
- Git view: inline buttons on changed-file rows (#31). When the pointer moves over a row (or it's selected while the list has keyboard focus), a
  row of small buttons appears on the right, in VS Code's order: Open File · Discard Changes · Stage for unstaged files, Open File · Delete File
  · Stage for untracked files, Open File · Unstage for staged files, and Delete · Stage for whole untracked folders.
  Discard and delete still ask first. When the row under the pointer is part of a multi-selection, the buttons act on the whole selection (the
  tooltip gives the count). Clicking a button doesn't change the selection or open the Diff; long file names are truncated with an ellipsis
  first, the buttons hug the right edge, and the row height doesn't jump.
  The “Unstaged” group header gains “Discard All Unstaged Changes…” to the left of +.
- Git view keyboard: with the list focused, Space stages / unstages the selected changes and ⌘⌫ discards (asking first), acting on all of them
  when several are selected; both are in the Command Palette and the shortcut settings (a new “Git Sidebar” group).
- Encoding detection: when judging EUC-KR, the share of common syllables counts distinct syllables, so short GB18030 Chinese text with the
  same character repeated a few times is no longer taken for Korean.
- Git view: after staging / unstaging / discarding the selected row (via the context menu, inline buttons or keyboard), the selection stays on
  what was the next row instead of being lost.

- Git view: each repository's section header gains Pull and Push buttons on the right that act only on that repository, with the ahead (↑n) /
  behind (↓n) commit counts next to them (previously after the branch name).
  Tooltips match the same buttons in the title bar: with no upstream, Pull is grayed out and Push becomes “Push and Set Upstream”; while
  pulling / pushing, the button spins and can't be clicked again (the title bar's too).
  The push button's context menu has Force Push and Sync.
- Larger sidebar text (#5): body text in the file tree, Git view, Search, Outline and Open Editors goes from 12.5–13 pt to 14 pt, and secondary
  information such as branches, counts, folders and group titles from 10.5–11 pt to 12 pt; file tree rows go from 22 to 24 high, and the Git
  view's repository section headers, group titles, stash rows and worktree block headers are each 2 higher.
- Editor: when moving the cursor with the keyboard or typing scrolls the view, 5 lines are kept above and below the cursor (one fifth of the
  view height when the view is short), at the end of the file too, so the cursor no longer hugs the bottom edge of the editor area. Mouse
  clicks, the scroll wheel and dragging the scroll bar are unaffected.
- File tree: right-clicking low in the window no longer gives a folded-up menu that needs a click to see in full. The file tree's context menus
  (files, folders, repositories, blank space, pinned rows) became menus drawn inside the window that open upward when there's no room below
  and scroll only when even the window can't fit them (#6).
- Editor: typing in a large file no longer recomputes the whole document on every keystroke (#11). The sticky-scroll outline used to wait for
  you to stop typing before reparsing only for files over 256 KB; now files whose last parse took over 1 ms do the same (the old outline is
  used while typing, and it reparses after 0.5 s of quiet). The gutter's change hunks (used by the card that shows the original text) are
  always computed incrementally from the same comparison, re-diffing only the lines around the edit instead of diffing the whole file against
  HEAD on every key. Small files of a few hundred lines are still recomputed every time.
- Editor: for large files with unsaved edits, mouse movement, scrolling and cursor blinking no longer copy the whole text twice per frame to
  compare it (the tab's unsaved dot and the status bar both ask “is it modified?”), so scrolling and typing in large files have fewer
  occasional GC hitches. Undoing back to the on-disk content still clears the dot (#12).
- Tabs: tabs with unsaved changes (including the current tab) show a dot on the right, which turns into × only when the pointer is over the
  tab; clicking it still closes the tab and asks whether to save. Previously the current tab always showed ×, so you couldn't tell the file you
  were editing was unsaved (#17).
- File tree: the pinned repository / folder levels used to have a hard 1-px line underneath, which made the half-covered row below look like a
  drawing bug; it's now a soft shadow like the editor's sticky scroll (tuned separately for light and dark), so it reads as the row scrolling
  under the stack (#18).
- File tree: the currently open file's name is no longer bold and is distinguished only by the selection background; the ellipsis in long
  names no longer jumps around when files are opened / switched away from (#15).
- File tree noise reduction (#20):
  - A row's tooltip (“5.0 KB · just now · no changes”) appears only after resting on the row for a full second (previously 0.6 s), below the row
    instead of covering the row above; moving between rows restarts the wait for each new row, so the next tooltip doesn't pop up the moment
    one disappears. Clicking the row hides its tooltip, and it doesn't come back until the pointer leaves. When the name is truncated with an
    ellipsis, the tooltip's first line is the full name. Tooltips elsewhere are unchanged.
  - Colored type icons in the file tree and Open Editors lost their background blocks and are softer (closer to the secondary text color), and
    programming-language letters are no longer bold; colors and shapes still follow the type. Icons in tabs, Quick Open, breadcrumbs and
    elsewhere are unchanged.
- Editor (#21): inside selected text, the space dots and tab arrows are much fainter (about like indent guides), no longer louder than the text.
  When the cursor rests on a word (with no selection) for about 250 ms, other occurrences of the same word in the file get a light background
  (whole word, case-sensitive); not shown while typing, and in very large files only the visible lines are marked.
- Editor tab switching shortcuts (#19): ⇧⌘[ / ⇧⌘] and ⌥⌘← / ⌥⌘→ switch to the previous / next tab in the current editor group, wrapping at the
  ends; with the keyboard in the terminal they still switch terminal tabs (the terminal now also accepts ⌥⌘← / ⌥⌘→). ⌃Tab switches to the
  previously used tab (pressing repeatedly toggles between two), and ⌃⇧Tab to the least recently used. All are in the “View” menu, the Command
  Palette and the shortcut settings.
- Editor: clicking a file shows the text first instead of waiting for git to read the HEAD version (used only for the gutter's change markers,
  Peek and “compare with HEAD”); that version is read in the background and filled in afterwards, and on rereads the old markers are kept so
  nothing flickers. History version tabs do the same, with the parent commit's version filled in afterwards. “Loading…” appears only when a read
  hasn't finished after about 150 ms, so fast reads (the vast majority) no longer flash a spinner, and preview tabs no longer flash blank when
  switching files (#22).
- The file tree and sidebar lists (with the default faint selection) now distinguish three row backgrounds (#16): with keyboard focus in the
  list, the selected row has a faint accent background plus a bar on the left (active selection, with a slightly stronger accent than before);
  with focus in the editor or elsewhere it turns neutral gray with no bar and unchanged text color (inactive selection, as in VS Code); hover is
  fainter than inactive selection, and in the dark theme it no longer looks almost the same as selection. The Command Palette's list always
  draws the active selection. Unchanged when the faint background is turned off in Settings (solid selection block).

## v0.1.0 — 2026-11-02

First public release.

- After opening a `.code-workspace` or folder, all repositories are shown at once: one root per repository in the file tree, one section per
  repository in the Git view. You can multi-select and drag across repositories, and undo the last file operation.
- Editor: multiple tabs, drag to an edge to split, and the same file can be open in two groups. Outline, sticky scroll, snippets, Markdown
  preview (tables, with images inside tables too; Mermaid, math), and image and hex viewing. Recognizes encodings such as GB18030, Big5 and
  Shift-JIS.
- Git: stage by hunk or by line, resolve conflicts with a three-way merge, and view the commit graph for one repository or all of them combined.
  Merge, rebase (including interactive), cherry-pick, stash, tags, worktrees and submodules can all be done in the interface. Refreshing
  status uses only read-only commands, with `GIT_OPTIONAL_LOCKS=0`.
- Terminal: based on Ghostty's libghostty-vt, several per repository, with the layout and recent output saved when the window closes.
- Search and replace across repositories, ⌘P to find files, ⇧⌘P for the Command Palette.
- Formatting (gofmt, prettier, etc.) and linting (go vet, eslint, etc.) can run on save, with problems collected in the “Problems” panel.
- Local history: a copy is kept on every save, so mistaken edits and deletions can be recovered.
- Multiple windows, single instance; `open -a Grove <path>` opens in the already running Grove.
- Chinese and English interface, light and dark themes, customizable shortcuts.
- Based on mygo v0.7.4.
