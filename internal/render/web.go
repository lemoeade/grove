package render

import (
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// assets 是渲染页和它要的脚本（mermaid.js、MathJax 4 和它的 TeX 扩展、字体分片，版本和许可证见
// assets/LICENSES），从 wbrender:// 协议给 web view（不在 Go 这边拼几 MB 的字符串，页面自己按需加载）。
// 脚本压成 .gz 放着（16 MB 压到 5 MB），给的时候解开。
//
//go:embed assets
var assets embed.FS

const scheme = "wbrender"

// Register 注册渲染页的协议 wbrender://。mygo 要在建窗口之前注册（main 里调一次）。
func Register() error {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		return err
	}
	return mygo.Protocol.Handle(scheme, assetHandler(sub))
}

// assetHandler 给 fsys 里的文件：有原样的给原样的，没有就找 <名字>.gz 解开了给。别的（目录、找不到）404。
func assetHandler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		data, err := readAsset(fsys, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})
}

// readAsset 读 fsys 里的 name，没有就读 name.gz 解开。
func readAsset(fsys fs.FS, name string) ([]byte, error) {
	if name == "" || strings.HasSuffix(name, ".gz") {
		return nil, fs.ErrNotExist
	}
	if st, err := fs.Stat(fsys, name); err == nil {
		if st.IsDir() {
			return nil, fs.ErrNotExist
		}
		return fs.ReadFile(fsys, name)
	}
	f, err := fsys.Open(name + ".gz")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(zr)
}

// webBackend 是看不见的 web view：建了不放 ui.WebView 元素，它就只在后台跑页面。
type webBackend struct {
	v *mygo.WebView
}

// WebView 是在 win（要是 Content 窗口）里建 web view 的后端，给 New 用。建好、渲染页加载完才返回（在后台 goroutine 里等）。
func WebView(win *mygo.Window) func() (Backend, error) {
	return func() (Backend, error) {
		if win == nil || win.IsDestroyed() {
			return nil, errors.New("no window")
		}
		v, err := win.NewWebView(mygo.WebViewOptions{Transparent: true})
		if err != nil {
			return nil, err
		}
		p := v.Page()
		loaded := make(chan error, 1)
		var once sync.Once
		done := func(err error) { once.Do(func() { loaded <- err }) }
		off1 := p.OnDidFinishLoad(func() { done(nil) })
		off2 := p.OnDidFailLoad(func(e *mygo.LoadError) { done(e) })
		defer off1()
		defer off2()
		if err := p.LoadURL(scheme + "://localhost/page.html"); err != nil {
			v.Destroy()
			return nil, err
		}
		select {
		case err = <-loaded:
		case <-time.After(10 * time.Second):
			err = errors.New("render page did not load")
		}
		if err != nil {
			v.Destroy()
			return nil, err
		}
		return &webBackend{v: v}, nil
	}
}

// Render 在页面里跑 wbRender（page.html）：公式是 SVG 文本，别的解出 PNG。脚本抛的错是源码的错（SourceError）。
func (b *webBackend) Render(ctx context.Context, q Request, maxPixels int) (Output, error) {
	code := fmt.Sprintf("wbRender(%s, %s, %t, %g, %g, %s, %d, %s)", js(q.Kind), js(q.Src), q.Dark, q.Scale, q.Size, js(q.Color),
		maxPixels, js(q.Palette))
	v, err := b.v.Page().EvalContext(ctx, code)
	if err != nil {
		var ee *mygo.EvalError
		if errors.As(err, &ee) {
			return Output{}, &SourceError{Msg: strings.TrimPrefix(ee.Message, "Error: ")} // WebKit 给的带 Error: 前缀
		}
		return Output{}, err
	}
	m, _ := v.(map[string]any)
	num := func(k string) float32 { f, _ := m[k].(float64); return float32(f) }
	out := Output{W: num("w"), H: num("h"), Depth: num("depth")}
	if s, _ := m["svg"].(string); s != "" {
		out.SVG = s
		return out, nil
	}
	s, _ := m["png"].(string)
	if out.PNG, err = base64.StdEncoding.DecodeString(s); err != nil || len(out.PNG) == 0 {
		return Output{}, fmt.Errorf("bad picture from the render page: %v", err)
	}
	return out, nil
}

func (b *webBackend) Close() {
	if !b.v.IsDestroyed() {
		b.v.Destroy()
	}
}

// js 是 s 的 JavaScript 字符串字面量。
func js(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
