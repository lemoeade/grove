package gmd

import (
	"sync"

	ui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// imageEntry 是 LoadImage 在读或读好的一张图。
type imageEntry struct {
	mu   sync.Mutex
	src  ui.ImageSource
	err  error
	done bool
}

// image 是 url 的图，读好了才有：先问 Options.Image（调用方管缓存），没给就第一次用的时候在后台 LoadImage。
func (st *viewState) image(c *ui.Context, url string) ui.ImageSource {
	o := st.o
	if url == "" {
		return nil
	}
	if o.Image != nil {
		src, err, ok := o.Image(url)
		if !ok || err != nil {
			return nil
		}
		return src
	}
	if o.LoadImage == nil {
		return nil
	}
	v, loaded := o.images.LoadOrStore(url, &imageEntry{})
	e := v.(*imageEntry)
	if !loaded {
		load, services := o.LoadImage, c.Services()
		go func() {
			src, err := load(url)
			e.mu.Lock()
			e.src, e.err, e.done = src, err, true
			e.mu.Unlock()
			services.Invalidate()
		}()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.src
}

// imageSize 是图本来的大小（不知道是 0）。
func imageSize(src ui.ImageSource) (w, h float32) {
	switch s := src.(type) {
	case *ui.Bitmap:
		iw, ih := s.Size()
		return float32(iw), float32(ih)
	case *ui.SVG:
		return s.Size()
	}
	return 0, 0
}

// images 画只有图片的段落：每张照原大小、最宽和视图一样，读好之前显示它的说明。
func (st *viewState) images(c *ui.Context, i int) ui.Element {
	th := &st.th
	in := st.doc.Inlines(i)
	row := ui.Row(c).Wrap().Gap(em(th, 0.5)).AlignSelf(ui.Stretch)
	return row.Children(func() {
		for k := 0; k < len(in); k = in[k].Next {
			n := &in[k]
			if n.Kind != parser.Image {
				continue
			}
			alt := parser.PlainText(in[k+1 : n.Next])
			src := st.image(c, n.Dest)
			if src == nil {
				label := alt
				if label == "" {
					label = st.o.Strings.or(st.o.Strings.Image, "Image")
				}
				ph := ui.Row(c).Padding(em(th, 0.5), em(th, 0.75)).Radius(em(th, 0.3)).Background(th.CodeBackground)
				ph.Role(ui.RoleImage).Label(alt)
				ph.Children(func() { ui.Text(c, label).TextColor(th.Muted).FontSize(em(th, 0.875)) })
				continue
			}
			img := ui.Image(c, src).Radius(em(th, 0.3)).MaxWidthPercent(100).Label(alt)
			if w, h := imageSize(src); w > 0 && h > 0 {
				img.AspectRatio(w / h)
			}
			if n.Title != "" {
				img.Tooltip(n.Title)
			}
		}
	})
}
