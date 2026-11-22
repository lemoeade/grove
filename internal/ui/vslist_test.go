package ui

import (
	"image/color"
	"testing"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/testutil"
	"github.com/lemoeade/grove/internal/workspace"
)

// VS Code 风格的侧栏列表：行满宽、方的；非活动选中是 #E4E6F1（list.inactiveSelectionBackground），
// 活动选中是强调色的淡底加一道 1 pt 强调色的框（list.focusOutline），没有 Grove 那道 3 pt 的竖条。
func TestVSListSelection(t *testing.T) {
	r0 := testutil.NewRepo(t, "vslist")
	for _, name := range []string{"go.mod", "main.go", "zz.txt"} {
		r0.Write(name, "x\n")
	}
	r0.Commit("初始提交")
	a, tt, wait := newUI(t, testutil.Workspace(t, r0.Root), "")
	const scale = 2
	tt.SetScale(scale)
	a.St.BottomBar, a.St.RightBar = false, false
	a.St.Settings.Look = ""
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	a.ShowSidebar(workspace.ModeFiles)
	a.EnsureFiles(r)
	wait("文件树", func() bool { return idle(r)() && r.Files.Loaded && tt.HasText("zz.txt") })
	a.Reveal(r, "main.go")
	wait("选中 main.go", func() bool { row, ok := a.TreeRowAt(a.FileSel); return ok && row.Path == "main.go" })
	tt.Move(1000, 400)

	near := func(c color.RGBA, w wui.Color, tol int) bool {
		d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
		return d(c.R, w.R) <= tol && d(c.G, w.G) <= tol && d(c.B, w.B) <= tol
	}
	shot := func() (px func(x, y float32) color.RGBA, row wui.Rect) {
		t.Helper()
		tt.Frame()
		tt.Frame()
		row, ok := tt.Find("main.go")
		if !ok {
			t.Fatalf("找不到 main.go：%q", tt.Texts())
		}
		img := tt.Image()
		return func(x, y float32) color.RGBA { return img.RGBAAt(int(x*scale), int(y*scale)) }, row
	}

	a.FocusTree = false
	px, row := shot()
	inactive := wui.Hex("#e4e6f1")
	mid := row.Y + row.H/2
	// 满宽：行的左头（侧栏的边往里半个像素）、右头往里 12 pt（滚动条之外）都是选中的底；方的：左上角那个像素也是
	for _, p := range []struct {
		name string
		x, y float32
	}{{"左头", row.X + 0.5, mid}, {"右头", row.X + row.W - 12, mid}, {"左上角", row.X + 0.5, row.Y + 0.5}} {
		if c := px(p.x, p.y); !near(c, inactive, 3) {
			t.Errorf("非活动选中的%s是 %v，应是 %v", p.name, c, inactive)
		}
	}

	a.FocusTree = true
	px, row = shot()
	accent := wui.Hex("#005fb8")
	if c := px(row.X+0.25, mid); !near(c, accent, 6) {
		t.Errorf("活动选中左边的框是 %v，应是强调色 %v", c, accent)
	}
	if c := px(row.X+row.W/2, row.Y+0.25); !near(c, accent, 6) {
		t.Errorf("活动选中顶上的框是 %v，应是强调色 %v", c, accent)
	}
	if c := px(row.X+4, mid); near(c, accent, 24) {
		t.Errorf("活动选中不该有竖条：左头往里 4 pt 是 %v", c)
	}
}

// VS Code 风格的筛选命中是 list.highlightForeground 的蓝（浅色 #0066BF）、加粗，不垫底色。
func TestVSHitText(t *testing.T) {
	repo := testutil.NewRepo(t, "proj")
	repo.Write("notes/docs.txt", "n\n")
	repo.Write("main.go", "package main\n")
	repo.Commit("init")
	a, tt, wait := filesUI(t, testutil.Workspace(t, repo.Root), "main.go")
	a.St.Settings.Look = ""
	tt.Move(1000, 400)
	const scale = 2
	tt.SetScale(scale)
	a.SetFilter("docs")
	wait("筛选", func() bool { return tt.HasText("docs.txt") && !tt.HasText("main.go") })
	tt.Frame()
	tt.Frame()
	row, ok := tt.Find("docs.txt")
	if !ok {
		t.Fatalf("找不到 docs.txt：%q", tt.Texts())
	}
	img := tt.Image()
	blue := color.RGBA{0x00, 0x66, 0xbf, 255}
	n := 0
	for y := int(row.Y * scale); y < int((row.Y+row.H)*scale); y++ {
		for x := int(row.X * scale); x < int((row.X+row.W)*scale); x++ {
			if inkDist(img.RGBAAt(x, y), blue) <= 30 {
				n++
			}
		}
	}
	if n < 40 {
		t.Errorf("命中的字该是蓝的：docs.txt 那行只有 %d 个蓝像素", n)
	}
}
