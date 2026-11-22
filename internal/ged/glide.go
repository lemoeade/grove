package ged

import (
	"time"

	wui "github.com/egoist/mygo/ui"
)

// glideTime 是滚轮一格滑多久：和 mygo 自己的滚动容器（浏览器也是这样）一样，一格在 150 ms 里缓出地滑过去，
// 不是一帧跳 40 点。
const glideTime = 150 * time.Millisecond

// glide 是鼠标滚轮还在路上的那几格（触控板、妙控鼠标一点点地来，本来就跟手，不滑）。
// 滑着时别的挪过 ScrollY 就不滑了（交给它），所以每帧直接按起点算，滑完正好停在一格的整数倍上。
type glide struct {
	on   bool
	from float32   // 这一段从哪起
	dist float32   // 这一段一共要滚多远（新的一格来时，没走完的接上新的一格重新起一段）
	done float32   // 这一段已经滚了多远
	at   time.Time // 这一段从哪一帧起；零值是还没起（下一帧建视图时起）
	y    float32   // 上一帧滑完后 ScrollY 在哪：对不上就是别的（触控板、键、拖滚动条、预览带着）挪过它，不滑了
}

// wheel 收一格滚轮：接在没走完的后面（反方向就扔掉没走完的，立刻掉头）。滚到头、一点都挪不动时返回 false（让外面接）。
func (d *Diff) wheel(dy float32, g geom) bool {
	gl := &d.glide
	left := float32(0)
	if gl.on && gl.y == d.ScrollY && (dy > 0) == (gl.dist > gl.done) {
		left = gl.dist - gl.done
	}
	end := d.ScrollY + left // 不来这一格的话滑到哪
	to := max(0, min(end+dy, max(0, g.content-g.h+d.tail(g))))
	if to == end {
		return false
	}
	*gl = glide{on: true, from: d.ScrollY, dist: to - d.ScrollY, y: d.ScrollY}
	return true
}

// stepGlide 每帧建视图时走一步：按缓出曲线算到这一帧该走多远，ScrollY 设到起点加那么多；还没走完就再要一帧。
func (d *Diff) stepGlide(c *wui.Context) {
	gl := &d.glide
	if !gl.on {
		return
	}
	if gl.y != d.ScrollY { // 别的挪过：交给它
		gl.on = false
		return
	}
	now := c.Now()
	if gl.at.IsZero() {
		gl.at = now.Add(-time.Second / 60) // 收到的这一帧就先走一帧的路，不白等一帧
	}
	t := float32(now.Sub(gl.at)) / float32(glideTime)
	p := gl.dist
	if t < 1 {
		p = gl.dist * wui.EaseOut(max(0, t))
	}
	d.ScrollY, gl.done = gl.from+p, p
	if t >= 1 {
		gl.on = false
	} else {
		c.AnimationFrame()
	}
	gl.y = d.ScrollY
}
