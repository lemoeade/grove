package ui

import (
	"os"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/ged"
	"github.com/lemoeade/grove/internal/i18n"
)

// 文件标签的编辑器用 Grove 自己画的单栏编辑器（ged.NewEditor）：排字、画、光标、编辑、查找、折叠、换行、粘性滚动
// 都是 ged 的；mygo 的 editor.State（d.St）还在，缓冲区、光标和选中的字和它对齐着，所以 App 那边照旧问 St 的光标、
// 调 St.GoTo / St.Select，代码片段、词补全、格式化、拖进来的字都不用改。对齐的规矩：
//   - 每帧画之前：St 的选择和上次对齐时（Doc.EdSel）不一样，就是 App 那边挪过光标，把最新的那个给 ged（滚进视图），
//     别的光标（多光标）当 Style.Extra 画；
//   - ged 接了事件以后（Style.Changed）：ged 的选择变了就推给 St（只剩一个光标）。
//
// 多光标由 St 做（ged 只有一个光标）：St 有不止一个选择时，打字、按键在 Style.Intercept 里交给 St；⌥点击加光标。
// 补全列表、悬停提示是这里画的（ged 不认 App），跳到定义走 App。

// gedEditorOn：文件标签用 ged 的单栏编辑器（默认）；WORKBENCH_GED_EDITOR=off 退回 mygo 的。
var gedEditorOn = os.Getenv("WORKBENCH_GED_EDITOR") != "off"

// gedEditorMaxLines 是用 ged 编辑器的最多行数：再长的照旧用 mygo 的（它按需排版，ged 每次改都整份重读行）。
const gedEditorMaxLines = 50000

// useGedEditor 是这个文件标签这一帧用不用 ged 的编辑器：开了、能改的普通文件，不是十六进制、大文件、blame、冲突。
func useGedEditor(a *app.App, d *docs.Doc, conflicts bool) bool {
	return gedEditorOn && d.Kind == docs.File && d.Buf != nil && !d.Hex && !d.Big && !d.St.ReadOnly && d.Editable() &&
		!conflicts && !(d.ShowBlame && a.CanBlame(d)) && d.Buf.LineCount() <= gedEditorMaxLines
}

type gedFileKey struct{ d *docs.Doc }
type gedFileRow struct{ d *docs.Doc }

// gedBufKey 是 ged.Diff.SetText 的 key：缓冲区和版本，一样就不重读。
type gedBufKey struct {
	b   *editor.Buffer
	ver uint64
}

// edRange / edSel 在 St 的选择（字节偏移）和 ged 的选择（行、行里的字节）之间换。
func edRange(b *editor.Buffer, s editor.Selection) ged.Range {
	a, h := b.Pos(s.Anchor), b.Pos(s.Head)
	return ged.Range{AL: a.Line, AO: a.Col, BL: h.Line, BO: h.Col}
}

func edSel(b *editor.Buffer, r ged.Range) editor.Selection {
	return editor.Selection{Anchor: b.Offset(editor.Pos{Line: r.AL, Col: r.AO}), Head: b.Offset(editor.Pos{Line: r.BL, Col: r.BO})}
}

// edPull 把 St 的选择给 ged（App 那边挪过光标时），返回别的光标（多光标时 ged 当 Extra 画）。
func edPull(d *docs.Doc) []ged.Range {
	sels := d.St.Selections()
	prim := d.St.Selection()
	if !slices.Equal(sels, d.EdSel) {
		d.Ed.SetSelection(edRange(d.Buf, prim), true)
		d.EdSel = sels
		d.EdRange = d.Ed.Selection()
	}
	if len(sels) < 2 {
		return nil
	}
	extra := make([]ged.Range, 0, len(sels)-1)
	for _, s := range sels {
		if s != prim {
			extra = append(extra, edRange(d.Buf, s))
		}
	}
	return extra
}

// edPush 把 ged 的选择推给 St（ged 的变了才推）。
func edPush(d *docs.Doc) {
	r := d.Ed.Selection()
	if r == d.EdRange {
		return
	}
	d.EdRange = r
	d.St.SetSelections(edSel(d.Buf, r))
	d.EdSel = d.St.Selections()
}

