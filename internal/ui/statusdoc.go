package ui

import (
	"fmt"
	"slices"
	"strings"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/git"
	"github.com/lemoeade/grove/internal/highlight"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/workspace"
)

// 这个文件是状态栏右边关于当前标签的几项（docStatus，statusbar.go 摆进去）：光标位置、语言、缩进、换行、编码、
// 大小、状态，和点语言 / 缩进 / 换行 / 编码弹的菜单（编码那几项也给只读打开时的编码条 encodingBar 用）。

// docStatus 是状态栏右边关于当前标签的几项：光标位置、语言、换行、编码、大小、状态。
func docStatus(a *app.App, c *wui.Context, d *docs.Doc) {
	t := c.Theme()
	muted := t.TextMuted
	item := func(s string) wui.Element {
		return wui.Text(c, s).FontSize(10.5).TextColor(muted).SingleLine().Shrink(0)
	}
	if d.Bitmap != nil {
		w, h := d.Bitmap.Size()
		item(fmt.Sprintf("%d × %d · %s", w, h, zoomLabel(d.Img.Shown)))
	}
	if d.Hex && d.Buf != nil { // 十六进制：光标那个字节的偏移，没有语言、缩进、换行、编码
		off := docs.HexOffset(d.St.CursorPos())
		item(i18n.Tf("偏移 0x%X（%s）", off, docs.Grouped(off)))
	} else if d.Kind != docs.Diff && d.Buf != nil {
		line, col, sels, chars := d.CursorInfo()
		pos := i18n.Tf("行 %d，列 %d", line, col)
		switch {
		case sels > 1:
			pos += i18n.Tf("（%d 处选中）", sels)
		case chars > 0:
			pos += i18n.Tn("（选中 %d 字）", chars)
		}
		item(pos)
		lang := i18n.T("纯文本")
		if l := d.Buf.Language(); l != nil {
			lang = l.Name
		}
		filterMenuButton(a, c, item(lang).Label(i18n.Tf("语言 %s", lang)).Tooltip(i18n.T("按哪种语言高亮、缩进和注释；点开可以换一种")),
			menuOwner{"status.lang", d.Where()}, i18n.T("打字筛选语言"), func(m menu) { languageMenu(a, m, d) })
		if d.Kind == docs.File {
			ind := a.IndentLabel(d)
			menuButton(a, c, item(ind).Label(i18n.Tf("缩进 %s", ind)).Tooltip(i18n.T("这个文件的缩进；点开可以换，或把缩进转成空格 / Tab")),
				menuOwner{"status.indent", d.Where()}, func(m menu) { indentMenu(a, m, d) })
		}
		if d.Kind != docs.TermText && d.Kind != docs.SearchDoc && d.Kind != docs.SettingsDoc {
			eol := "LF"
			if d.Buf.LineEnding() == "\r\n" {
				eol = "CRLF"
			}
			x := item(eol).Tooltip(i18n.T("换行符"))
			if d.Editable() {
				menuButton(a, c, x.Label(i18n.Tf("换行 %s", eol)).Tooltip(i18n.T("换行符；点开可以换成 LF 或 CRLF，保存时按它写")),
					menuOwner{"status.eol", d.Where()}, func(m menu) { eolMenu(a, m, d) })
			}
			enc := item(d.Enc.Label()).Tooltip(i18n.T("文件的编码；保存按同样的编码写回"))
			if d.Kind == docs.File || d.Kind == docs.Version {
				menuButton(a, c, enc.Label(i18n.Tf("编码 %s", d.Enc.Label())).Tooltip(i18n.T("文件的编码；点开可以用别的编码重新打开")),
					menuOwner{"status.enc", d.Where()}, func(m menu) { encodingMenu(a, m, d) })
			}
		}
	}
	if d.Size > 0 || d.Buf != nil {
		item(docs.HumanSize(d.Size))
	}
	if d.ShowBlame && d.Blame != nil {
		x := item(i18n.Tn("blame · %d 次提交", app.BlameCommits(d.Blame))).Tooltip(app.Tip(i18n.T("关掉 blame 装订线"), "view.blame"))
		if x.Clicked() {
			a.ToggleBlame()
		}
	}
	if d.Formatting {
		item(i18n.T("正在格式化…"))
	}
	if st := app.SnippetStatus(d); st != "" {
		item(st).FontWeight(600).TextColor(t.Text).Tooltip(i18n.T("在片段的跳位之间：Tab 下一个，⇧Tab 上一个，Esc 结束"))
	}
	if n := len(d.ConflictBlocks()); n > 0 {
		x := item(i18n.Tn("%d 处冲突", n)).FontWeight(600).TextColor(t.Danger).Label(i18n.T("下一处冲突")).Tooltip(app.Tip(i18n.T("下一处冲突"), "merge.next"))
		if x.Clicked() {
			a.GoToConflict(d, 1)
		}
	}
	r := a.RepoOf(d)
	switch {
	case d.Kind == docs.Version:
		item(i18n.Tf("只读 · 提交 %s", git.Short(d.Commit)))
	case app.DiffDirty(d):
		item(app.KeyHint("file.save", "就地编辑 · 没保存 · %s", "就地编辑 · 没保存")).FontWeight(600).TextColor(t.Warning)
	case app.DiffEditing(d):
		item(i18n.Tf("就地编辑 · %s", d.Change.Kind.String()))
	case d.Kind == docs.Diff:
		item(i18n.Tf("只读 · %s", d.Change.Kind.String()))
	case d.Kind == docs.TermText:
		item(i18n.T("只读 · 终端输出"))
	case d.Kind == docs.SearchDoc:
		item(i18n.T("只读 · 搜索结果"))
	case d.Kind == docs.ReplacePreview:
		item(i18n.T("只读 · 替换预览，没有写盘"))
	case d.Kind == docs.Compare && d.Snapshot != 0:
		item(i18n.T("只读 · 本地历史"))
	case d.Kind == docs.Compare:
		item(i18n.T("只读 · 比较"))
	case d.Kind == docs.MDPreview:
		item(app.KeyHint("view.markdownSidePreview", "跟着源文件 · %s 关掉", "跟着源文件"))
	case d.Kind == docs.Term:
		if _, tab := a.TermTabOf(d); tab != nil {
			if p := tab.Current(); p.Exited {
				item(i18n.Tf("已退出 %d", p.ExitCode))
			} else if p.Title != "" {
				item(p.Title)
			}
		}
		item(i18n.T("终端"))
	case d.Kind == docs.Rebase:
		item(i18n.Tf("交互式变基 · %s", app.RebaseCounts(d.Rebase)))
	case d.Kind == docs.SettingsDoc && d.Modified():
		item(app.KeyHint("file.save", "没应用 · %s", "没应用")).FontWeight(600).TextColor(t.Warning)
	case d.Kind == docs.SettingsDoc:
		item(i18n.T("和现在的设置一样"))
	case d.Modified():
		item(app.KeyHint("file.save", "未保存 · %s", "未保存")).FontWeight(600).TextColor(t.Warning)
	case r != nil && app.ChangedIn(r, d.Rel) != 0:
		item(i18n.T("有未提交的改动"))
	}
}

