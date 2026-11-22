package ui

import (
	"sync/atomic"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
)

// 这个文件是设置里的“风格”。默认是 VS Code 风格：照 VS Code 1.147 起的两个默认主题 Light 2026 / Dark 2026，
// 不透明的平面——标题栏、侧栏、状态栏、终端和右栏都是 #FAFAFD（深色 #191A1B），只有编辑器组是白底（深色 #121314），
// 中间一道 #F0F1F2（深色 #2A2B2C）的线；卡片不要圆角和阴影，窗口底不画色斑、不透；代码是它的配色（GitHub 那一套），
// 行号、光标、选中、查找、配对括号、缩进参考线也照它。界面基本是中性的灰：列表的选中是灰的、当前标签顶上是黑线，
// 强调色（#0069CC，深色 #297AA0）只留给按钮、焦点框、角标、链接和选中的文字。“Grove”是原来的样子：柔和底（或纯色、通透）上的圆角卡片、GitHub 配色。
//
// 取颜色的小函数（cardBg、windowBg、headerBg……）只拿得到主题、拿不到 App，而设置是整个应用一份（几个窗口一样），
// 所以每帧开头（root、设置窗口、关于窗口：applyAccent）照设置记在 vsLook 里。

var vsLook atomic.Bool

// setLook 照设置记下这一帧是不是 VS Code 风格。
// 浮层的不透明度（frost.go）也在这里记。
func setLook(a *app.App) {
	vsLook.Store(a.St.Settings.Look == "")
	frostPct.Store(int32(a.St.Settings.FrostPercent()))
}

// vscode 是 VS Code 风格。
func vscode() bool { return vsLook.Load() }

// VS Code 2026 主题的界面颜色：side 是标题栏、侧栏、面板、状态栏、标签条，editor 是编辑器，line 是它们之间的线。
var (
	vsSideLight, vsSideDark     = wui.Hex("#fafafd"), wui.Hex("#191a1b")
	vsEditorLight, vsEditorDark = wui.Hex("#ffffff"), wui.Hex("#121314")
	vsLineLight, vsLineDark     = wui.Hex("#f0f1f2"), wui.Hex("#2a2b2c")
)

// vsActiveLine 是当前标签顶上、底栏当前标题底下那道线（tab.activeBorderTop、panelTitle.activeBorder）：浅色是黑的，深色 #3994BC。
func vsActiveLine(t *wui.Theme) wui.Color { return pick(t, wui.Hex("#000000"), wui.Hex("#3994bc")) }

// vsActivityLine 是视图切换（活动栏）当前那个底下的线（activityBarTop.activeBorder）：浅色黑、深色是正文色。
func vsActivityLine(t *wui.Theme) wui.Color { return pick(t, wui.Hex("#000000"), wui.Hex("#bfbfbf")) }

// vsSecondary 是次要按钮（对话框的“取消”）的底和框：button.secondaryBackground / secondaryBorder，浅色 #EAEAEA，
// 深色透明（这里给对话框的底色）描 #333536。
func vsSecondary(t *wui.Theme) (bg, border wui.Color) {
	return pick(t, wui.Hex("#eaeaea"), vsOverlayBg(t)), pick(t, wui.Hex("#eaeaea"), wui.Hex("#333536"))
}

// vsHighlight 是列表里筛选命中的字色（list.highlightForeground）。
func vsHighlight(t *wui.Theme) wui.Color { return pick(t, wui.Hex("#0069cc"), wui.Hex("#48a0c7")) }

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
// 正文 #202020（深色 #BFBFBF）、次要的字 #606060（#8C8C8C）、线 #F0F1F2（#2A2B2C）、底白（#121314），
// 控件的面（mygo 的输入框、下拉框、步进器拿它垫底）是 input.background 的白（#191A1B），悬停、按下深一档。强调色另由 applyAccent 换。
func applyLook(t *wui.Theme) {
	if !vscode() {
		return
	}
	if t.Dark {
		t.Text, t.TextMuted = wui.Hex("#bfbfbf"), wui.Hex("#8c8c8c")
		t.Surface, t.SurfaceHover, t.SurfacePressed = wui.Hex("#191a1b"), wui.Hex("#323233"), wui.Hex("#3c3d3e")
	} else {
		t.Text, t.TextMuted = wui.Hex("#202020"), wui.Hex("#606060")
		t.Surface, t.SurfaceHover, t.SurfacePressed = wui.Hex("#ffffff"), wui.Hex("#e3e3e5"), wui.Hex("#d6d6d8")
	}
	t.Background, t.Border = vsEditor(t), vsLine(t)
	t.ScrollbarStyle = vsScrollbar(t)
}