// gedFile 建文件标签的 ged 编辑器（返回编辑器本身，好交键盘）：外面套一层 Row，补全列表、悬停提示叠在上面。
func gedFile(a *app.App, c *wui.Context, d *docs.Doc, keys func(wui.InputEvent) bool) wui.Element {
	if d.Ed == nil {
		d.Ed = ged.NewEditor()
	}
	g := d.Ed
	d.EdOn = true
	g.SetText(gedBufKey{d.Buf, d.Buf.Version()}, func() []string { return bufLines(d.Buf) }, d.Buf.Language())
	extra := edPull(d)
	t := c.Theme()
	s := gedEditorStyle(a, t)
	s.Label = d.St.Label // “编辑 a.go”，和 mygo 的编辑器一样
	s.Buf = d.Buf
	s.Extra = extra
	s.Paste = func() string { return app.ReadClipboard() }
	s.Copy = func(text string) { app.CopyText(text) }
	s.Bindings = app.EditorBindings()
	s.Indent = a.DocIndent(d)
	s.Whitespace = a.St.Settings.Whitespace
	s.Guides = true
	s.Wrap = a.WrapOf(d)
	if stickyOn(a, d) {
		s.Sticky = func(_ int, rowLine func(k int) int, n int) []int {
			tree, wait := d.StickyOutline(c.Now())
			if wait > 0 {
				c.After(wait)
			}
			if tree == nil {
				return nil
			}
			idx := tree.Sticky(rowLine, n, nil)
			out := make([]int, len(idx))
			for i, k := range idx {
				out[i] = tree.Flat[k].Line
			}
			return out
		}
		s.StickyShadow = pick(t, wui.RGBA(0, 0, 0, 0.12), wui.RGBA(0, 0, 0, 0.5))
	}
	s.FindBar = func(c *wui.Context, bar wui.Element) { gedFindBar(c, bar, g) }

	list := edCompletions(d)
	typed := false // 这次事件打的是标识符里的字：ged 插好以后要补全
	accept := func(it editor.Completion) {
		text := it.Insert
		if text == "" {
			text = it.Label
		}
		d.St.Select(d.EdCompl.Start, d.St.Cursor())
		d.St.Insert(text)
		d.EdCompl = nil
	}
	s.Keys = func(ev wui.InputEvent) bool {
		if ev.Kind == wui.InputKeyDown && ev.Key == wui.KeyEscape && ev.Mods == 0 {
			switch {
			case d.EdCompl != nil:
				d.EdCompl = nil
				return true
			case d.EdTip != nil:
				d.EdTip = nil
				return true
			case g.Find.Open:
				g.CloseFind()
				return true
			}
		}
		return keys(ev)
	}
	s.Intercept = func(ev wui.InputEvent) bool {
		edPull(d) // App 在两帧之间挪了 St 的光标（St.Select 后紧接着打字）：先对齐再接这个事件
		if len(list) > 0 && ev.Kind == wui.InputKeyDown && ev.Mods&(wui.Super|wui.Ctrl|wui.Alt) == 0 {
			cl := d.EdCompl
			switch ev.Key {
			case wui.KeyDown:
				cl.Sel = (cl.Sel + 1) % len(list)
				return true
			case wui.KeyUp:
				cl.Sel = (cl.Sel + len(list) - 1) % len(list)
				return true
			case wui.KeyEnter, wui.KeyTab:
				if ev.Mods == 0 {
					accept(list[min(cl.Sel, len(list)-1)])
					return true
				}
			case wui.KeyEscape:
				d.EdCompl = nil
				return true
			}
		}
		if ev.Kind == wui.InputPointerDown && ev.Button == 0 && ev.Mods&wui.Alt != 0 && ev.Mods&(wui.Super|wui.Ctrl|wui.Shift) == 0 {
			if line, off, ok := g.PointAt(ev.X, ev.Y); ok { // ⌥点击：多加一个光标
				o := d.Buf.Offset(editor.Pos{Line: line, Col: off})
				d.St.SetSelections(append(d.St.Selections(), editor.Selection{Anchor: o, Head: o})...)
				g.Focus()
				return true
			}
		}
		if ev.Kind == wui.InputText {
			typed = isWordText(ev.Text)
			if !typed {
				d.EdCompl = nil
			}
		}
		if len(d.St.Selections()) < 2 {
			return false
		}
		return edMulti(a, d, ev, s.Keys)
	}
	s.Changed = func() {
		edPush(d)
		if typed {
			typed = false
			if d.EdCompl == nil {
				edComplete(a, d, false)
			}
		}
	}
	s.Fallback = func(cmd editor.Command) bool { return edFallback(a, d, cmd) }
	s.Hover = func(line, off int) {
		if line < 0 {
			d.EdOut = true
			return
		}
		d.EdOut = false
		edHover(a, d, d.Buf.Offset(editor.Pos{Line: line, Col: off}))
	}
	a.SyncPeek(d)
	s.Changes = edChanges(d)
	if len(s.Changes) > 0 {
		s.GutterClick = func(line int) bool { return a.TogglePeek(d, line) }
	}
	s.Squiggles = edSquiggles(d, fullCodeTheme(t))
	if in := d.BlameInlay; in.Text != "" && in.Offset <= d.Buf.Len() { // 行内 blame（a.SyncInlineBlame 算好的）
		s.Notes = []ged.Note{{Line: d.Buf.Pos(in.Offset).Line, Text: in.Text}}
		s.NoteColor = t.TextMuted
	}
	if app.CanGoToDefinition(d) {
		s.Definition = func(line, off int) { a.GoToDefinition(d, d.Buf.Offset(editor.Pos{Line: line, Col: off})) }
	}
	owner := menuOwner{"gedfile", d.Where()}
	s.Menu = menuPointer(a, c, g.Bounds(), owner, func(m menu) { gedFileMenu(a, m, d) })

	row := wui.Row(c.Key(gedFileRow{d})).Grow(1).MinHeight(0).AlignItems(wui.Stretch)
	var el wui.Element
	row.Children(func() {
		el = ged.View(c, gedFileKey{d}, g, s).Grow(1).MinWidth(0)
		if len(list) > 0 {
			edCompletionList(a, c, d, list, row, accept)
		}
		if d.EdTip != nil {
			edTipCard(a, c, d, row)
		}
		edPeekCard(a, c, d, row)
	})
	if el.FocusWithin() {
		sp, x, y := g.KeySpot()
		b := g.Bounds()
		a.MenuKeyAt = &app.MenuKeyTarget{Owner: owner, X: b.X + x, Y: b.Y + y,
			Build: func(m app.MenuSink) { g.SetSpot(sp); gedFileMenu(a, m, d) }}
	}
	return el
}

