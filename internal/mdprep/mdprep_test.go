package mdprep

import (
	"slices"
	"strings"
	"testing"
)

func lineCount(s string) int { return strings.Count(s, "\n") }

// front matter 换成围栏代码块，行数不变；读出顶层键值、列表连起来。
func TestFrontMatter(t *testing.T) {
	src := "---\ntitle: \"Grove 指南\"\ntags: [a, b]\nauthors:\n  - 甲\n  - 乙\n---\n\n# 正文\n"
	d := Prepare(src)
	if len(d.Regions) != 1 || d.Regions[0].Kind != FrontMatter || d.Regions[0].Format != "yaml" {
		t.Fatalf("regions = %+v", d.Regions)
	}
	if lineCount(d.Src) != lineCount(src) {
		t.Fatalf("行数变了：\n%s", d.Src)
	}
	want := []KV{{"title", "Grove 指南"}, {"tags", "a, b"}, {"authors", "甲, 乙"}}
	if !slices.Equal(d.Regions[0].Meta, want) {
		t.Fatalf("meta = %+v", d.Regions[0].Meta)
	}
	if !strings.HasPrefix(d.Src, "```"+Lang+" 0\n") || !strings.Contains(d.Src, "\n```\n\n# 正文") {
		t.Fatalf("src = %q", d.Src)
	}
	r := d.Region(Lang+" 0", d.Regions[0].Code)
	if r == nil || d.Region(Lang+" 0", "别的") != nil || d.Region("go", "") != nil {
		t.Fatal("Region 认错")
	}

	toml := Prepare("+++\ntitle = 'x'\n[params]\ndraft = true\n+++\n正文\n")
	if len(toml.Regions) != 1 || !slices.Equal(toml.Regions[0].Meta, []KV{{"title", "x"}, {"params.draft", "true"}}) {
		t.Fatalf("toml = %+v", toml.Regions)
	}
	// 分隔线加 setext 标题不是 front matter
	for _, s := range []string{"---\n\n标题\n---\n", "---\n---\n", "正文\n---\na: b\n---\n"} {
		if d := Prepare(s); len(d.Regions) != 0 || d.Src != s {
			t.Fatalf("%q 被当成 front matter：%+v", s, d.Regions)
		}
	}
}

// 跨块的 <details>、<div align>：开头、结尾那一行换成围栏，中间原样；嵌套的同名标签配平；代码块里的不算。
func TestContainers(t *testing.T) {
	src := "# 标题\n\n<details>\n<summary>更多</summary>\n\n正文 **粗**\n\n<details><summary>里面</summary>\n\n内层\n\n</details>\n\n```\n</details>\n```\n\n</details>\n\n<div align=\"center\">\n\n![logo](logo.png)\n\n</div>\n尾\n"
	d := Prepare(src)
	if lineCount(d.Src) != lineCount(src) {
		t.Fatalf("行数变了：\n%s", d.Src)
	}
	if len(d.Regions) != 2 {
		t.Fatalf("regions = %+v", d.Regions)
	}
	det, div := d.Regions[0], d.Regions[1]
	if det.Tag != "details" || det.Line != 2 || det.Open != "<details>\n<summary>更多</summary>\n" ||
		!strings.Contains(det.Body, "<details><summary>里面</summary>") || !strings.HasSuffix(det.Body, "```\n</details>\n```\n\n") {
		t.Fatalf("details = %+v", det)
	}
	if div.Tag != "div" || div.Body != "\n![logo](logo.png)\n\n" {
		t.Fatalf("div = %+v", div)
	}
	// 围栏比里面的 ``` 长
	if !strings.Contains(d.Src, "````"+Lang+" 0\n<summary>") {
		t.Fatalf("src = %q", d.Src)
	}
	// 内层单独整理
	inner := Prepare(det.Body)
	if len(inner.Regions) != 1 || inner.Regions[0].Body != "\n内层\n\n" {
		t.Fatalf("inner = %+v", inner.Regions)
	}

	// 不整理的：一个块里就配平了、闭标签后面还有别的字、没有闭标签、代码块里的、缩进的
	for _, s := range []string{
		"<details><summary>a</summary>b</details>\n\n正文\n",
		"<div>\n\n正文\n\n</div> 尾巴\n",
		"<details>\n\n正文\n",
		"```html\n<details>\n\n正文\n\n</details>\n```\n",
		"  <div>\n\n正文\n\n</div>\n",
		"<!--\n<div>\n\n-->\n正文\n\n</div>\n",
	} {
		if d := Prepare(s); len(d.Regions) != 0 || d.Src != s {
			t.Fatalf("%q 被整理了：%+v", s, d.Regions)
		}
	}
}

// 链接引用定义、脚注。
func TestDefsFootnotes(t *testing.T) {
	d := Prepare("见 [文档][doc]。\n\n[doc]: https://example.com \"标题\"\n  [x]: /y\n```\n[no]: z\n```\n")
	if d.Defs != "[doc]: https://example.com \"标题\"\n[x]: /y\n" || d.Footnotes {
		t.Fatalf("defs = %q footnotes = %v", d.Defs, d.Footnotes)
	}
	if d := Prepare("正文[^1]\n\n[^1]: 脚注\n"); !d.Footnotes || d.Defs != "" {
		t.Fatalf("footnotes = %v defs = %q", d.Footnotes, d.Defs)
	}
	plain := "# 普通\n\n正文\n"
	if d := Prepare(plain); d.Src != plain || len(d.Regions) != 0 {
		t.Fatal("没有要整理的也改了")
	}
}