// vsScrollbar 是 VS Code 的滚动条：方的、贴着两头、10 宽（列表和编辑器；Diff 自己的概览条 14 宽），
// 轨道透明，滑块是淡淡的灰：不照 2026 主题那块几乎不透的（#646464 的 75%），比 VS Code 以前的默认主题（Light+ / Dark+，40%）
// 还淡：#646464（深色 #797979）的 25%，悬停 40%、拖着 55%，盖在代码上底下的字看得清。
func vsScrollbar(t *wui.Theme) wui.ScrollbarStyle {
	s := wui.ScrollbarStyle{TrackWidth: 10, ThumbWidth: 10, Square: true, Flush: true, MinLength: 20}
	g := uint8(pickF(t, 100, 121))
	s.Thumb, s.Hover, s.Active = wui.RGBA(g, g, g, 0x40/255.0), wui.RGBA(g, g, g, 0x66/255.0), wui.RGBA(g, g, g, 0x8c/255.0)
	return s
}

// vsCodeText、vsLineNumber 是 VS Code 2026 编辑器的正文色和行号色（softTheme 拿它们认出已经套好的配色）。
func vsCodeText(t *wui.Theme) wui.Color   { return pick(t, wui.Hex("#202020"), wui.Hex("#bbbebf")) }
func vsLineNumber(t *wui.Theme) wui.Color { return pick(t, wui.Hex("#606060"), wui.Hex("#858889")) }

