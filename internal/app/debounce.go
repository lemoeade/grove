package app

import "time"

// 这个文件是"停手一会儿再做"的小工具：搜索框（search.go）、历史搜索框（histfilter.go）、设置里的
// "隐藏的文件"（exclude.go）共用。按时间合并、到点只做一次但不随新事件重新计时的（搜索结果跟随磁盘
// searchfollow.go、大文件重读 tabs.go 的 reloadBig）是节流，不用它。

// debounce 是停手 d 之后在主线程上做一次：每次 kick 都重新计时；stop（下一次 kick 也会先 stop）作废
// 还没做的那次——定时器已经到点、回调已经排进主线程的也不做（按 tick 认）。零值可用。
// 回调里经指针读 tick：换工作区重写 *a 时新的 App 要接着旧的 tick 往上数（见 switchTo），旧回调才认得出自己过期了。
type debounce struct {
	timer *time.Timer
	tick  int
}

// kick 重新计时：停手 d 之后经 up 回主线程做 f。
func (b *debounce) kick(up func(func()), d time.Duration, f func()) {
	b.stop()
	tick := b.tick
	b.timer = time.AfterFunc(d, func() {
		up(func() {
			if b.tick == tick {
				b.timer = nil
				f()
			}
		})
	})
}

// pending 是有一次在等着做。
func (b *debounce) pending() bool { return b.timer != nil }

// stop 作废还没做的那次。
func (b *debounce) stop() {
	b.tick++
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
}
