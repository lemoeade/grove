package ui

import (
	"sync/atomic"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
)

// 这个文件是设置里的“风格”。默认是 VS Code 风格：照 VS Code 的 Light Modern / Dark Modern 两个默认主题，
// 不透明的平面——标题栏、侧栏、状态栏、终端和右栏都是 #F8F8F8（深色 #181818），只有编辑器组是白底（深色 #1F1F1F），
// 中间一道 #E5E5E5（深色 #2B2B2B）的线；卡片不要圆角和阴影，窗口底不画色斑、不透；代码是 VS Code 的配色（Light+ / Dark+），
// 行号、光标、选中、查找、配对括号、缩进参考线也照它。“Grove”是原来的样子：柔和底（或纯色、通透）上的圆角卡片、GitHub 配色。
//
// 取颜色的小函数（cardBg、windowBg、headerBg……）只拿得到主题、拿不到 App，而设置是整个应用一份（几个窗口一样），
// 所以每帧开头（root、设置窗口、关于窗口：applyAccent）照设置记在 vsLook 里。

var vsLook atomic.Bool

// setLook 照设置记下这一帧是不是 VS Code 风格。
func setLook(a *app.App) { vsLook.Store(a.St.Settings.Look == "") }

// vscode 是 VS Code 风格。
func vscode() bool { return vsLook.Load() }

// VS Code Modern 主题的界面颜色：side 是标题栏、侧栏、面板、状态栏、标签条，editor 是编辑器，line 是它们之间的线。
var (
	vsSideLight, vsSideDark     = wui.Hex("#f8f8f8"), wui.Hex("#181818")
	vsEditorLight, vsEditorDark = wui.Hex("#ffffff"), wui.Hex("#1f1f1f")
	vsLineLight, vsLineDark     = wui.Hex("#e5e5e5"), wui.Hex("#2b2b2b")
)

// vsSide、vsEditor、vsLine 是 t 的深浅对应的那一个。
func vsSide(t *wui.Theme) wui.Color   { return pick(t, vsSideLight, vsSideDark) }
func vsEditor(t *wui.Theme) wui.Color { return pick(t, vsEditorLight, vsEditorDark) }
func vsLine(t *wui.Theme) wui.Color   { return pick(t, vsLineLight, vsLineDark) }

func pick(t *wui.Theme, light, dark wui.Color) wui.Color {
	if t.Dark {
		return dark
	}
	return light
}

// applyLook 在 VS Code 风格时把这一帧的主题换成 VS Code 的字色、线色和弹出层的底：
// 正文 #3B3B3B（深色 #CCCCCC）、次要的字 #616161（#9D9D9D）、线 #E5E5E5（#2B2B2B）、底白（#1F1F1F），
// 控件的面淡一档。强调色另由 applyAccent 换。
func applyLook(t *wui.Theme) {
	if !vscode() {
		return
	}
	if t.Dark {
		t.Text, t.TextMuted = wui.Hex("#cccccc"), wui.Hex("#9d9d9d")
		t.Surface, t.SurfaceHover, t.SurfacePressed = wui.Hex("#2a2a2a"), wui.Hex("#323232"), wui.Hex("#3c3c3c")
	} else {
		t.Text, t.TextMuted = wui.Hex("#3b3b3b"), wui.Hex("#616161")
		t.Surface, t.SurfaceHover, t.SurfacePressed = wui.Hex("#f2f2f2"), wui.Hex("#e8e8e8"), wui.Hex("#dddddd")
	}
	t.Background, t.Border = vsEditor(t), vsLine(t)
	t.ScrollbarStyle = vsScrollbar(t)
}

