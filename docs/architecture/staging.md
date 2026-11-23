# Diff and staging

[← Architecture overview](../architecture.md) · [Git](git.md)

Staging / unstaging / discarding hunks and lines, and how the patches are built; the Diff tab (by default Grove's own
`internal/ged`) and the in-place operations and in-place editing on it; the peek card on the editor's change bar. For the
single-pane ged editor in file tabs, see [editor.md](editor.md#editor-component-internalged).

## The two Diff code paths

- **Default: ged**. Diff tabs, compare tabs and the replace preview all use Grove's own `internal/ged` (`useGed`,
  `ui/gedview.go`); side by side, unified and editing in place are all drawn by it. The file-tab editor is also ged by
  default (`useGedEditor`, `ui/gedfile.go`; see [editor.md](editor.md#editor-component-internalged)).
- **Falling back to mygo**: `WORKBENCH_GED=off` (`gedOn`) makes Diffs fall back to mygo's `editor.DiffView`;
  `WORKBENCH_GED_EDITOR=off` (`gedEditorOn`) makes file tabs fall back to mygo's `editor.View`. Tests of the mygo path pin
  it with `mygoDiff(t)` / `gedEditor(t, false)`.
- Shared by both paths: `docs.Doc.DiffSt` (mygo's `editor.DiffState`: both sides' buffers, `SelectedLines`, `Layout`,
  `Editable`), the hunk buttons `hunkActions`, the `linesBar` shown after selecting lines, and the staging logic in
  `app/hunks.go` / `app/lines.go`. When `Doc.UsingGed()` is true (a `Doc.Ged` has been built), previous / next change and
  the pairing for line staging follow ged's `Hunks`.

## Staging, unstaging and discarding

### Hunks (`app/hunks.go`)

In a Diff tab, each hunk has stage / unstage / discard buttons at its top right (in ged they are also in the context menu).

- What can be clicked is decided by `HunkOps`: an unstaged modified file (`Code == 'M'`) can be staged and discarded; a
  staged modified / renamed file (`M` / `R`) can be unstaged. Untracked files, conflicted files, files in a commit, and
  whole files that were added / deleted / changed type have no hunk buttons (they're handled per file in the Git panel);
  neither do compare tabs or the replace preview.
- `diffContent` (`ui/tabs.go`) sets `DiffState.HunkActions` accordingly; it doesn't while editing in place (see "Editing in place" below).
- The patch (`hunkPatch`) is computed from the raw contents of both sides as read by this Diff tab (`Doc.Raw[0]` /
  `Raw[1]`): first `git.ReplaceLines` applies that hunk to one side to get the other full text, then `git.HunkPatch`
  diffs them into a patch containing only that hunk:
  - Stage (`StageHunk`): apply the hunk to the old side, `git apply --cached -`.
  - Unstage (`UnstageHunk`): remove the hunk from the new side, `git apply --cached -R -`.
  - Discard (`DiscardHunk`): likewise remove the hunk, with `git apply -R -` on the working tree; `a.AskDanger` asks first.
- If the line numbers don't match (the diff was just reread) or the patch is empty, it reports "This hunk no longer
  matches the file on disk". git apply only applies when the context matches, so if the file on disk changed after the
  diff was read, it fails instead of making a mess.

### Lines (`app/lines.go`)

Selecting a few lines and staging them is line staging: click line numbers to select lines (⇧-click extends to that
line), then stage / unstage / discard only the selected lines. What's allowed is the same as for hunks (`LinesOps`); the
commands are `diff.stageLines` / `diff.unstageLines` / `diff.discardLines`, and the `linesBar` under the Diff has buttons too.

- `LinesEdits` turns the selected lines into a list of `git.LineEdit`: within a hunk, old line i and new line i form a
  pair (for i less than the smaller of the two sides' line counts).
  - Stage (changes the old side): for a selected new line, a paired one replaces its counterpart, an unpaired one is
    inserted at the end of the hunk's old lines; a selected old line is deleted.
  - Unstage, discard (change the new side): for a selected new line, a paired one is changed back to its counterpart, an
    unpaired one is deleted; for a selected old line, a paired one is inserted back before its counterpart, an unpaired
    one is inserted at the end of the hunk's new lines.
  - Unchanged context lines in the selection do nothing; if every selected line is unchanged, a toast says "No changes on the selected lines".
- `git.LinesPatch` generates a patch containing only those lines and **always applies forward** (never `-R`), so there's
  no need to deal with the context problems of reverse patches, nor to compute a second "without the selected lines" full text:
  - Stage: applied on top of the old side (the index), `git apply --cached`.
  - Unstage: applied on top of the new side (the new side of a staged Diff is the index), `git apply --cached`; the working tree is untouched.
  - Discard: applied on top of the new side (the working tree), `git apply`; asks first.
- The hunks used for pairing are `diffHunks`: ged's `Hunks` when `UsingGed`, otherwise mygo's `DiffState.Hunks()` (the
  two align the sides differently, and the lines the patch pairs must match what you see).
- Afterwards the selection is cleared (`ClearSelectedLines`); Esc clears it too.

### Building the patch (`git/hunk.go`)

- `HunkPatch(path, from, to)`: `from` and `to` differ in only one contiguous run; strip the identical head and tail, and
  what's left in the middle is the hunk, which is handed to `LinesPatch`.
- `LinesPatch(path, old, edits)`: git diff format, one hunk per change, each with `HunkContext` (3) lines of context
  above and below; changes close together (no more than 2×`HunkContext` unchanged lines between them) are merged into one hunk.
  `edits` must be sorted by `Start` and non-overlapping; anything out of range returns "". When something is inserted
  after a last line with no trailing newline, that line is counted as changed too and gets its newline; file names with
  quotes, backslashes or control characters are escaped the way git does it (`patchName`).
- Line numbers: the Diff view splits lines on `"\n"` (CRLF counts as one line), while `git.Lines` splits them the way git
  does (each line keeps its own `"\n"`); for a file ending in a newline the view has one extra empty line compared with git, which isn't counted.

## Diff tabs

### Previous / next change, previous / next file

- Previous / next change (J / K, `diff.nextHunk` / `diff.prevHunk`; the tab-bar buttons, the "change i / n" in the
  bottom bar) are `HunkPos` / `GoToHunk` in `app/diffnav.go`: with `UsingGed` they ask `Diff.Current` / `Diff.GoToHunk`,
  otherwise mygo's `CurrentHunk` / `GoToHunk`.
- ⌥J / ⌥K jump to the next / previous file (`diff.nextFile` / `diff.prevFile` → `StepFile`, `app/hunks.go`): uncommitted
  changes walk all repositories in Git panel order, while a commit's files walk the files of that commit; they open as preview tabs.

### Editing in place (`app/diffedit.go`)

A Diff against the working-tree side (unstaged, untracked, a text Diff whose file still exists: `CanEditDiff`) has a
pencil toggle on its tab bar (command `diff.editInPlace`, `ToggleDiffEdit`).

- Turning it on opens a tab for that file behind it (pinned) and pins the Diff tab too, with focus staying on the Diff;
  `Doc.EditDoc` points at that file tab.
- `BindDiffEdit` connects `DiffState.New` to the file tab's buffer every frame and sets `Editable` to true (while the
  file hasn't finished loading or can't be edited, it is view-only for now). Typing, undo and save all belong to the file
  tab: `Save` on the Diff saves that tab (⌘S works as usual, `app/tabs.go`), and the dot on the tab, the save button on the
  tab bar and the status bar's "Editing in place · unsaved" follow `DiffDirty`.
- While editing there are no hunk-level stage / discard buttons and no line selection (the file on disk and the buffer
  don't match); key bindings switch to `DiffEditBindings` (dropping unmodified J / K, which you need for typing).
- Clicking the pencil again, closing the file tab, or the Diff no longer being against the working tree (it got staged)
  calls `Doc.StopEdit`: the new side becomes a copy and the file is reread from disk, so the view-only Diff no longer
  follows unsaved text in the file tab; the file tab stays open, with its unsaved changes intact.
- The "Open in Editor" icon on the tab bar is a file (the same as on Git panel rows); the pencil only means editing in place.
- For typing and undo on the ged side, see "Editing in place in ged" below.

### `diffLook`: per-frame look (shared by both paths)

`diffLook` (`ui/tabs.go`) sets the look of Diff tabs, compare tabs and the replace preview every frame:

- Side by side / unified: when one side is empty (`oneSided`: a new file, a deleted file), it is a single unified column
  regardless of `unified`, and the side-by-side / unified button on the tab bar is grayed out. ged follows `DiffState.Layout` (`SetUnified`).
- Soft wrap: `DiffState.Wrap` follows `WrapOf` (⌥Z's `canWrap` also accepts text Diffs), and ged's `Style.Wrap` takes it too.
- When freshly read, or after switching between around-changes and full-file (`Doc.ToHunk`), the hunk at the cursor is
  scrolled into view; for ged this is `Doc.GedFocus`: a positive number is the new-side line to land on (+1), and -1 means
  the change just jumped to (or the first one if there isn't one yet).
- The bottom bar's `compareLabel` reads "Staged changes" / "Unstaged changes" for staged / unstaged, with things like `HEAD ↔ index` in the tooltip.
- The remaining settings (`Indicators`, `RenderSeparator`) only affect the mygo Diff; see "The mygo Diff" at the end.

## The ged Diff (`internal/ged`)

`internal/ged` is Grove's own editor widget. It only uses mygo's public interfaces (`ui.ShapeRichText` for text shaping,
`Painter` for drawing, `Element.HandleInput` for input), doesn't touch mygo's `ui/editor` views, and knows nothing about
Grove's theme or `internal/ui`. The UI-side wiring is in `ui/gedview.go` (`gedDiff`, `gedStyle`, `gedMenu`, `gedKeys`, `gedFindBar`).

### Model (`ged/diff.go`, no UI)

- `Align` aligns the lines of the two sides into `Row`s (`Same` / `Changed` / `Deleted` / `Added`): it strips the
  identical head and tail, then runs Myers on the middle; past `diffMax` (1000) steps it stops matching and treats the
  whole run as deletions plus additions. Where d lines are deleted and a lines added, the first min(d, a) lines pair up as
  `Changed`, and the extra lines face blanks.
- `Folded` folds runs of unchanged lines more than `Context` (3, `ged/view.go`) lines away from a change into a single
  `Fold` row. After the number, the fold bar shows the function containing the change below it, as given by
  `Style.FoldScope`: `changeAfter` finds that change's line on the old side, and on the UI side `scopeAt`
  (`ui/diffsep.go`) looks it up in `DiffOutline`, shared with the mygo Diff's `sepScope`.
- `Words` diffs changed lines word by word to find which byte ranges changed; when almost everything differs (less than
  20% the same), the whole line counts as changed, minus indentation and trailing whitespace (`whole`).
- `Hunks` are the individual changes: consecutive non-`Same` rows after alignment, with both sides' line ranges
  zero-based and half-open, the same as `editor.DiffHunk`.
- `Unified` lays the rows out in one column: each change lists its old-side lines as `Deleted`, then its new-side lines as `Added`.

### View (`ged/view.go`)

- A single element that manages its own scrolling: `ScrollY` moves both sides together, while horizontally each side
  scrolls on its own; `Draw` only lays out and paints visible lines, and the whole `paint` is clipped to the view (a
  half-visible last line doesn't paint over the bottom bar).
- One mouse-wheel notch glides over 150 ms with ease-out, like mygo's own scroll containers (`ged/glide.go`: building the
  view computes this frame's `ScrollY` from the start point and curve; consecutive notches chain together, reversing
  direction turns around immediately, and gliding stops if something else moves `ScrollY`); the trackpad still tracks
  your fingers directly, and with "Reduce motion" each notch jumps in one step.
- Each laid-out line's `TextLayout` is kept per (side, line number) and invalidated when the font / colors change. Syntax
  highlighting uses the language's `Tokenizer`, with the tokenizer state at the start of each line computed downward on demand.
- The background of changed text is positioned with `TextLayout.SelectionRects` and fills the line height; lines facing a
  blank get 45° hatching (`StrokePolyline`, since `Painter.Line` only draws horizontal and vertical lines).
- A 14 pt overview ruler on the right (`rulerW`) marks the changes; press and drag on it to scroll.
- The state, `ged.Diff`, lives in `Doc.Ged`; `Set` uses both sides' buffers and versions (`gedText`) to tell whether the
  content changed; `SetFull` follows `St.FullDiff` (no folding in full-file mode).
- Colors are supplied by `gedStyle` from Grove's theme: the backgrounds are the same `diffAddedLine` / `diffDeletedLine`
  and `diffTint` as the mygo Diff; in the VS Code style (the default) they switch to VS Code 2026's Diff colors,
  `PlainNumbers` keeps the line numbers of changed lines uncolored, and brackets are colored by nesting level (see [Appearance](look.md)).
- You can scroll three lines past the last line (`Diff.tail`).

### Hunk buttons, previous / next change

- The buttons at the top right of the hunk under the pointer (or the hunk jumped to, if none) are drawn by
  `Style.HunkBar` (Grove's `hunkActions`, shared with the mygo Diff; only provided when `DiffState.HunkActions`). It is an
  absolutely positioned child that sticks to the top of the hunk while scrolling; its position is computed in `View` from
  this frame's scroll, and `Bounds()` is only used for its size (last frame's position is wrong after jumping to another
  change). It moves down out of the way when the find bar is open.
- J / K are taken by `Style.Keys` (`gedKeys`) using the keys from the command registry (`diff.nextHunk` / `diff.prevHunk`); Esc closes the find bar first.
- `applyWant` in `View` turns the target line (`want`), the restored top (`wantTop`) and the line held across a soft-wrap toggle (`wrapAt`) into `ScrollY`.

### Selecting text and lines

- A text selection is a pair of (side, line, byte) positions; `HandleInput` handles drag selection, double-click to select
  a word (`tokens`), triple-click to select a line and ⇧-click; `InputCommand`'s copy / selectAll go through `Style.Copy` (`app.CopyText`).
- The element is `Focusable` and takes focus on press; arrow keys, Page Up / Down, Space and Home / End scroll, Esc clears the selection.
- Pressing on the line numbers or + − (`inGutter`, left of the text) selects lines: what `Style.Lines` / `SetLines` read
  and write is `DiffState.SelectedLines` itself (one copy shared with the mygo Diff), so `linesBar` and `StageLines` /
  `UnstageLines` / `DiscardLines` need no changes. Only provided when `DiffState.LineSelection`.
- Dragging extends the selection (`picking`, auto-scrolling at the top and bottom edges); ⇧-click extends from the start
  point (`anchor`; if the selection wasn't made here, from its original head) to that line. Selected lines are filled with
  `LineSel` plus a `LineMark` bar on the left.
- Selected text (within one line, not all whitespace, at most `occurMax` 200 bytes) is recorded each frame as `occ`
  (`occurText`, `ged/find.go`); when painting each line, `occurrences` finds the identical spots and fills them with
  `Code.Occurrence` (case-sensitive; the selected spot itself isn't filled).

### Unified view

- `SetUnified` (following `DiffState.Layout == DiffUnified`): `refold` runs the rows through `Unified` again.
- The two sides' cells merge into one full-width row (`paneX` is 0 for both), with two columns of line numbers on the
  left (`numR` old, `numR2` new); which side a row's text comes from is decided by `rowSide` (deleted lines are old, everything else is new).
- Changed text within a row finds its counterpart line through `pairs` (old ↔ new line numbers recorded from `Changed`
  rows in `Set`) and diffs against it; a change's extent in rows is `hunkRow` / `hunkEnd`.
- Line and text selection are still (side, line number); on press, the side comes from that row's `rowSide`, and unchanged lines count as selected on both sides.

### The side-by-side divider

`Diff.split` (the fraction taken by the left side; 0 means half, clamped to 15%–85%, `splitMin`) is managed by
`splitInput`: press and drag to change it, double-click to reset, and the pointer turns into a left-right resize cursor
over the line; `geom` splits the two sides by it, and the wrap width follows. It isn't saved to state.json (VS Code doesn't save it either).

### Editing in place in ged (`ged/edit.go`)

- When `DiffState.Editable`, `gedStyle` sets `Style.Buf` to `DiffState.New` (which is the file tab's buffer, connected
  by `BindDiffEdit`), `Style.Paste` reads the clipboard (`app.ReadClipboard`), `Style.Bindings` is
  `app.DiffEditBindings()`, and `Style.Indent` is `a.DocIndent(d)`.
- The caret is the b end of the new side's text selection (`caret`; a point when a == b); click, drag, ⇧-click and
  double-click work as before, and typing replaces the selected text. With no text selected, the caret line gets
  `Code.ActiveLine` behind it and its line number uses `ActiveLineNumber`.
- `editKey` handles the arrow keys, ⌫ / ⌦, Return (copying the indentation), Tab, Home / End and Page Up / Down. Keys it
  doesn't handle (no ⌘ / ⌃, not Esc) make `input` return false right away without asking `Style.Keys`, so that the
  following `InputText` reaches it (which is why J / K are text, not change navigation, while there's a caret).
- `editInput` handles text, composition (`InputCompose`; composed text is only inserted into that line in `layout`, not
  cached) and the Edit menu's cut / paste / delete / undo / redo.
- Changes go through `replace`. `Buf.Edit` is one undo step per call and mygo's typing coalescing isn't exported, so ged
  records how many edits each group combined itself (`undoGroups`: `typeGroup` merges same-kind consecutive typing /
  deleting within 1 s into one group; if the buffer version doesn't match — something else changed it — the groups are
  cleared); `undo` calls `Buf.Undo` / `Redo` that many times per group and puts the caret at `changedEnd`.
- After a change it immediately runs `reload` → `update`: if the old side didn't change, it keeps the fold expansions, the
  old side's layouts and the selection, discarding only the new side's layouts and tokenizer states.
- The caret doesn't blink (no extra frames while idle; see [the editor section of mygo-pitfalls](../mygo-pitfalls.md#editor));
  `paintCaret` records `caretBox`, and `View` hands it to the input method via `el.TextCaret`; `revealCaret` scrolls the
  caret into view in `View` and `Draw` (expanding a fold if it's inside one).

### Editing keys (`ged/keys.go`)

- `command` maps a single key to an `editor.Command` via `Style.Bindings` (remapped keys) and then
  `editor.DefaultBindings`, and `runEdit` carries it out itself: move line, copy line, delete line, indent, comment, join
  lines, transpose, ⌘L, ⌘D (ged has only one caret: with no text selected it selects the word at the caret, with text
  selected it selects the next identical occurrence), matching bracket…
- Commands ged doesn't do (multiple carets, completion…) are handed to `Style.Fallback` (only the single-pane editor
  provides one); without it, it returns false and the key goes back to `Style.Keys` / the window. Chords (like ⌘K ⌘L) and
  folding are only recognized in the single-pane editor, not in Diffs.
- Commands that change several lines all go through `rewriteLines`: a single `replace` swaps out those lines (one undo
  step), and both ends of the selection shift by how many characters their line gained or lost.
- One indent level (`indentUnit`) follows `Style.Indent`: when automatic, it checks whether the file has more tabs or
  spaces and the smallest number of spaces, then falls back to the language.
- Typing goes through `typeChar`: skipping over a closing bracket, inserting the other half (`autoPairs` is the language's
  `AutoClose`; `NotIn` uses the token kind from `tokenAt` to skip quotes inside strings and comments), wrapping selected
  text, and aligning a closing bracket to its matching line (`alignCloser`).
- Matching brackets (`matchBracket` / `scanBracket`) only recognize single-character brackets and look at most
  `bracketScan` (5000) lines; it is computed once per frame when there's a caret (`brk`) and drawn as a `Code.MatchingBracket` box.
- Whitespace is drawn by `paintSpaces` according to `Style.Whitespace` (the "Show Whitespace" setting).

### Soft wrap (`ged/wrap.go`)

- When `Style.Wrap` (`a.WrapOf(d)`, ⌥Z) is on, `layout` lays out to the width of that cell's text (`setWrapWidth`; the
  right cell leaves room for the overview ruler), and `lineLayout.n` is how many lines it wraps into.
- How many "screen lines" (`lh` tall) a `Row` takes is the larger of the two sides' wrap counts (in the unified view, its
  one side). `tops` is a prefix sum: lines not yet laid out are estimated by cell count (wide characters count as two),
  and when the laid-out result differs from the estimate, `topsDirty` marks it for recomputation next time — so the visible
  stretch is exact and the unseen part above is estimated.
- Without wrapping, `top(i)` / `rowIndex(v)` are just i / v; scrolling, clicking, painting, the overview ruler and
  previous / next change all ask them instead. The kth wrapped line is painted at `top(i)+k` (`rowY`), `posAt` asks
  `IndexAt` by wrapped line, and ↑ ↓ first move among a line's wrapped lines (`vertical`).
- Toggling it keeps the scroll position on the top line (`wrapAt`).

### Find and replace (`ged/find.go`)

- `Diff.Find` is the find bar's state (open or not, the input text); the bar itself is drawn by `Style.FindBar`
  (`gedFindBar`) and placed at the top right by `View` (the adjacent "Stage Hunk" moves down out of the way).
- `SetFind` compares against the input text every frame and only searches again when it changed (`search`: ASCII-only
  case folding, recording in aligned order every line of the new side and the changed lines of the old side, at most
  `findMax` (20000) matches); the current match is the first one from the viewport down.
- `FindStep` jumps forward and backward with wrap-around, `revealMatch` expands folds and moves the bottom bar's current
  change along, and `revealFind` scrolls there in `View`.
- `edit.find` / `edit.findNext` (`findIn`), when the current tab is drawn by ged (`findGed`: a Diff's `UsingGed`, or the
  single-pane editor's `UsingEd`), open the find bar / jump to the next match; otherwise they go to the mygo editor as before.
- Replace: `Find.Replace` / `With` is the row below the find bar (only present when editable); `edit.replace` (⌥⌘F,
  `replaceIn`) opens `OpenReplace`; `ReplaceOne` replaces only the current match on the new side (jumping first if the
  current one is on the old side), and `ReplaceAll` is a single `rewriteLines`.

### Context menu (`ged/menu.go`)

- `Style.Menu` gets the right-click (⌃-click) press, drag and release first; before the press, `menuAt` records the
  `Spot` (which side, which line, which change, and which new-side line to go to when jumping to the file); pressing
  outside the selected text moves the selection there (when editing in place, it places the caret).
- On the UI side, `Style.Menu` is `menuPointer` (`ui/gmenu.go`, the event-receiving half of `menuPress`). It doesn't
  register `AddTarget`: while the menu is open, right-clicking elsewhere in the Diff only closes it, because the receiving
  layer covers it and ged can't record where the new press landed.
- The menu is `gedMenu` (`ui/gedview.go`): undo / redo (editing in place), cut / copy / paste, expanding the fold bar,
  staging / unstaging / discarding this hunk, the same for the selected lines, Copy Line ("Copy Original Line" on the old
  side), find, previous / next change, Open Line in Editor, unified / full-file / ignore leading/trailing whitespace / soft
  wrap / edit in place, select all, and paths (`pathItems`). It targets the spot where the right-click happened, which
  isn't necessarily the current tab, so every action works on d (soft wrap uses `ToggleWrapOf(d)`).
- Cut, copy, paste and select all go through `Diff.Run` to the same `InputCommand` as the Edit menu.
- ⇧F10 / the menu key use `KeySpot` to position from the caret / current change, and `MenuKeyAt`'s Build calls `SetSpot` first.

### Expanding fold bars step by step

- `Diff.open` records a `Reveal` keyed by the old-side first line of the unchanged run (`Top`: how many lines to reveal
  downward from the change above, `Bottom`: how many upward from the change below, `All`: the whole run).
- `foldRows` leaves those lines out when folding, and also records `foldInfo` keyed by the `Fold` row's `Old` (the run's
  key, and whether it borders a change above / below).
- `ExpandMore(i, dir)` reveals `FoldStep` (20) lines at a time; if no more than that remain, the whole run. Clicking the middle of the bar expands the whole run.
- The arrows (`foldArrows`) are drawn in the line-number cell; `foldZone` tells which one was clicked, and the hovered one gets a background (`hoverZ`).
- Right-click records `Spot.Fold`, and the menu offers "Expand 20 Lines Down / Up" according to `FoldDirs`.

### Indent guides and sticky scroll

- Indent guides (`ged/guides.go`, `Style.Guides`): `prepGuides` determines each frame how many cells one level is from
  `indentUnit`, and when editing in place finds the caret's block (the innermost guide on the caret line, plus the
  consecutive lines above and below that are indented deeper); `paintGuides` draws a 1 pt line per level starting at cell
  0 of each cell's text (`Code.IndentGuide`; `ActiveIndentGuide` for that block). Blank lines look at the nearest non-blank
  lines within `guideScan` (200) lines above and below (taking the shallower one plus one level, but no deeper than the deeper one).
- Sticky scroll (`ged/sticky.go`, `Style.Sticky`; not provided when turned off in Settings): at most `stickyMax` (5)
  lines stick, and no more than a third of the view's height. `prepSticky` asks the caller every frame, for each side (in
  the unified view only the new side), which line sits below when k lines are stuck at the top (`rowLine`, searching
  downward past blanks facing the other side); on the UI side, `gedStyle` answers with `outline.Tree.Sticky` from
  `Doc.DiffOutlineOf(side)` (parsed separately for each side, redone only when the buffer or version changes).
  `paintSticky` covers the top of that column with the editor background, line numbers, the laid-out line (first wrapped
  line only) and a 3 px fading shadow (`StickyShadow`); `stickyInput` adds a hover background, and a press scrolls to that
  line so it just appears below the lines still stuck.

### Ignore leading/trailing whitespace

- `workspace.State.DiffTrim` (command `diff.ignoreTrim` / `ToggleDiffTrim`, saved in state.json) is passed to ged every
  frame via `SetIgnoreTrim`, and `AlignTrim` matches lines with leading and trailing whitespace removed.
- A line that only changed whitespace is `Same`, but the two sides' text differs, so each side is painted with its own
  text; find uses `sameText` to decide whether the old side borrows the new side's results.
- Changes and patches follow the visible `Hunks`, and a patch's context is taken from the copy it applies to, so hidden whitespace changes don't get in the way.
- The mygo Diff ignores this toggle (the command is grayed out when ged isn't in use).

### Go to line

`edit.goToLine` (⌃G), under `gedFind`, opens the Command Palette's ":" (`goToLine`). For a ged Diff, `lineItems` shows
the total line count from `Ged.LineCount`, and `JumpHere` hands off to `Diff.GoToLine` (the new side, or the old side
when the new side is empty; `rowOfLine` expands folds and `want` scrolls there).

### Tests

`internal/ui` (`gedview_test.go`):

- `TestGedDiff`: opens a Diff in a temp repo; checks folding, expanding by clicking a fold bar, the scroll wheel, and switching to full-file.
- `TestGedHunks`: previous / next change; Stage Hunk stages only that hunk.
- `TestGedSelect`: drag selection, ⌘C, double-click, Esc, J, paging.
- `TestGedLines`: drag to select lines, Esc, click one line and ⇧-click another, staging the selected lines stages only those two.
- `TestGedUnified`: switching to unified gives one column, previous / next change, clicking a deleted line's number
  selects the old side, staging the selected lines, then switching back to side by side.
- `TestGedEdit`: editing in place — the caret, typing, one-step undo, Return indentation, J being typed, composition, cut and paste, ⌘S saving to disk.
- `TestGedFoldScope`: the function shown on the fold bar.
- `TestGedWrap`: with soft wrap, long lines get taller and short ones don't, clicking the second segment places the caret, ↑ ↓ move within wrapped lines, turning it off restores.
- `TestGedFind`: ⌘F opens the bar, the match count, Return jumping to a match inside a fold expands it, Esc closes.
- `TestGedMenu`: right-click opens ged's menu, the recorded spot, no cut / paste when view-only, Copy Line (both sides),
  Stage Hunk from the menu stages only that hunk, ⇧F10 opens the menu and then picks Find.
- `TestGedFoldExpand`: the leading run has only an up arrow, clicking an arrow reveals 20 lines, a middle run has two
  arrows, the fold bar's context menu, no arrows when few lines remain, clicking the middle of the bar expands the whole run.
- `TestGedTrim`: after checking ignore whitespace in the menu, only the real change remains, and Stage Hunk stages only that line.
- `TestGedGoToLine`: ⌃G opens ":", and jumping to folded line 150 expands it and scrolls it into view.
- `TestGedOccurrences`: after selecting a word, identical words elsewhere on both sides are highlighted, but not the selected spot itself.
- `TestGedSplit`: drags the divider to 30%, the right side's text moves left with it without selecting text, double-click
  resets, then edits in place and places the caret (screenshot ged-caretline.png shows the caret line).
- `TestGedEditKeys`: real keys for ⌥↓, ⌘/, ⇧⌘K, ⌘], typing ( to get ), undo, then ⌥⌘F to replace one and Replace All
  (screenshot ged-keys.png shows the bracket box and whitespace, ged-replace.png shows the replace bar).

`internal/ged`:

- `TestUnified` (single-column order and folding), `TestAlignTrim`, `TestFoldReveal` / `TestExpandMore` (folding leaves
  out revealed lines, expanding the whole run when few remain), `TestOccurrences` (all-whitespace and multi-line selections aren't highlighted).
- `TestChangedEnd` / `TestWordMoves` (the caret after undo, moving by word), `TestChangeAfter` (the change a fold bar finds).
- `TestEditCommands` / `TestAutoClose` / `TestEditCommandUndo` / `TestCommandLookup` / `TestMatchBracket` / `TestReplace`.

Staging itself: in the `git` package `TestHunkPatchApplies` / `TestHunkPatchOneOfTwo` / `TestHunkPatchFormat` /
`TestLinesPatch`; in the `app` package `TestLinesEdits`, `TestDiffEditInPlace` / `TestDiffEditBindings`,
`TestPeekHunkAt` / `TestPeekRevert` / `TestPeekStage`; in the `ui` package `TestHunkOps`, `TestDiffEditUI`.

## The ged editor in file tabs

File tabs use ged by default too (the single-pane mode of `ged.NewEditor`, `ui/gedfile.go`): whether it's used, how it
lines up with mygo's `editor.State`, completion and hover, the change bars, keys and the context menu, folding and the
minimap are covered in [Editor widget (internal/ged)](editor.md#editor-component-internalged). Only one staging-related
detail belongs here: the peek card opened by clicking the line number of a changed line (`Style.GutterClick` →
`a.TogglePeek`) shares `peekCardAt` with the mygo editor (`edPeekCard` computes the card's top from `CharBox`); see
"Editor change bar and peek card" below.

## The mygo Diff (`WORKBENCH_GED=off`)

When `useGed` is false, Diffs go through `restingDiff` (`ui/caret.go`, wrapping `editor.DiffView`). The following only apply on this path:

- `Indicators` is `DiffBars` (red / green bars at the start of lines): mygo only colors changed words red / green when the bars are on; otherwise they're gray.
- Grove paints the background of changed lines itself (`ui/difftint.go`): mygo mixes them into the background at a fixed,
  too-faint ratio, so `restingDiff` tells it not to paint the background (`clearDiffBackground`: `NoBackground`,
  `Theme.Background` transparent) and wraps it in a `Column` with the editor background; `Draw(diffUnder)` computes the
  positions every frame from `DiffState.Hunks` / `LineTop` / `ScrollY` (`diffTints`, watched by `TestDiffTints`) and paints
  VS Code's default-theme `#9bb95533` / `#ff000033`; selected lines are painted here too. The bars, line numbers and
  changed words are still painted on top by mygo, in the `Theme.Added` / `Deleted` colors substituted by `diffTint`.
- Folded unchanged lines are drawn by `RenderSeparator = diffSep` (`ui/diffsep.go`) as a full-width faint band with "N
  unchanged lines" on the left; clicking it goes to `expandSep` (a leading / trailing run expands `Step` lines toward the
  change (mygo's default is 100); a middle run, a run shorter than `Step`, or holding ⇧ expands everything).
- The text after the fold bar is `sepScope`: the symbol line containing the first line of the change below (separator
  start + context + hidden line count + context). `Doc.DiffOutline` is the old side's outline (cached by buffer and
  version), and `outline.Tree.Scope` finds the innermost symbol containing that line. mygo only exposes the fold bar's line
  count and whether it's at the head or tail; where the separator starts is the unexported `DiffSeparator.a`, which
  `sepStart` reads via reflect — if it can't, nothing is shown (watched by `TestSepStart`).
- The hunk buttons are `DiffState.RenderHunkActions` (the same `hunkActions`), and line selection is mygo's own `LineSelection`.
- The change marks in the scrollbar track are drawn by mygo itself; the hatching on the empty side and mygo's built-in fold bar style live in mygo, and Grove doesn't change them.

## Editor change bar and peek card (`app/peek.go`, `ui/peek.go`)

The change bar next to the editor's line numbers can be clicked to see the original text of that hunk: click the change
bar (or a line number inside a changed hunk), or run the command `diff.peekHunk`, and a card pops up below the hunk.

- State and actions are in `app/peek.go`: `Peek` (the open card: which tab, which hunk), `DocHunks`, `HunkAt`,
  `TogglePeek`, `SyncPeek`, `PeekStep` (the card's ↑ ↓); the card is drawn in `ui/peek.go` (`peekCardAt`).
- Handling clicks: on the mygo editor, `peekSetup` sets `St.OnGutterClick` (when there are hunks it takes over the whole
  gutter; line numbers outside hunks still select that line); on the ged editor it's `Style.GutterClick`, and the card is placed by `edPeekCard`.
- What the card can do:
  - Revert (`PeekRevert`: `DiffState.Revert`, which changes the buffer directly; ⌘Z undoes it, no confirmation).
  - Stage (`PeekStage`): the file must be saved first; the patch is computed from the raw bytes in HEAD and the file on
    disk, again through `ReplaceLines` + `HunkPatch`, with `git apply --cached`. If the index already has adjacent changes
    to this file, git may reject it, and the error is reported.
  - Open in Diff (`PeekOpenDiff`: unstaged preferred; sets `Doc.HunkLine`, and `diffLook` uses `HunkFromLine` to scroll
    to the hunk containing the same line of the new file — the Diff compares against the index, so the hunk index may differ).
- Hunks are computed by the editor's `DiffState{Old: d.Base, New: d.Buf}` (against HEAD; historical revisions against
  their parent commit), the same algorithm as the change bar. The `DiffState` is kept per tab (the most recent
  `peekCacheMax` 4); when the buffer changes, mygo only re-diffs the lines near the change rather than the whole document
  each time (with the gutter taking over clicks, every frame has to ask whether there are hunks).
- The editor's hunks are recomputed as the buffer changes, and `SyncPeek` follows along: if the hunk disappears, the card
  closes; when the selection changes, if the caret lands at the start of a hunk (previous / next change, the card's ↑ ↓) the card follows it, otherwise it closes.
- The card is overlaid on the code (covering a few lines below the hunk), in both the mygo editor and the ged editor. To
  insert it between two lines and push the code below down like VS Code does, mygo would need the regular editor
  (`editor.State`) to reserve some height between lines for the caller to draw in; currently only mygo's Diff viewer has
  that (`DiffState.Annotations` + `RenderAnnotation`, internally `blockAnnotation`), and `State` has no corresponding hook.
