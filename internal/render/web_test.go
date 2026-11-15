package render

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

func gz(t *testing.T, s string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write([]byte(s))
	w.Close()
	return b.Bytes()
}

// 协议的处理：原样的给原样的，.js 没有就解开 .js.gz 给（类型按 .js 算）；.gz 本身、目录、没有的、跑出根的都 404。
func TestAssetHandler(t *testing.T) {
	fsys := fstest.MapFS{
		"page.html":         {Data: []byte("<!doctype html><p>hi")},
		"lib/a.js.gz":       {Data: gz(t, "window.a = 1")},
		"lib/b.js":          {Data: []byte("window.b = 2")},
		"lib/b.js.gz":       {Data: gz(t, "stale")},
		"lib/sub/c.js.gz":   {Data: gz(t, "window.c = 3")},
		"lib/broken.js.gz":  {Data: []byte("not gzip")},
		"lib/sub/README.md": {Data: []byte("x")},
	}
	h := assetHandler(fsys)
	get := func(p string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "wbrender://app"+p, nil))
		return w
	}
	for _, tc := range []struct{ path, body, ctype string }{
		{"/page.html", "<!doctype html><p>hi", "text/html"},
		{"/lib/a.js", "window.a = 1", "javascript"},
		{"/lib/b.js", "window.b = 2", "javascript"}, // 原样的优先
		{"/lib/sub/c.js", "window.c = 3", "javascript"},
		{"/lib/../lib/a.js", "window.a = 1", "javascript"},
	} {
		w := get(tc.path)
		if w.Code != 200 || w.Body.String() != tc.body || !strings.Contains(w.Header().Get("Content-Type"), tc.ctype) {
			t.Errorf("%s：%d %q %q", tc.path, w.Code, w.Body.String(), w.Header().Get("Content-Type"))
		}
	}
	for _, p := range []string{"/lib/a.js.gz", "/lib", "/lib/", "/nope.js", "/lib/broken.js", "/../page.html/x", "/"} {
		if w := get(p); w.Code != 404 {
			t.Errorf("%s 该 404，是 %d", p, w.Code)
		}
	}
}

// 仓库里的渲染页要的脚本都在（压成 .gz 的也算），压着的都解得开；不引网上的东西。
func TestAssetsPresent(t *testing.T) {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		t.Fatal(err)
	}
	page, err := fs.ReadFile(sub, "page.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile(`load\("([^"]+)"\)`).FindAllStringSubmatch(string(page), -1) {
		if _, err := readAsset(sub, m[1]); err != nil {
			t.Errorf("page.html 要的 %s 没有：%v", m[1], err)
		}
	}
	if regexp.MustCompile(`(?i)(src|href)\s*=\s*["']?https?:`).Match(page) {
		t.Error("page.html 引了网上的东西")
	}
	n := 0
	fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".gz") {
			return err
		}
		n++
		if _, err := readAsset(sub, strings.TrimSuffix(p, ".gz")); err != nil {
			t.Errorf("%s 解不开：%v", p, err)
		}
		return nil
	})
	if n < 40 {
		t.Errorf("只有 %d 个压着的脚本", n)
	}
	for _, p := range []string{"mathjax/input/tex/extensions/mhchem.js", "mathjax/fonts/mathjax-mhchem-font-extension/svg.js",
		"mathjax/fonts/mathjax-newcm-font/svg/dynamic/arrows.js"} {
		if _, err := readAsset(sub, p); err != nil {
			t.Errorf("少了 %s：%v", p, err)
		}
	}
}
