package ui

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/gmd"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/mdprep"
)

// 这个文件是预览里 mygo 不画、或画得不像 GitHub 的几种块：front matter 卡片、跨块的 <details> / <div align>（mdprep 换成的围栏）、
// 只有图片的段落（徽章一排、截图；包在链接里的也是图片，按 HTML 写的宽高）、代码里的路径链接。

// codeBlock 是 CodeBlock 钩子：mdprep 的围栏画成卡片、折叠块，别的照 mygo 的。
func (e *mdEnv) codeBlock(c *wui.Context, b *gmd.CodeBlockView) {
	if b.Language == mdprep.Lang {
		if r := e.prep.Region(b.Info, b.Code()); r != nil {
			switch r.Kind {
			case mdprep.FrontMatter:
				mdFrontMatter(c, e, r)
			default:
				e.container(c, r)
			}
			return
		}
	}
	gmd.DefaultCodeBlock(c, b)
}

type mdOpenKey struct{}

// mdFrontMatter 是文件开头的元数据：一张淡底的卡片，标题行点了收起；读出了键值的一行一项，没读出的照原样等宽显示。
func mdFrontMatter(c *wui.Context, e *mdEnv, r *mdprep.Region) {
	th := e.th
	em := func(v float32) float32 { return mdEm(th, v) }
	box := wui.Column(c).AlignSelf(wui.Stretch).Background(th.CodeBlockBackground).Border(1, th.CodeBlockBorder).
		Radius(th.CodeBlockRadius).Clip().FontSize(em(0.86))
	box.Role(wui.RoleGroup).Label(i18n.T("元数据"))
	open := wui.Local(box, mdOpenKey{}, func() bool { return len(r.Meta) <= 12 })
	box.Children(func() {
		p := wui.CollapsibleBase(c, open)
		p.Trigger.AlignSelf(wui.Stretch).AlignItems(wui.Center).Gap(6).Padding(em(0.45), em(0.85)).Cursor(wui.CursorPointer).
			Label(i18n.T("元数据"))
		p.Trigger.Children(func() {
			ic := iconChevronRight
			if *open {
				ic = iconChevron
			}
			wui.Icon(c, ic).FontSize(iconInline).TextColor(th.Muted)
			wui.Text(c, i18n.T("元数据")).TextColor(th.Muted).FontWeight(600).Unselectable()
			n := strings.ToUpper(r.Format)
			if len(r.Meta) > 0 {
				n = i18n.Tn("%s · %d 项", n, len(r.Meta))
			}
			wui.Text(c, n).TextColor(th.Muted.Alpha(0.75)).Unselectable()
		})
		p.Panel(func() {
			body := wui.Column(c).AlignSelf(wui.Stretch).Padding(0, em(0.85), em(0.6)).Gap(em(0.3))
			body.Children(func() {
				if len(r.Meta) == 0 {
					wui.Text(c, strings.TrimRight(r.Code, "\n")).Font(th.CodeFont).TextColor(th.Text).Selectable()
					return
				}
				for _, kv := range r.Meta {
					wui.Row(c).AlignSelf(wui.Stretch).Gap(em(0.8)).AlignItems(wui.Start).Children(func() {
						wui.Text(c, kv.Key).Font(th.CodeFont).TextColor(th.Muted).Width(em(9)).Shrink(0).Ellipsis("…").SingleLine()
						wui.Text(c, kv.Value).TextColor(th.Text).Grow(1).MinWidth(0)
					})
				}
			})
		})
	})
}

var (
	reSummary = regexp.MustCompile(`(?is)<summary[^>]*>(.*?)</summary>`)
	reTag     = regexp.MustCompile(`(?s)<[^>]*>`)
	reAlign   = regexp.MustCompile(`(?i)\balign\s*=\s*["']?(center|right|left|middle)`)
	reOpenTag = regexp.MustCompile(`(?is)^\s*<[a-z]+[^>]*>`)
)

