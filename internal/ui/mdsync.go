package ui

import (
	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
)

// 这个文件是 Markdown 预览和源码之间的对应（照 VS Code 的 markdown 预览）：
//   - 每个顶层块里放一个看不见、铺满它的探针，记下这块对着源码哪几行、上一帧在窗口里的框（mdView.blocks）；
//   - 并排预览两边同步：编辑器顶上那一行（带小数）换算成块里的位置滚过去；指针在预览上滚它时反算回行号滚编辑器，
//     一边拖着另一边走之后 mdSyncQuiet 之内另一边的变化不往回传；
//   - 源文件光标所在的块左边一道竖条（markEditorSelection）；
//   - 双击预览回到源码那一行（doubleClickToSwitchToEditor）；
//   - 标题左边悬停出 🔗：点了复制这个标题的链接、滚到它（GitHub 的样子）。

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// track 在预览的滚动容器上接指针：记下指针在哪（标题的 🔗），双击记下地方（下一帧回到源码）。都不吃掉事件。
func (v *mdView) track(el wui.Element) {
	if !el.Hovered() {
		v.ptrOK = false
	}
	r := el.Bounds() // 上一帧的框：事件在两帧之间送来，元素本身那时不能再用
	el.HandleInput(func(ev wui.InputEvent) bool {
		switch ev.Kind {
		case wui.InputPointerMove:
			v.ptr, v.ptrOK = wui.Point{X: r.X + ev.X, Y: r.Y + ev.Y}, true
		case wui.InputPointerDown:
			v.ptr, v.ptrOK = wui.Point{X: r.X + ev.X, Y: r.Y + ev.Y}, true
			if ev.Button == 0 && ev.Clicks == 2 {
				v.dblY, v.dblOK = r.Y+ev.Y, true
			}
		}
		return false
	})
}

// lineAt 是预览里窗口坐标 y 那里对着源码的哪一行（带小数）。照 VS Code：一块的顶到下一块的顶（连着中间的空当）
// 对着这块的第一行到下一块的第一行（连着块后面的空行），中间按源文件编辑器的屏幕行摊匀（v.row / v.unrow）——
// 这样编辑器匀速滚，不管折不折行，预览在一块里也匀速走。不挨着的块（中间有没排出来的）之间算下面那块的第一行，
// 过了最后一块算最后一块的最后一行。
func (v *mdView) lineAt(y float32) (float64, bool) {
	last := -1
	for i, b := range v.blocks {
		if b.r.H <= 0 {
			continue
		}
		if y < b.r.Y {
			return float64(b.line), true
		}
		last = i
		if end := v.spanEnd(v.blocks, b); y < end {
			r0, r1 := v.row(float64(b.line)), v.row(float64(b.next))
			return v.unrow(r0 + float64((y-b.r.Y)/(end-b.r.Y))*(r1-r0)), true
		}
	}
	if last >= 0 { // 过了最后一块
		b := v.blocks[last]
		return float64(max(b.next-1, b.line)), true
	}
	return 0, false
}

// sync 在预览画完以后：指针在预览上、预览顶上对着的行变了（用户滚了预览），编辑器跟过去。
func (v *mdView) sync(a *app.App, c *wui.Context, d *docs.Doc, el wui.Element) {
	if d.Kind != docs.MDPreview || d.MDSource == nil {
		return
	}
	L, ok := v.lineAt(el.Bounds().Y)
	if !ok {
		return
	}
	now := c.Now()
	if v.pvSeen && abs64(L-v.pvTop) > 1e-3 && now.After(v.pvQuiet) && el.Hovered() && d.AnchorLine == 0 && !a.St.Settings.NoEditorSync {
		d.MDSource.ScrollToFrac(L)
		v.srcTop = L
		v.edQuiet = now.Add(mdSyncQuiet)
	}
	v.pvTop, v.pvSeen = L, true
}

// spanEnd 是 bs 里块 b 管到的地方的底（窗口坐标）：下一块（从 b.next 行起的）排出来了是它的顶，不然是 b 自己的底。
// b 要是 bs 里的一项：两个框是同一次排版量出来的，滚着时也一起动。
func (v *mdView) spanEnd(bs []mdBlock, b mdBlock) float32 {
	end := b.r.Y + b.r.H
	for _, n := range bs {
		if n.line == b.next && n.r.H > 0 {
			return max(n.r.Y, end)
		}
	}
	return end
}

// row、unrow 把源码的行（带小数）换成源文件编辑器的屏幕行、再换回来：折行的行占几行、折起来的不占。
// 就地预览、源文件还没排过时一行一个屏幕行。
func (v *mdView) row(line float64) float64 {
	if v.src == nil {
		return line
	}
	return v.src.RowOf(line)
}

