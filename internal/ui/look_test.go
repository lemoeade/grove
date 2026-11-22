package ui

import (
	"testing"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
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
		text, kw, bg := "#202020", "#cf222e", "#ffffff"
		if dark {
			text, kw, bg = "#bfbfbf", "#ff7b72", "#121314"
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
	// VS Code 2026 的代码配色和 mygo 默认的一样是 GitHub 那一套，认底色和正文色
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
