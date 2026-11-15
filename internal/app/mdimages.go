package app

import (
	"bytes"
	"container/list"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/render"
)

// 这个文件是 Markdown 预览里的图片：本地文件、data: 地址、网上的（https，徽章、截图）。读好的放进一个进程级的缓存
// （几个窗口、几个预览共用，按像素字节数挤），读的时候在后台 goroutine 里，读好了叫界面重画。
// 带文字的 SVG（shields.io 那种徽章）mygo 画不了字，交给窗口的渲染器（internal/render，WebKit）画成位图。

// FetchImage 取一张网上的图片：测试里换成桩，不上网。
var FetchImage = fetchImage

// 图片的几个上限。
const (
	imageMaxBytes   = 8 << 20  // 一张图片文件最大多少字节（再大的不读）
	imageCacheBytes = 48 << 20 // 缓存里的图片一共占多少字节（位图按像素算）
	imageTimeout    = 15 * time.Second
	imageRetry      = time.Minute // 网上的读不到：过这么久再试
	imageFetchers   = 6           // 同时取几张网上的
)

// MDImage 是预览里的一张图片：Src 拿去画，W、H 是它本来的大小（DIP）；读不出来时 Err 不为空。
type MDImage struct {
	Src  ui.ImageSource
	W, H float32
	Err  error
}

var (
	errRemoteOff = errors.New("remote images are off")
	errInsecure  = errors.New("http images are not loaded")
)

// RemoteImagesOff 是图片读不出来是因为设置里关了网上的图片（预览里换一句提示）。
func RemoteImagesOff(err error) bool { return errors.Is(err, errRemoteOff) }

type imageEntry struct {
	key     string
	done    bool
	img     *MDImage
	bytes   int
	expires time.Time // 网上的读不到：到点当没有
}

type imageCache struct {
	mu    sync.Mutex
	lru   *list.List // *imageEntry，前面是最近用的
	m     map[string]*list.Element
	bytes int
	sem   chan struct{}
}

var images = &imageCache{lru: list.New(), m: map[string]*list.Element{}, sem: make(chan struct{}, imageFetchers)}

// PreviewImage 是 d 的预览里地址 u 的图片：读好了给（ok 为 true，读不出来时 Err 不为空）；没读过就在后台读，
// 读好了叫界面重画。在主线程上调。
func (a *App) PreviewImage(d *docs.Doc, u string) (*MDImage, bool) {
	key, remote, err := a.imageKey(d.Dir(), u)
	if err != nil {
		return &MDImage{Err: err}, true
	}
	var rend *render.Renderer
	var scale float32
	if rend = a.Renderer(); rend != nil {
		scale = rend.Scale(2) * a.ZoomFactor()
		key += fmt.Sprintf("|%g", scale) // 带字的 SVG 按这个密度画
	}
	images.mu.Lock()
	if e, ok := images.m[key]; ok {
		en := e.Value.(*imageEntry)
		if !en.done {
			images.mu.Unlock()
			return nil, false
		}
		if en.expires.IsZero() || time.Now().Before(en.expires) {
			images.lru.MoveToFront(e)
			images.mu.Unlock()
			return en.img, true
		}
		images.drop(e)
	}
	en := &imageEntry{key: key}
	images.m[key] = images.lru.PushFront(en)
	images.mu.Unlock()

	up := a.Update // 后台 goroutine 别读 a.Update
	dir := d.Dir()
	go func() {
		img := loadImage(dir, u, rend, scale)
		images.mu.Lock()
		en.done, en.img = true, img
		en.bytes = 256
		switch s := img.Src.(type) {
		case *ui.Bitmap:
			w, h := s.Size()
			en.bytes += w * h * 4
		case *ui.SVG:
			en.bytes += 64 << 10
		}
		if img.Err != nil && remote {
			en.expires = time.Now().Add(imageRetry)
		}
		if e, ok := images.m[key]; ok && e.Value == en {
			images.bytes += en.bytes
			images.trim()
		}
		images.mu.Unlock()
		up(func() {})
	}()
	return nil, false
}

// ImageLoader 是给 mygo 的 markdown.Options.LoadImage（预览里自己没接手的图片走它）：在 mygo 的 goroutine 里同步读。
func (a *App) ImageLoader(d *docs.Doc) func(string) (ui.ImageSource, error) {
	dir := d.Dir()
	off := a.St.Settings.NoRemoteImages
	return func(u string) (ui.ImageSource, error) {
		if off && isRemote(u) {
			return nil, errRemoteOff
		}
		img := loadImage(dir, u, nil, 0)
		return img.Src, img.Err
	}
}

// imageKey 是缓存里认 u 的键：本地文件是绝对路径加修改时间和大小（改了图片重读），网上的是地址本身。
func (a *App) imageKey(dir, u string) (key string, remote bool, err error) {
	switch {
	case strings.HasPrefix(u, "data:"):
		return u, false, nil
	case isRemote(u):
		if a.St.Settings.NoRemoteImages {
			return "", true, errRemoteOff
		}
		if strings.HasPrefix(strings.ToLower(u), "http:") {
			return "", true, errInsecure
		}
		return u, true, nil
	case strings.Contains(u, "://"):
		return "", false, fmt.Errorf("unsupported image address %s", u)
	}
	p := localPath(dir, u)
	fi, err := os.Stat(p)
	if err != nil {
		return "", false, err
	}
	return fmt.Sprintf("%s|%d|%d", p, fi.ModTime().UnixNano(), fi.Size()), false, nil
}

