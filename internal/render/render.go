// Package render 渲染 Markdown 预览里的 Mermaid 图和 TeX 公式：一个看不见的 WebView 跑 mermaid.js（Grove
// 自己的外观）和 MathJax 4（web.go）。公式只有路径（字形是 currentColor），拿回 SVG 文本由 mygo 画成矢量；图带文字（mygo 的 SVG
// 不画文字），画到 canvas 上取 PNG。每个主窗口一个 Renderer：预览每帧来取（Get），没有的排进队列，后台一个
// goroutine 一条条渲染，好了叫界面重画；60 秒没活就销毁 WebView，再要再建。
package render

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
)

// 渲染的几种东西（Request.Kind）。
const (
	Mermaid = "mermaid" // ```mermaid 代码块
	Math    = "math"    // 块级公式（$$…$$、\[…\]）
	Inline  = "inline"  // 行内公式（$…$、\(…\)）
	SVG     = "svg"     // 带文字的 SVG 图片（徽章）：mygo 的 SVG 解析器不画文字，交给 WebKit 画
)

// 缓存和队列的上限，后端的几个时间。
const (
	maxEntries = 512
	maxBytes   = 32 << 20
	maxQueue   = 128
	// inUse 是多久之内取过的算正在显示：挤缓存时不挤它们（超上限也不挤，免得一屏装不下时画了挤、挤了再画转个不停），
	// 闲下来（idle）再挤到上限里。
	inUse = 2 * time.Second
	// MaxPixels 是一张位图最多多少像素（4 字节一个）：超了按比例降分辨率，免得一张大图吃掉整个缓存。
	MaxPixels = 4 << 20
	// IdleClose 是最后一次渲染之后多久销毁 WebView（加载了 Mermaid 和 MathJax 的 WKWebView 连 WebContent、GPU 进程
	// 要两三百 MB）。
	IdleClose     = 60 * time.Second
	renderTimeout = 20 * time.Second // 头一次要先加载几 MB 的脚本（mermaid.js 3.5 MB，MathJax 和字体）
	bumpEvery     = 250 * time.Millisecond
	retryAfter    = 30 * time.Second // 不是源码的错（后端建不起来、超时）：记着的错过这么久再试
	openRetry     = 5 * time.Second  // 后端建不起来之后这么久之内不再建
)

// Request 是要渲染的一张。
type Request struct {
	Kind  string
	Src   string  // Mermaid 源码或 TeX（不带 $）
	Dark  bool    // 深色主题
	Scale float32 // 一个 DIP 几个像素（Retina 是 2，界面缩放也算进去）；画成矢量的公式用不着
	Size  float32 // 公式：周围文字的字号（DIP），1 em 多大；图不用
	Color string  // 公式画成位图时的颜色（#rrggbb）；矢量的跟着画的时候的字色
	// Palette 是图的配色（Palette.JSON），跟着界面主题；公式不用
	Palette string
}

// Palette 是 Mermaid 图的配色（#rrggbb）：底色、字、连线、强调（箭头）、淡字、节点的底（两档）、边框。
type Palette struct {
	Bg       string `json:"bg"`
	Fg       string `json:"fg"`
	Line     string `json:"line"`
	Accent   string `json:"accent"`
	Muted    string `json:"muted"`
	Surface  string `json:"surface"`
	Surface2 string `json:"surface2"`
	Border   string `json:"border"`
}

// JSON 是传给渲染页的写法（Request.Palette）。
func (p Palette) JSON() string {
	b, _ := json.Marshal(p)
	return string(b)
}

// Result 是渲染好的一张：位图或矢量（二者有一）和它该占的大小（DIP；行内公式 Depth 是基线以下多少）。
// 失败时 Err 是错误信息，什么图都没有。
type Result struct {
	Bitmap      *ui.Bitmap
	Vector      *ui.SVG // 只有路径的公式：mygo 自己画，currentColor 是画的时候的字色，多大都清楚
	W, H, Depth float32
	Err         string
	svgLen      int // 矢量的 SVG 文本多长（算缓存的字节数）
}

// Image 是这一张的图（位图或矢量），失败、还没有时是 nil。
func (r Result) Image() ui.ImageSource {
	switch {
	case r.Vector != nil:
		return r.Vector
	case r.Bitmap != nil:
		return r.Bitmap
	}
	return nil
}

// Output 是后端渲染出来的一张：PNG 或 SVG 文本（二者有一），和它的大小（DIP）。
type Output struct {
	PNG         []byte
	SVG         string
	W, H, Depth float32
}

// SourceError 是源码本身的错（Mermaid 语法错、TeX 错）：后端照常能用，只是这一张画不了。别的错当后端坏了，关掉再建。
type SourceError struct{ Msg string }

