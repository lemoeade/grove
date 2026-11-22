package ui

import (
	"fmt"
	"sync/atomic"

	"github.com/egoist/mygo/plugins/glass"
	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/workspace"
)

// 磨砂：照 VS Code 的 workbench.modernUIFrostedGlass。浮在上面的面板（Grove 画的菜单、命令面板、对话框、
// 编辑器的悬停提示和补全列表）不画纯色的底：先把窗口画在它底下的东西糊开 12（backdrop-filter: blur(12px)），
// 再盖一层它原来那个底色的一部分（color-mix(in srgb, 底色 65%, transparent)；多少是设置里的“浮层不透明度”，frostAlpha）。糊的是窗口自己画的内容，不是窗口后面的桌面。
// 系统开了“减少透明度”或“增强对比度”、或者不透明度设成 100 时退回纯色（canFrost）。
const frostBlur = 12 // 糊开多少（mygo glass.Blur 的 Radius，同 CSS blur() 的标准差）

// frostPct 是设置里的浮层不透明度（百分数）：取颜色的函数拿不到 App，每帧开头 setLook 记下。
var frostPct atomic.Int32

func init() { frostPct.Store(workspace.DefaultFrost) }

// frostAlpha 是浮层底色盖多少（0.5–1）。
func frostAlpha() float32 { return float32(frostPct.Load()) / 100 }

// frostMaterial 是磨砂的那层：glass.Blur 糊开底下，上面按面板的圆角盖 tint。做成一个 Material，
// 底色就不用另起一个铺满的孩子（铺满的孩子会占 Gap、被内边距挤）。
type frostMaterial struct{ tint wui.Color }

func (f frostMaterial) PaintMaterial(p *wui.Painter, box wui.Rect, radii [4]float32) {
	glass.Blur{Radius: frostBlur}.PaintMaterial(p, box, radii)
	p.FillRadii(box, radii, f.tint)
}

// String 是检查器里显示的。
func (f frostMaterial) String() string {
	return fmt.Sprintf("frost(blur %d, %v)", frostBlur, f.tint)
}

// canFrost 报这一帧能不能磨砂：系统的“减少透明度”“增强对比度”都没开，不透明度不是 100。
func canFrost(c *wui.Context) bool {
	p := c.Preferences()
	return !p.ReduceTransparency && !p.HighContrast && frostPct.Load() < 100
}

// frostFill 给浮层 el 上底：能磨砂时是糊开的底下盖 bg 的 frostAlpha()，不能时是纯色 bg。圆角照 el 自己的 Radius。
func frostFill(c *wui.Context, el wui.Element, bg wui.Color) wui.Element {
	if !canFrost(c) {
		return el.Background(bg)
	}
	return el.Material(frostMaterial{bg.Alpha(frostAlpha())})
}

// VS Code 2026 主题里浮层的底：menu.background（菜单）、quickInput.background（命令面板）、editorWidget / editorHoverWidget /
// editorSuggestWidget.background（对话框、悬停提示、补全）都是 #FAFAFD，深色 #202122。Grove 风格还是原来的底色。
func vsOverlayBg(t *wui.Theme) wui.Color { return pick(t, wui.Hex("#fafafd"), wui.Hex("#202122")) }
func vsQuickBg(t *wui.Theme) wui.Color   { return vsOverlayBg(t) }
func vsWidgetBg(t *wui.Theme) wui.Color  { return vsOverlayBg(t) }

// paletteBg、dialogBg、widgetBg 是命令面板、对话框、编辑器里浮层（悬停提示、补全）不磨砂时的底色。
func paletteBg(t *wui.Theme) wui.Color {
	if vscode() {
		return vsQuickBg(t)
	}
	return t.Background
}

func dialogBg(t *wui.Theme) wui.Color {
	if vscode() {
		return vsWidgetBg(t)
	}
	return t.Background
}

func widgetBg(t *wui.Theme) wui.Color {
	if vscode() {
		return vsWidgetBg(t)
	}
	return raisedBg(t)
}
