package gmd

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wui "github.com/egoist/mygo/ui"
)

// show 在一个 w×h 的无头窗口里画 src，返回 Tester。
func show(t *testing.T, src string, o *Options, w, h int, dark bool) *wui.Tester {
	t.Helper()
	if o == nil {
		o = &Options{}
	}
	view := func(c *wui.Context) {
		wui.Scroll(c).Width(float32(w)).Height(float32(h)).Children(func() {
			wui.Column(c).FillWidth().Padding(16, 24).Children(func() { View(c, src, o) })
		})
	}
	tt := wui.NewTester(view, w, h)
	tt.SetDark(dark)
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	return tt
}

func find(t *testing.T, tt *wui.Tester, s string) wui.Rect {
	t.Helper()
	rc, ok := tt.Find(s)
	if !ok {
		t.Fatalf("界面上没有 %q；texts=%q", s, tt.Texts())
	}
	return rc
}

func shot(t *testing.T, tt *wui.Tester, name string) {
	t.Helper()
	dir := os.Getenv("WORKBENCH_SHOT")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

// at 是截图里 (x, y)（点）那一个像素的颜色。
func at(tt *wui.Tester, x, y float32) (r, g, b uint8) {
	s := scaleOf(tt)
	c := tt.Image().RGBAAt(int(x*s), int(y*s))
	return c.R, c.G, c.B
}

var testW = 900

func scaleOf(tt *wui.Tester) float32 {
	return float32(tt.Image().Bounds().Dx()) / float32(testW)
}

func TestMarkdownBasics(t *testing.T) {
	src := "# 标题一\n\n一段 **粗体** 和 `代码`，还有[链接](https://example.com)。\n\n" +
		"- 第一项\n- 第二项\n\n1. 甲\n2. 乙\n\n| 名字 | 值 |\n|---|---|\n| a | 1 |\n\n```go\nfunc main() {}\n```\n\n> 引用的话\n"
	tt := show(t, src, nil, testW, 700, false)
	for _, s := range []string{"标题一", "第一项", "第二项", "名字", "引用的话"} {
		find(t, tt, s)
	}
	if !tt.HasText("func main() {}") && !tt.HasText("func") {
		t.Fatalf("代码块没画出来：%q", tt.Texts())
	}
	if a, b := find(t, tt, "第一项"), find(t, tt, "第二项"); b.Y <= a.Y {
		t.Fatalf("列表项没往下排：%v %v", a, b)
	}
}

// cpptLike 是照架构文档常见写法造的一段带样式的 HTML：<style> 里的 class、三列的 grid 卡片、flex 一行、
// 左边框着色的提示、带底色的表头、行内的小圆点、::before 的记号、display:none 的不显示。
const cpptLike = `# 架构

<style>
.arch { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin: 8px 0; }
.card { background: #e8f1ff; border: 1px solid #9ab8e8; border-radius: 8px; padding: 10px 12px; }
.card h4 { margin: 0 0 4px; color: #1d4ed8; font-size: 15px; }
.card.warn { background: #fff4e5; border-color: #f0b46a; }
.row { display: flex; gap: 8px; align-items: center; }
.pill { background: #2563eb; color: white; border-radius: 999px; padding: 2px 10px; }
.note { border-left: 4px solid #22c55e; background: #f0fdf4; padding: 6px 10px; }
.dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; background: #ef4444; margin-right: 6px; }
.tag::before { content: "▶ "; color: #9333ea; }
.hidden { display: none; }
table.t { border-collapse: collapse; }
table.t th { background: #334155; color: #fff; padding: 4px 8px; }
table.t td { border: 1px solid #cbd5e1; padding: 4px 8px; }
</style>

<div class="arch">
  <div class="card"><h4>接入层</h4>网关与鉴权</div>
  <div class="card"><h4>服务层</h4>订单服务</div>
  <div class="card warn"><h4>存储层</h4>主从数据库</div>
</div>

<div class="row"><span class="pill">步骤一</span><span class="pill">步骤二</span><span class="tag">尾巴</span></div>

<div class="note"><span class="dot"></span>注意：写操作走主库<span class="hidden">看不见的字</span></div>

<table class="t"><tr><th>模块</th><th>负责人</th></tr><tr><td>网关</td><td>甲组</td></tr></table>

正文接着往下。
`

func TestHTMLCSS(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tt := show(t, cpptLike, nil, testW, 760, dark)
			shot(t, tt, "gmd-html-"+name+".png")
			// grid：三张卡片一行，从左到右
			a, b, c := find(t, tt, "接入层"), find(t, tt, "服务层"), find(t, tt, "存储层")
			if !(a.X < b.X && b.X < c.X) || abs(a.Y-b.Y) > 2 || abs(b.Y-c.Y) > 2 {
				t.Fatalf("grid 的三张卡片没排成一行：%v %v %v", a, b, c)
			}
			if b.X-a.X < 200 {
				t.Fatalf("1fr 的三列没铺满：%v %v", a, b)
			}
			body := find(t, tt, "网关与鉴权")
			if body.Y <= a.Y {
				t.Fatalf("卡片正文该在标题下面：%v %v", a, body)
			}
			// 卡片的底色画出来了（标题左上方一点，在卡片的内边距里）
			if r, g, bl := at(tt, a.X-4, a.Y+2); !(bl > r && bl > 200 && g > 200) {
				t.Fatalf("卡片底色不对：%d %d %d", r, g, bl)
			}
			if r, g, bl := at(tt, c.X-4, c.Y+2); !(r > 240 && bl < 240) {
				t.Fatalf("warn 卡片的底色不对：%d %d %d", r, g, bl)
			}
			// flex：两个胶囊同一行
			p1, p2 := find(t, tt, "步骤一"), find(t, tt, "步骤二")
			if abs(p1.Y-p2.Y) > 2 || p2.X <= p1.X+p1.W {
				t.Fatalf("flex 一行不对：%v %v", p1, p2)
			}
			// ::before 的记号
			if !tt.HasText("▶ ") && !tt.HasText("▶") {
				found := false
				for _, s := range tt.Texts() {
					found = found || strings.Contains(s, "▶")
				}
				if !found {
					t.Fatalf("::before 没画：%q", tt.Texts())
				}
			}
			// display:none
			for _, s := range tt.Texts() {
				if strings.Contains(s, "看不见的字") {
					t.Fatal("display:none 的还画出来了")
				}
			}
			// 表格
			m, g := find(t, tt, "模块"), find(t, tt, "网关")
			if g.Y <= m.Y {
				t.Fatalf("表格的行不对：%v %v", m, g)
			}
			// HTML 前后的 Markdown 照常
			find(t, tt, "架构")
			end := find(t, tt, "正文接着往下。")
			if end.Y <= g.Y {
				t.Fatalf("HTML 后面的段落位置不对：%v", end)
			}
		})
	}
}

