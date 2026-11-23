# Markdown rendering

[← Architecture overview](../architecture.md)

The Markdown preview (internal/gmd), styles for HTML blocks (internal/css), source preprocessing (internal/mdprep, internal/emoji), Mermaid and math (internal/render), plus the side-by-side preview and synchronized scrolling.

## Markdown rendering

### Overview: how a Markdown document gets drawn

| Step | Code | What it does |
|---|---|---|
| Get the source | `markdownPreview` in `ui/mdpreview.go` | Fetches the source every frame (the side-by-side preview takes the source file's buffer) |
| Preprocess | `internal/mdprep` (`mdprep.Prepare`) | Turns front matter and HTML containers spanning several blocks into fenced code blocks; finally replaces emoji shortcodes via `internal/emoji` |
| Draw | `internal/gmd` (`mdShow` in `ui/gmdview.go` → `gmd.View`) | Parsing, layout, selection and copy; `WORKBENCH_GMD=off` falls back to mygo's `markdown.View` |
| Styled HTML blocks | `internal/css` + `gmd/html.go`, `gmd/htmlrender.go` | Computes the cascade and lays out as grid / flex / boxes |
| Content Grove adds | Hooks installed by `mdEnv.setup` in `ui/mdpreview.go` | Mermaid diagrams and TeX math (through the WebView in `internal/render`), tables with images (`ui/mdtable.go`), front matter cards, collapsible blocks, rows of badges… |

Both views consume the same set of hooks: the fields of `gmd.Options` map one-to-one to mygo's `markdown.Options` (`CodeBlock`, `Code`, `Block`, `HTML`, `Math`, `InlineMath`, `LinkMenu`, `OnLink`, `CodeLink`, `Preview`, `LoadImage`…), and `gmd.Options.Mygo()` copies the fields into mygo's version each time (the code block hook gets wrapped, swapping mygo's `CodeBlockView` for gmd's). gmd additionally has `CSS`, `Image` and `Strings` (the few strings shown in the view; `ui.mdStrings` supplies the translated ones).

### internal/gmd: Grove's own Markdown view

It follows the same approach as `internal/ged`: parsing still uses mygo's `ui/markdown/parser`, while layout and drawing use only mygo's public UI APIs (Column / Row / Grid, RichText, Painter). It knows nothing about Grove's theme or `internal/ui`: its look comes from `markdown.Theme` (`gmd/theme.go`) and its strings from `Options.Strings`. The rendering of paragraphs, lists, tables and code blocks is adapted from mygo's `ui/markdown`, with features Grove doesn't use — streaming display, mentions / citations and link icons — removed.

Main files:

- `view.go`: `View(c, src, o)` draws a document. State (the parse result, paragraph text, code block highlighting, table column widths, the computed styles of HTML blocks) is kept on the returned element (`viewState`), and blocks whose src hasn't changed aren't recomputed next frame. It dispatches by block kind (paragraph, heading, thematic break, code block, HTML block, `$$` math, blockquotes and GitHub alerts, lists, tables), and also handles footnote numbering and the footnote list.
- `options.go`: `Options` (see above) and `Mygo()`.
- `spans.go`: a paragraph is styled text (RichText), split at links, with each link being a `ui.Link` inside; it records where each span of text sits in the source, for copying.
- `code.go`: `CodeBlockView` and `DefaultCodeBlock` (the default look when no hook takes over, with a copy button); only changed lines are re-highlighted.
- `table.go`: automatic table layout (see [Tables and tables with images](#tables-and-tables-with-images) below).
- `image.go`: images. It asks `Options.Image` first (the caller manages the cache); if that isn't provided, it calls `LoadImage` in a background goroutine on first use and caches the result in `Options` (`sync.Map`).
- `html.go`, `htmlrender.go`: the HTML block's own HTML + CSS layout (see [Styled HTML](#styled-html) below).
- `copy.go`: selection and copying. Copying as text puts a blank line between paragraphs, code blocks and tight lists line by line, and table cells separated by Tab. "Copy as Markdown" (context menu, or ⇧⌘C when the view has focus) copies the source between the two ends of the selection, completing any emphasis, code or link syntax that was cut off.
- `alert_icons.go`: Octicons icons for GitHub alerts (Note, Tip…).

`ShareSelection`: when the view is embedded in another selectable container (the body of a `<details>`, a chunk of a large file, a table cell), it shares the selection with the outer container and is no longer Selectable itself or handles copy.

### internal/css: style sheets for HTML blocks

UI-free CSS parsing and cascading. Elements are supplied by the caller implementing the `css.Element` interface (`Tag` / `Attr` / `Parent` / `Prev` / `Next`); gmd's `hnode` (`gmd/html.go`, a node parsed with `x/net/html`) is that implementation.

- `parse.go`: `ParseSheet` (contents of `<style>` → `Sheet`), `ParseDecls` (`style=` → `[]Decl`); strips comments and recognizes `!important`.
- `selector.go`: selectors (compound selectors plus the `' '`, `>`, `+`, `~` combinators) and specificity.
- `cascade.go`: `Compute(author, e, pseudo, inline, parent, rootPx)` computes the `Style` of an element (or of `::before` / `::after`). Cascade order: UA → presentational attributes → the document's `<style>` → `style=`; reversed for `!important`. Within the same level, by specificity, then by order. It expands shorthands and handles inheritance and `var()`; `font-size` is always converted to px, `font-weight` to a number, em / % / rem in `line-height` to px, and `currentcolor` to the element's `color`.
- `ua.go`: `UA` (following the Rendering section of the HTML standard, with the browser default styles that can't be drawn removed) and `Presentational` (declarations derived from `align`, `valign`, `width` / `height`, `bgcolor`, `<font>`, `<table>`'s `border` / `cellpadding` / `cellspacing`, and `nowrap`).
- `color.go`, `length.go`: `ParseColor` (hex, `rgb()` / `hsl()`, color names), `ParseLength` (px, em, rem, %, fr, vw / vh; pt and the like are converted to px), `ParseTracks` (grid tracks).

Only what the preview can actually draw is supported. Unrecognized rules, selectors and values are skipped silently — no errors, no panics.

### Styled HTML

1. `gmd.DocCSS` collects `<style>` only from HTML blocks (not from fenced code). `ui` caches it per preprocessing result (`mdView.styles`), puts it in `mdEnv.css`, and passes it to `Options.CSS`, so large files drawn in chunks and paragraphs nested inside `<details>` are all laid out with the whole document's style sheet.
2. The HTML hook in `ui/mdhtml.go` first asks `gmd.WantsCSS`: does the block contain `<style>`; or does the document have a style sheet and the block uses `class=` / `id=`; or does `style=` contain properties such as background, border, `display`, margin / padding, grid, flex, positioning, shadow or width? If so, it hands off to `Options.HTMLView` (laid out with `Options.CSS` plus the block's own `<style>`); if not, it takes the README-style path described in [Preview hooks](#preview-hooks) below (centering, `<details>` and badges are unaffected). With `WORKBENCH_GMD=off` everything takes the README-style path.
3. `gmd/htmlrender.go` builds mygo elements from the styled nodes:
   - block flow (with margin collapsing), flex (Row / Column, wrapping, alignment), grid (fr / px / `repeat` / `minmax` in `grid-template-columns`, `grid-column: span`), tables (columns sized by content, `colspan`), absolute positioning;
   - background colors and linear gradients, borders (native borders when all four sides match, trapezoids otherwise), rounded corners, shadows, opacity, `overflow: hidden`;
   - inline content joined into one run of rich text (links, decorations, inline backgrounds, empty inline-block dots drawn as objects).
4. Text color (`textColor`): when the author sets a background but no text color, the text switches to a color that is readable on that background; when the author's text color is nearly indistinguishable from the background (for example dark gray text hard-coded into a page viewed in a dark theme), it is also switched to a readable one (`readable`).
5. HTML blocks containing only `<style>` (plus comments, `<script>` and the like) take up no space.

### Tables and tables with images

The preview spans the full width of the tab (as in VS Code; no column-width cap).

Tables follow the browser's automatic table layout (`gmd/table.go`):

- Each column asks for the width of its widest cell laid out on one line (max-content), and shrinks no further than its longest unbreakable piece (min-content; `minContent` measures by splitting at spaces, hyphens and CJK characters).
- When it fits, the whole table is sized by content; when it doesn't, the table takes the column's width and each cell gives way in proportion to what it asked for, via `Width(max).Shrink(1).MinWidth(min)`.
- Only when even the sum of min-content widths doesn't fit is it wrapped in `ScrollHorizontal` for horizontal scrolling (based on the column width from the previous frame; if `Draw` notices it changed, it requests another frame).

Tables with images are in `ui/mdtable.go`: when the `Block` hook meets a table with an image in a cell (`tableHasImage`), it builds the table itself — equal-width columns filling the column, an all-empty header row not drawn, and each cell a small `ShareSelection` preview view (`mdShow`; options are stored on the table element's `ui.Local`, so the image cache survives). `cellSource` backslash-escapes leading text that looks like block markers, so a cell isn't parsed on its own as a heading or list. Other tables are still laid out by the preview itself.

### internal/mdprep and internal/emoji: preprocessing before parsing

`mdprep.Prepare` handles constructs that mygo's parser doesn't recognize but that are common in READMEs:

- front matter at the start of the file (`---` YAML `---` or `+++` TOML `+++`);
- HTML containers spanning several blocks (`<details>`, `<div align>`, `<p align>`, `<center>`… with Markdown written between the opening and closing tags, separated by blank lines).

Each such region is replaced with a fenced code block whose info string is `grove-region N`, with the contents of region N recorded in `Doc.Regions`; the preview recognizes it in the `CodeBlock` hook and draws it itself. `Doc` also carries the link reference definitions (`Defs`, appended after each chunk when drawing in chunks) and whether there are footnotes (`Footnotes`; if so, no chunking).

At the end, `Prepare` replaces shortcodes like `:tada:` with emoji via `emoji.Replace` from `internal/emoji`:

- The table `emoji/gemoji.txt` holds GitHub's gemoji aliases (one "alias TAB emoji" per line, embedded in the binary and read into a map on first use; `Lookup` looks one up).
- Shortcodes inside fenced code blocks and inline code aren't replaced, and colons with a letter or digit directly on either side (`10:100:20`) don't count; shortcodes inside the fenced blocks produced by preprocessing aren't replaced either (the Markdown inside a container is replaced when it is preprocessed separately).

Both steps only replace text within a line and never add or remove line breaks: line k of the source is still line k after preprocessing, so outline navigation and synchronized scrolling line up by line number.

### internal/render: Mermaid and math

There is one `Renderer` per window (`a.Renderer()` in `app/render.go`, created lazily; the `NoDiagrams` setting turns it off, showing only the code block and the raw TeX).

- The preview requests results every frame (`Get`): a cache hit returns immediately; otherwise the request is queued, and in the meantime the version of the image from before the theme / scale change stands in, or, if there's no old version, the code block or raw TeX is shown.
- A single background goroutine renders requests one by one and notifies the UI to redraw when each finishes.
- The cache holds at most 512 images / 32 MB; the queue holds 128 requests and drops the oldest when full (new requests take priority).
- After 60 seconds idle the backend is destroyed and recreated when next needed. An error in the source itself (`SourceError`) only means that one image can't be drawn; any other error is treated as a broken backend, which is closed and recreated.

The backend (`web.go`) is an invisible `mygo.WebView` running `page.html` (`wbRender`). Resources are embedded in the binary, with scripts stored as `.js.gz` and decompressed when served to the WebView over the custom `wbrender://` scheme (`assetHandler`: serves the plain file if there is one, otherwise looks for the `.gz`). The page loads scripts on demand.

#### Mermaid

mermaid.js 12.3 draws all diagrams; their look is set by `groveConfig` in `page.html`:

- `theme: "base"`, `look: "classic"`; `layout` is left unset so Mermaid 12 picks one itself (flowcharts, state diagrams, class diagrams and ER diagrams use ELK, with right-angle edges and rounded corners; setting `elk` globally breaks mind maps).
- `themeVariables` are blended from the palette into the black-and-white document style common in VS Code's Markdown preview: nodes with white fill (in dark mode, a gray one step lighter than the background) and thin dark borders, edges in the text color, edge labels on a light gray background, and nodes with Mermaid's own faint drop shadow offset to the lower right (`themeCSS` applies it only to shapes, not text).
- Pie charts, mind maps, git graphs and Gantt charts use a low-saturation set of six colors (pale in light mode, dark in dark mode, with text always in the body text color; VS Code 2026's `charts.*` and Mermaid's own default theme are both too saturated).
- `minNodeWidth` is turned off for `flowchart` / `state` (Mermaid 12 defaults to 120, which stretches short nodes).
- After drawing, `polish` does only one thing: moves the text of a mind map's circular root node back to the exact center.
- The user's own front matter / `%%{init}%%` still overrides all of this (for example `layout: dagre` to go back to the old layout).
- `htmlLabels: false`, so labels are drawn as SVG text, which is the only way the canvas can capture them. Every diagram is drawn onto a canvas and captured as PNG.
- Mermaid 12 needs WebKit from Safari 17.4 or later: `mermaidSVG` checks first and throws `render.OldWebKit` on older versions, which the preview replaces with a message asking the user to update macOS ("Mermaid diagrams need Safari 17.4 or later: update macOS in System Settings → General → Software Update").

Colors: `mdPalette` in `ui/mdrender.go` computes them from the theme (background, text, line, accent, node fill, border); the JSON goes in `Request.Palette` and is part of the cache key. When the theme changes, the old image stands in while the new one renders. The background is taken from the window background of mygo's light / dark theme, not from the current element's `Background` (if it differed between layout and drawing, the diagram would be rendered twice).

#### Math

MathJax 4 (`tex-svg.js`, with the New Computer Modern font; the font chunks and the font extensions for mhchem / bbm / dsfont are in `mathjax/fonts/`, which `loader.paths.fonts` points to, so nothing is fetched from the network).

- When the SVG contains only paths (`fontCache: "none"`, glyphs in `currentColor`), the SVG text is returned, `render` parses it with `ui.ParseSVG` into `Result.Vector`, and mygo draws it with the text color and pixel density at draw time.
- Only output mygo can't draw, such as `<text>` (characters missing from the font, for example Chinese inside `\text{}`), is rendered as PNG.
- Vector results don't depend on theme, scale or text color: `Get` hands back the same image (`near`) directly, without queuing.

#### Hooking into the preview

`mdRender` in `ui/mdrender.go` wires the renderer into `gmd.Options`: it installs `Code["mermaid"]`, `Math` and `InlineMath`, and `$…$` inline math relies on `ExtDollarMath`. Result keys include the theme and palette, and sizes are multiplied by the UI scale's `ZoomFactor`. It returns `Gen()`, which the preview uses as that column's key (when new inline math results arrive, paragraphs are laid out again). When there is no renderer (turned off in Settings, or not provided in tests), these hooks are removed and `$…$` isn't recognized either, staying as plain text.

### Preview hooks

The preview (`markdownPreview` in `ui/mdpreview.go`) fetches the source every frame, runs it through `mdprep.Prepare`, then passes it to `mdShow`. Its options are `MarkdownOptions` from `app/markdown.go` (stored in `Doc.MD`, created on first preview and rebuilt after a rename or after the remote images setting changes), and all hooks are installed in `mdEnv.setup`:

- Blocks: `CodeBlock` (`codeBlock` in `ui/mdblocks.go`) recognizes `grove-region` fences: front matter is drawn as a collapsible key-value card (`mdFrontMatter`), and containers as collapsible blocks / aligned columns, with the Markdown inside rendered by a nested `ShareSelection` view (`mdEnv.nested`; options and the preprocessing result are stored on the element's `ui.Local`). The `Block` hook is always installed: it places jump markers and the sync probes (below), and takes over paragraphs consisting only of images (a row of badges) and tables with images.
- HTML: the `HTML` hook (`mdHTML` in `ui/mdhtml.go`) hands styled blocks to `Options.HTMLView` (see above); everything else is parsed with `x/net/html`. Block-level tags each become their own block, runs of adjacent inline nodes are converted to Markdown and laid out as one paragraph, image-only content is drawn as a row of images sized by `width` / `height`, and `<picture>` picks a `<source media>` based on the theme. Unknown tags show only their text; `<script>`, `<style>` and comments aren't shown.
- Images take one of two paths:
  - Images gmd draws itself (Markdown's `![]()`) go through `ImageLoader` in `app/mdimages.go` (that is, `Options.LoadImage`): local paths are read relative to the document's folder and cached in gmd's `Options`.
  - Images Grove builds itself (rows of badges, `<img>` in HTML; `mdImage` in `ui/mdblocks.go`) go through `PreviewImage`: a process-wide cache (shared by all windows and previews, sized by pixel bytes, 48 MB) that tells the UI to redraw once the background load finishes. SVGs with text (shields.io-style badges) are rendered to bitmaps by the window's renderer (WebKit); only when there's no renderer does it fall back to mygo's drawing (which can't draw the text, leaving just the colored blocks).
  - Both paths accept local files, `data:` URLs and remote URLs; remote ones are fetched in the background via `FetchImage` (GET only, no cookies, 8 MB limit, at most 6 at once; stubbed out in tests). The `PreviewImage` path doesn't load `http:` URLs (`imageKey` returns `errInsecure`); `ImageLoader` has no such check. The `NoRemoteImages` setting turns off remote images, showing only their alt text. `ForgetImages` clears the process-wide cache (for tests).
- Theme: `mdTheme` in `ui/mdtheme.go` adjusts font size, line height, and heading and block spacing on top of mygo's default theme, using the editor font for code; the VS Code style follows its `markdown.preview.fontSize` 14 / `lineHeight` 1.6. Cached per (UI theme, font, style).
- Emoji shortcodes: see `internal/emoji` above.
- Links:
  - `LinkMenu` (`mdLinkMenu` in `ui/mdlinks.go`): Open Link and Copy Link, plus Copy Path and Show in Finder for local files, followed by editing items for the selected text.
  - `Preview` (`mdLinkPreview`): resting on a link to a local file shows its path within the repository (written as `~/…` outside a repository), or says if it can't be found.
  - `CodeLink` (`codeLink` in `ui/mdblocks.go`) turns inline code that looks like a path, and that `os.Stat` finds under the document's folder or the repository root, into a link (results cached per code string in `mdEnv.links`).
  - All of these only stat on the main thread; they never read contents.

Clicking a link (`MarkdownLink` in `app/markdown.go`): `#heading` scrolls to that heading; http, https and mailto are handed to the system; anything else is treated as a file path (relative to the document's folder, optionally with `:line[:col]`) and opened in the editor — Markdown files open straight into the preview (scrolling to the heading if there's a `#fragment`), and directories are revealed in the file tree. Other schemes aren't opened.

### Jumping

Clicking a `#heading` link sets `d.Anchor`; clicking an item in the sidebar outline, or scrolling the editor alongside, sets `d.AnchorLine` (1-based).

- `markdownPreview` only honors values present at the start of the frame (one set by clicking a link during this frame takes effect next frame).
- When the `Block` hook reaches that heading / the top-level block covering that line, it places an invisible marker taller than the view (`mdMarker`) and calls `ScrollIntoView`, which scrolls that block to the top.
- If nothing is found, it shows a notice (`#heading`) or gives up silently (line numbers).

### Large files

After preprocessing, documents over 32 KB (`mdChunkMin`) with no footnotes are split into chunks by top-level blocks (`mdSplit`: a new chunk starts at the next heading once a chunk reaches 512 bytes, or at any block once it reaches 4 KB). Each chunk is its own `mdShow` view, placed in a `ui.List` that lays out only the visible ones.

- When jumping to a place not yet laid out, it first does `ListState.ScrollTo` to that chunk (`seekChunk`), and places the marker next frame once the chunk is laid out, trying for at most a few frames.
- The line range of each chunk's last block extends up to the first line of the next chunk.
- `mdView` (stored in `Doc.MDView`, kept per document) remembers the preprocessing result, the chunks and the list position: switching between source and preview doesn't preprocess again; when the source changes while typing, it waits for a 150 ms pause before laying out again (`mdSettle`), but right after switching over (not drawn for 500 ms, `mdIdle`) it lays out immediately.

### Side-by-side preview

`app/mdside.go`. Entry points: ⌥⇧⌘V (`view.markdownSidePreview`), or the "Open Preview to the Side" button on a Markdown tab strip (as in VS Code, the tab strip has no eye button for the in-place preview; the in-place preview, ⇧⌘V, is reached through the shortcut and the menu).

- It is a `docs.MDPreview` tab: `MDSource` points to the source file's Doc, and its Key is `(MDPreview, Root, Rel)` (an absolute path when outside a repository).
- It goes into the neighboring group (moving there if a neighboring group already exists; with only one group, `splitOff` splits off a new group to the right), as a pinned tab. On opening, it scrolls to the section the editor is currently showing, and the keyboard goes back to the source file.
- It doesn't read from disk; it takes `MDSource`'s buffer every frame.
- When the source file's editor isn't visible (the preview was dragged into the same group, or the source file is showing the in-place preview; `App.EditorShown`), the preview doesn't follow it: an invisible editor isn't laid out, and its `TopLine` stays at the last frame drawn.
- These tabs aren't saved in the session (state.json is unchanged).

#### Mapping to the source (`ui/mdsync.go`)

Modeled on VS Code's Markdown preview:

- Probes: each top-level block contains an invisible probe filling it (`mdEnv.topBlock`), which records the block's line range and its frame from the previous frame in `mdView.blocks`; `lineAt(y)` converts a position within a block proportionally into a fractional line number.
- Spread by screen rows: as in VS Code, the span from the top of one block to the top of the next (including the gap between them) maps to the span from the block's first line to the next block's first line. Within it, positions are spread by the source editor's screen rows (`Doc.RowOf` / `LineAtRow`, accounting for ged's wrapping and folding; in mygo's editor each line is one screen row), not by source line count. With line counts, a paragraph wrapped onto three rows and the blank line after it would each count as one line, so when the editor scrolls at a steady speed the preview would alternate between fast and slow frames.
- When placing the jump marker, the frames of this block and the next are both taken from the previous frame's `mdView.prev` (measured in the same layout pass, so while scrolling the difference doesn't carry a frame's worth of scroll).
- `RowOf` / `LineAtRow` use ged's `etops`, computed only from estimated wrap counts (`TopLine` still uses `tops`, from actual layout): while scrolling, newly laid-out lines differ from the estimates, so `tops` changes and so does the denominator of the in-block ratio, making the preview jump around; `etops` is recomputed only when the lines, width or font change, and stays put while scrolling.
- For a chunked preview (`seekChunk`), if the block that needs the marker was already laid out last frame, it places the marker directly instead of first doing `ScrollTo` to the chunk's start; otherwise it would flicker between the chunk start and the original spot every other frame.
  Blocks that draw nothing (`<style>`, comments) are always zero height, so they count as laid out when another block in
  the same chunk was (chunks are laid out whole); otherwise scrolling the editor through a long `<style>` block flickers the same way.

#### Two-way sync

- Editor → preview: when `Doc.TopLine()` changes (for ged it's `Editor.TopLine`, fractional: how much of the top line has scrolled past), `AnchorLine` is set and the fraction stored in `mdView.frac`, and when placing the marker it is moved proportionally into the block. This corresponds to VS Code's `scrollPreviewWithEditor`; the `NoPreviewSync` setting (`noPreviewScrollSync` in state.json) turns it off.
- Preview → editor: when the pointer is over the preview and the line at the preview's top changes, `Doc.ScrollToFrac` (ged's `ScrollToLineFrac`, which stops partway through that line by the fraction) scrolls the editor. This corresponds to VS Code's `scrollEditorWithPreview`; the `NoEditorSync` setting (`noEditorScrollSync`) turns it off.
- After one side drags the other along, for `mdSyncQuiet` (200 ms) changes on the other side only update the baseline and aren't sent back, so the two sides don't pull each other back and forth.

#### Other mappings

- The block containing the source file's caret gets a vertical bar on its left (VS Code's `markEditorSelection`, drawn in `topBlock`).
- Double-clicking the preview (the scroll container's `HandleInput`, `Clicks == 2`) runs `RevealSource` next frame to go back to that line (VS Code's `doubleClickToSwitchToEditor`).
- Hovering to the left of a heading shows 🔗 (`headingLink`); clicking it copies `repo-relative-path#slug` and scrolls to the heading.

#### Following the current tab

- When the keyboard switches to another Markdown file in another group (one without its own side-by-side preview), `FollowSidePreview` retargets this preview to it and scrolls to where that editor is currently looking.
- A preview whose source tab is gone (when the preview tab is pushed out by `addDoc` without going through `RemoveDoc`) also follows, so it follows first and then checks whether the source file still exists.
- `view.markdownLockPreview` ("Lock Preview" is also in the tab context menu) locks it so it doesn't follow; its title becomes `[Preview] x` (unlocked it's `Preview x`).
- When the source file is closed (`RemoveDoc` → `closeSidePreviews`), the preview switches to another copy of the same file, or closes along with it if there is none.