// gedEditorStyle 是文件编辑器的样子：编辑器的颜色和字（不是 Diff 的小半号）。
func gedEditorStyle(a *app.App, t *wui.Theme) ged.Style {
	ct := fullCodeTheme(t)
	s := gedStyleBase(t, ct, a.EditorFont())
	s.PlainNumbers = vscode()
	s.Brackets = bracketColors(t)
	s.Minimap = vscode() // 小地图：VS Code 默认开着
	return s
}

// bracketColors 是 VS Code 风格里括号按层轮着用的三种颜色（editorBracketHighlight.foreground1–3，Light+ / Dark+ 的），
// Grove 风格不上色（nil）。
func bracketColors(t *wui.Theme) []wui.Color {
	if !vscode() {
		return nil
	}
	if t.Dark {
		return []wui.Color{wui.Hex("#FFD700"), wui.Hex("#DA70D6"), wui.Hex("#179FFF")}
	}
	return []wui.Color{wui.Hex("#0431FA"), wui.Hex("#319331"), wui.Hex("#7B3814")}
}

// edMulti 是多光标时的事件：交给 St 做（ged 只画结果）。键按键位表查成命令，St 执行；打字、粘贴插到每个光标处；
// Esc 只留最新的光标。查找、跳行、补全、跳到定义这些照单个光标的路走（edFallback）。
func edMulti(a *app.App, d *docs.Doc, ev wui.InputEvent, keys func(wui.InputEvent) bool) bool {
	switch ev.Kind {
	case wui.InputText:
		d.St.Insert(ev.Text)
		return true
	case wui.InputCommand:
		switch ev.Text {
		case "copy", "cut", "paste", "undo", "redo", "selectAll":
			return edRunMulti(a, d, map[string]editor.Command{"copy": editor.Copy, "cut": editor.Cut, "paste": editor.Paste,
				"undo": editor.Undo, "redo": editor.Redo, "selectAll": editor.SelectAll}[ev.Text])
		}
	case wui.InputKeyDown:
		if keys(ev) {
			return true
		}
		if ev.Key == wui.KeyEscape && ev.Mods == 0 {
			d.St.SetSelections(d.St.Selection())
			return true
		}
		cmd, eaten := d.Ed.Lookup(ev)
		if eaten {
			return true
		}
		if cmd == editor.NoCommand {
			return false
		}
		return edRunMulti(a, d, cmd)
	}
	return false
}