// container 是跨块的 HTML 容器：<details> 折叠块（标题取 <summary>，带 open 的一开始展开），<div align> 等按 align 对齐，
// <blockquote> 是引用；正文是嵌套的 Markdown（还能套 <details>）。
func (e *mdEnv) container(c *wui.Context, r *mdprep.Region) {
	th := e.th
	em := func(v float32) float32 { return mdEm(th, v) }
	open := r.Open
	rest := reOpenTag.ReplaceAllString(open, "") // 开标签后面同一个 HTML 块里的（<summary>、一行 <img>…）
	align := mdAlignOf(open)
	if r.Tag == "center" {
		align = wui.Center
	}
	if e.depth >= mdMaxDepth {
		wui.Text(c, open+r.Body+"</"+r.Tag+">").Font(th.CodeFont).FontSize(em(th.CodeSize)).TextColor(th.Muted).Selectable()
		return
	}
	switch r.Tag {
	case "details":
		summary := i18n.T("详情")
		if m := reSummary.FindStringSubmatch(rest); m != nil {
			if s := strings.TrimSpace(html.UnescapeString(reTag.ReplaceAllString(m[1], ""))); s != "" {
				summary = strings.Join(strings.Fields(s), " ")
			}
			rest = strings.Replace(rest, m[0], "", 1)
		}
		box := wui.Column(c).AlignSelf(wui.Stretch).Border(1, th.CodeBlockBorder).Radius(th.CodeBlockRadius).Clip()
		box.Role(wui.RoleGroup).Label(summary)
		isOpen := wui.Local(box, mdOpenKey{}, func() bool { return reDetailsOpen.MatchString(open) })
		box.Children(func() {
			p := wui.CollapsibleBase(c, isOpen)
			p.Trigger.AlignSelf(wui.Stretch).AlignItems(wui.Center).Gap(em(0.4)).Padding(em(0.45), em(0.75)).
				Cursor(wui.CursorPointer).Label(summary)
			if *isOpen {
				p.Trigger.BorderWidth(0, 0, 1, 0).BorderColor(th.CodeBlockBorder)
			}
			if p.Trigger.Hovered() {
				p.Trigger.Background(th.CodeBackground)
			}
			p.Trigger.Children(func() {
				ic := iconChevronRight
				if *isOpen {
					ic = iconChevron
				}
				wui.Icon(c, ic).FontSize(iconInline).TextColor(th.Muted)
				wui.Text(c, summary).FontWeight(th.StrongWeight).Grow(1).MinWidth(0)
			})
			p.Panel(func() {
				body := wui.Column(c).AlignSelf(wui.Stretch).Padding(em(0.7), em(0.9))
				e.containerBody(c, body, rest, r.Body, align)
			})
		})
	case "blockquote":
		q := wui.Column(c).AlignSelf(wui.Stretch).BorderStartWidth(th.QuoteBarWidth).BorderColor(th.QuoteBar).
			PaddingStart(em(0.85)).TextColor(th.QuoteText)
		e.containerBody(c, q, rest, r.Body, align)
	default:
		e.containerBody(c, wui.Column(c).AlignSelf(wui.Stretch), rest, r.Body, align)
	}
}

var reDetailsOpen = regexp.MustCompile(`(?i)^\s*<details[^>]*\sopen\b`)

// containerBody 画容器里的：开标签那个块里剩下的 HTML（一行 <img> 之类）在前，正文的 Markdown 在后，按 align 对齐。
func (e *mdEnv) containerBody(c *wui.Context, el wui.Element, head, body string, align wui.Align) {
	mdAlign(el, align)
	el.Children(func() {
		if strings.TrimSpace(head) != "" {
			mdHTML(c, &mdEnv{a: e.a, d: e.d, prep: e.prep, th: e.th, depth: e.depth, align: align, links: e.links, anchor: e.anchor, o: e.o, css: e.css}, strings.TrimSpace(head))
		}
		e.nested(c, wui.Column(c).AlignSelf(wui.Stretch), body, align)
	})
}