// vsCodeTheme 把 ct 换成 VS Code 2026 的代码配色（Light 2026 / Dark 2026 的 tokenColors，就是 GitHub 那一套：
// 关键字和运算符红、函数紫、类型和变量定义橙、常量和属性名蓝、字符串深蓝、标签绿、注释灰）：
// 底色是编辑器的底，行号 #606060（深色 #858889），光标所在行的行号、光标和正文一色，选中是强调色 25%（#0069CC40），
// 查找的命中、同名高亮、配对括号、缩进参考线、空白符号也是它的值。粗体、斜体照 mygo 的（Markdown 的标题、强调）。
func vsCodeTheme(ct *editor.Theme, t *wui.Theme) {
	tok := func(k editor.TokenKind, hex string) { ct.Tokens[k].Color = wui.Hex(hex) }
	ct.Background = vsEditor(t)
	ct.Text, ct.LineNumber = vsCodeText(t), vsLineNumber(t)
	ct.ActiveLineNumber, ct.Cursor = ct.Text, ct.Text
	if t.Dark {
		ct.Selection, ct.InactiveSelection = wui.Hex("#276782").Alpha(0xdd/255.0), wui.Hex("#276782").Alpha(0x60/255.0)
		ct.ActiveLine = wui.Hex("#242526")
		ct.MatchingBracket = wui.Hex("#3994bc").Alpha(0x55 / 255.0)
		ct.Match, ct.CurrentMatch = wui.Hex("#276782").Alpha(0x80/255.0), wui.Hex("#276782").Alpha(0x90/255.0)
		ct.Occurrence = wui.Hex("#276782").Alpha(0x50 / 255.0)
		ct.IndentGuide, ct.ActiveIndentGuide = wui.Hex("#838485").Alpha(0x4d/255.0), wui.Hex("#838485")
		ct.WrapGuide = wui.Hex("#848484")
		ct.Whitespace = wui.Hex("#8c8c8c").Alpha(0x4d / 255.0)
		ct.Error, ct.Warning, ct.Info, ct.Hint = wui.Hex("#f14c4c"), wui.Hex("#cca700"), wui.Hex("#3794ff"), wui.Hex("#eeeeee").Alpha(0.7)
		ct.Added, ct.Deleted, ct.Modified = wui.Hex("#72c892"), wui.Hex("#f28772"), wui.Hex("#3994bc")
		for k, hex := range map[editor.TokenKind]string{
			editor.TokenText: "#bbbebf", editor.TokenKeyword: "#ff7b72", editor.TokenOperator: "#ff7b72", editor.TokenPunctuation: "#bbbebf",
			editor.TokenType: "#ffa657", editor.TokenFunction: "#d2a8ff", editor.TokenMeta: "#d2a8ff", editor.TokenVariable: "#c9d1d9",
			editor.TokenProperty: "#79c0ff", editor.TokenConstant: "#79c0ff", editor.TokenNumber: "#79c0ff", editor.TokenAttribute: "#79c0ff",
			editor.TokenString: "#a5d6ff", editor.TokenEscape: "#ff7b72", editor.TokenRegexp: "#a5d6ff", editor.TokenTag: "#7ee787",
			editor.TokenComment: "#8b949e", editor.TokenHeading: "#79c0ff", editor.TokenEmphasis: "#c9d1d9", editor.TokenStrong: "#c9d1d9",
			editor.TokenLink: "#a5d6ff", editor.TokenCode: "#79c0ff", editor.TokenInserted: "#7ee787", editor.TokenDeleted: "#ffa198",
			editor.TokenInvalid: "#ffa198",
		} {
			tok(k, hex)
		}
		return
	}
	ct.Selection, ct.InactiveSelection = wui.Hex("#0069cc").Alpha(0x40/255.0), wui.Hex("#0069cc").Alpha(0x1a/255.0)
	ct.ActiveLine = wui.Hex("#eaeaea").Alpha(0x40 / 255.0)
	ct.MatchingBracket = wui.Hex("#0069cc").Alpha(0x40 / 255.0)
	ct.Match, ct.CurrentMatch = wui.Hex("#0069cc").Alpha(0x1a/255.0), wui.Hex("#0069cc").Alpha(0x40/255.0)
	ct.Occurrence = wui.Hex("#0069cc").Alpha(0x26 / 255.0)
	ct.IndentGuide, ct.ActiveIndentGuide = wui.Hex("#f7f7f7").Alpha(0x40/255.0), wui.Hex("#eeeeee")
	ct.WrapGuide = wui.Hex("#d3d3d3")
	ct.Whitespace = wui.Hex("#606060").Alpha(0x40 / 255.0)
	ct.Error, ct.Warning, ct.Info, ct.Hint = wui.Hex("#e51400"), wui.Hex("#bf8803"), wui.Hex("#1a85ff"), wui.Hex("#6c6c6c")
	ct.Added, ct.Deleted, ct.Modified = wui.Hex("#587c0c"), wui.Hex("#ad0707"), wui.Hex("#0069cc")
	for k, hex := range map[editor.TokenKind]string{
		editor.TokenText: "#202020", editor.TokenKeyword: "#cf222e", editor.TokenOperator: "#cf222e", editor.TokenPunctuation: "#202020",
		editor.TokenType: "#953800", editor.TokenFunction: "#8250df", editor.TokenMeta: "#8250df", editor.TokenVariable: "#1f2328",
		editor.TokenProperty: "#0550ae", editor.TokenConstant: "#0550ae", editor.TokenNumber: "#0550ae", editor.TokenAttribute: "#0550ae",
		editor.TokenString: "#0a3069", editor.TokenEscape: "#cf222e", editor.TokenRegexp: "#0a3069", editor.TokenTag: "#116329",
		editor.TokenComment: "#6e7781", editor.TokenHeading: "#0550ae", editor.TokenEmphasis: "#1f2328", editor.TokenStrong: "#1f2328",
		editor.TokenLink: "#0a3069", editor.TokenCode: "#0550ae", editor.TokenInserted: "#116329", editor.TokenDeleted: "#82071e",
		editor.TokenInvalid: "#82071e",
	} {
		tok(k, hex)
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

// sidePadX 是侧栏列表左右留的：Grove 内缩 4（圆角的选中块不贴边），VS Code 风格的行满宽、不留。
func sidePadX() float32 { return lookSize(4, 0) }

// sideRowRadius 是侧栏列表选中块的圆角：Grove 5，VS Code 风格方的（mygo 的实心选中块看它；淡底的由 rowAt 自己画方的）。
func sideRowRadius() float32 { return lookSize(rowRadius, 0) }