// 暗色主题里：卡片是作者写的浅底、字是主题的浅色时，字换成看得清的深色。
func TestHTMLContrast(t *testing.T) {
	src := `<div style="background:#ffffff;padding:8px">白底上的字</div>`
	tt := show(t, src, nil, testW, 200, true)
	rc := find(t, tt, "白底上的字")
	img := tt.Image()
	s := scaleOf(tt)
	dark := 0
	for y := int(rc.Y * s); y < int((rc.Y+rc.H)*s); y++ {
		for x := int(rc.X * s); x < int((rc.X+rc.W)*s); x++ {
			c := img.RGBAAt(x, y)
			if int(c.R)+int(c.G)+int(c.B) < 200 {
				dark++
			}
		}
	}
	if dark < 20 {
		t.Fatalf("白底上的字没换成深色（深像素 %d 个）", dark)
	}
}

func TestDocCSS(t *testing.T) {
	src := "<style>.a{color:red}</style>\n\n```html\n<style>.b{color:blue}</style>\n```\n"
	got := DocCSS(src)
	if !strings.Contains(got, ".a{color:red}") || strings.Contains(got, ".b") {
		t.Fatalf("DocCSS = %q", got)
	}
	if DocCSS("# 没有样式\n") != "" {
		t.Fatal("没有 <style> 该是空的")
	}
}

