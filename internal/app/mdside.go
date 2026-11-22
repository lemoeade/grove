package app

import (
	"slices"

	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
)

// 这个文件是并排的 Markdown 预览：源文件留在这一组接着改，旁边一组放一个跟着它画的预览标签（docs.MDPreview），
// 编辑器滚到哪预览跟到哪（ui 的 markdownPreview 看源文件编辑器最上面那一行）。

// sidePreviewKey 是 d 的并排预览标签的 Key（和 Doc.Key 算出来的一样）。
func sidePreviewKey(d *docs.Doc) docs.Key {
	k := docs.Key{Kind: docs.MDPreview, Root: d.Root, Path: d.Rel}
	if d.Root == "" {
		k.Path = d.Abs
	}
	return k
}

// SidePreviewOf 是开着的 d 的并排预览标签，没有是 nil。
func (a *App) SidePreviewOf(d *docs.Doc) *docs.Doc {
	if d == nil || d.Kind != docs.File {
		return nil
	}
	g, i := a.locate(sidePreviewKey(d))
	if g == nil {
		return nil
	}
	ds, _ := a.GroupDocs(g)
	return ds[i]
}

// CanSidePreview 是当前标签能不能开 / 关并排预览：Markdown 文件，或者就是一个并排预览。
func (a *App) CanSidePreview() bool {
	d := a.CurrentDoc()
	return d != nil && (d.Kind == docs.MDPreview || d.Kind == docs.File && d.CanPreview() && docs.IsMarkdown(d.Name()))
}

// sidePreviewOn 是菜单里“在旁边预览”打不打勾：当前标签旁边开着预览，或者它就是一个并排预览。
func (a *App) sidePreviewOn() bool {
	d := a.CurrentDoc()
	return d != nil && (d.Kind == docs.MDPreview || a.SidePreviewOf(d) != nil)
}

// ToggleSidePreview 是 ⌥⇧⌘V：Markdown 文件旁边没有预览就在旁边一组开一个（只有一组时向右分出一组），键盘留在源文件；
// 已经有了就关掉它。在并排预览上按是关掉它自己，键盘回源文件。
func (a *App) ToggleSidePreview() {
	d := a.CurrentDoc()
	switch {
	case d == nil:
		return
	case d.Kind == docs.MDPreview:
		src := d.MDSource
		a.RemoveDoc(d)
		if src != nil && slices.Contains(a.AllDocs(), src) {
			a.focusDocIn(src)
		}
		return
	case d.Kind != docs.File || !d.CanPreview() || !docs.IsMarkdown(d.Name()):
		return
	}
	if pv := a.SidePreviewOf(d); pv != nil {
		a.RemoveDoc(pv)
		a.focusDocIn(d)
		return
	}
	from, side := a.Grp, a.sideOf(a.Grp)
	if side == nil && !from.roomFor(true) {
		a.Toast = i18n.T("这一组太小，分不开了")
		return
	}
	pv := &docs.Doc{Kind: docs.MDPreview, Root: d.Root, Rel: d.Rel, Abs: d.Abs, Title: i18n.Tf("预览 %s", d.Title), MDSource: d}
	if first, last := d.VisibleLines(); last > 0 && first > 0 {
		pv.AnchorLine = first + 1 // 一开就是编辑器正看着的那一节
	}
	a.Docs = append(a.Docs, pv)
	pv.Pinned = true
	if side == nil {
		a.splitOff(pv, from, SideRight)
	} else {
		a.MoveDocTo(pv, side, -1)
	}
	a.focusDocIn(d)
}

// focusDocIn 把键盘还给 d 所在的组、选中 d、交给它的编辑器。
func (a *App) focusDocIn(d *docs.Doc) {
	g := a.GroupOf(d)
	if g == nil {
		return
	}
	a.swapTo(g)
	if i := slices.Index(a.Docs, d); i >= 0 {
		a.SelectDoc(i)
	}
	a.FocusDoc = true
	a.syncChangeSel()
}

// closeSidePreviews 在 d 关了以后（RemoveDoc 末尾调）处理跟着它的并排预览：同一个文件还有别的份开着就改跟那一份，没有就关掉。
func (a *App) closeSidePreviews(d *docs.Doc) {
	if d.Kind != docs.File {
		return
	}
	all := a.AllDocs()
	var other *docs.Doc
	if i := slices.IndexFunc(all, func(o *docs.Doc) bool { return o != d && o.Kind == docs.File && o.Key() == d.Key() }); i >= 0 {
		other = all[i]
	}
	for _, pv := range all {
		if pv.Kind != docs.MDPreview || pv.MDSource != d {
			continue
		}
		if other != nil {
			pv.MDSource = other
		} else {
			a.RemoveDoc(pv)
		}
	}
}