// edRunMulti 在 St 上执行一个命令（多光标时）：剪贴板走 Grove 的（St 没有画就碰不到窗口的剪贴板），
// 要界面的（查找、跳行、补全……）走 edFallback。
func edRunMulti(a *app.App, d *docs.Doc, cmd editor.Command) bool {
	switch cmd {
	case editor.Copy, editor.Cut:
		if text := d.St.SelectedText(); text != "" {
			app.CopyText(text)
			if cmd == editor.Cut {
				d.St.Insert("")
			}
		}
		return true
	case editor.Paste:
		d.St.Insert(app.ReadClipboard())
		return true
	case editor.Find, editor.FindReplace, editor.FindNext, editor.FindPrevious, editor.GoToLine, editor.ShowCompletions,
		editor.GoToDefinition, editor.ShowHover:
		d.St.SetSelections(d.St.Selection())
		return d.Ed.Command(cmd)
	}
	d.St.Run(cmd)
	return true
}

// edFallback 是 ged 自己不做的命令：跳行开命令面板的 ":"，补全、跳到定义、悬停走 App，别的交给 St（多加光标、
// 选下一处一样的……），做完把 St 的选择拉回 ged。
func edFallback(a *app.App, d *docs.Doc, cmd editor.Command) bool {
	switch cmd {
	case editor.GoToLine:
		a.OpenPalette(":")
		return true
	case editor.ShowCompletions:
		edComplete(a, d, true)
		return true
	case editor.GoToDefinition:
		if app.CanGoToDefinition(d) {
			a.GoToDefinition(d, d.St.Cursor())
		}
		return true
	case editor.ShowHover:
		if app.CanGoToDefinition(d) {
			d.EdOut = false
			edHover(a, d, d.St.Cursor())
		}
		return true
	}
	ok := d.St.Run(cmd)
	edPull(d)
	return ok
}

// edHover 给 off 处的名字开悬停提示（没有声明的关掉）。
func edHover(a *app.App, d *docs.Doc, off int) {
	var h *app.Hover
	if !a.St.Settings.NoHoverDef && app.CanGoToDefinition(d) {
		h = a.HoverAt(d, off)
	}
	ds := edDiagnosticsAt(d, off)
	if h == nil && len(ds) == 0 {
		d.EdTip = nil
		return
	}
	start, end, code := off, off, ""
	if h != nil {
		start, end, code = h.Start, h.End, h.Code
	} else {
		start, end = ds[0].Start, ds[0].End
	}
	d.ShowHover(start, end, code, func(c *wui.Context) {
		if len(ds) > 0 {
			diagnosticRows(c, ds)
		}
		if h != nil {
			hoverBody(a, c, d, h)
		}
	})
}

// edDiagnosticsAt 是盖住 off 的诊断（一个点的诊断算到那个词的末尾）。
func edDiagnosticsAt(d *docs.Doc, off int) []editor.Diagnostic {
	var out []editor.Diagnostic
	for _, g := range d.St.Diagnostics() {
		end := g.End
		if end <= g.Start {
			end = g.Start + 1
		}
		if g.Start <= off && off <= end {
			out = append(out, g)
		}
	}
	return out
}

// diagnosticRows 是悬停提示里诊断的那几行：按严重程度上色的一个点，消息，后面淡字写来源。
func diagnosticRows(c *wui.Context, ds []editor.Diagnostic) {
	t := c.Theme()
	ct := fullCodeTheme(t)
	for _, g := range ds {
		wui.Row(c).Gap(6).AlignItems(wui.Start).Children(func() {
			wui.Text(c, "●").FontSize(t.FontSize-3).TextColor(severityColor(ct, g.Severity)).Shrink(0).Padding(2, 0, 0, 0)
			wui.Text(c, g.Message).FontSize(t.FontSize - 1).TextColor(t.Text).Shrink(1)
			if g.Source != "" {
				wui.Text(c, g.Source).FontSize(t.FontSize - 1).TextColor(t.TextMuted).Shrink(0)
			}
		})
	}
}

func severityColor(ct *editor.Theme, s editor.Severity) wui.Color {
	switch s {
	case editor.SeverityError:
		return ct.Error
	case editor.SeverityWarning:
		return ct.Warning
	case editor.SeverityInfo:
		return ct.Info
	}
	return ct.Hint
}

