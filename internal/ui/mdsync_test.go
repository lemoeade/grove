package ui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/testutil"
)

// lineAt 按位置比例换算行号：一块的顶到下一块的顶摊这块的第一行到下一块的第一行（就地预览一行一个屏幕行），
// 下一块没排出来时只摊到这块的底，过了最后一块算最后一块的最后一行。
func TestMdLineAt(t *testing.T) {
	v := &mdView{blocks: []mdBlock{
		{line: 0, next: 2, r: wui.Rect{Y: 100, H: 40}},
		{line: 2, next: 10, r: wui.Rect{Y: 160, H: 80}},
		{line: 10, next: 12, r: wui.Rect{H: 0}}, // 没排出来的不算
	}}
	for _, c := range []struct {
		y    float32
		want float64
	}{{50, 0}, {100, 0}, {130, 1}, {145, 1.5}, {160, 2}, {200, 6}, {300, 9}} {
		if got, ok := v.lineAt(c.y); !ok || got != c.want {
			t.Errorf("lineAt(%v) = %v %v，要 %v", c.y, got, ok, c.want)
		}
	}
	if _, ok := (&mdView{}).lineAt(0); ok {
		t.Error("没有块时不该有行")
	}
}

// 并排预览（照 VS Code 的 markdown 预览）：双击预览回到源码那一行；在预览上滚，编辑器跟着滚；编辑器滚到半行，
// 预览也滚到对应的地方；标题左边悬停出 🔗，点了复制链接；换到别的 Markdown 标签预览跟过去，锁定了就不跟。
func TestMarkdownPreviewSync(t *testing.T) {
	root, ws := testutil.TempRepo(t)
	var b strings.Builder
	b.WriteString("# 开头 :tada:\n\n")
	for i := range 160 {
		b.WriteString("第 " + itoa(i+1) + " 段。\n\n")
	}
	writeDoc(t, root, "guide.md", b.String())
	writeDoc(t, root, "other.md", "# 另一篇\n\n正文\n")
	var copied string
	keep := app.CopyText
	app.CopyText = func(s string) { copied = s }
	t.Cleanup(func() { app.CopyText = keep })

	a, tt, wait := newUI(t, ws, "")
	a.OpenFile(a.Repos[0], "guide.md", true)
	a.FocusDoc = true
	wait("打开 guide.md", func() bool { return tt.Focused("编辑 guide.md") })
	guide := a.CurrentDoc()
	mustClick(t, tt, "在旁边打开预览（左右并排，跟着编辑器滚）（⌥⇧⌘V）") // 标签条上的按钮
	var pv *docs.Doc
	wait("并排预览", func() bool {
		pv = a.SidePreviewOf(guide)
		return pv != nil && tt.HasText("第 3 段。")
	})
	if !tt.HasText("开头 🎉") {
		t.Fatal(":tada: 没换成 emoji")
	}

	// 双击“第 3 段。”：键盘回到源文件，光标在第 7 行（从 0 数第 6 行）
	rc, ok := tt.Find("第 3 段。")
	if !ok || rc.X < 600 {
		t.Fatalf("预览里没有第 3 段：%+v", rc)
	}
	tt.ClickAt(rc.X+4, rc.Y+rc.H/2)
	tt.ClickAt(rc.X+4, rc.Y+rc.H/2)
	wait("回到源码那一行", func() bool {
		p, ok := guide.Cursor()
		return ok && p.Line == 6 && a.CurrentDoc() == guide
	})

	// 标题左边的 🔗：指针移到标题上才出来，点了复制 guide.md#slug
	h, ok := tt.Find("开头 🎉")
	if !ok {
		t.Fatal("没有标题")
	}
	if _, ok := tt.Find("复制标题链接"); ok {
		t.Fatal("没悬停就有 🔗")
	}
	tt.Move(h.X+8, h.Y+h.H/2)
	wait("悬停出 🔗", func() bool { _, ok := tt.Find("复制标题链接"); return ok })
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		writeShot(t, tt, dir, "md-sync.png")
	}
	mustClick(t, tt, "复制标题链接")
	if want := "guide.md#" + markdown.Slug("开头 🎉"); copied != want {
		t.Fatalf("复制了 %q，要 %q", copied, want)
	}

	// 在预览上往下滚：编辑器跟着滚
	tt.Move(rc.X+4, rc.Y+rc.H/2)
	for range 6 {
		tt.Scroll(rc.X+4, rc.Y+rc.H/2, 0, 200)
		tt.Frame()
	}
	wait("编辑器跟着预览滚", func() bool { L, ok := guide.TopLine(); return ok && L > 20 })

	// 指针挪到编辑器上，编辑器滚到第 200.5 行：预览顶上对着的也是那儿
	tt.Move(40, 300)
	tt.Advance(mdSyncQuiet + 50*time.Millisecond) // 刚才预览拖着编辑器走：安静期里编辑器的变化不往回传
	tt.Frame()
	v := mdViewOf(pv)
	guide.ScrollToFrac(200.5)
	wait("预览跟着编辑器滚", func() bool {
		return v.pvSeen && abs64(v.pvTop-200.5) < 1.5
	})

	// 换到 other.md：预览跟过去；锁定以后换回 guide.md 不跟
	a.FocusGroup(a.GroupOf(guide)) // 点过预览：键盘在预览那组，新文件要开在源文件那一组
	other := a.OpenFile(a.Repos[0], "other.md", true)
	wait("预览跟到 other.md", func() bool { return pv.MDSource == other && tt.HasText("另一篇") })
	if pv.Title != "预览 other.md" {
		t.Fatalf("标题 %q", pv.Title)
	}
	a.Exec("view.markdownLockPreview")
	if !pv.MDLocked || pv.Title != "[预览] other.md" || !app.Find("view.markdownLockPreview").Checked(a) {
		t.Fatalf("没锁上：%v %q", pv.MDLocked, pv.Title)
	}
	a.OpenFile(a.Repos[0], "guide.md", true)
	tt.Frame()
	tt.Frame()
	if pv.MDSource != other || !tt.HasText("另一篇") {
		t.Fatal("锁定了还跟着换")
	}
}

