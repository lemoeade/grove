package ui

import (
	"testing"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/outline"
)

// 风格：默认（存成 ""）是 VS Code——字色、线色、底色、代码配色照 VS Code 的 2026 主题，卡片是平的；
// “grove” 是原来的样子，主题不动、代码是 GitHub 配色。
func TestLook(t *testing.T) {
	defer vsLook.Store(true)
	a := &app.App{}
	for _, dark := range []bool{false, true} {
		th := *wui.LightTheme()
		if dark {
			th = *wui.DarkTheme()
		}
		applyAccent(a, &th)
		text, kw, bg := "#202020", "#0000ff", "#ffffff"
		if dark {
			text, kw, bg = "#bfbfbf", "#569cd6", "#121314"
		}
		if !vscode() || th.Text != wui.Hex(text) || th.Background != wui.Hex(bg) || th.Border != vsLine(&th) {
			t.Errorf("深色 %v：默认风格的字 %v、底 %v、线 %v", dark, th.Text, th.Background, th.Border)
		}
		ct := fullCodeTheme(&th)
		if ct.Tokens[editor.TokenKeyword].Color != wui.Hex(kw) || ct.Background != wui.Hex(bg) {
			t.Errorf("深色 %v：代码的关键字 %v、底 %v", dark, ct.Tokens[editor.TokenKeyword].Color, ct.Background)
		}
		if cardBg(&th) != wui.Hex(bg) || windowBg(&th) != vsSide(&th) || headerBg(&th) != vsSide(&th) {
			t.Errorf("深色 %v：卡片 %v、窗口底 %v、标题 %v", dark, cardBg(&th), windowBg(&th), headerBg(&th))
		}
	}

	a.St.Settings.Look = "grove"
	th := *wui.DarkTheme()
	want := th
	applyAccent(a, &th)
	if vscode() || th.Text != want.Text || th.Background != want.Background || th.Border != want.Border {
		t.Errorf("Grove 风格不该动字色、底色、线色：%v %v %v", th.Text, th.Background, th.Border)
	}
	// 认底色和正文色（Grove 风格是 mygo 默认的 GitHub 配色）
	if ct := fullCodeTheme(&th); ct.Background == vsEditorDark || ct.Tokens[editor.TokenText].Color == vsCodeText(&th) {
		t.Errorf("Grove 风格的代码用的还是 VS Code 配色：底 %v、字 %v", ct.Background, ct.Tokens[editor.TokenText].Color)
	}
}

// VS Code 风格的滚动条轨道透明、滑块淡（静止时不超过 25%，悬停、拖着时一档比一档深，最深也不到 60%）。
func TestVSScrollbarClear(t *testing.T) {
	for _, th := range []*wui.Theme{wui.LightTheme(), wui.DarkTheme()} {
		s := vsScrollbar(th)
		if s.Track.A != 0 {
			t.Errorf("dark=%v：轨道该是透明的：%v", th.Dark, s.Track)
		}
		if s.Thumb.A == 0 || s.Thumb.A > 0x40 || s.Hover.A <= s.Thumb.A || s.Active.A <= s.Hover.A || s.Active.A >= 0x99 {
			t.Errorf("dark=%v：滑块该是通透的灰、一档比一档深：%v / %v / %v", th.Dark, s.Thumb, s.Hover, s.Active)
		}
		if s.Thumb.R != s.Thumb.G || s.Thumb.G != s.Thumb.B {
			t.Errorf("dark=%v：滑块该是灰的：%v", th.Dark, s.Thumb)
		}
	}
}

// 深色里默认的黑强调色是中灰，编辑器选中的底换成浅灰叠上去（中灰叠在深底上看不出）；带颜色的强调色照旧用自己。
func TestVSSelectionNeutralDark(t *testing.T) {
	th := wui.DarkTheme()
	th.Accent = wui.Hex("#5C5C61")
	if got := vsSelection(th); got.R != 0xd4 || got.G != 0xd4 || got.B != 0xd4 {
		t.Errorf("深色灰强调色的选中该叠浅灰：%v", got)
	}
	th.Accent = wui.Hex("#297AA0")
	if got := vsSelection(th); got.R != 0x29 || got.B != 0xa0 {
		t.Errorf("深色蓝强调色的选中该叠蓝：%v", got)
	}
	lt := wui.LightTheme()
	lt.Accent = wui.Hex("#333333")
	if got := vsSelection(lt); got.R != 0x33 {
		t.Errorf("浅色黑强调色的选中该叠黑：%v", got)
	}
}

// VS Code 风格的符号小字照 VS Code 的符号图标上色：类橙、函数紫、变量蓝，标题灰；Grove 风格都是次级色。
func TestSymbolColor(t *testing.T) {
	defer vsLook.Store(vsLook.Load())
	th := wui.LightTheme()
	muted := th.TextMuted
	vsLook.Store(true)
	for k, want := range map[outline.Kind]wui.Color{
		outline.Class: wui.Hex("#d67e00"), outline.Func: wui.Hex("#652d90"), outline.Method: wui.Hex("#652d90"),
		outline.Var: wui.Hex("#007acc"), outline.Heading: muted, outline.Type: muted,
	} {
		if got := symbolColor(th, k, muted); got != want {
			t.Errorf("VS Code 风格 %s：%v，应为 %v", symbolGlyph(k), got, want)
		}
	}
	if got := symbolColor(wui.DarkTheme(), outline.Class, muted); got != wui.Hex("#ee9d28") {
		t.Errorf("深色的类：%v", got)
	}
	vsLook.Store(false)
	if got := symbolColor(th, outline.Class, muted); got != muted {
		t.Errorf("Grove 风格的类该是次级色：%v", got)
	}
}
