package ui

import (
	"os"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/ged"
	"github.com/lemoeade/grove/internal/i18n"
)

// gedOn：设了 WORKBENCH_GED 时，只看的 Diff（并排、统一）用 Grove 自己画的（internal/ged）；就地编辑
// 还是 mygo 的 editor.DiffView。试做阶段，默认不开。
var gedOn = os.Getenv("WORKBENCH_GED") != ""

// useGed 是这个 Diff 这一帧用不用 ged：开了、不在就地编辑（和 docs.Doc.UsingGed 一样，只是那个还要建过 Ged）。
func useGed(d *docs.Doc) bool {
	return gedOn && d.DiffSt != nil && !d.DiffSt.Editable
}

type gedKey struct{ d *docs.Doc }

// gedText 是 ged.Diff.Set 的 key：两边的缓冲区和版本，一样就不重新对齐。
type gedText struct {
	old, new       *editor.Buffer
	oldVer, newVer uint64
}

// gedDiff 建 ged 的并排 Diff：内容取 DiffSt 的两个缓冲区，颜色和 mygo 那个 Diff 一样（diffTint、diffAddedLine……）。
func gedDiff(a *app.App, c *wui.Context, d *docs.Doc) wui.Element {
	st := d.DiffSt
	if d.Ged == nil {
		d.Ged = ged.NewDiff()
	}
	lang := st.New.Language()
	if lang == nil {
		lang = st.Old.Language()
	}
	g := d.Ged
	g.Set(gedText{st.Old, st.New, st.Old.Version(), st.New.Version()}, func() (o, n []string) {
		return bufLines(st.Old), bufLines(st.New)
	}, lang)
	g.SetFull(a.St.FullDiff)
	g.SetUnified(st.Layout == editor.DiffUnified) // diffLook 定的：设置里的统一，或者一边是空的（新文件、删掉的文件）
	switch f := d.GedFocus; {
	case f > 0:
		g.GoToHunk(g.HunkAtLine(1, f-1))
	case f < 0 && g.Refocus():
	case f < 0:
		g.GoToHunk(0)
	}
	d.GedFocus = 0
	s := gedStyle(a, c.Theme())
	s.Label = d.Where()
	if st.HunkActions { // diffContent 定的：未暂存 / 已暂存的改了的文件，不在就地编辑
		stage, unstage, discard := app.HunkOps(d)
		s.HunkBar = func(c *wui.Context, bar wui.Element, _ int, h ged.Hunk) {
			hunkActions(a, c, bar, d, editor.DiffHunk{OldStart: h.OldStart, OldEnd: h.OldEnd, NewStart: h.NewStart, NewEnd: h.NewEnd},
				stage, unstage, discard)
		}
	}
	if st.LineSelection { // 点行号选行（行级暂存）：和 mygo 的 Diff 用同一份选中的行，底下那栏、暂存选中的行照旧
		s.Lines = func() (side, start, end int, ok bool) {
			r, ok := st.SelectedLines()
			return int(r.Side), r.Start, r.End, ok
		}
		s.SetLines = func(side, start, end int, ok bool) {
			st.SetSelectedLines(editor.DiffLineRange{Side: editor.DiffSide(side), Start: start, End: end}, ok)
		}
		s.LineSel, s.LineMark = c.Theme().Accent.Alpha(0.2), c.Theme().Accent
	}
	s.Copy = func(text string) { app.CopyText(text) }
	s.Keys = func(ev wui.InputEvent) bool { return gedKeys(a, d, ev) }
	return ged.View(c, gedKey{d}, g, s)
}

// gedKeys 是 ged 的 Diff 有焦点时界面这边接的键：下一处 / 上一处改动（默认 J / K，按命令注册表里现在的键）。
func gedKeys(a *app.App, d *docs.Doc, ev wui.InputEvent) bool {
	mods := ev.Mods & (wui.Shift | wui.Ctrl | wui.Alt | wui.Super)
	for _, k := range []struct {
		id  string
		dir int
	}{{"diff.nextHunk", 1}, {"diff.prevHunk", -1}} {
		if m, key, ok := app.KeyOf(k.id); ok && key == ev.Key && m == mods {
			a.GoToHunk(d, k.dir)
			return true
		}
	}
	return false
}

func bufLines(b *editor.Buffer) []string {
	out := make([]string, b.LineCount())
	for i := range out {
		out[i] = b.Line(i)
	}
	return out
}

// gedStyle 是 ged 的 Diff 按 Grove 的主题的样子。
func gedStyle(a *app.App, t *wui.Theme) ged.Style {
	ct := fullCodeTheme(t)
	ct.Background = diffBase(t)
	diffTint(ct, t)
	font := a.EditorFont()
	font.Size -= 0.5
	return ged.Style{
		Code:        ct,
		Font:        wui.Font{Family: font.Family, Size: font.Size},
		LineHeight:  font.LineHeight,
		AddedLine:   diffAddedLine,
		DeletedLine: diffDeletedLine,
		AddedWord:   wui.Hex("#9ccc2c").Alpha(0.3),
		DeletedWord: wui.Hex("#ff0000").Alpha(0.25),
		AddedMark:   ct.Added,
		DeletedMark: ct.Deleted,
		Filler:      t.TextMuted.Alpha(0.16),
		FoldBack:    ct.Text.Alpha(0.035),
		FoldHover:   toolFill(t, 0.08),
		FoldText:    t.TextMuted,
		Divider:     t.Border,
		Ruler:       ct.Text.Alpha(0.02),
		UIFont:      wui.Font{Size: 11},
		FoldLabel:   func(n int) string { return i18n.Tn("%d 行未修改", n) },
	}
}
