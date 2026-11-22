package ui

import (
	"testing"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
)

// 风格：默认（存成 ""）是 VS Code——字色、线色、底色、代码配色照 VS Code 的 Modern 主题，卡片是平的；
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
		text, kw, bg := "#3b3b3b", "#0000ff", "#ffffff"
		if dark {
			text, kw, bg = "#cccccc", "#569cd6", "#1f1f1f"
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
	if kw := fullCodeTheme(&th).Tokens[editor.TokenKeyword].Color; kw == wui.Hex("#569cd6") {
		t.Errorf("Grove 风格的代码用的还是 VS Code 配色：%v", kw)
	}
}
