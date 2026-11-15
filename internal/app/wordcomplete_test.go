package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/testutil"
)

// labels 是补全列表里每项的字。
func labels(items []editor.Completion) []string {
	out := make([]string, len(items))
	for i, c := range items {
		out[i] = c.Label
	}
	return out
}

// 按词补全（三个仓库）：这个文件里离光标近的在前、大纲里的带种类；开着的同一种语言的文件、
// 符号索引里同一仓库的也给，小字写在哪个文件；别的语言、别的仓库的不给；不给正好是前缀的；
// 打完一个整词时列表里拿掉它（回车好换行）；设置关了敲字不给、⌃Space 照样给；Markdown 只在 ⌃Space 时给。
func TestWordCompletion(t *testing.T) {
	alpha, beta, _, ws := testutil.ThreeRepos(t)
	alpha.Write("main.go", "package main\n\nfunc computeTotal() int { return 0 }\n\nfunc main() {\n\tvar counter int\n\tco\n}\n")
	alpha.Write("other.go", "package main\n\nvar colorTable = 1\n")
	alpha.Write("tool.py", "collections_py = 1\n")
	alpha.Write("lib/lib.go", "package lib\n\nfunc CopyAll() {}\n")
	alpha.Commit("go")
	beta.Write("x.go", "package beta\n\nfunc CommonBeta() {}\n")
	beta.Commit("go")
	a, wait := newTestApp(t, ws)
	t.Cleanup(a.Close)
	r := a.Repos[0]
	a.OpenPalette("#")
	wait("建索引", func() bool {
		a.UpdatePalette()
		return a.Repos[1].Files.Loaded && a.syms.files != nil && !a.defPending() && len(a.syms.files) >= 4
	})
	a.ClosePalette()
	for _, rel := range []string{"other.go", "tool.py"} {
		o := a.OpenFile(r, rel, true)
		wait("读好 "+rel, func() bool { return o.Buf != nil && !o.Loading })
	}
	d := a.OpenFile(r, "main.go", true)
	wait("读好", func() bool { return d.Buf != nil && !d.Loading })
	d.Outline() // 界面画第一帧时解析过；补全只看上次解析的，不自己解析
	off := d.Buf.Offset(editor.Pos{Line: 6, Col: 3})
	d.St.Select(off, off)
	req := func(prefix string, explicit bool) editor.CompletionRequest {
		return editor.CompletionRequest{Offset: d.St.Cursor(), Prefix: prefix, Version: d.Buf.Version(), Explicit: explicit}
	}

	items := a.wordItems(d, req("co", false))
	if got := labels(items); !slices.Equal(got, []string{"counter", "computeTotal", "colorTable", "CopyAll"}) {
		t.Fatalf("co：%q", got)
	}
	if items[1].Kind != editor.CompletionFunction || items[2].Detail != "other.go" ||
		items[3].Detail != "lib/lib.go" || items[3].Kind != editor.CompletionFunction {
		t.Errorf("种类、小字：%+v", items)
	}
	if got := labels(a.wordItems(d, req("counter", false))); slices.Contains(got, "counter") {
		t.Errorf("不该给正好是前缀的：%q", got)
	}
	if got := labels(a.wordItems(d, req("cT", false))); !slices.Contains(got, "computeTotal") || !slices.Contains(got, "colorTable") {
		t.Errorf("cT 按字的顺序匹配：%q", got)
	}

	// 打完 counter：列表里拿掉它
	a.Complete(d, req("co", false))
	if a.words.offer == nil || !slices.Contains(labels(a.words.offer.items), "counter") {
		t.Fatal("Complete 没记下给的词")
	}
	d.Buf.Edit(editor.Edit{Start: off, End: off, Text: "unter"})
	d.St.Select(off+5, off+5)
	a.CompleteTick(d)
	if o := a.words.offer; o == nil || slices.Contains(labels(o.items), "counter") || len(o.items) != 3 {
		t.Fatalf("打完 counter 应拿掉它：%+v", o)
	}
	// 换了行，那批作废
	d.Buf.Edit(editor.Edit{Start: off + 5, End: off + 5, Text: "\n"})
	d.St.Select(off+6, off+6)
	if a.CompleteTick(d); a.words.offer != nil {
		t.Error("换了行这批该作废")
	}

	// 设置关了：敲字不给，⌃Space 照样给
	a.St.Settings.NoWordComplete = true
	if got := a.wordItems(d, req("co", false)); got != nil {
		t.Errorf("关了还给：%q", labels(got))
	}
	if got := a.wordItems(d, req("co", true)); len(got) == 0 {
		t.Error("⌃Space 该照样给")
	}
	a.St.Settings.NoWordComplete = false

	// Markdown：敲字不给，⌃Space 给
	md := a.OpenFile(r, "notes.md", true)
	wait("读好 notes.md", func() bool { return md.Buf != nil && !md.Loading })
	md.Buf.Edit(editor.Edit{Start: md.Buf.Len(), End: md.Buf.Len(), Text: "drafting dr"})
	end := md.Buf.Len()
	md.St.Select(end, end)
	mreq := editor.CompletionRequest{Offset: end, Prefix: "dr", Version: md.Buf.Version()}
	if got := a.wordItems(md, mreq); got != nil {
		t.Errorf("Markdown 敲字不该给：%q", labels(got))
	}
	mreq.Explicit = true
	if got := labels(a.wordItems(md, mreq)); !slices.Equal(got, []string{"drafting"}) {
		t.Errorf("Markdown ⌃Space：%q", got)
	}
}