// 源文件是预览标签（斜体、单击文件树开的）：被别的文件顶掉以后并排预览不留一个空的“源文件的标签关了”，
// 顶上来的是 Markdown 就跟过去；顶上来的不是 Markdown 时等下一个 Markdown 文件。Markdown 标签条上只有“在旁边打开预览”，没有眼睛。
func TestMarkdownPreviewOrphan(t *testing.T) {
	root, ws := testutil.TempRepo(t)
	writeDoc(t, root, "one.md", "# 第一篇\n")
	writeDoc(t, root, "two.md", "# 第二篇\n")
	writeDoc(t, root, "three.md", "# 第三篇\n")
	a, tt, wait := newUI(t, ws, "")
	one := a.OpenFile(a.Repos[0], "one.md", false)
	a.FocusDoc = true
	wait("打开 one.md", func() bool { return tt.Focused("编辑 one.md") })
	eye := app.Tip(i18n.T("Markdown 预览"), "view.markdownPreview")
	if _, ok := tt.Find(eye); ok {
		t.Fatal("Markdown 标签条上还有眼睛")
	}
	mustClick(t, tt, "在旁边打开预览（左右并排，跟着编辑器滚）（⌥⇧⌘V）")
	var pv *docs.Doc
	wait("并排预览", func() bool { pv = a.SidePreviewOf(one); return pv != nil && tt.HasText("第一篇") })

	a.FocusGroup(a.GroupOf(one))
	two := a.OpenFile(a.Repos[0], "two.md", false) // 顶掉 one.md
	wait("跟到 two.md", func() bool {
		return pv.MDSource == two && tt.HasText("第二篇") && !tt.HasText("源文件的标签关了")
	})

	a.OpenFile(a.Repos[0], "a.txt", false) // 顶掉 two.md，不是 Markdown
	wait("等下一个 Markdown", func() bool {
		return tt.HasText("源文件的标签关了") && tt.HasText("打开一个 Markdown 文件，预览就跟过去")
	})
	three := a.OpenFile(a.Repos[0], "three.md", true)
	wait("跟到 three.md", func() bool { return pv.MDSource == three && tt.HasText("第三篇") })

	a.Exec("view.markdownPreview") // 就地预览里留一个回源码的按钮
	wait("就地预览", func() bool { return three.Preview })
	if _, ok := tt.Find(app.Tip(i18n.T("显示源码"), "view.markdownPreview")); !ok {
		t.Fatal("就地预览里没有回源码的按钮")
	}
}

// 编辑器像触控板那样一甩、越滚越慢时，并排预览也顺顺地跟着慢下来，不一帧快一帧慢：一块的顶到下一块的顶
// 按编辑器的屏幕行摊这块的第一行到下一块的第一行（以前按源码行数摊，折成三行的一段和它后面的空行各算一行，
// 预览每帧 25、60、24、57 地抖）。
func TestMarkdownPreviewSmooth(t *testing.T) {
	// 160 段不分段；400 段过了 mdChunkMin，分段放进虚拟列表（跟着滚曾经每步先 ScrollTo 到段首，一帧段首一帧原处地闪）
	for _, n := range []int{160, 400} {
		t.Run(itoa(n), func(t *testing.T) { testPreviewSmooth(t, n) })
	}
}