// mdAlignOf 是开标签上 align= 的对齐；没写是 0。
func mdAlignOf(tag string) wui.Align {
	m := reAlign.FindStringSubmatch(tag)
	if m == nil {
		return 0
	}
	switch strings.ToLower(m[1]) {
	case "center", "middle":
		return wui.Center
	case "right":
		return wui.End
	}
	return 0
}

// mdAlign 让 el 里的字和块按 a 对齐（居中的 README 头）。
func mdAlign(el wui.Element, a wui.Align) {
	if a == wui.Center || a == wui.End {
		el.TextAlign(a).AlignItems(a)
	}
}

// mdImg 是一排图片里的一张。
type mdImg struct {
	src, alt, title, href string
	w, h                  float32 // HTML 写的宽高（DIP）；0 是没写
	pct                   float32 // width="50%"
}

// mdImageItems 是第 i 段的图片：段落里只有图片、包着图片的链接和空白时给出，不然 nil。
func mdImageItems(doc *parser.Document, i int) []mdImg {
	in := doc.Inlines(i)
	var items []mdImg
	space := func(n *parser.Inline) bool {
		switch n.Kind {
		case parser.SoftBreak, parser.LineBreak:
			return true
		case parser.Text:
			return strings.TrimSpace(n.Text) == ""
		case parser.HTML:
			return mdIsBr(n.Text)
		}
		return false
	}
	img := func(k int, href string) mdImg {
		n := &in[k]
		return mdImg{src: n.Dest, title: n.Title, alt: parser.PlainText(in[k+1 : n.Next]), href: href}
	}
	for k := 0; k < len(in); k = in[k].Next {
		n := &in[k]
		switch {
		case n.Kind == parser.Image:
			if n.Pending {
				return nil
			}
			items = append(items, img(k, ""))
		case n.Kind == parser.Link:
			got := false
			for j := k + 1; j < n.Next; j = in[j].Next {
				switch {
				case in[j].Kind == parser.Image:
					items, got = append(items, img(j, n.Dest)), true
				case !space(&in[j]):
					return nil
				}
			}
			if !got {
				return nil
			}
		case !space(n):
			return nil
		}
	}
	return items
}

func mdIsBr(s string) bool {
	s = strings.ToLower(strings.ReplaceAll(s, " ", ""))
	return s == "<br>" || s == "<br/>"
}

// mdHTMLImages 是段落里只有 HTML 标签（其中有 <img>）和空白：README 里 <a href><img></a> 写在一行的徽章。
func mdHTMLImages(in []parser.Inline) bool {
	img := false
	for k := 0; k < len(in); k = in[k].Next {
		n := &in[k]
		switch n.Kind {
		case parser.HTML:
			if l := strings.ToLower(n.Text); strings.HasPrefix(l, "<img") || strings.HasPrefix(l, "<source") {
				img = true
			}
		case parser.SoftBreak, parser.LineBreak:
		case parser.Text:
			if strings.TrimSpace(n.Text) != "" {
				return false
			}
		default:
			return false
		}
	}
	return img
}

// mdImages 是一排图片：放不下折行，按 e.align 对齐。
func mdImages(c *wui.Context, e *mdEnv, items []mdImg) {
	th := e.th
	row := wui.Row(c).Wrap().Gap(mdEm(th, 0.3)).AlignSelf(wui.Stretch).AlignItems(wui.Center)
	if e.align == wui.Center || e.align == wui.End {
		row.Justify(e.align)
	}
	row.Children(func() {
		for _, it := range items {
			mdImage(c, e, it)
		}
	})
}