// edChanges 是跟 HEAD 比改了的行（app.DocHunks），给 ged 画行号右边的竖条。
func edChanges(d *docs.Doc) []ged.Change {
	hs := app.DocHunks(d)
	if len(hs) == 0 {
		return nil
	}
	out := make([]ged.Change, 0, len(hs))
	for _, h := range hs {
		k := ged.ChangeModified
		switch {
		case h.NewEnd <= h.NewStart:
			k = ged.ChangeDeleted
		case h.OldEnd <= h.OldStart:
			k = ged.ChangeAdded
		}
		out = append(out, ged.Change{Start: h.NewStart, End: max(h.NewEnd, h.NewStart), Kind: k})
	}
	return out
}

// edSquiggles 是诊断的波浪线（颜色按严重程度）。
func edSquiggles(d *docs.Doc, ct *editor.Theme) []ged.Squiggle {
	ds := d.St.Diagnostics()
	if len(ds) == 0 {
		return nil
	}
	n := d.Buf.Len()
	out := make([]ged.Squiggle, 0, len(ds))
	for _, g := range ds {
		a, b := d.Buf.Pos(min(max(g.Start, 0), n)), d.Buf.Pos(min(max(g.End, g.Start, 0), n))
		out = append(out, ged.Squiggle{Range: ged.Range{AL: a.Line, AO: a.Col, BL: b.Line, BO: b.Col}, Color: severityColor(ct, g.Severity)})
	}
	return out
}

// edPeekCard 是 ged 编辑器上改动的就地卡（点了改动竖条开的，和 mygo 编辑器的 peekCard 同一张卡）：卡顶在这一块下面。
func edPeekCard(a *app.App, c *wui.Context, d *docs.Doc, row wui.Element) {
	h, i, n, ok := a.PeekHunk(d)
	if !ok {
		return
	}
	g := d.Ed
	lh := g.RowHeight()
	lines := d.Buf.LineCount()
	var top float32
	if h.NewEnd < lines {
		box, ok := g.CharBox(h.NewEnd, 0)
		if !ok {
			return
		}
		top = box.Y
	} else {
		box, ok := g.CharBox(lines-1, 0)
		if !ok {
			return
		}
		top = box.Y + lh
	}
	y0 := g.ScrollY
	svc := c.Services()
	check := func(*wui.Painter, wui.Rect) {
		if g.ScrollY != y0 {
			svc.Invalidate()
		}
	}
	maxH := float32(240)
	if rb := row.Bounds(); rb.H > 0 {
		maxH = max(rb.H/3, 90)
	}
	peekCardAt(a, c, d, g.LineX(1)-g.Bounds().X-4, top, maxH, lh, check, h, i, n)
}

// edComplete 在光标处要补全（敲字时 explicit 是 false，⌃Space 是 true）：词、片段由 App 给（a.Complete → d.ShowCompletions）。
func edComplete(a *app.App, d *docs.Doc, explicit bool) {
	if d.Kind != docs.File || !d.Editable() || len(d.St.Selections()) > 1 {
		return
	}
	cur := d.St.Cursor()
	p := d.Buf.Pos(cur)
	line := d.Buf.Line(p.Line)
	start := p.Col
	for start > 0 {
		r, n := utf8.DecodeLastRuneInString(line[:start])
		if !isWordRune(r) {
			break
		}
		start -= n
	}
	a.Complete(d, editor.CompletionRequest{Offset: cur, Prefix: line[start:p.Col], Version: d.Buf.Version(), Explicit: explicit})
}

func isWordRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func isWordText(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isWordRune(r) {
			return false
		}
	}
	return true
}

// edCompletions 是补全列表这一帧给的项：按光标前打的字筛（字按顺序出现、不分大小写，开头对得上的在前）。
// 光标出了那个词、换了行、多光标、筛完没了，列表关掉。
func edCompletions(d *docs.Doc) []editor.Completion {
	cl := d.EdCompl
	if cl == nil {
		return nil
	}
	cur, n := d.St.Cursor(), d.Buf.Len()
	if cur < cl.Start || cl.Start > n || len(d.St.Selections()) > 1 || d.Buf.Pos(cur).Line != d.Buf.Pos(cl.Start).Line {
		d.EdCompl = nil
		return nil
	}
	typed := strings.ToLower(d.Buf.Slice(cl.Start, cur))
	var first, rest []editor.Completion
	for _, it := range cl.Items {
		key := it.FilterText
		if key == "" {
			key = it.Label
		}
		low := strings.ToLower(key)
		switch {
		case strings.HasPrefix(low, typed):
			first = append(first, it)
		case subsequence(low, typed):
			rest = append(rest, it)
		}
	}
	out := append(first, rest...)
	if len(out) == 0 {
		d.EdCompl = nil
		return nil
	}
	cl.Sel = min(cl.Sel, len(out)-1)
	return out
}