func testPreviewSmooth(t *testing.T, n int) {
	root, ws := testutil.TempRepo(t)
	var b strings.Builder
	b.WriteString("# 开头\n\n")
	for i := range n {
		if i%10 == 0 {
			b.WriteString("## 节 " + itoa(i) + "\n\n")
		}
		b.WriteString("第 " + itoa(i+1) + " 段，这一段写长一点，长到在预览里折成两三行，好看出块里的位置：一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十。\n\n")
	}
	writeDoc(t, root, "guide.md", b.String())
	a, tt, wait := newUI(t, ws, "")
	a.OpenFile(a.Repos[0], "guide.md", true)
	a.FocusDoc = true
	wait("打开 guide.md", func() bool { return tt.Focused("编辑 guide.md") })
	guide := a.CurrentDoc()
	a.Exec("view.markdownSidePreview")
	var pv *docs.Doc
	wait("并排预览", func() bool { pv = a.SidePreviewOf(guide); return pv != nil && tt.HasText("开头") })
	v := mdViewOf(pv)
	if chunked := n > 200; (v.chunks != nil) != chunked {
		t.Fatalf("%d 段：分段 = %v，应为 %v", n, v.chunks != nil, chunked)
	}
	tt.Move(500, 300) // 指针在编辑器上
	tt.Frame()
	// 每一帧（不只是 Frame 跑完的最后一帧）同一块都只往一个方向走：分段时曾经一帧滚到段首、下一帧回来
	var last []mdBlock
	sign := float32(0)
	back := ""
	mdPassHook = func(v *mdView) {
		if v.env.d != pv {
			return
		}
	match:
		for _, b := range v.blocks {
			for _, o := range last {
				if o.line == b.line && o.r.H > 0 && b.r.H > 0 {
					if dy := b.r.Y - o.r.Y; dy*sign < -1 && back == "" {
						back = fmt.Sprintf("第 %d 行那块往回跳了 %.0f", b.line, dy)
					}
					break match
				}
			}
		}
		last = append(last[:0], v.blocks...)
	}
	defer func() { mdPassHook = nil }()
	same := func(b mdBlock) (float32, bool) { // 上一帧也排着的同一块挪了多少
		for _, n := range v.blocks {
			if n.line == b.line && n.r.H > 0 && b.r.H > 0 {
				return b.r.Y - n.r.Y, true
			}
		}
		return 0, false
	}
	for dir := range 2 {
		var steps []float32
		for i := range 30 {
			d := 45 - float32(i)*1.5
			if dir == 1 {
				d = -d
			}
			sign = -1 // 往下滚（d > 0）时块往上走
			if d < 0 {
				sign = 1
			}
			old := append([]mdBlock(nil), v.blocks...)
			tt.Scroll(500, 300, 0, d)
			tt.Frame()
			for _, b := range old {
				if dy, ok := same(b); ok {
					steps = append(steps, dy)
					break
				}
			}
		}
		for i := 3; i < len(steps); i++ { // 头几帧是起步
			p, q := abs64(float64(steps[i-1])), abs64(float64(steps[i]))
			if abs64(p-q) > 0.25*max(p, q)+3 {
				t.Errorf("方向 %d 第 %d 帧：预览一帧走 %.0f、下一帧 %.0f，不顺：%v", dir, i, steps[i-1], steps[i], steps)
				break
			}
		}
		if len(steps) < 20 {
			t.Fatalf("方向 %d：预览没跟着动：%v", dir, steps)
		}
		if back != "" {
			t.Fatalf("方向 %d：%s（跟着编辑器滚时某一帧闪回去了）", dir, back)
		}
		last = last[:0]
	}
}

