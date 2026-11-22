package ui

import (
	"image/color"
	"testing"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/testutil"
	"github.com/lemoeade/grove/internal/workspace"
)

// VS Code 风格的侧栏列表：行满宽、方的；非活动选中是 #DADADA 60% 盖在侧栏的 #FAFAFD 上（list.inactiveSelectionBackground），
// 活动选中是黑 15% 的灰底，不描 VS Code 的 focusOutline 蓝框，也没有 Grove 那道 3 pt 的竖条：整行一个颜色。
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
	inactive := vsSideLight.Mix(wui.Hex("#dadada"), 0x99/255.0)
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
	act := vsSideLight.Mix(wui.RGBA(0, 0, 0, 1), 0x25/255.0)
	for _, p := range []struct {
		name string
		x, y float32
	}{{"左边", row.X + 0.25, mid}, {"左头往里 4 pt", row.X + 4, mid}, {"顶上", row.X + row.W/2, row.Y + 0.25}, {"右头", row.X + row.W - 12, mid}} {
		if c := px(p.x, p.y); !near(c, act, 4) {
			t.Errorf("活动选中的%s是 %v，应是一色的灰 %v（没有框、没有竖条）", p.name, c, act)
		}
	}
}

// VS Code 风格的筛选命中只加粗：不垫底色，也不换成蓝的（行里没有带颜色的像素），和不命中的同名行比，字的墨迹多（粗了）。
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
	ink := func() (dark, tinted int) {
		img := tt.Image()
		for y := int(row.Y * scale); y < int((row.Y+row.H)*scale); y++ {
			for x := int((row.X + 40) * scale); x < int((row.X+row.W)*scale); x++ { // 跳过左边的图标
				c := img.RGBAAt(x, y)
				hi, lo := max(c.R, c.G, c.B), min(c.R, c.G, c.B)
				if hi-lo > 40 {
					tinted++
				}
				if int(c.R)+int(c.G)+int(c.B) < 300 {
					dark++
				}
			}
		}
		return
	}
	hit, tinted := ink()
	if tinted > 0 {
		t.Errorf("命中的字该不换色：docs.txt 那行有 %d 个带颜色的像素", tinted)
	}
	a.SetFilter("")
	wait("不筛选", func() bool { return tt.HasText("main.go") })
	a.Reveal(a.Repos[0], "notes/docs.txt")
	tt.Frame()
	tt.Frame()
	if row, ok = tt.Find("docs.txt"); !ok {
		t.Fatalf("找不到 docs.txt：%q", tt.Texts())
	}
	if plain, _ := ink(); hit <= plain {
		t.Errorf("命中的字该加粗：命中时 %d 个深色像素，不命中 %d", hit, plain)
	}
}

// VS Code 风格的文件树：悬停块和选中一样满行、方的（左上角那个像素也垫着）；缩进参考线是 tree.indentGuidesStroke
// （选中行的上一级那条 #A9A9A9，别的 40%），压在选中的灰底上只会更深，不会把底切出一条条浅色的缝。
func TestVSTreeHoverGuides(t *testing.T) {
	r0 := testutil.NewRepo(t, "vsguide")
	for _, name := range []string{"dir/a.txt", "dir/sub/deep.txt", "dir/sub/other.txt", "dir/sub/third.txt"} {
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
	wait("文件树", func() bool { return idle(r)() && r.Files.Loaded && tt.HasText("dir") })
	a.Reveal(r, "dir/sub/deep.txt")
	wait("选中 deep.txt", func() bool {
		row, ok := a.TreeRowAt(a.FileSel)
		return ok && row.Path == "dir/sub/deep.txt" && tt.HasText("third.txt")
	})
	a.FocusTree = true
	find := func(name string) wui.Rect {
		t.Helper()
		b, ok := tt.Find(name)
		if !ok {
			t.Fatalf("找不到 %s：%q", name, tt.Texts())
		}
		return b
	}
	other := find("other.txt")
	tt.Move(other.X+other.W-30, other.Y+other.H/2)
	tt.Frame()
	tt.Frame()
	img := tt.Image()
	px := func(x, y float32) color.RGBA { return img.RGBAAt(int(x*scale), int(y*scale)) }
	near := func(c color.RGBA, w wui.Color, tol int) bool {
		d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
		return d(c.R, w.R) <= tol && d(c.G, w.G) <= tol && d(c.B, w.B) <= tol
	}
	black := wui.RGBA(0, 0, 0, 1)

	hov := vsSideLight.Mix(black, 0x14/255.0)
	if c := px(other.X+0.5, other.Y+0.5); !near(c, hov, 3) {
		t.Errorf("悬停块的左上角是 %v，应是满行、方的悬停色 %v", c, hov)
	}

	deep, third := find("deep.txt"), find("third.txt")
	act := vsSideLight.Mix(black, 0x25/255.0)
	mid := deep.Y + deep.H/2
	for k := 0; k < 3; k++ {
		c := px(deep.X+guideX(k), mid)
		if int(c.R)+int(c.G)+int(c.B) > int(act.R)+int(act.G)+int(act.B)+3 {
			t.Errorf("选中行第 %d 级参考线那里是 %v，比选中的底 %v 浅：底被切开了", k, c, act)
		}
	}
	guide := wui.Hex("#a9a9a9")
	tm := third.Y + third.H/2
	if c := px(third.X+guideX(2), tm); !near(c, guide, 6) {
		t.Errorf("选中行上一级的参考线是 %v，应是 %v", c, guide)
	}
	if c := px(third.X+guideX(0), tm); !near(c, vsSideLight.Mix(guide, 0.4), 6) {
		t.Errorf("别的参考线是 %v，应是 %v 的 40%%", c, guide)
	}
}
