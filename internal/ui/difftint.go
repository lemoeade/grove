package ui

import (
	"math"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// Diff 改了的行的底色由 Grove 自己画在 Diff 视图底下（diffUnder），不用 mygo 的：mygo 按固定比例把 Theme.Added /
// Deleted 混进背景（浅色 12%），浓淡改不了，到不了 VS Code 那么醒目。做法是让 mygo 不涂底（NoBackground，
// Theme.Background 透明），外面包一层编辑器底色的盒子，在它的 Draw 里按 DiffState.LineTop / ScrollY 每帧现算位置涂红绿；
// 改了的词和行首竖条仍是 mygo 画在上面（竖条色的 22%，见 diffTint）。
//
// 颜色照 VS Code 的默认主题：diffEditor.insertedLineBackground #9bb95533、removedLineBackground #ff000033，深浅色一样。
var (
	diffAddedLine   = wui.RGBA(155, 185, 85, 0.2)
	diffDeletedLine = wui.RGBA(255, 0, 0, 0.2)
)

// diffBase 是 Diff 视图里没改的行的底色（mygo 编辑器的默认底色；VS Code 风格是编辑器的底），涂在包着它的盒子上。
func diffBase(t *wui.Theme) wui.Color {
	if vscode() {
		return vsEditor(t)
	}
	return editor.DefaultTheme(t).Background
}

// clearDiffBackground 让 mygo 不涂 Diff 的底：改了的行不涂（NoBackground），没改的也不涂（Background 的 A 是 0；
// RGB 留着编辑器底色，mygo 拿它判断深浅色，全零的颜色 mygo 当成没设、会补成不透明的默认值）。
func clearDiffBackground(d *editor.DiffState, base wui.Color) {
	d.NoBackground = true
	base.A = 0
	d.Theme.Background = base
}

// diffUnder 画 Diff 改了的行的底色和选中的行（mygo 选中行的底色是混进 Background 的，底透明后几乎看不见，也由这里画）。
func diffUnder(d *editor.DiffState, sel wui.Color) func(p *wui.Painter, r wui.Rect) {
	return func(p *wui.Painter, r wui.Rect) {
		for _, t := range diffTints(d, r, p.Scale(), sel) {
			p.Fill(t.R, t.C, 0)
		}
	}
}

// diffTintRect 是 diffTints 算出的一块底色。
type diffTintRect struct {
	R wui.Rect
	C wui.Color
}

// diffTints 是 Diff 视图（框 r，屏幕缩放 scale）这一帧要涂的底色：按 DiffState.LineTop / ScrollY 现算，只留看得见的那几段。
func diffTints(d *editor.DiffState, r wui.Rect, scale float32, sel wui.Color) []diffTintRect {
	var out []diffTintRect
	y0 := float32(d.ScrollY())
	top := func(s editor.DiffSide, line int) float32 { return r.Y + float32(d.LineTop(s, line)) - y0 }
	// 一行多高：和 mygo 的 view.setStyle 一样按屏幕像素取整
	size, lhm := d.Font.Size, d.LineHeight
	if size <= 0 {
		size = 13
	}
	if lhm <= 0 {
		lhm = 1.5
	}
	scale = max(scale, 1)
	lh := float32(math.Round(float64(size*lhm*scale))) / scale

	split := d.Layout == editor.DiffSplit
	xs := [2]float32{r.X, r.X} // 旧的、新的一边从哪开始、多宽（并排时中间隔一条 1 pt 的线）
	ws := [2]float32{r.W, r.W}
	if split {
		share := float32(0.5)
		if d.SplitPercent > 0 {
			share = min(d.SplitPercent, 100) / 100
		}
		ws[0] = (r.W - 1) * share
		xs[1], ws[1] = r.X+ws[0]+1, r.W-ws[0]-1
	}
	bottom := r.Y + r.H
	fill := func(side int, a, b float32, c wui.Color) {
		a, b = max(a, r.Y), min(b, bottom)
		if b > a {
			out = append(out, diffTintRect{wui.Rect{X: xs[side], Y: a, W: ws[side], H: b - a}, c})
		}
	}
	// 一段行 [from, to) 涂上 c。最后一行只涂它自己：下面可能是并排时给对面多出的行留的空，或折叠条。
	// 软换行时最后一行可能折成几行，量到下一行的距离；对面行多（下面留了空）时还是只涂一行。
	span := func(s editor.DiffSide, from, to, other int, c wui.Color) {
		if to <= from {
			return
		}
		side := 1
		if s == editor.DiffOld {
			side = 0
		}
		a := top(s, from)
		if a > bottom {
			return
		}
		last := top(s, to-1)
		h := lh
		if d.Wrap && other <= to-from {
			h = max(top(s, to)-last, lh)
		}
		fill(side, a, last+h, c)
	}
	for _, hk := range d.Hunks() {
		na, nb := hk.NewEnd-hk.NewStart, hk.OldEnd-hk.OldStart
		if split {
			span(editor.DiffOld, hk.OldStart, hk.OldEnd, na, diffDeletedLine)
		} else if nb > 0 {
			// 统一视图：删掉的那几行在这段新行的上面，到新的第一行的顶为止
			fill(0, top(editor.DiffOld, hk.OldStart), top(editor.DiffNew, hk.NewStart), diffDeletedLine)
		}
		span(editor.DiffNew, hk.NewStart, hk.NewEnd, nb, diffAddedLine)
	}
	if lr, ok := d.SelectedLines(); ok {
		side := 1
		if lr.Side == editor.DiffOld && split {
			side = 0
		}
		fill(side, top(lr.Side, lr.Start), top(lr.Side, lr.End)+lh, sel)
	}
	return out
}