// OldWebKit 是 Result.Err 的一个值：系统的 WebKit 太老（Mermaid 12 要 Safari 17.4 起的），画不了 Mermaid 图。
// 渲染页（page.html 的 mermaidSVG）抛的就是这一句，界面认出来换成说明。当源码错算（后端没坏，公式照常画）。
const OldWebKit = "grove:old-webkit"

func (e *SourceError) Error() string { return e.Msg }

// Backend 渲染一张。真的是 web.go 的 WebView，测试里换成假的。方法在后台 goroutine 里调。
type Backend interface {
	Render(ctx context.Context, req Request, maxPixels int) (Output, error)
	Close()
}

// key 认一张位图；base 认它的大小（换主题、换缩放不变），near 拿它找别的版本先顶着。
type key struct {
	base
	dark    bool
	scale   float32
	color   string
	palette string
}

type base struct {
	kind string
	sum  [32]byte
	size float32
}

func keyOf(q Request) key {
	size := q.Size
	if q.Kind == Mermaid || q.Kind == SVG {
		size = 0
	}
	return key{base{q.Kind, sha256.Sum256([]byte(q.Src)), size}, q.Dark, q.Scale, q.Color, q.Palette}
}

type entry struct {
	k       key
	res     Result
	bytes   int
	used    time.Time
	expires time.Time // 不是源码的错：过了这个时候当没有（再排一次）；零值是一直有效
}

type job struct {
	k    key
	req  Request
	bump bool // 行内公式第一次要、没有可顶的：好了要让段落重排（Gen 加一）
}

// Renderer 是一个窗口的渲染器。方法可以在任何 goroutine 里调。
type Renderer struct {
	open func() (Backend, error) // 建后端：第一次要时、销毁后再要时
	wake func()                  // 有结果了，叫界面重画（在后台 goroutine 里调）
	// Idle 是没活多久销毁后端（New 里是 IdleClose，测试改短）。
	Idle time.Duration

	mu       sync.Mutex
	be       Backend
	lru      *list.List // 前面是最近用的
	cache    map[key]*list.Element
	near     map[base]*list.Element
	bytes    int
	queue    []job // 后来的先渲染：正在看的、刚改的先出来
	queued   map[key]bool
	waiters  map[key][]chan struct{} // Wait 等着的：渲染完（不管成败）关掉
	running  bool
	timer    *time.Timer
	gen      uint64
	dirty    bool
	bumped   time.Time
	closed   bool
	scale    float32
	renders  int
	opens    int
	openErr  string
	lastFail time.Time
}

// New 是一个渲染器：open 建后端（web.go 的 WebView），wake 在有结果时叫界面重画。
func New(open func() (Backend, error), wake func()) *Renderer {
	return &Renderer{open: open, wake: wake, Idle: IdleClose, lru: list.New(), cache: map[key]*list.Element{},
		near: map[base]*list.Element{}, queued: map[key]bool{}, waiters: map[key][]chan struct{}{}}
}

// Get 是 q 渲染好的结果：有就给（ok）；没有就排进队列，先给同一张换主题 / 缩放之前的版本顶着（大小一样），
// 那也没有就 ok 为 false（块级先显示代码块，行内先显示 TeX）。
func (r *Renderer) Get(q Request) (res Result, ok bool) {
	k := keyOf(q)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return Result{}, false
	}
	if e, ok := r.cache[k]; ok {
		en := e.Value.(*entry)
		if now := time.Now(); en.expires.IsZero() || now.Before(en.expires) {
			r.lru.MoveToFront(e)
			en.used = now
			return en.res, true
		}
		r.drop(e)
	}
	e, near := r.near[k.base]
	if near && e.Value.(*entry).res.Vector != nil {
		// 矢量的不分主题、缩放、颜色：换了也不用再画
		en := e.Value.(*entry)
		r.lru.MoveToFront(e)
		en.used = time.Now()
		return en.res, true
	}
	r.enqueue(job{k: k, req: q, bump: q.Kind == Inline && !near})
	if near {
		return e.Value.(*entry).res, true
	}
	return Result{}, false
}

// Wait 等 q 渲染好再返回（后台 goroutine 里读图片时用，界面线程别调）：已经有的直接给；ctx 到点、渲染器关了
// 返回 ok 为 false。只认这一张本身，不拿换主题、缩放之前的版本顶。
func (r *Renderer) Wait(ctx context.Context, q Request) (Result, bool) {
	k := keyOf(q)
	for range 2 {
		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return Result{}, false
		}
		if e, ok := r.cache[k]; ok {
			en := e.Value.(*entry)
			if en.expires.IsZero() || time.Now().Before(en.expires) {
				r.lru.MoveToFront(e)
				en.used = time.Now()
				r.mu.Unlock()
				return en.res, true
			}
			r.drop(e)
		}
		ch := make(chan struct{})
		r.waiters[k] = append(r.waiters[k], ch)
		r.enqueue(job{k: k, req: q})
		if !r.queued[k] { // 后端刚建不起来，没排上
			r.waiters[k] = r.waiters[k][:len(r.waiters[k])-1]
			r.mu.Unlock()
			return Result{Err: r.openErrText()}, r.openErrText() != ""
		}
		r.mu.Unlock()
		select {
		case <-ctx.Done():
			return Result{}, false
		case <-ch:
		}
	}
	return Result{}, false
}