// mdImage 是一张图片：读好了按写的（没写按本来的）大小、最宽一栏宽；没好时是写着说明的一小块；包在链接里的点了开链接。
func mdImage(c *wui.Context, e *mdEnv, it mdImg) {
	th := e.th
	a, d := e.a, e.d
	img, ok := a.PreviewImage(d, it.src)
	var box wui.Element
	if ok && img.Err == nil && img.Src != nil && img.W > 0 && img.H > 0 {
		w, h := img.W, img.H
		switch {
		case it.w > 0 && it.h > 0:
			w, h = it.w, it.h
		case it.w > 0:
			w, h = it.w, it.w*img.H/img.W
		case it.h > 0:
			w, h = it.h*img.W/img.H, it.h
		}
		src := img.Src
		box = wui.Box(c).MaxWidthPercent(100).AspectRatio(w / h).Shrink(1)
		if it.pct > 0 {
			box.WidthPercent(it.pct)
		} else {
			box.Width(w)
		}
		if h >= 48 && w >= 48 {
			box.Radius(mdEm(th, 0.3)).Clip()
		}
		box.Role(wui.RoleImage).Label(it.alt).Draw(func(p *wui.Painter, rc wui.Rect) { p.Image(src, rc, wui.FillBox) })
	} else {
		label := it.alt
		if label == "" {
			label = i18n.T("图片")
		}
		box = wui.Row(c).Padding(mdEm(th, 0.3), mdEm(th, 0.6)).Radius(mdEm(th, 0.3)).Background(th.CodeBackground).
			Role(wui.RoleImage).Label(it.alt)
		box.Children(func() { wui.Text(c, label).TextColor(th.Muted).FontSize(mdEm(th, 0.8)).SingleLine() })
		switch {
		case ok && app.RemoteImagesOff(img.Err):
			box.Tooltip(i18n.T("设置里关了网上的图片"))
		case ok && img.Err != nil:
			box.Tooltip(i18n.Tf("读不出图片：%v", img.Err))
		}
	}
	switch {
	case it.href != "":
		box.Cursor(wui.CursorPointer).Description(it.href)
		if it.title != "" || box.Hovered() {
			box.Tooltip(strings.TrimSpace(it.title + "\n" + it.href))
		}
		if box.Clicked() {
			a.MarkdownLink(d, it.href)
		}
	case it.title != "":
		box.Tooltip(it.title)
	}
}

// mdSize 读 HTML 的 width / height："120"、"120px" 是 DIP，"50%" 给 pct。
func mdSize(s string) (v, pct float32) {
	s = strings.TrimSpace(strings.ToLower(s))
	if p, ok := strings.CutSuffix(s, "%"); ok {
		f, _ := strconv.ParseFloat(p, 32)
		return 0, float32(min(max(f, 0), 100))
	}
	f, _ := strconv.ParseFloat(strings.TrimSuffix(s, "px"), 32)
	return float32(max(f, 0)), 0
}

// codeLink 是 CodeLink 钩子：`src/app.go`、`docs/guide.md:12` 这种行内代码，相对文件所在目录或仓库根找得到就成链接。
func (e *mdEnv) codeLink(code string) (string, bool) {
	if len(code) < 3 || len(code) > 200 || strings.ContainsAny(code, " \t\n<>\"'`|*?") || !strings.ContainsAny(code, "./") ||
		strings.Contains(code, "://") || strings.HasPrefix(code, "-") {
		return "", false
	}
	if dest, ok := e.links[code]; ok {
		return dest, dest != ""
	}
	dest := ""
	p := code
	if i := strings.IndexByte(p, ':'); i > 0 {
		p = p[:i]
	}
	if strings.HasSuffix(p, ".") || !strings.ContainsAny(p, "/") && !strings.Contains(strings.TrimPrefix(p, "."), ".") {
		e.links[code] = ""
		return "", false
	}
	for _, dir := range []string{e.d.Dir(), e.d.Root} {
		if dir == "" || filepath.IsAbs(p) {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err == nil {
			dest = filepath.Join(dir, filepath.FromSlash(code))
			break
		}
	}
	e.links[code] = dest
	return dest, dest != ""
}
