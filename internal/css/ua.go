package css

import (
	"strconv"
	"strings"
)

// uaSrc 是浏览器自带样式表里预览用得到的那些（照 HTML 标准的 Rendering 一节，删掉了画不出来的）。
// 链接的颜色、代码的底色这些跟着主题，由调用方给，不在这里。
const uaSrc = `
html, body, div, p, h1, h2, h3, h4, h5, h6, ul, ol, dl, dt, dd, blockquote, pre, hr, figure, figcaption,
section, article, header, footer, nav, aside, main, address, details, summary, center, form, fieldset,
legend, hgroup, menu, dir, search, picture { display: block }
li { display: list-item }
table { display: table; border-collapse: separate; border-spacing: 2px }
caption { display: table-caption; text-align: center }
thead { display: table-header-group; vertical-align: middle }
tbody { display: table-row-group; vertical-align: middle }
tfoot { display: table-footer-group; vertical-align: middle }
tr { display: table-row; vertical-align: inherit }
td, th { display: table-cell; vertical-align: inherit; padding: 1px }
col { display: table-column }
colgroup { display: table-column-group }
head, script, style, template, title, meta, link, base, noscript, datalist, param, rp { display: none }
[hidden] { display: none }
img, video, canvas, svg, iframe, input, button, select, textarea, progress, meter { display: inline-block }
b, strong { font-weight: bolder }
i, em, cite, var, dfn, address { font-style: italic }
code, kbd, samp, tt, pre, listing, xmp, plaintext { font-family: monospace }
pre, listing, xmp, plaintext { white-space: pre; margin: 1em 0 }
small { font-size: smaller }
big { font-size: larger }
sub { vertical-align: sub; font-size: smaller }
sup { vertical-align: super; font-size: smaller }
u, ins { text-decoration: underline }
s, strike, del { text-decoration: line-through }
abbr[title], acronym[title] { text-decoration: underline dotted }
mark { background-color: yellow; color: black }
center { text-align: center }
th { font-weight: bold; text-align: center }
h1 { font-size: 2em; margin: 0.67em 0; font-weight: bold }
h2 { font-size: 1.5em; margin: 0.83em 0; font-weight: bold }
h3 { font-size: 1.17em; margin: 1em 0; font-weight: bold }
h4 { font-size: 1em; margin: 1.33em 0; font-weight: bold }
h5 { font-size: 0.83em; margin: 1.67em 0; font-weight: bold }
h6 { font-size: 0.67em; margin: 2.33em 0; font-weight: bold }
p, dl, figure { margin: 1em 0 }
figure { margin: 1em 40px }
dd { margin-left: 40px }
ul, ol, menu, dir { margin: 1em 0; padding-left: 40px }
ul, menu, dir { list-style-type: disc }
ol { list-style-type: decimal }
ul ul, ol ul, ul ol, ol ol { margin-top: 0; margin-bottom: 0 }
ul ul, ol ul { list-style-type: circle }
ul ul ul, ul ol ul, ol ul ul, ol ol ul { list-style-type: square }
blockquote { margin: 1em 40px }
hr { border: 1px inset; margin: 0.5em auto; color: gray }
summary { display: list-item }
`

// UA 是浏览器自带的样式表，Compute 总是先用它。
var UA = ParseSheet(uaSrc)

