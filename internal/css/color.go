// Package css 是 Markdown 预览里 HTML 块的样式表：解析 <style> 和 style= 的 CSS，按选择器、优先级、先后层叠，
// 算出每个元素的样式（展开简写、继承、var()、font-size 换成 px）。不碰界面，元素由调用方包成 Element 给进来。
//
// 只认预览里画得出来的那些：颜色、长度、盒子（边框、圆角、内外边距）、grid / flex、字体和文字。认不出的规则、
// 选择器、值一律跳过，不报错也不 panic——文档里的 CSS 是别人写的，坏了也要照常显示。
package css

import (
	"math"
	"strconv"
	"strings"
)

// Color 是一个颜色：R、G、B 是 0–255，A 是 0–1 的不透明度。
type Color struct {
	R, G, B uint8
	A       float32
}

// ParseColor 读一个 CSS 颜色：#rgb、#rgba、#rrggbb、#rrggbbaa，rgb()/rgba()（逗号或空格写法、/ 后面的透明度、百分数），
// hsl()/hsla()，CSS 的全部颜色名和 transparent。currentcolor 不算（要看元素的 color，调用方处理）。
func ParseColor(s string) (Color, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return Color{}, false
	}
	if s[0] == '#' {
		return parseHex(s[1:])
	}
	if s == "transparent" {
		return Color{}, true
	}
	if i := strings.IndexByte(s, '('); i > 0 && strings.HasSuffix(s, ")") {
		args := s[i+1 : len(s)-1]
		switch strings.TrimSpace(s[:i]) {
		case "rgb", "rgba":
			return parseRGB(args)
		case "hsl", "hsla":
			return parseHSL(args)
		}
		return Color{}, false
	}
	if v, ok := named[s]; ok {
		return Color{uint8(v >> 16), uint8(v >> 8), uint8(v), 1}, true
	}
	return Color{}, false
}

func hexVal(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	}
	return 0, false
}

func parseHex(h string) (Color, bool) {
	var d [8]uint8
	if len(h) != 3 && len(h) != 4 && len(h) != 6 && len(h) != 8 {
		return Color{}, false
	}
	for i := 0; i < len(h); i++ {
		v, ok := hexVal(h[i])
		if !ok {
			return Color{}, false
		}
		d[i] = v
	}
	switch len(h) {
	case 3, 4:
		c := Color{d[0] * 17, d[1] * 17, d[2] * 17, 1}
		if len(h) == 4 {
			c.A = float32(d[3]*17) / 255
		}
		return c, true
	default:
		c := Color{d[0]<<4 | d[1], d[2]<<4 | d[3], d[4]<<4 | d[5], 1}
		if len(h) == 8 {
			c.A = float32(d[6]<<4|d[7]) / 255
		}
		return c, true
	}
}

// colorArgs 把 rgb()/hsl() 括号里的切成三个分量和透明度（没写透明度时 alpha 为空）。
func colorArgs(args string) (parts []string, alpha string, ok bool) {
	if i := strings.IndexByte(args, '/'); i >= 0 {
		alpha = strings.TrimSpace(args[i+1:])
		args = args[:i]
		if alpha == "" {
			return nil, "", false
		}
	}
	parts = strings.Fields(strings.ReplaceAll(args, ",", " "))
	if len(parts) == 4 && alpha == "" {
		alpha, parts = parts[3], parts[:3]
	}
	return parts, alpha, len(parts) == 3
}

