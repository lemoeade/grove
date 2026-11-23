package gmd

import (
	ui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// Theme 是 mygo 的 markdown.Theme：Grove 的 mdTheme 两边共用一份。
type Theme = markdown.Theme

// Color 是 mygo 的颜色。
type Color = ui.Color

// resolve 是 th 补上它没写（零值）的、从 def 取的（照 mygo 的 resolve）。
func resolve(th *Theme, def *Theme) Theme {
	if th == nil {
		return *def
	}
	out := *th
	f := func(v *float32, d float32) {
		if *v == 0 {
			*v = d
		}
	}
	c := func(v *Color, d Color) {
		if *v == (Color{}) {
			*v = d
		}
	}
	s := func(v *string, d string) {
		if *v == "" {
			*v = d
		}
	}
	f(&out.FontSize, def.FontSize)
	s(&out.Font, def.Font)
	s(&out.CodeFont, def.CodeFont)
	f(&out.LineHeight, def.LineHeight)
	f(&out.CodeLineHeight, def.CodeLineHeight)
	c(&out.Text, def.Text)
	c(&out.Muted, def.Muted)
	c(&out.Link, def.Link)
	c(&out.Strong, out.Text)
	if out.StrongWeight == 0 {
		out.StrongWeight = def.StrongWeight
	}
	c(&out.Code, out.Text)
	c(&out.CodeBackground, def.CodeBackground)
	f(&out.CodeSize, def.CodeSize)
	f(&out.BlockSpacing, def.BlockSpacing)
	f(&out.ItemSpacing, def.ItemSpacing)
	for i := range out.Headings {
		h, d := &out.Headings[i], def.Headings[i]
		f(&h.Size, d.Size)
		if h.Weight == 0 {
			h.Weight = d.Weight
		}
		if i == 5 {
			c(&h.Color, out.Muted)
		} else {
			c(&h.Color, out.Text)
		}
		f(&h.Top, d.Top)
		f(&h.Bottom, d.Bottom)
	}
	f(&out.ListIndent, def.ListIndent)
	c(&out.QuoteBar, def.QuoteBar)
	c(&out.QuoteText, out.Muted)
	f(&out.QuoteBarWidth, def.QuoteBarWidth)
	for k := range out.Alerts {
		c(&out.Alerts[k], def.Alerts[k])
	}
	c(&out.Rule, def.Rule)
	c(&out.CodeBlockBackground, def.CodeBlockBackground)
	c(&out.CodeBlockBorder, def.CodeBlockBorder)
	f(&out.CodeBlockRadius, def.CodeBlockRadius)
	f(&out.CodeBlockPadding, def.CodeBlockPadding)
	if out.CodeColors == nil {
		out.CodeColors = def.CodeColors
	}
	c(&out.TableBorder, def.TableBorder)
	c(&out.TableHeader, def.TableHeader)
	f(&out.TableCellPadding[0], def.TableCellPadding[0])
	f(&out.TableCellPadding[1], def.TableCellPadding[1])
	f(&out.TableMaxColumn, def.TableMaxColumn)
	c(&out.Mark, def.Mark)
	c(&out.Checkbox, def.Checkbox)
	return out
}
