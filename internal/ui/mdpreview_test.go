package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/testutil"
)

// writeDoc 在临时仓库 root 下写一个文件（目录跟着建）。
func writeDoc(t *testing.T, root, rel, s string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stubFetch 把取网上图片换成桩：回一张不带字的 SVG（120×20），数一共取了几次。
func stubFetch(t *testing.T) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	keep := app.FetchImage
	app.FetchImage = func(string) ([]byte, error) {
		n.Add(1)
		return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="120" height="20"><rect width="120" height="20" fill="#4c1"/></svg>`), nil
	}
	app.ForgetImages()
	t.Cleanup(func() { app.FetchImage = keep; app.ForgetImages() })
	return &n
}

// 预览里的 front matter 是一张元数据卡片；<p align="center"> 里网上的徽章取来画出来（设置里关了就只有说明、不取）；
// <details> 默认收着，点 summary 展开；行内代码里像路径的能点开；链接的右键菜单能复制路径、在访达中显示。
func TestMarkdownRichPreview(t *testing.T) {
	root, ws := testutil.TempRepo(t)
	writeDoc(t, root, "docs/guide.md", "---\ntitle: 指南标题\nowner: 小组\n---\n\n"+
		"<p align=\"center\">\n  <img src=\"https://img.example.com/badge\" alt=\"构建徽章\" width=\"120\">\n</p>\n\n"+
		"<details>\n<summary>点开看细节</summary>\n\n藏起来的正文\n\n</details>\n\n"+
		"看 `docs/other.md` 这个文件，`不是路径` 不能点。\n\n去看[另一篇](other.md)吧\n")
	writeDoc(t, root, "docs/other.md", "# 另一篇\n")
	fetched := stubFetch(t)
	var copied string
	keepCopy := app.CopyText
	app.CopyText = func(s string) { copied = s }
	t.Cleanup(func() { app.CopyText = keepCopy })

	a, tt, wait := newUI(t, ws, "")
	var shown []string
	a.ShowPath = func(p string) error { shown = append(shown, p); return nil }
	a.OpenFile(a.Repos[0], "docs/guide.md", true)
	guide := a.CurrentDoc()
	wait("读好 guide.md", func() bool { return guide.Buf != nil })
	a.Exec("view.markdownPreview")
	wait("预览", func() bool { return guide.Preview && tt.HasText("点开看细节") })

	// front matter：卡片，不是一条线加一段文字
	for _, s := range []string{"元数据", "title", "指南标题", "owner", "小组"} {
		mustText(t, tt, s)
	}
	if tt.HasText("title: 指南标题") {
		t.Fatal("front matter 还是原文")
	}

	// 徽章：取了一次，按写的宽度画
	wait("徽章", func() bool {
		rc, ok := tt.Find("构建徽章")
		return ok && rc.W > 100 && rc.W < 140 && rc.H > 10
	})
	if fetched.Load() != 1 {
		t.Fatalf("取了 %d 次图片", fetched.Load())
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		writeShot(t, tt, dir, "md-rich.png")
	}

	// <details>：收着，点开
	if tt.HasText("藏起来的正文") {
		t.Fatal("details 默认该收着")
	}
	mustClick(t, tt, "点开看细节")
	wait("展开 details", func() bool { return tt.HasText("藏起来的正文") })

	// 链接的右键菜单
	mustRightClick(t, tt, "另一篇", "复制路径")
	if want := filepath.Join(root, "docs", "other.md"); copied != want {
		t.Fatalf("复制的路径 %q，要 %q", copied, want)
	}
	mustRightClick(t, tt, "另一篇", "在访达中显示")
	if len(shown) != 1 || shown[0] != filepath.Join(root, "docs", "other.md") {
		t.Fatalf("在访达中显示了 %q", shown)
	}

	// 关了网上的图片：只剩说明，不再取
	a.St.Settings.NoRemoteImages = true
	app.ForgetImages()
	wait("徽章变成说明", func() bool {
		rc, ok := tt.Find("构建徽章")
		return ok && tt.HasText("构建徽章") && rc.H > 0
	})
	if fetched.Load() != 1 {
		t.Fatalf("关了还在取图片：%d 次", fetched.Load())
	}

	// 行内代码里的路径：点了打开那个文件
	mustClick(t, tt, "docs/other.md")
	wait("打开 other.md", func() bool { d := a.CurrentDoc(); return d != guide && d.Rel == "docs/other.md" })
}

// 大文件的预览分段画在虚拟列表里：#标题 的链接、大纲（AnchorLine）都滚得到还没排出来的那一段；切回源码再切回来接着用分好的段。
func TestMarkdownBigPreview(t *testing.T) {
	root, ws := testutil.TempRepo(t)
	var b strings.Builder
	b.WriteString("# 大文档\n\n点这里[跳到结尾](#结尾一节)看看\n\n")
	line := 4
	mid := 0
	for i := range 900 {
		if i == 450 {
			b.WriteString("## 中间一节\n\n")
			mid = line
			line += 2
		}
		b.WriteString("第 " + itoa(i+1) + " 段，写长一点好让文件超过分段的门槛。\n\n")
		line += 2
	}
	b.WriteString("## 结尾一节\n\n")
	for i := range 60 { // 后面还得有一屏多，标题才滚得到最上面
		b.WriteString("结尾的第 " + itoa(i+1) + " 段。\n\n")
	}
	writeDoc(t, root, "big.md", b.String())

	a, tt, wait := newUI(t, ws, "")
	a.OpenFile(a.Repos[0], "big.md", true)
	d := a.CurrentDoc()
	wait("读好 big.md", func() bool { return d.Buf != nil })
	a.Exec("view.markdownPreview")
	wait("预览", func() bool { return tt.HasText("跳到结尾") })
	v := mdViewOf(d)
	if len(v.chunks) < 2 {
		t.Fatalf("大文件没分段：%d 段", len(v.chunks))
	}
	if rc, ok := tt.Find("结尾一节"); ok && rc.H > 0 {
		t.Fatal("结尾一开始就排出来了")
	}

	mustClick(t, tt, "跳到结尾")
	wait("滚到结尾", func() bool {
		rc, ok := tt.Find("结尾一节")
		return ok && rc.H > 0 && rc.Y < 160 && d.Anchor == ""
	})

	d.AnchorLine = mid + 1 // 大纲里点了“中间一节”
	wait("滚到中间", func() bool {
		rc, ok := tt.Find("中间一节")
		return ok && rc.H > 0 && rc.Y < 160 && d.AnchorLine == 0
	})

	chunks := v.chunks
	a.Exec("view.markdownPreview")
	wait("源码", func() bool { return !d.Preview })
	a.Exec("view.markdownPreview")
	wait("又是预览", func() bool { return d.Preview && tt.HasText("中间一节") })
	if mdViewOf(d) != v || len(v.chunks) != len(chunks) || &v.chunks[0] != &chunks[0] {
		t.Fatal("切回来重新分了段")
	}
}

// ⌥⇧⌘V 在旁边一组开并排预览，键盘留在源文件；打字停一下预览跟上；编辑器滚到哪预览跟到哪；
// 再按一次关掉；源文件关了预览跟着关。
func TestMarkdownSidePreview(t *testing.T) {
	root, ws := testutil.TempRepo(t)
	var b strings.Builder
	b.WriteString("# 开头\n\n")
	for i := range 120 {
		b.WriteString("第 " + itoa(i+1) + " 段。\n\n")
	}
	end := 2 + 240
	b.WriteString("## 结尾一节\n\n")
	for i := range 60 { // 后面还得有一屏多，标题才滚得到最上面
		b.WriteString("结尾的第 " + itoa(i+1) + " 段。\n\n")
	}
	writeDoc(t, root, "guide.md", b.String())

	a, tt, wait := newUI(t, ws, "")
	a.OpenFile(a.Repos[0], "guide.md", true)
	a.FocusDoc = true
	wait("打开 guide.md", func() bool { return tt.Focused("编辑 guide.md") })
	guide := a.CurrentDoc()

	tt.Key(wui.Alt|wui.Shift|wui.Cmd, wui.KeyV)
	var pv *docs.Doc
	wait("并排预览", func() bool {
		pv = a.SidePreviewOf(guide)
		return pv != nil && len(a.Groups()) == 2 && tt.HasText("开头")
	})
	if _, ok := tt.Find("预览 guide.md"); !ok {
		t.Fatal("没有预览区")
	}
	wait("键盘还在源文件", func() bool { return tt.Focused("编辑 guide.md") && a.CurrentDoc() == guide })
	if !app.Find("view.markdownSidePreview").Checked(a) {
		t.Fatal("菜单没打勾")
	}

	// 打字：停下来才重排
	tt.Type("# 新标题\n\n")
	tt.Frame()
	if tt.HasText("新标题") {
		t.Fatal("一打字就重排了")
	}
	tt.Advance(300 * time.Millisecond)
	wait("预览跟上", func() bool { return tt.HasText("新标题") })
	end += 2

	// 编辑器滚到结尾那一节：预览把它滚到最上面
	guide.Top = end
	wait("预览跟着滚", func() bool {
		rc, ok := tt.Find("结尾一节")
		return ok && rc.H > 0 && rc.Y < 200 && rc.X > 720
	})
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		writeShot(t, tt, dir, "md-side.png")
	}

	// 再按一次：关掉，键盘还在源文件
	tt.Key(wui.Alt|wui.Shift|wui.Cmd, wui.KeyV)
	wait("关掉并排预览", func() bool { return a.SidePreviewOf(guide) == nil && len(a.Groups()) == 1 })

	// 源文件关了：预览跟着关
	tt.Key(wui.Alt|wui.Shift|wui.Cmd, wui.KeyV)
	wait("又开了", func() bool { return a.SidePreviewOf(guide) != nil })
	a.RemoveDoc(guide)
	wait("跟着关", func() bool {
		return !slices.ContainsFunc(a.AllDocs(), func(d *docs.Doc) bool { return d.Kind == docs.MDPreview })
	})
}

// mdLinkPath：相对文件、带行号的、file: 的是本地文件；网址、#标题、别的协议不是。
func TestMarkdownLinkPath(t *testing.T) {
	d := docs.NewFile("/r", "docs/guide.md")
	for _, tc := range []struct {
		u, want string
		ok      bool
	}{
		{"other.md", "/r/docs/other.md", true},
		{"../a.txt:3", "/r/a.txt", true},
		{"../a.txt:3:5", "/r/a.txt", true},
		{"sub/x%20y.md#节", "/r/docs/sub/x y.md", true},
		{"file:///tmp/z.md", "/tmp/z.md", true},
		{"/abs/p.md?x=1", "/abs/p.md", true},
		{"https://example.com/a.md", "", false},
		{"mailto:x@y.z", "", false},
		{"vscode://open", "", false},
		{"#结尾", "", false},
		{"", "", false},
	} {
		got, ok := mdLinkPath(d, tc.u)
		if ok != tc.ok || ok && got != tc.want {
			t.Errorf("mdLinkPath(%q) = %q, %v；要 %q, %v", tc.u, got, ok, tc.want, tc.ok)
		}
	}
}