// vsScrollbar 是 VS Code 的滚动条：方的、贴着两头、10 宽（列表和编辑器；Diff 自己的概览条 14 宽），
// 半透明的灰（scrollbarSlider.background / hoverBackground / activeBackground），悬停、拖着时深一档。
func vsScrollbar(t *wui.Theme) wui.ScrollbarStyle {
	s := wui.ScrollbarStyle{TrackWidth: 10, ThumbWidth: 10, Square: true, Flush: true, MinLength: 20}
	if t.Dark {
		s.Thumb, s.Hover, s.Active = wui.RGBA(121, 121, 121, 0.4), wui.RGBA(100, 100, 100, 0.7), wui.RGBA(191, 191, 191, 0.4)
	} else {
		s.Thumb, s.Hover, s.Active = wui.RGBA(100, 100, 100, 0.4), wui.RGBA(100, 100, 100, 0.7), wui.RGBA(0, 0, 0, 0.6)
	}
	return s
}

// vsCodeTheme 把 ct 换成 VS Code 的代码配色（Light Modern / Dark Modern 用的 Light+ / Dark+）：
// 底色是编辑器的底，行号 #6E7681，光标所在行的行号深色，选中 #ADD6FF（#264F78），查找的命中、配对括号的框、
// 缩进参考线、空白符号也是 VS Code 的默认值。粗体、斜体照 mygo 的（Markdown 的标题、强调）。
func vsCodeTheme(ct *editor.Theme, t *wui.Theme) {
	tok := func(k editor.TokenKind, hex string) { ct.Tokens[k].Color = wui.Hex(hex) }
	ct.Background = vsEditor(t)
	ct.LineNumber = wui.Hex("#6e7681")
	if t.Dark {
		ct.Text = wui.Hex("#cccccc")
		ct.ActiveLineNumber = wui.Hex("#cccccc")
		ct.Cursor = wui.Hex("#aeafad")
		ct.Selection, ct.InactiveSelection = wui.Hex("#264f78"), wui.Hex("#3a3d41")
		ct.ActiveLine = wui.RGBA(255, 255, 255, 0.04)
		ct.MatchingBracket = wui.Hex("#888888")
		ct.Match, ct.CurrentMatch = wui.Hex("#ea5c00").Alpha(0.33), wui.Hex("#9e6a03")
		ct.Occurrence = wui.Hex("#575757").Alpha(0.72)
		ct.IndentGuide, ct.ActiveIndentGuide = wui.Hex("#404040"), wui.Hex("#707070")
		ct.WrapGuide = wui.Hex("#5a5a5a")
		ct.Whitespace = wui.Hex("#e3e4e2").Alpha(0.16)
		ct.Error, ct.Warning, ct.Info, ct.Hint = wui.Hex("#f14c4c"), wui.Hex("#cca700"), wui.Hex("#3794ff"), wui.Hex("#eeeeee").Alpha(0.7)
		ct.Added, ct.Deleted, ct.Modified = wui.Hex("#2ea043"), wui.Hex("#f85149"), wui.Hex("#0078d4")
		tok(editor.TokenText, "#cccccc")
		tok(editor.TokenKeyword, "#569cd6")
		tok(editor.TokenOperator, "#d4d4d4")
		tok(editor.TokenPunctuation, "#cccccc")
		tok(editor.TokenType, "#4ec9b0")
		tok(editor.TokenFunction, "#dcdcaa")
		tok(editor.TokenMeta, "#dcdcaa")
		tok(editor.TokenVariable, "#9cdcfe")
		tok(editor.TokenProperty, "#9cdcfe")
		tok(editor.TokenConstant, "#4fc1ff")
		tok(editor.TokenNumber, "#b5cea8")
		tok(editor.TokenAttribute, "#9cdcfe")
		tok(editor.TokenString, "#ce9178")
		tok(editor.TokenEscape, "#d7ba7d")
		tok(editor.TokenRegexp, "#d16969")
		tok(editor.TokenTag, "#569cd6")
		tok(editor.TokenComment, "#6a9955")
		tok(editor.TokenHeading, "#569cd6")
		tok(editor.TokenEmphasis, "#cccccc")
		tok(editor.TokenStrong, "#cccccc")
		tok(editor.TokenLink, "#3794ff")
		tok(editor.TokenCode, "#ce9178")
		tok(editor.TokenInserted, "#b5cea8")
		tok(editor.TokenDeleted, "#ce9178")
		tok(editor.TokenInvalid, "#f44747")
	} else {
		ct.Text = wui.Hex("#3b3b3b")
		ct.ActiveLineNumber = wui.Hex("#171184")
		ct.Cursor = wui.Hex("#000000")
		ct.Selection, ct.InactiveSelection = wui.Hex("#add6ff"), wui.Hex("#e5ebf1")
		ct.ActiveLine = wui.RGBA(0, 0, 0, 0.03)
		ct.MatchingBracket = wui.Hex("#b9b9b9")
		ct.Match, ct.CurrentMatch = wui.Hex("#ea5c00").Alpha(0.33), wui.Hex("#a8ac94")
		ct.Occurrence = wui.Hex("#575757").Alpha(0.25)
		ct.IndentGuide, ct.ActiveIndentGuide = wui.Hex("#d3d3d3"), wui.Hex("#939393")
		ct.WrapGuide = wui.Hex("#d3d3d3")
		ct.Whitespace = wui.Hex("#333333").Alpha(0.2)
		ct.Error, ct.Warning, ct.Info, ct.Hint = wui.Hex("#e51400"), wui.Hex("#bf8803"), wui.Hex("#1a85ff"), wui.Hex("#6c6c6c")
		ct.Added, ct.Deleted, ct.Modified = wui.Hex("#2ea043"), wui.Hex("#f85149"), wui.Hex("#005fb8")
		tok(editor.TokenText, "#3b3b3b")
		tok(editor.TokenKeyword, "#0000ff")
		tok(editor.TokenOperator, "#3b3b3b")
		tok(editor.TokenPunctuation, "#3b3b3b")
		tok(editor.TokenType, "#267f99")
		tok(editor.TokenFunction, "#795e26")
		tok(editor.TokenMeta, "#795e26")
		tok(editor.TokenVariable, "#001080")
		tok(editor.TokenProperty, "#001080")
		tok(editor.TokenConstant, "#0070c1")
		tok(editor.TokenNumber, "#098658")
		tok(editor.TokenAttribute, "#e50000")
		tok(editor.TokenString, "#a31515")
		tok(editor.TokenEscape, "#ee0000")
		tok(editor.TokenRegexp, "#811f3f")
		tok(editor.TokenTag, "#800000")
		tok(editor.TokenComment, "#008000")
		tok(editor.TokenHeading, "#800000")
		tok(editor.TokenEmphasis, "#3b3b3b")
		tok(editor.TokenStrong, "#3b3b3b")
		tok(editor.TokenLink, "#0451a5")
		tok(editor.TokenCode, "#800000")
		tok(editor.TokenInserted, "#098658")
		tok(editor.TokenDeleted, "#a31515")
		tok(editor.TokenInvalid, "#cd3131")
	}
}

