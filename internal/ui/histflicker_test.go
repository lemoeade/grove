package ui

import (
	"fmt"
	"testing"
	"time"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/testutil"
)

// 历史往上滚时整个窗口不闪：滚上来的行（日期段头、展开的卡片）是列表在布局时才建的，换主题会把窗口根的底色
// 重设成那份主题的底色（卡片色），建完要换回窗口底，不然那一帧整个窗口底变成卡片色。
// Tester 一次输入会连推几帧直到不再要帧，只看最后一帧看不出来，所以每一帧建界面之前取上一帧的像素。
func TestHistScrollKeepsWindowBg(t *testing.T) {
	repo := testutil.NewRepo(t, "flicker")
	base := time.Date(2025, 3, 1, 9, 0, 0, 0, time.Local)
	for d := range 8 {
		for i := range 8 {
			msg := fmt.Sprintf("第 %d 天 %d", d, i)
			repo.Write("a.txt", msg+"\n")
			repo.CommitAt(msg, base.AddDate(0, 0, d).Add(time.Duration(i)*time.Minute))
		}
	}
	a, tt, _, r := histUI(t, repo, 64)
	a.St.Settings.Backdrop = "flat" // 纯色窗口底：右栏卡片右边那道缝里就是 windowBg
	tt.SetSize(1100, 500)
	tt.Frame()

	var want, got []uint8
	frames, bad := 0, 0
	beforeFrame = func() {
		if want == nil {
			return
		}
		img := tt.Image()
		W, H := img.Bounds().Dx(), img.Bounds().Dy()
		px := img.RGBAAt(W-3, H/2)
		frames++
		if c := []uint8{px.R, px.G, px.B}; string(c) != string(want) {
			bad++
			got = c
		}
	}
	t.Cleanup(func() { beforeFrame = nil })

	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		th := wui.LightTheme()
		if dark {
			th = wui.DarkTheme()
		}
		rows := app.HistRows(r)
		a.HistList.ScrollTo(len(rows)-1, wui.End)
		tt.Frame()
		tt.Frame()
		last, ok := tt.Find("第 0 天 0")
		if !ok {
			t.Fatalf("没滚到底；texts=%q", tt.Texts())
		}
		x, y := last.X+20, last.Y-40
		w := rgba(windowBg(th))
		want, frames, bad = []uint8{w.R, w.G, w.B}, 0, 0
		for step := range 10 {
			tt.Move(x, y)
			tt.Scroll(x, y, 0, -float32(60*(step+1))) // 越滚越快：上面的行在布局时才建
		}
		exp := want
		want = nil
		if _, ok := tt.Find("第 0 天 0"); ok {
			t.Fatalf("dark=%v 没往上滚", dark)
		}
		if bad > 0 {
			t.Errorf("dark=%v 往上滚的 %d 帧里有 %d 帧窗口底是 %v，应是 %v", dark, frames, bad, got, exp)
		}
	}
}