// 分段的预览里有一大块 <style>（排出来高 0）：编辑器滚过它那几十行时，预览停在下一块的顶上，不一帧滚到段首
// 一帧回来地抖（laidOut 曾经只认高度大于 0 的块，<style> 那块永远算“没排着”，每步都先 ScrollTo 到段首）。
func TestMarkdownPreviewStyleBlock(t *testing.T) {
	root, ws := testutil.TempRepo(t)
	var b strings.Builder
	b.WriteString("# 开头\n\n")
	para := func(i int) {
		if i%10 == 0 {
			b.WriteString("## 节 " + itoa(i) + "\n\n")
		}
		b.WriteString("第 " + itoa(i+1) + " 段，写长一点：一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十。\n\n")
	}
	for i := range 300 {
		para(i)
	}
	b.WriteString("## 时序\n\n<style>\n")
	style := strings.Count(b.String(), "\n")
	for i := range 80 {
		b.WriteString(".c" + itoa(i) + "{color:#333;margin:2px}\n")
	}
	b.WriteString("</style>\n\n<div style=\"padding:16px;border:1px solid #ccc\">时序图</div>\n\n")
	for i := 300; i < 500; i++ {
		para(i)
	}
	writeDoc(t, root, "guide.md", b.String())
	a, tt, wait := newUI(t, ws, "")
	guide := a.OpenFile(a.Repos[0], "guide.md", true)
	a.FocusDoc = true
	wait("打开 guide.md", func() bool { return tt.Focused("编辑 guide.md") })
	a.Exec("view.markdownSidePreview")
	var pv *docs.Doc
	wait("并排预览", func() bool { pv = a.SidePreviewOf(guide); return pv != nil && tt.HasText("开头") })
	if mdViewOf(pv).chunks == nil {
		t.Fatal("没分段")
	}
	tt.Move(500, 300) // 指针在编辑器上
	guide.ScrollToFrac(float64(style - 6))
	wait("预览跟到时序那一节", func() bool { tt.Frame(); return tt.HasText("时序图") })
	var last []mdBlock
	back := ""
	mdPassHook = func(v *mdView) {
		if v.env.d != pv {
			return
		}
	match:
		for _, b := range v.blocks {
			for _, o := range last {
				if o.line == b.line && o.r.H > 0 && b.r.H > 0 {
					if dy := b.r.Y - o.r.Y; dy > 1 && back == "" {
						back = fmt.Sprintf("第 %d 行那块往回跳了 %.0f", b.line, dy)
					}
					break match
				}
			}
		}
		last = append(last[:0], v.blocks...)
	}
	defer func() { mdPassHook = nil }()
	for range 60 {
		tt.Scroll(500, 300, 0, 30)
		tt.Frame()
	}
	if L, _ := guide.TopLine(); L < float64(style+80) {
		t.Fatalf("编辑器没滚过 <style>：顶上是第 %.0f 行", L)
	}
	if back != "" {
		t.Fatalf("编辑器滚过 <style> 时预览抖了：%s", back)
	}
}

// 并排预览拖进源文件那一组（源文件的标签看不见了）：在预览上滚，滚到哪停在哪，不被源文件编辑器停在最后一帧的位置拉回去。
func TestMarkdownPreviewSameGroup(t *testing.T) {
	root, ws := testutil.TempRepo(t)
	var b strings.Builder
	b.WriteString("# 开头\n\n")
	for i := range 160 {
		b.WriteString("第 " + itoa(i+1) + " 段。\n\n")
	}
	writeDoc(t, root, "guide.md", b.String())
	a, tt, wait := newUI(t, ws, "")
	guide := a.OpenFile(a.Repos[0], "guide.md", true)
	a.FocusDoc = true
	wait("打开 guide.md", func() bool { return tt.Focused("编辑 guide.md") })
	a.Exec("view.markdownSidePreview")
	var pv *docs.Doc
	wait("并排预览", func() bool { pv = a.SidePreviewOf(guide); return pv != nil && tt.HasText("第 3 段。") })
	a.MoveDocTo(pv, a.GroupOf(guide), -1)
	wait("预览搬进源文件那一组", func() bool {
		return a.GroupOf(pv) == a.GroupOf(guide) && a.CurrentDoc() == pv && !a.EditorShown(guide) && tt.HasText("第 3 段。")
	})
	rc, ok := tt.Find("第 3 段。")
	if !ok {
		t.Fatal("预览里没有第 3 段")
	}
	tt.Move(rc.X+4, rc.Y+rc.H/2)
	tt.Frame()
	tt.Move(rc.X+6, rc.Y+rc.H/2)
	tt.Frame()
	for range 8 { // 一格一格地滚，每格之间停一会儿（下一帧在安静期以后才来，源文件的编辑器停着的位置才会拉回来）
		tt.Scroll(rc.X+6, rc.Y+rc.H/2, 0, 200)
		tt.Advance(mdSyncQuiet + 50*time.Millisecond)
		tt.Frame()
	}
	v := mdViewOf(pv)
	wait("预览滚下去", func() bool { return v.pvSeen && v.pvTop > 30 })
	at := v.pvTop
	// 真窗口里滚完的最后一帧是预览把行推给源文件编辑器（srcTop = 那一行），下一帧要等指针动了、光标闪了才来，
	// 常常过了安静期；看不见的编辑器 TopLine 还是最后画的那一帧的。Tester 每次 Frame 都紧跟着再跑一帧，
	// 这里直接摆出那一刻
	v.srcTop, v.srcSeen = at, true
	for range 3 {
		tt.Advance(mdSyncQuiet + 50*time.Millisecond)
		tt.Move(rc.X+8, rc.Y+rc.H/2)
		tt.Frame()
	}
	if v.pvTop < at-1 {
		t.Fatalf("预览被拉回去了：滚到 %v，过一会儿 %v", at, v.pvTop)
	}
	if r, ok := tt.Find("第 3 段。"); ok && r.Y > rc.Y-100 {
		t.Fatalf("预览回到了开头：第 3 段在 %v", r)
	}
}