// 侧栏的字号和行高跟着风格：Grove 正文 14、仓库名 14.5、文件树一行 24；VS Code 风格照它的 13 号字、22 高的行，
// 仓库名和正文一样 13（VS Code 的列表、源代码管理的仓库行都是 13）；右栏历史一个提交一行 30，VS Code 风格照源代码管理图的 22。
const (
	sideFontGrove, sideFontVS = 14, 13
	repoFontGrove, repoFontVS = 14.5, 13
	treeRowGrove, treeRowVS   = 24, 22
	histRowGrove, histRowVS   = 30, 22
)

// sideFont 是侧栏正文（文件名、仓库名、变更、结果）的字号。
func sideFont() float32 { return lookSize(sideFontGrove, sideFontVS) }

// repoFont 是 Git 栏仓库段头、右栏历史头部的仓库名的字号。
func repoFont() float32 { return lookSize(repoFontGrove, repoFontVS) }

// treeRowH 是文件树目录 / 文件行的高（上下各 3 再凑成整数，行和吸顶层都按它对齐）。
func treeRowH() float32 { return lookSize(treeRowGrove, treeRowVS) }

func lookSize(grove, vs float32) float32 {
	if vscode() {
		return vs
	}
	return grove
}

// histRowH 是右栏历史一个提交一行的高。
func histRowH() float32 { return lookSize(histRowGrove, histRowVS) }