// subsequence 是 q 的字按顺序出现在 w 里（两个都已经是小写）。
func subsequence(w, q string) bool {
	for _, c := range q {
		i := strings.IndexRune(w, c)
		if i < 0 {
			return false
		}
		w = w[i+utf8.RuneLen(c):]
	}
	return true
}

// edCompletionRows 是补全列表最多露几行（多的跟着选中的那项滚）。
const edCompletionRows = 10

// edCompletionList 画补全列表：贴在正在打的词下面（底下放不下放上面），选中的那行垫强调色，点一项接它。
func edCompletionList(a *app.App, c *wui.Context, d *docs.Doc, list []editor.Completion, row wui.Element, accept func(editor.Completion)) {
	t := c.Theme()
	cl := d.EdCompl
	p := d.Buf.Pos(cl.Start)
	box, ok := d.Ed.CharBox(p.Line, p.Col)
	if !ok {
		return
	}
	font := a.EditorFont()
	rowH := box.H + 2
	n := min(len(list), edCompletionRows)
	h := float32(n)*rowH + 6
	y := box.Y + box.H
	if area := row.Bounds().H; area > 0 && y+h > area && box.Y-h > 0 {
		y = box.Y - h
	}
	first := min(max(0, cl.Sel-n+1), len(list)-n)
	frostFill(c, wui.Column(c.Key("edcompl")).Absolute().Left(max(0, box.X-8)).Top(y).Width(380).Padding(3).Radius(ctlRadius), widgetBg(t)).
		Border(1, t.Border).Shadow(shadowFloat()).Children(func() {
		for i := first; i < first+n; i++ {
			it := list[i]
			r := wui.Row(c.Key(i)).Height(rowH).Padding(0, 6).Gap(8).AlignItems(wui.Center).Radius(ctlRadius).
				Cursor(wui.CursorPointer).Label(it.Label)
			if i == cl.Sel {
				r.Background(accentTint(t).Alpha(softAlpha(t)))
			} else if r.Hovered() {
				r.Background(toolFill(t, 0.06))
			}
			r.Children(func() {
				wui.Text(c, completionMark(it.Kind)).Font(font.Family).FontSize(t.FontSize - 2).TextColor(t.TextMuted).
					SingleLine().Width(16).Shrink(0)
				wui.Text(c, it.Label).Font(font.Family).FontSize(font.Size).TextColor(t.Text).SingleLine().Shrink(0)
				wui.Text(c, it.Detail).FontSize(t.FontSize - 1).TextColor(t.TextMuted).SingleLine().Ellipsis("…").
					Grow(1).MinWidth(0)
			})
			if r.Clicked() {
				accept(it)
				d.Ed.Focus()
			}
		}
	})
}

// completionMark 是补全项前面的小记号：片段是 {}，别的（词、符号）是 ab。
func completionMark(k editor.CompletionKind) string {
	if k == editor.CompletionSnippet {
		return "{}"
	}
	return "ab"
}

// edTipCard 画悬停提示：贴在那个名字上面（上面放不下放下面），等宽字的声明，下面是注释和位置。指针离开名字后
// 指着提示的话留着（好点里面的链接），不指着了关。
func edTipCard(a *app.App, c *wui.Context, d *docs.Doc, row wui.Element) {
	tip := d.EdTip
	t := c.Theme()
	if tip.Start > d.Buf.Len() {
		d.EdTip = nil
		return
	}
	p := d.Buf.Pos(tip.Start)
	box, ok := d.Ed.CharBox(p.Line, p.Col)
	if !ok {
		d.EdTip = nil
		return
	}
	card := frostFill(c, wui.Column(c.Key("edtip")).Absolute().Left(max(0, box.X-8)).MaxWidth(520).Padding(6, 8).Gap(4).Radius(ctlRadius), widgetBg(t)).
		Border(1, t.Border).Shadow(shadowFloat())
	if box.Y > row.Bounds().H/3 {
		card.Bottom(row.Bounds().H - box.Y + 2)
	} else {
		card.Top(box.Y + box.H + 2)
	}
	card.Children(func() {
		if tip.Code != "" {
			wui.Text(c, tip.Code).Font(a.EditorFont().Family).FontSize(a.EditorFont().Size).TextColor(t.Text).Shrink(1)
		}
		if tip.Build != nil {
			tip.Build(c)
		}
	})
	if d.EdOut && !card.Hovered() {
		d.EdTip = nil
		c.Invalidate()
	}
}

