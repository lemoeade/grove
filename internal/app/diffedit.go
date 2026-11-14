package app

import (
	"slices"

	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/git"
)

// 这个文件是 Diff 里就地编辑（行为参考 Pierre Diffs 的 Edit mode）：比工作区那一边的 Diff（未暂存、未跟踪）
// 点标签条上的铅笔后，新的一边可以直接改。改的是同一个文件标签的缓冲区（没开就在后面开一个），撤销、保存都是那个标签的，
// ⌘S 在 Diff 里也行。局部 / 全文、并排 / 统一照旧能切；编辑时不出块级的暂存、丢弃（磁盘上的和缓冲区里的会对不上），
// J / K 也不再跳改动（要打字）。

// CanEditDiff 是这个 Diff 标签能不能就地编辑：比的是工作区里的文件（未暂存、未跟踪），文本 diff，文件还在。
func (a *App) CanEditDiff(d *docs.Doc) bool {
	if d == nil || d.Kind != docs.Diff || d.DiffSt == nil || d.Change.Code == 'D' || d.Change.Dir || d.Change.Sub != "" {
		return false
	}
	if d.Change.Kind != git.Unstaged && d.Change.Kind != git.Untracked {
		return false
	}
	return a.RepoOf(d) != nil
}

// ToggleDiffEdit 开关 Diff 里的就地编辑：开的时候文件没开着就在后面开一个（固定的标签），焦点留在 Diff 上，
// Diff 标签也固定下来（正改着的不该被下一个预览顶掉）。关的时候文件标签留着，没存的改动还在它里面。
func (a *App) ToggleDiffEdit(d *docs.Doc) {
	if d == nil {
		return
	}
	if d.EditDoc != nil {
		d.StopEdit()
		a.loadDoc(d) // 新的一边换回从磁盘读的
		return
	}
	if !a.CanEditDiff(d) {
		return
	}
	f := a.OpenFile(a.RepoOf(d), d.Change.Path, true)
	a.reuse(d.Key(), true) // 回到 Diff
	d.EditDoc = f
}

// BindDiffEdit 每帧画 Diff 之前调：就地编辑时新的一边接上文件标签现在的缓冲区（重读会换缓冲区，ApplyDiff 会换 DiffState），
// 文件标签关了、或者这个 Diff 不再是比工作区的（暂存了）就退回只看。文件标签还没读完、不能编辑（编码、太大）时先只看。
func (a *App) BindDiffEdit(d *docs.Doc) {
	if d.DiffSt == nil {
		return
	}
	f := d.EditDoc
	if f != nil && (!slices.Contains(a.AllDocs(), f) || !a.CanEditDiff(d)) {
		d.StopEdit()
		a.loadDoc(d)
		return
	}
	if f == nil || f.Buf == nil || f.Loading || !f.Editable() {
		d.DiffSt.Editable = false
		return
	}
	if d.DiffSt.New != f.Buf {
		d.DiffSt.New = f.Buf
	}
	d.DiffSt.Editable = true
}

// DiffEditing 是这个 Diff 标签正在就地编辑。
func DiffEditing(d *docs.Doc) bool { return d != nil && d.EditDoc != nil }

// DiffDirty 是就地编辑着的 Diff 标签有没保存的改动（在它的文件标签里）。
func DiffDirty(d *docs.Doc) bool { return d != nil && d.EditDoc != nil && d.EditDoc.Modified() }