// openErrText 是后端建不起来的错。r.mu 锁着。
func (r *Renderer) openErrText() string { return r.openErr }

// Pending 是 q 还在排队 / 正在渲染（Get 没给或给的是顶着的）。
func (r *Renderer) Pending(q Request) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.queued[keyOf(q)]
}

// enqueue 把 j 排进队列（已经在里面的不重复），叫起后台 goroutine。队列满了扔最早的。r.mu 锁着。
func (r *Renderer) enqueue(j job) {
	if r.queued[j.k] {
		return
	}
	if r.openErr != "" && time.Since(r.lastFail) < openRetry {
		return // 后端刚建不起来：别每帧都试
	}
	if len(r.queue) >= maxQueue {
		k := r.queue[0].k
		delete(r.queued, k)
		for _, ch := range r.waiters[k] { // 等它的别干等：Wait 醒来再排一次
			close(ch)
		}
		delete(r.waiters, k)
		r.queue = r.queue[1:]
	}
	r.queue = append(r.queue, j)
	r.queued[j.k] = true
	if r.timer != nil {
		r.timer.Stop()
	}
	if !r.running {
		r.running = true
		go r.run()
	}
}

// run 是后台 goroutine：一条条渲染到队列空，然后数 Idle 之后销毁后端。
func (r *Renderer) run() {
	for {
		r.mu.Lock()
		if r.closed || len(r.queue) == 0 {
			r.running = false
			r.flushBump(true)
			if !r.closed {
				r.timer = time.AfterFunc(r.Idle, r.idle)
			}
			closed := r.closed
			r.mu.Unlock()
			if !closed {
				r.wake()
			}
			return
		}
		j := r.queue[len(r.queue)-1]
		r.queue = r.queue[:len(r.queue)-1]
		be := r.be
		r.mu.Unlock()

		res, broken, retry := r.render(be, j.req)

		r.mu.Lock()
		delete(r.queued, j.k)
		closed := r.closed
		waiting := r.waiters[j.k] // 结果放进缓存、解了锁之后再叫醒
		delete(r.waiters, j.k)
		if !closed {
			r.store(j.k, res, retry)
			if j.bump {
				r.dirty = true
			}
			r.flushBump(false)
		}
		if broken && r.be != nil {
			r.be.Close()
			r.be = nil
		}
		r.mu.Unlock()
		for _, ch := range waiting {
			close(ch)
		}
		if !closed { // 关了的窗口不叫
			r.wake()
		}
	}
}

// render 用后端渲染一张（没有后端先建）；broken 是后端该关掉重建（超时、页面没了），retry 是这个错过一会儿再试（不是源码的错）。
func (r *Renderer) render(be Backend, q Request) (res Result, broken, retry bool) {
	if be == nil {
		r.mu.Lock()
		failed := r.openErr
		if failed != "" && time.Since(r.lastFail) >= openRetry {
			failed = ""
		}
		r.mu.Unlock()
		if failed != "" { // 刚建不起来：队列里剩下的不再一个个试
			return Result{Err: failed}, false, true
		}
		var err error
		if be, err = r.open(); err != nil {
			r.mu.Lock()
			r.openErr, r.lastFail = err.Error(), time.Now()
			r.mu.Unlock()
			return Result{Err: err.Error()}, false, true
		}
		r.mu.Lock()
		r.be, r.opens, r.openErr = be, r.opens+1, ""
		if r.closed { // 建的时候窗口关了
			r.be = nil
			r.mu.Unlock()
			be.Close()
			return Result{Err: "closed"}, false, true
		}
		r.mu.Unlock()
	}
	ctx, cancel := context.WithTimeout(context.Background(), renderTimeout)
	out, err := be.Render(ctx, q, MaxPixels)
	cancel()
	r.mu.Lock()
	r.renders++
	r.mu.Unlock()
	if err != nil {
		var se *SourceError
		bad := !errors.As(err, &se)
		return Result{Err: errLine(err.Error())}, bad, bad
	}
	res = Result{W: out.W, H: out.H, Depth: out.Depth}
	if out.SVG != "" {
		res.svgLen = len(out.SVG)
		if res.Vector, err = ui.ParseSVG([]byte(out.SVG)); err != nil {
			return Result{Err: err.Error()}, false, false
		}
		return res, false, false
	}
	if res.Bitmap, err = ui.DecodeBitmap(out.PNG); err != nil {
		return Result{Err: err.Error()}, false, false
	}
	return res, false, false
}