// edBindingOf 是编辑器命令 cmd 的键：先看改过的（app.EditorBindings），再看 mygo 默认的；只要一下的、不要和弦。
// 一个命令有几个键时，挑 VS Code 菜单里写的那个（edMenuKeys），没有就要第一个。
func edBindingOf(cmd editor.Command) (editor.Binding, bool) {
	for _, bs := range [][]editor.Binding{app.EditorBindings(), editor.DefaultBindings()} {
		var first editor.Binding
		for _, b := range bs {
			if b.Command != cmd || b.ThenKey != 0 || b.Key == 0 {
				continue
			}
			if want, ok := edMenuKeys[cmd]; !ok || b.Key == want {
				return b, true
			}
			if first.Key == 0 {
				first = b
			}
		}
		if first.Key != 0 {
			return first, true
		}
	}
	return editor.Binding{}, false
}

// edMenuKeys 是有几个键的命令在菜单上写哪个：更改所有匹配项 VS Code 写 ⌘F2（⇧⌘L 是“选择所有匹配项”的）。
var edMenuKeys = map[editor.Command]wui.Key{editor.SelectAllMatches: wui.KeyF2}

// gedFileMenu 是 ged 的文件编辑器的右键菜单，照 VS Code 编辑器的右键菜单排：跳到定义、查找引用；更改所有匹配项、
// 格式化文档；剪切、复制、粘贴；命令面板，到这为止。Grove 自己的（拆分、本地历史、Finder、复制路径）和 VS Code 一样在标签的右键菜单里，
// 保存、检查、blame 在菜单栏和命令面板里；撤销、重做、全选 VS Code 的右键菜单里也没有（菜单栏“编辑”里有）。
func gedFileMenu(a *app.App, m menu, d *docs.Doc) {
	g := d.Ed
	if g == nil {
		return
	}
	cur := a.CurrentDoc() == d
	key := func(it menuItem, id string) menuItem {
		if cur {
			shortcut(it, id)
		}
		return it
	}
	edKey := func(it menuItem, cmd editor.Command) menuItem { // 编辑器自己的键（改过的照改后的，不归菜单栏）
		if cur {
			if b, ok := edBindingOf(cmd); ok {
				it.Shortcut(b.Mods, b.Key)
			}
		}
		return it
	}
	mergeMenu(a, m, d)
	if app.CanGoToDefinition(d) {
		if edKey(m.Item(i18n.T("跳到定义")), editor.GoToDefinition).Chosen() {
			a.GoToDefinition(d, d.St.Cursor())
		}
		if key(m.Item(i18n.T("查找引用")), "editor.findReferences").Chosen() {
			a.FindReferences(d, d.St.Cursor())
		}
		m.Separator()
	}
	if d.Editable() {
		all := m.Item(i18n.T("更改所有匹配项")).Disabled(d.Buf.Len() > app.MaxWholeTextBytes)
		if edKey(all, editor.SelectAllMatches).Chosen() {
			d.Run(editor.SelectAllMatches)
		}
		formatItem(a, m, d)
		m.Separator()
	}
	if m.Item(i18n.T("剪切")).Shortcut(wui.Super, wui.KeyX).Disabled(!g.HasSelection() || !d.Editable()).Chosen() {
		g.Run("cut")
	}
	if m.Item(i18n.T("复制")).Shortcut(wui.Super, wui.KeyC).Disabled(!g.HasSelection()).Chosen() {
		g.Run("copy")
	}
	if m.Item(i18n.T("粘贴")).Shortcut(wui.Super, wui.KeyV).Disabled(!d.Editable()).Chosen() {
		g.Run("paste")
	}
	m.Separator()
	if key(m.Item(i18n.T("命令面板…")), "palette.commands").Chosen() {
		a.Exec("palette.commands")
	}
}