func (v *mdView) unrow(r float64) float64 {
	if v.src == nil {
		return r
	}
	return v.src.LineAtRow(r)
}

// topBlock 处理顶层文档里的一个顶层块：放探针记下它的行和框，要跳到它里面的话放跳转记号，
// 并排预览里源文件光标在它里面时画竖条，标题再放 🔗。
func (e *mdEnv) topBlock(c *wui.Context, doc *parser.Document, i int, ready bool) {
	v, d := e.v, e.d
	b := doc.Block(i)
	start := v.lineOf(e.base + b.Start)
	next := len(v.lines)
	switch {
	case b.Next < doc.Len():
		next = v.lineOf(e.base + doc.Block(b.Next).Start)
	case e.end >= 0: // 这一段的最后一块：到下一段的第一行为止，不是文末
		next = v.lineOf(e.end)
	}
	next = max(next, start+1)
	probe := wui.Box(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).IgnoreInput(true).AccessHidden(true)
	r := probe.Bounds()
	v.blocks = append(v.blocks, mdBlock{line: start, next: next, r: r})

	if ready && e.line > 0 && d.AnchorLine == e.line {
		L := float64(e.line - 1)
		if v.fracLine == e.line {
			L += v.frac
		}
		if int(L) < next && (float64(start) <= L || i == 1) {
			off := float32(0)
			if r.H > 0 && L > float64(start) { // 和 lineAt 反过来：这块的第一行到下一块的第一行按屏幕行摊在这块的顶到下一块的顶上
				span := r.H
				for _, b := range v.prev { // 上一帧的框：和下一块的框是同一次排版的
					if b.line == start && b.r.H > 0 {
						span = v.spanEnd(v.prev, b) - b.r.Y
					}
				}
				if r0, r1 := v.row(float64(start)), v.row(float64(next)); r1 > r0 {
					off = float32(min((v.row(L)-r0)/(r1-r0), 1)) * span
				}
			}
			mdMarker(c, off)
			d.AnchorLine, v.seek, v.fracLine = 0, 0, 0
		}
	}
	t := c.Theme()
	if d.Kind == docs.MDPreview && d.MDSource != nil {
		if p, ok := d.MDSource.Cursor(); ok && start <= p.Line && p.Line < next {
			wui.Box(c).Absolute().Top(0).Bottom(0).Left(-mdCaretGap).Width(mdCaretBar).Background(t.Text.Alpha(0.22)).IgnoreInput(true).AccessHidden(true)
		}
	}
	if b.Kind == parser.Heading {
		e.headingLink(c, doc, i, r)
	}
}

const (
	mdCaretGap   = 14 // 光标竖条在块左边多远（VS Code 是 -12px 加 3px 的边）
	mdCaretBar   = 3
	mdLinkGutter = 28 // 标题左边放 🔗 的那一条
)

// headingLink 在标题左边留一条：指针在标题上或这一条上时出一个 🔗，点了复制这个标题的链接（仓库里的路径#slug）并滚到它。
func (e *mdEnv) headingLink(c *wui.Context, doc *parser.Document, i int, r wui.Rect) {
	v, d, a := e.v, e.d, e.a
	slug := markdown.Slug(parser.PlainText(doc.Inlines(i)))
	if slug == "" {
		return
	}
	gutter := wui.Column(c.Key(mdLinkKey{slug})).Absolute().Top(0).Bottom(0).Left(-mdLinkGutter).Width(mdLinkGutter).Justify(wui.Center)
	over := v.ptrOK && v.ptr.Y >= r.Y && v.ptr.Y < r.Y+r.H && v.ptr.X >= r.X-mdLinkGutter && v.ptr.X < r.X+r.W
	if !over && !gutter.Hovered() {
		return
	}
	gutter.Children(func() {
		rowButton(c, iconLink, i18n.T("复制标题链接"), i18n.T("复制这个标题的链接，并滚到它"), func() {
			app.CopyText(mdHeadingLink(d, slug))
			a.Toast = i18n.T("已复制标题链接")
			d.Anchor = slug
		})
	})
}

type mdLinkKey struct{ slug string }

// mdHeadingLink 是标题的链接：仓库里的文件写仓库里的相对路径，别的写绝对路径，后面接 #slug。
func mdHeadingLink(d *docs.Doc, slug string) string {
	path := d.Rel
	if d.Root == "" || path == "" {
		path = d.Abs
	}
	return path + "#" + slug
}