// Presentational 是元素上的表现属性折成的声明：align、valign、width / height、bgcolor、<font> 的 color / size / face、
// <table> 的 border / cellpadding / cellspacing、nowrap。它们排在 UA 后面、文档的 CSS 前面。
func Presentational(e Element) []Decl {
	if e == nil {
		return nil
	}
	tag := strings.ToLower(e.Tag())
	var out []Decl
	add := func(p, v string) { out = append(out, Decl{Prop: p, Value: v}) }
	attr := func(name string) string {
		v, _ := e.Attr(name)
		return strings.TrimSpace(v)
	}

	if a := strings.ToLower(attr("align")); a != "" {
		switch tag {
		case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "td", "th", "tr", "caption", "thead", "tbody", "tfoot",
			"section", "article", "header", "footer", "legend":
			switch a {
			case "left", "right", "center", "justify":
				add("text-align", a)
			case "middle":
				add("text-align", "center")
			}
		case "table":
			switch a {
			case "center":
				add("margin-left", "auto")
				add("margin-right", "auto")
			case "left", "right":
				add("float", a)
			}
		case "img", "object", "iframe", "video":
			switch a {
			case "left", "right":
				add("float", a)
			case "top", "middle", "bottom":
				add("vertical-align", a)
			}
		case "hr":
			switch a {
			case "left":
				add("margin-left", "0")
				add("margin-right", "auto")
			case "right":
				add("margin-left", "auto")
				add("margin-right", "0")
			case "center":
				add("margin-left", "auto")
				add("margin-right", "auto")
			}
		}
	}

	switch tag {
	case "img", "table", "td", "th", "col", "colgroup", "video", "iframe", "canvas", "hr", "object", "embed", "svg", "pre":
		for _, dim := range [...]string{"width", "height"} {
			if tag == "pre" && dim == "height" {
				continue
			}
			if v, ok := htmlLength(attr(dim)); ok {
				add(dim, v)
			}
		}
	}

	if c := attr("bgcolor"); c != "" {
		switch tag {
		case "body", "table", "tr", "td", "th", "thead", "tbody", "tfoot":
			if _, ok := ParseColor(c); ok {
				add("background-color", c)
			}
		}
	}

	if v := strings.ToLower(attr("valign")); v != "" {
		switch tag {
		case "td", "th", "tr", "thead", "tbody", "tfoot", "col", "colgroup":
			switch v {
			case "top", "middle", "bottom", "baseline":
				add("vertical-align", v)
			}
		}
	}

	switch tag {
	case "font", "basefont":
		if c := attr("color"); c != "" {
			if _, ok := ParseColor(c); ok {
				add("color", c)
			}
		}
		if f := attr("face"); f != "" {
			add("font-family", f)
		}
		if s := attr("size"); s != "" {
			if k, ok := fontSizeAttr(s); ok {
				add("font-size", k)
			}
		}
	case "table":
		if n, ok := tableBorder(e); ok && n > 0 {
			add("border", strconv.Itoa(n)+"px outset gray")
		}
		if v, ok := pixels(attr("cellspacing")); ok {
			add("border-spacing", v)
		}
	case "td", "th":
		if t := enclosingTable(e); t != nil {
			if n, ok := tableBorder(t); ok && n > 0 {
				add("border", "1px inset gray")
			}
			if v, ok := t.Attr("cellpadding"); ok {
				if px, ok := pixels(strings.TrimSpace(v)); ok {
					add("padding", px)
				}
			}
		}
		if _, ok := e.Attr("nowrap"); ok {
			add("white-space", "nowrap")
		}
	}
	return out
}

// htmlLength 读 width= / height=：数是 px，带 % 的是百分数。
func htmlLength(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	if p, ok := strings.CutSuffix(s, "%"); ok {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil || f < 0 {
			return "", false
		}
		return strconv.FormatFloat(f, 'f', -1, 32) + "%", true
	}
	return pixels(strings.TrimSuffix(s, "px"))
}

// pixels 读一个非负的数，给 "Npx"。
func pixels(s string) (string, bool) {
	f, err := strconv.ParseFloat(s, 32)
	if err != nil || f < 0 || f > 100000 {
		return "", false
	}
	return strconv.FormatFloat(f, 'f', -1, 32) + "px", true
}

// tableBorder 读 <table border>：只写 border（空值）是 1。
func tableBorder(t Element) (int, bool) {
	v, ok := t.Attr("border")
	if !ok {
		return 0, false
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return 1, true
	}
	n, err := strconv.Atoi(strings.TrimSuffix(v, "px"))
	if err != nil || n < 0 {
		return 1, true
	}
	return min(n, 100), true
}

// enclosingTable 是单元格所在的 <table>（往上最多找三层：tr、thead / tbody / tfoot、table）。
func enclosingTable(e Element) Element {
	p := e.Parent()
	for range 3 {
		if p == nil {
			return nil
		}
		if strings.EqualFold(p.Tag(), "table") {
			return p
		}
		p = p.Parent()
	}
	return nil
}

// fontSizeAttr 把 <font size> 的 1–7（或 +n / -n，对着 3）换成字号关键字。
func fontSizeAttr(s string) (string, bool) {
	n := 0
	rel := s[0] == '+' || s[0] == '-'
	v, err := strconv.Atoi(s)
	if err != nil {
		return "", false
	}
	if rel {
		n = 3 + v
	} else {
		n = v
	}
	keys := [...]string{"x-small", "small", "medium", "large", "x-large", "xx-large", "xxx-large"}
	return keys[min(max(n, 1), 7)-1], true
}
