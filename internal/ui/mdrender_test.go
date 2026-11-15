package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/render"
	"github.com/lemoeade/grove/internal/testutil"
)

// fakeRender 是假的渲染后端：等 gate 关了才画；源码里有 bad 的是源码错；公式里有 vec 的给一张矢量（只有路径，
// 用字色）；别的画一张洋红的 PNG（宽是源码长度 × 8 DIP）。记下每种画了几次、Mermaid 收到的配色。
type fakeRender struct {
	gate     chan struct{}
	mu       sync.Mutex
	kinds    map[string]int
	palettes []string
	closed   int
}

func (f *fakeRender) Render(ctx context.Context, q render.Request, maxPixels int) (render.Output, error) {
	select {
	case <-f.gate:
	case <-ctx.Done():
		return render.Output{}, ctx.Err()
	}
	f.mu.Lock()
	f.kinds[q.Kind]++
	if q.Kind == render.Mermaid {
		f.palettes = append(f.palettes, q.Palette)
	}
	f.mu.Unlock()
	if strings.Contains(q.Src, "bad") {
		return render.Output{}, &render.SourceError{Msg: "Parse error on line 1:\nbad\n^"}
	}
	w, h := float32(len(q.Src)*8), float32(40)
	if q.Kind == render.Inline {
		h = 16
	}
	if q.Kind != render.Mermaid && strings.Contains(q.Src, "vec") {
		return render.Output{SVG: fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%g" height="%g" viewBox="0 0 %g %g">`+
			`<rect width="%g" height="%g" fill="currentColor"/></svg>`, w, h, w, h, w, h), W: w, H: h, Depth: 4}, nil
	}
	img := image.NewRGBA(image.Rect(0, 0, int(w*q.Scale), int(h*q.Scale)))
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			img.SetRGBA(x, y, color.RGBA{255, 0, 255, 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return render.Output{PNG: b.Bytes(), W: w, H: h, Depth: 4}, nil
}

func (f *fakeRender) Close() {
	f.mu.Lock()
	f.closed++
	f.mu.Unlock()
}

// magenta 是这一帧有多少个洋红的像素（假渲染画的）。
func magenta(img *image.RGBA) int {
	n := 0
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i] > 240 && img.Pix[i+1] < 16 && img.Pix[i+2] > 240 {
			n++
		}
	}
	return n
}

const diagramsMD = "# 图\n\n```mermaid\ngraph TD; A-->B\n```\n\n$$\nx^2+y^2=z^2\n$$\n\n欧拉：$e^{i\\pi}+1=0$。\n\n```mermaid\nbad graph\n```\n"

// Markdown 预览里的图和公式（假后端）：没画好时显示代码块 / TeX 和“渲染中…”，行内公式是原文；
// 画好了图和块级公式是图片（读屏念 Mermaid 图 / TeX），行内公式成了段落里的对象；源码错是一行红字加代码块；
// 设置里关了：渲染器关掉，$…$ 照旧是字，代码块回来。
func TestRenderHooks(t *testing.T) {
	root, ws := testutil.TempRepo(t)
	os.WriteFile(filepath.Join(root, "m.md"), []byte(diagramsMD), 0o644)
	a, tt, wait := newUI(t, ws, termState(t, false))
	tt.SetScale(2) // Retina：跟没画过时猜的一样，不重画
	f := &fakeRender{gate: make(chan struct{}), kinds: map[string]int{}}
	opened := 0
	a.RenderOpen = func() (render.Backend, error) { opened++; return f, nil }
	t.Cleanup(func() {
		if a.Rend != nil {
			a.Rend.Close()
		}
	})
	d := a.OpenFile(a.Repos[0], "m.md", true)
	wait("读好", func() bool { return d.Buf != nil && !d.Loading })
	a.Exec("view.markdownPreview")
	tt.Frame()
	tt.Frame()
	if !tt.HasText("渲染中…") || !tt.HasText("graph TD; A-->B") || !tt.HasText("x^2+y^2=z^2") || !tt.HasText(`e^{i\pi}+1=0`) {
		t.Fatalf("没画好时：%q", tt.Texts())
	}
	if magenta(tt.Image()) != 0 {
		t.Fatal("还没画就有图")
	}

	close(f.gate)
	wait("都画好", func() bool {
		_, block := tt.Find(i18n.T("Mermaid 图"))
		return block && !tt.HasText("渲染中…") && tt.HasText("￼") && tt.HasText(i18n.Tf("画不出来：%s", "Parse error on line 1:"))
	})
	tt.Frame()
	if tt.HasText("graph TD; A-->B") || tt.HasText(`e^{i\pi}+1=0`) || !tt.HasText("bad graph") {
		t.Errorf("画好后还显示原文，或坏了的没留代码块：%q", tt.Texts())
	}
	if _, ok := tt.Find("x^2+y^2=z^2"); !ok {
		t.Error("块级公式的图片该念 TeX")
	}
	if n := magenta(tt.Image()); n == 0 {
		t.Error("画好了却没画出来")
	}
	f.mu.Lock()
	kinds := f.kinds[render.Mermaid] == 2 && f.kinds[render.Math] == 1 && f.kinds[render.Inline] >= 1
	f.mu.Unlock()
	if !kinds || opened != 1 {
		t.Errorf("渲染了 %v，建了 %d 次后端", f.kinds, opened)
	}

	// 挪到 1× 的显示器：重画，没好之前画手里的（不闪回代码块）
	tt.SetScale(1)
	tt.Frame()
	if _, block := tt.Find(i18n.T("Mermaid 图")); !block || tt.HasText("渲染中…") || !tt.HasText("\uFFFC") {
		t.Errorf("换像素密度时闪回原文：%q", tt.Texts())
	}
	wait("1× 的画好", func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.kinds[render.Mermaid] == 4 // 两张图（坏了的那张也按新密度再试一次）
	})

	// 设置里关了
	a.St.Settings.NoDiagrams = true
	tt.Frame()
	tt.Frame()
	if a.Rend != nil || f.closed != 1 {
		t.Fatalf("关了渲染器还在（关了 %d 次后端）", f.closed)
	}
	if !tt.HasText(`$e^{i\pi}+1=0$`) || !tt.HasText("graph TD; A-->B") || tt.HasText("渲染中…") || magenta(tt.Image()) != 0 {
		t.Errorf("关了之后：%q", tt.Texts())
	}
}