// errLine 是错误信息的头一行。Mermaid 的语法错头一行是 Parse error on line 2:，中间是源码和箭头，
// 最后一行才说要什么（Expecting …, got …），接上它。
func errLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	first, last := strings.TrimSpace(lines[0]), strings.TrimSpace(lines[len(lines)-1])
	if len(lines) > 2 && strings.HasSuffix(first, ":") && strings.ContainsFunc(last, unicode.IsLetter) {
		return first + " " + last
	}
	return first
}

// store 把结果放进缓存，挤掉最久没用的直到条数和字节数都在上限里；正在显示的（inUse 之内取过的）不挤。
// retry 的（不是源码的错）retryAfter 之后失效。r.mu 锁着。
func (r *Renderer) store(k key, res Result, retry bool) {
	if e, ok := r.cache[k]; ok {
		r.drop(e)
	}
	n := 64
	if res.Bitmap != nil {
		w, h := res.Bitmap.Size()
		n += w * h * 4
	}
	if res.Vector != nil { // 解析好的路径大约是源码的几倍；mygo 画的位图在它自己的缓存里
		n += 4 * res.svgLen
	}
	now := time.Now()
	en := &entry{k: k, res: res, bytes: n, used: now}
	if retry {
		en.expires = now.Add(retryAfter)
	}
	e := r.lru.PushFront(en)
	r.cache[k], r.near[k.base] = e, e
	r.bytes += n
	r.trim(now.Add(-inUse))
}

// trim 挤掉最久没用的直到在上限里，before 之后取过的不挤（后面的都更近，停）。r.mu 锁着。
func (r *Renderer) trim(before time.Time) {
	for r.lru.Len() > 0 && (r.lru.Len() > maxEntries || r.bytes > maxBytes) {
		e := r.lru.Back()
		if e.Value.(*entry).used.After(before) {
			return
		}
		r.drop(e)
	}
}

func (r *Renderer) drop(e *list.Element) {
	en := e.Value.(*entry)
	r.lru.Remove(e)
	delete(r.cache, en.k)
	if r.near[en.k.base] == e {
		delete(r.near, en.k.base)
	}
	r.bytes -= en.bytes
}

// flushBump 在行内公式有了新结果时让 Gen 加一：一批一起，队列空了或隔 bumpEvery 才加，免得一屏公式重排几十次。r.mu 锁着。
func (r *Renderer) flushBump(done bool) {
	if r.dirty && (done || time.Since(r.bumped) >= bumpEvery) {
		r.gen++
		r.dirty, r.bumped = false, time.Now()
	}
}

// idle 是没活 Idle 之后：销毁后端，缓存挤到上限里（正在显示的照样不挤：挤了下一帧又要建后端重画）。
func (r *Renderer) idle() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.running || len(r.queue) > 0 {
		return
	}
	r.trim(time.Now().Add(-inUse))
	if r.be != nil {
		r.be.Close()
		r.be = nil
	}
}

// Gen 是行内公式结果的代数：变了预览要重建（mygo 的段落只在内容变了时才再问 InlineMath）。
func (r *Renderer) Gen() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gen
}

// Scale 是显示器的像素密度：画的时候看到的 Painter.Scale（SeenScale 记下），还没画过时是 fallback。渲染时再乘界面缩放。
func (r *Renderer) Scale(fallback float32) float32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.scale > 0 {
		return r.scale
	}
	return fallback
}

// SeenScale 记下画的时候的 Painter.Scale（换了显示器、界面缩放时跟着换）。
func (r *Renderer) SeenScale(s float32) {
	r.mu.Lock()
	r.scale = s
	r.mu.Unlock()
}

// Busy 是还有没画完的（排着队或正在画）。
func (r *Renderer) Busy() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running || len(r.queue) > 0
}

// Alive 是后端（WebView）现在建着没有。
func (r *Renderer) Alive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.be != nil
}

// Stats 是渲染过几张、建过几次后端、缓存里几张多少字节（测试和量性能时看）。
func (r *Renderer) Stats() (renders, opens, entries, bytes int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.renders, r.opens, r.lru.Len(), r.bytes
}

// Close 关掉渲染器（窗口关了、设置里关了）：后端销毁，缓存清空，之后 Get 什么都不给。
func (r *Renderer) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	if r.timer != nil {
		r.timer.Stop()
	}
	if r.be != nil {
		r.be.Close()
		r.be = nil
	}
	r.lru.Init()
	clear(r.cache)
	clear(r.near)
	r.bytes, r.queue = 0, nil
	clear(r.queued)
	for _, chs := range r.waiters {
		for _, ch := range chs {
			close(ch)
		}
	}
	clear(r.waiters)
}