func parseNum(s string) (float64, bool) {
	if s == "none" {
		return 0, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// parseAlpha 读透明度：0–1 的数或百分数；空的是 1。
func parseAlpha(s string) (float32, bool) {
	if s == "" {
		return 1, true
	}
	scale := 1.0
	if p, ok := strings.CutSuffix(s, "%"); ok {
		s, scale = p, 0.01
	}
	f, ok := parseNum(s)
	if !ok {
		return 0, false
	}
	return float32(min(max(f*scale, 0), 1)), true
}

func clamp255(f float64) uint8 { return uint8(math.Round(min(max(f, 0), 255))) }

func parseRGB(args string) (Color, bool) {
	parts, alpha, ok := colorArgs(args)
	if !ok {
		return Color{}, false
	}
	var ch [3]uint8
	for i, p := range parts {
		q, pct := strings.CutSuffix(p, "%")
		f, ok := parseNum(q)
		if !ok {
			return Color{}, false
		}
		if pct {
			f = f * 255 / 100
		}
		ch[i] = clamp255(f)
	}
	a, ok := parseAlpha(alpha)
	if !ok {
		return Color{}, false
	}
	return Color{ch[0], ch[1], ch[2], a}, true
}

func parseHSL(args string) (Color, bool) {
	parts, alpha, ok := colorArgs(args)
	if !ok {
		return Color{}, false
	}
	h, s0 := parts[0], 1.0
	switch {
	case strings.HasSuffix(h, "deg"):
		h = h[:len(h)-3]
	case strings.HasSuffix(h, "grad"):
		h, s0 = h[:len(h)-4], 0.9
	case strings.HasSuffix(h, "rad"):
		h, s0 = h[:len(h)-3], 180/math.Pi
	case strings.HasSuffix(h, "turn"):
		h, s0 = h[:len(h)-4], 360
	}
	hue, ok := parseNum(h)
	if !ok {
		return Color{}, false
	}
	hue = math.Mod(hue*s0, 360)
	if hue < 0 {
		hue += 360
	}
	var sl [2]float64
	for i, p := range parts[1:] {
		f, ok := parseNum(strings.TrimSuffix(p, "%"))
		if !ok {
			return Color{}, false
		}
		sl[i] = min(max(f/100, 0), 1)
	}
	a, ok := parseAlpha(alpha)
	if !ok {
		return Color{}, false
	}
	sat, l := sl[0], sl[1]
	f := func(n float64) uint8 {
		k := math.Mod(n+hue/30, 12)
		x := sat * min(l, 1-l)
		return clamp255(255 * (l - x*max(-1, min(k-3, 9-k, 1))))
	}
	return Color{f(0), f(8), f(4), a}, true
}

// named 是 CSS 的颜色名（0xRRGGBB）。
var named = map[string]uint32{
	"aliceblue": 0xf0f8ff, "antiquewhite": 0xfaebd7, "aqua": 0x00ffff, "aquamarine": 0x7fffd4, "azure": 0xf0ffff,
	"beige": 0xf5f5dc, "bisque": 0xffe4c4, "black": 0x000000, "blanchedalmond": 0xffebcd, "blue": 0x0000ff,
	"blueviolet": 0x8a2be2, "brown": 0xa52a2a, "burlywood": 0xdeb887, "cadetblue": 0x5f9ea0, "chartreuse": 0x7fff00,
	"chocolate": 0xd2691e, "coral": 0xff7f50, "cornflowerblue": 0x6495ed, "cornsilk": 0xfff8dc, "crimson": 0xdc143c,
	"cyan": 0x00ffff, "darkblue": 0x00008b, "darkcyan": 0x008b8b, "darkgoldenrod": 0xb8860b, "darkgray": 0xa9a9a9,
	"darkgreen": 0x006400, "darkgrey": 0xa9a9a9, "darkkhaki": 0xbdb76b, "darkmagenta": 0x8b008b, "darkolivegreen": 0x556b2f,
	"darkorange": 0xff8c00, "darkorchid": 0x9932cc, "darkred": 0x8b0000, "darksalmon": 0xe9967a, "darkseagreen": 0x8fbc8f,
	"darkslateblue": 0x483d8b, "darkslategray": 0x2f4f4f, "darkslategrey": 0x2f4f4f, "darkturquoise": 0x00ced1,
	"darkviolet": 0x9400d3, "deeppink": 0xff1493, "deepskyblue": 0x00bfff, "dimgray": 0x696969, "dimgrey": 0x696969,
	"dodgerblue": 0x1e90ff, "firebrick": 0xb22222, "floralwhite": 0xfffaf0, "forestgreen": 0x228b22, "fuchsia": 0xff00ff,
	"gainsboro": 0xdcdcdc, "ghostwhite": 0xf8f8ff, "gold": 0xffd700, "goldenrod": 0xdaa520, "gray": 0x808080,
	"green": 0x008000, "greenyellow": 0xadff2f, "grey": 0x808080, "honeydew": 0xf0fff0, "hotpink": 0xff69b4,
	"indianred": 0xcd5c5c, "indigo": 0x4b0082, "ivory": 0xfffff0, "khaki": 0xf0e68c, "lavender": 0xe6e6fa,
	"lavenderblush": 0xfff0f5, "lawngreen": 0x7cfc00, "lemonchiffon": 0xfffacd, "lightblue": 0xadd8e6, "lightcoral": 0xf08080,
	"lightcyan": 0xe0ffff, "lightgoldenrodyellow": 0xfafad2, "lightgray": 0xd3d3d3, "lightgreen": 0x90ee90, "lightgrey": 0xd3d3d3,
	"lightpink": 0xffb6c1, "lightsalmon": 0xffa07a, "lightseagreen": 0x20b2aa, "lightskyblue": 0x87cefa,
	"lightslategray": 0x778899, "lightslategrey": 0x778899, "lightsteelblue": 0xb0c4de, "lightyellow": 0xffffe0,
	"lime": 0x00ff00, "limegreen": 0x32cd32, "linen": 0xfaf0e6, "magenta": 0xff00ff, "maroon": 0x800000,
	"mediumaquamarine": 0x66cdaa, "mediumblue": 0x0000cd, "mediumorchid": 0xba55d3, "mediumpurple": 0x9370db,
	"mediumseagreen": 0x3cb371, "mediumslateblue": 0x7b68ee, "mediumspringgreen": 0x00fa9a, "mediumturquoise": 0x48d1cc,
	"mediumvioletred": 0xc71585, "midnightblue": 0x191970, "mintcream": 0xf5fffa, "mistyrose": 0xffe4e1, "moccasin": 0xffe4b5,
	"navajowhite": 0xffdead, "navy": 0x000080, "oldlace": 0xfdf5e6, "olive": 0x808000, "olivedrab": 0x6b8e23,
	"orange": 0xffa500, "orangered": 0xff4500, "orchid": 0xda70d6, "palegoldenrod": 0xeee8aa, "palegreen": 0x98fb98,
	"paleturquoise": 0xafeeee, "palevioletred": 0xdb7093, "papayawhip": 0xffefd5, "peachpuff": 0xffdab9, "peru": 0xcd853f,
	"pink": 0xffc0cb, "plum": 0xdda0dd, "powderblue": 0xb0e0e6, "purple": 0x800080, "rebeccapurple": 0x663399,
	"red": 0xff0000, "rosybrown": 0xbc8f8f, "royalblue": 0x4169e1, "saddlebrown": 0x8b4513, "salmon": 0xfa8072,
	"sandybrown": 0xf4a460, "seagreen": 0x2e8b57, "seashell": 0xfff5ee, "sienna": 0xa0522d, "silver": 0xc0c0c0,
	"skyblue": 0x87ceeb, "slateblue": 0x6a5acd, "slategray": 0x708090, "slategrey": 0x708090, "snow": 0xfffafa,
	"springgreen": 0x00ff7f, "steelblue": 0x4682b4, "tan": 0xd2b48c, "teal": 0x008080, "thistle": 0xd8bfd8,
	"tomato": 0xff6347, "turquoise": 0x40e0d0, "violet": 0xee82ee, "wheat": 0xf5deb3, "white": 0xffffff,
	"whitesmoke": 0xf5f5f5, "yellow": 0xffff00, "yellowgreen": 0x9acd32,
}
