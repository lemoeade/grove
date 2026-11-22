package docs

import (
	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// 文件标签有两种编辑器：mygo 的（St 画）和 Grove 自己画的 ged 单栏编辑器（Ed，ui/gedfile.go）。用 ged 时 St 还在：
// 光标、选中的字、缓冲区和它对齐着，App 照旧问 St 的光标、调 St.GoTo、St.Select；只有要看界面的（看得见哪几行、
// 滚动、查找条、补全列表、悬停提示、编辑器命令）走下面这几个，画的是 ged 时问 ged。

// UsingEd 是上一帧画的是 ged 的单栏编辑器。
func (d *Doc) UsingEd() bool { return d.EdOn && d.Ed != nil }

// VisibleLines 是上一帧看得见的第一行和最后一行（从 0 起），还没画过是 0, 0。
func (d *Doc) VisibleLines() (first, last int) {
	if d.UsingEd() {
		return d.Ed.VisibleLines()
	}
	return d.St.VisibleLines()
}

// ScrollTo 把第 line 行滚到顶上。
func (d *Doc) ScrollTo(line int) {
	if d.UsingEd() {
		d.Ed.ScrollToLine(line)
		return
	}
	d.St.ScrollTo(line)
}

// Run 在编辑器里执行一个编辑器命令（菜单、命令面板来的），做了返回 true。ged 自己不做的它交给 St（见 ui/gedfile.go）。
func (d *Doc) Run(cmd editor.Command) bool {
	if d.UsingEd() {
		return d.Ed.Command(cmd)
	}
	return d.St.Run(cmd)
}

// Completions 是 ged 编辑器上的补全列表：从缓冲区第 Start 个字节到光标是正在打的词，Items 是给的候选
// （按打的字筛，见 ui/gedcomplete.go），Sel 是选中第几个（筛过的里面）。
type Completions struct {
	Start int
	Items []editor.Completion
	Sel   int
}

// ShowCompletions 开补全列表（和 State.ShowCompletions 一样）：从 start 到光标的字被选中的那项换掉。
func (d *Doc) ShowCompletions(start int, items []editor.Completion) {
	if d.UsingEd() {
		d.EdCompl = &Completions{Start: start, Items: items}
		return
	}
	d.St.ShowCompletions(start, items)
}

// Tip 是 ged 编辑器上的悬停提示：对着缓冲区 [Start, End) 那几个字，Code 是声明（等宽字画），Build 画下面的。
type Tip struct {
	Start, End int
	Code       string
	Build      func(c *wui.Context)
}

// ShowHover 开悬停提示（和 State.ShowHover 一样）。
func (d *Doc) ShowHover(start, end int, code string, build func(c *wui.Context)) {
	if d.UsingEd() {
		d.EdTip = &Tip{Start: start, End: end, Code: code, Build: build}
		return
	}
	d.St.ShowHover(start, end, code, build)
}

// EditorFocused 是编辑器上一帧有键盘焦点（ged 的或 mygo 的，看这一帧用的哪个）。
func (d *Doc) EditorFocused() bool {
	if d.UsingEd() {
		return d.Ed.Focused()
	}
	return d.St.Focused()
}