// 矢量的公式按这一帧的字色画（换深色不再渲染，颜色跟着变）；Mermaid 收到主题的配色，换深色换一套、重画。
func TestRenderVectorTheme(t *testing.T) {
	root, ws := testutil.TempRepo(t)
	os.WriteFile(filepath.Join(root, "v.md"), []byte("# 矢量\n\n$$\n\\vec{v}=1\n$$\n\n```mermaid\ngraph TD; A-->B\n```\n"), 0o644)
	a, tt, wait := newUI(t, ws, termState(t, false))
	tt.SetScale(2)
	tt.SetDark(false)
	f := &fakeRender{gate: make(chan struct{}), kinds: map[string]int{}}
	close(f.gate)
	a.RenderOpen = func() (render.Backend, error) { return f, nil }
	t.Cleanup(func() {
		if a.Rend != nil {
			a.Rend.Close()
		}
	})
	d := a.OpenFile(a.Repos[0], "v.md", true)
	wait("读好", func() bool { return d.Buf != nil && !d.Loading })
	a.Exec("view.markdownPreview")
	wait("都画好", func() bool {
		_, block := tt.Find(i18n.T("Mermaid 图"))
		_, math := tt.Find(`\vec{v}=1`)
		return block && math && !tt.HasText("渲染中…")
	})
	rc, _ := tt.Find(`\vec{v}=1`)
	ink := func() (dark, light int) { // 公式那一块里深色、浅色的像素各多少
		img := tt.Image()
		s := float32(2)
		for y := int((rc.Y + 4) * s); y < int((rc.Y+rc.H-4)*s); y++ {
			for x := int((rc.X + 4) * s); x < int((rc.X+rc.W-4)*s); x++ {
				c := img.RGBAAt(x, y)
				switch l := int(c.R) + int(c.G) + int(c.B); {
				case l < 150:
					dark++
				case l > 600:
					light++
				}
			}
		}
		return
	}
	if dk, _ := ink(); dk == 0 {
		t.Fatal("浅色主题下公式没用深色的字画")
	}

	tt.SetDark(true)
	wait("深色的图画好", func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.kinds[render.Mermaid] == 2
	})
	for range 5 { // 排版和画的时候算出来的配色一样：不多画一遍
		tt.Frame()
	}
	if _, lt := ink(); lt == 0 {
		t.Error("深色主题下公式没换成浅色的字")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.kinds[render.Math] != 1 {
		t.Errorf("矢量公式换主题又渲染了：%d 次", f.kinds[render.Math])
	}
	var light, dark render.Palette
	if len(f.palettes) != 2 || f.kinds[render.Mermaid] != 2 || json.Unmarshal([]byte(f.palettes[0]), &light) != nil || json.Unmarshal([]byte(f.palettes[1]), &dark) != nil ||
		light.Bg == dark.Bg || light.Fg == dark.Fg || light.Accent == "" || light.Surface == light.Bg || dark.Border == "" {
		t.Errorf("Mermaid 的配色：%q", f.palettes)
	}
}