func isRemote(u string) bool {
	l := strings.ToLower(u)
	return strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "http://")
}

// localPath 是相对 dir 的图片地址的磁盘路径（去掉 file:、?查询 和 #片段，解码 %xx）。
func localPath(dir, u string) string {
	p := strings.TrimPrefix(u, "file://")
	p = strings.TrimPrefix(p, "file:")
	p, _, _ = strings.Cut(p, "#")
	p, _, _ = strings.Cut(p, "?")
	if s, err := url.PathUnescape(p); err == nil {
		p = s
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, filepath.FromSlash(p))
	}
	return p
}

// loadImage 读一张图片并解码。rend 不为空时带字的 SVG 交给它画成 scale 倍的位图。
func loadImage(dir, u string, rend *render.Renderer, scale float32) *MDImage {
	var data []byte
	var err error
	switch {
	case strings.HasPrefix(u, "data:"):
		data, err = decodeDataURL(u)
	case isRemote(u):
		images.sem <- struct{}{}
		data, err = FetchImage(u)
		<-images.sem
	default:
		p := localPath(dir, u)
		var fi os.FileInfo
		if fi, err = os.Stat(p); err == nil && fi.Size() > imageMaxBytes {
			err = fmt.Errorf("image too large: %d bytes", fi.Size())
		} else if err == nil {
			data, err = os.ReadFile(p)
		}
	}
	if err != nil {
		return &MDImage{Err: err}
	}
	return decodeImage(data, rend, scale)
}

// decodeImage 把图片文件解成能画的：位图（PNG、JPEG、GIF、WebP、BMP）或 SVG。
func decodeImage(data []byte, rend *render.Renderer, scale float32) *MDImage {
	if isSVG(data) {
		if rend != nil && bytes.Contains(data, []byte("<text")) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*imageTimeout)
			res, ok := rend.Wait(ctx, render.Request{Kind: render.SVG, Src: string(data), Scale: scale})
			cancel()
			switch {
			case ok && res.Bitmap != nil:
				return &MDImage{Src: res.Bitmap, W: res.W, H: res.H}
			case ok && res.Err != "":
				return &MDImage{Err: errors.New(res.Err)}
			}
			// 渲染器忙不过来、关了：退回不画字的
		}
		svg, err := ui.ParseSVG(data)
		if err != nil {
			return &MDImage{Err: err}
		}
		w, h := svg.Size()
		return &MDImage{Src: svg, W: w, H: h}
	}
	bmp, err := ui.DecodeBitmap(data)
	if err != nil {
		return &MDImage{Err: err}
	}
	w, h := bmp.Size()
	return &MDImage{Src: bmp, W: float32(w), H: float32(h)}
}

// isSVG 看开头像不像 SVG（网上的徽章地址常常没有 .svg 后缀）。
func isSVG(data []byte) bool {
	head := data[:min(len(data), 1024)]
	head = bytes.TrimLeft(bytes.TrimPrefix(head, []byte("\xef\xbb\xbf")), " \t\r\n")
	return bytes.HasPrefix(head, []byte("<svg")) || (bytes.HasPrefix(head, []byte("<?xml")) || bytes.HasPrefix(head, []byte("<!")) || bytes.HasPrefix(head, []byte("<!--"))) && bytes.Contains(head, []byte("<svg"))
}

// decodeDataURL 解 data:[类型][;base64],数据。
func decodeDataURL(u string) ([]byte, error) {
	meta, payload, ok := strings.Cut(strings.TrimPrefix(u, "data:"), ",")
	if !ok {
		return nil, errors.New("bad data URL")
	}
	if strings.HasSuffix(meta, ";base64") {
		return base64.StdEncoding.DecodeString(strings.TrimRight(payload, "\n"))
	}
	s, err := url.PathUnescape(payload)
	return []byte(s), err
}

var imageClient = &http.Client{Timeout: imageTimeout}

// fetchImage 取网上的一张图片：只 GET，不带 Cookie，最多 imageMaxBytes。
func fetchImage(u string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Grove")
	req.Header.Set("Accept", "image/*")
	resp, err := imageClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, imageMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > imageMaxBytes {
		return nil, errors.New(i18n.T("图片太大"))
	}
	return data, nil
}

// trim 挤掉最久没用的直到在上限里（还在读的不挤）。images.mu 锁着。
func (c *imageCache) trim() {
	for e := c.lru.Back(); e != nil && c.bytes > imageCacheBytes; {
		prev := e.Prev()
		if e.Value.(*imageEntry).done {
			c.drop(e)
		}
		e = prev
	}
}

func (c *imageCache) drop(e *list.Element) {
	en := e.Value.(*imageEntry)
	c.lru.Remove(e)
	if c.m[en.key] == e {
		delete(c.m, en.key)
	}
	if en.done {
		c.bytes -= en.bytes
	}
}

// ForgetImages 清空图片缓存（测试用）。
func ForgetImages() {
	images.mu.Lock()
	images.lru.Init()
	clear(images.m)
	images.bytes = 0
	images.mu.Unlock()
}