func TestWantsCSS(t *testing.T) {
	cases := []struct {
		src, css string
		want     bool
	}{
		{`<p align="center"><img src="x.png"></p>`, "", false},
		{`<div class="card">x</div>`, "", false},
		{`<div class="card">x</div>`, ".card{}", true},
		{`<div style="background:#eee">x</div>`, "", true},
		{`<div style="text-align:center">x</div>`, "", false},
		{`<style>.x{}</style><div>x</div>`, "", true},
	}
	for _, c := range cases {
		if got := WantsCSS(c.src, c.css); got != c.want {
			t.Errorf("WantsCSS(%q, %q) = %v", c.src, c.css, got)
		}
	}
}

// 只有 <style> 的 HTML 块不占地方：上下两段之间和没有它时一样近。
func TestInvisibleBlock(t *testing.T) {
	plain := show(t, "上一段\n\n下一段\n", nil, testW, 200, false)
	styled := show(t, "上一段\n\n<style>.x{color:red}</style>\n\n下一段\n", nil, testW, 200, false)
	gap := func(tt *wui.Tester) float32 { return find(t, tt, "下一段").Y - find(t, tt, "上一段").Y }
	if g1, g2 := gap(plain), gap(styled); abs(g1-g2) > 1 {
		t.Fatalf("只有 <style> 的块占了地方：%v vs %v", g1, g2)
	}
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// 表格照浏览器的自动表格布局（VS Code 的预览就是）：列放得下时按内容宽、不铺满；放不下时整张表就是这一栏宽，
// 列按要的宽成比例让，长的那列多折几行，不会有一列挤成一字一行、整张表横着滚出去；折不开的也放不下才横着滚。
func TestTableAutoLayout(t *testing.T) {
	long := strings.Repeat("这一列写得很长，说明数据从哪里来、谁负责、出了问题找谁；", 4)
	src := "| 事实 | 来源 | 说明 |\n|---|---|---|\n" +
		"| 当前训练容器可写层用量 | kubelet stats/summary 的 workspace.rootfs | " + long + " |\n" +
		"| 节点、Pod、HAMi 设备和占用 | K8s API / HAMi 注解 | 短 |\n"
	tt := show(t, src, nil, testW, 700, false)
	shot(t, tt, "gmd-table.png")
	h1, h2, h3 := find(t, tt, "事实"), find(t, tt, "来源"), find(t, tt, "说明")
	right := float32(testW - 24)
	if !(h1.X < h2.X && h2.X < h3.X) || h3.X > right-100 {
		t.Fatalf("三列没都放进这一栏：%v %v %v", h1, h2, h3)
	}
	// 第三列最宽（要的最多），第一、二列不比它宽
	if w3, w1 := right-h3.X, h2.X-h1.X; w3 < w1 {
		t.Fatalf("长的那列反而窄：第一列 %v，第三列 %v", w1, w3)
	}
	// 短行不被撑高：第二行（“节点…”）紧跟着第一行的长文字下面，长文字只折了几行
	r1, r2 := find(t, tt, "当前训练容器可写层用量"), find(t, tt, "节点、Pod、HAMi 设备和占用")
	if r2.Y-r1.Y > 8*r1.H {
		t.Fatalf("第一行太高（长的那列挤窄了）：%v → %v", r1, r2)
	}

	// 放得下的小表按内容宽，不铺满一栏
	tt = show(t, "| 甲 | 乙 |\n|---|---|\n| 1 | 2 |\n", nil, testW, 300, false)
	if a, b := find(t, tt, "甲"), find(t, tt, "乙"); b.X-a.X > 120 {
		t.Fatalf("小表被撑宽了：%v %v", a, b)
	}

	// 折不开的长串放不下：横着滚，最后一列在这一栏右边外面
	tok := strings.Repeat("abcdefghij", 12)
	tt = show(t, "| x | y |\n|---|---|\n| "+tok+" | "+tok+"z |\n", nil, testW, 300, false)
	tt.Frame()
	if rc := find(t, tt, tok+"z"); rc.X < float32(testW)*0.75 { // Find 给的是滚动视口里看得见的那截
		t.Fatalf("折不开的第一列被挤窄了（第二列从 %v 起）", rc.X)
	}
}