// encodingMenu 是状态栏编码那项的菜单（文件、历史版本）：用别的编码重新打开，能转的转成 UTF-8。
func encodingMenu(a *app.App, m menu, d *docs.Doc) {
	encodingChoices(a, m, d)
	if d.Convertible() {
		m.Separator()
		if m.Item(i18n.T("转成 UTF-8…")).Chosen() {
			a.ConvertUTF8(d)
		}
	}
}

// encodingChoices 是"用别的编码重新打开"那几项：当前的打勾，手动选过的多一项"自动识别"。
func encodingChoices(a *app.App, m menu, d *docs.Doc) {
	m.Item(i18n.T("用别的编码重新打开")).Disabled(true)
	cur := d.Enc.Family()
	for i, g := range docs.ChoiceGroups {
		if i > 0 {
			m.Separator()
		}
		for _, enc := range g {
			if m.Item(enc.Label()).Checked(enc == cur).Chosen() && enc != cur {
				a.ReopenWithEncoding(d, enc)
			}
		}
	}
	if d.ForcedEnc != "" {
		m.Separator()
		if m.Item(i18n.T("自动识别编码重新打开")).Chosen() {
			a.ReopenWithEncoding(d, "")
		}
	}
}

// languageMenu 是状态栏语言那项的菜单："纯文本"和 mygo 认识的语言（按英文名排，中文界面也一样），当前的打勾；
// 手动选过的多一项"按文件名自动识别"。
func languageMenu(a *app.App, m menu, d *docs.Doc) {
	cur := ""
	if l := d.Buf.Language(); l != nil {
		cur = l.Name
	}
	if m.Item(i18n.T("纯文本")).Checked(cur == "").Chosen() && cur != "" {
		a.SetDocLang(d, docs.PlainLang)
	}
	m.Separator()
	langs := slices.Clone(highlight.Languages())
	slices.SortFunc(langs, func(x, y *editor.Language) int {
		return strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
	})
	for _, l := range langs {
		if m.Item(l.Name).Checked(l.Name == cur).Chosen() && l.Name != cur {
			a.SetDocLang(d, l.Name)
		}
	}
	if d.Lang != "" {
		m.Separator()
		if m.Item(i18n.T("按文件名自动识别")).Chosen() {
			a.SetDocLang(d, "")
		}
	}
}

// eolMenu 是状态栏换行那项的菜单（能保存的文件）：LF / CRLF，换了算改过，保存时按它写。
func eolMenu(a *app.App, m menu, d *docs.Doc) {
	cur := d.Buf.LineEnding()
	for _, e := range [...]struct{ label, eol string }{{"LF", "\n"}, {"CRLF", "\r\n"}} {
		if m.Item(e.label).Checked(cur == e.eol).Chosen() {
			a.SetDocEOL(d, e.eol)
		}
	}
}

// indentMenu 是状态栏缩进那项的菜单：这个文件用的缩进（和设置里的几样一样，当前的打勾），能保存的文件还能把每行开头的
// 缩进转成空格 / Tab。
func indentMenu(a *app.App, m menu, d *docs.Doc) {
	cur := a.DocIndent(d)
	for _, v := range workspace.Indents {
		label := i18n.T(indentNames[v])
		if v == "" {
			label = i18n.T("按语言默认")
		}
		if m.Item(label).Checked(v == cur).Chosen() && v != cur {
			a.SetDocIndent(d, v)
		}
	}
	if d.Editable() {
		m.Separator()
		if m.Item(i18n.T("把缩进转成空格")).Chosen() {
			a.ConvertIndent(d, false)
		}
		if m.Item(i18n.T("把缩进转成 Tab")).Chosen() {
			a.ConvertIndent(d, true)
		}
	}
}
