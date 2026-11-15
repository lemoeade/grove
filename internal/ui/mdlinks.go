package ui

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
)

// 这个文件是预览里链接的右键菜单和悬停提示：网址、#标题、相对的文件。

var reLineSuffix = regexp.MustCompile(`:\d+(:\d+)?$`)

// mdLinkPath 是链接指向的本地文件（相对 d 所在的目录，去掉 #片段、?查询、:行:列）；网址、#标题不是。
func mdLinkPath(d *docs.Doc, u string) (string, bool) {
	if u == "" || strings.HasPrefix(u, "#") {
		return "", false
	}
	if i := strings.Index(u, ":"); i > 1 && !reLineSuffix.MatchString(u[i:]) && !filepath.IsAbs(u) {
		if !strings.HasPrefix(strings.ToLower(u), "file:") {
			return "", false
		}
	}
	p := strings.TrimPrefix(strings.TrimPrefix(u, "file://"), "file:")
	p, _, _ = strings.Cut(p, "#")
	p, _, _ = strings.Cut(p, "?")
	if s, err := url.PathUnescape(p); err == nil {
		p = s
	}
	p = reLineSuffix.ReplaceAllString(p, "")
	if p == "" {
		return "", false
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(d.Dir(), filepath.FromSlash(p))
	}
	return filepath.Clean(p), true
}

// mdLinkMenu 是链接的右键菜单：打开、复制链接；本地文件还有复制路径、在访达中显示；后面是选中文字的拷贝、全选。
func mdLinkMenu(a *app.App, d *docs.Doc, m *wui.Menu, u string) {
	if m.Item(i18n.T("打开链接")).Chosen() {
		a.MarkdownLink(d, u)
	}
	if m.Item(i18n.T("复制链接")).Chosen() {
		app.CopyText(u)
	}
	if p, ok := mdLinkPath(d, u); ok {
		_, err := os.Stat(p)
		if it := m.Item(i18n.T("复制路径")); it.Chosen() {
			app.CopyText(p)
		}
		it := m.Item(i18n.T("在访达中显示"))
		if err != nil {
			it.Disabled(true)
		}
		if it.Chosen() {
			if err := a.ShowPath(p); err != nil {
				a.Fail(i18n.T("在访达中显示失败"), err)
			}
		}
	}
	m.Separator()
	m.EditItems()
}

// mdLinkPreview 是停在链接上时的提示：本地文件显示它在仓库里的路径，找不到的说一声；网址、#标题只显示地址（mygo 默认的）。
func mdLinkPreview(a *app.App, d *docs.Doc, c *wui.Context, dest string) func(wui.Element) {
	p, ok := mdLinkPath(d, dest)
	if !ok {
		return nil
	}
	show := p
	if d.Root != "" {
		if rel, err := filepath.Rel(d.Root, p); err == nil && !strings.HasPrefix(rel, "..") {
			show = filepath.ToSlash(rel)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && show == p && strings.HasPrefix(p, home+string(filepath.Separator)) {
		show = "~" + p[len(home):]
	}
	if m := reLineSuffix.FindString(dest); m != "" {
		show += m
	}
	_, err := os.Stat(p)
	t := c.Theme()
	return func(tip wui.Element) {
		tipStyle(t, tip).MaxWidth(480)
		tip.Children(func() {
			wui.Column(c).Gap(2).Children(func() {
				wui.Text(c, show)
				if err != nil {
					wui.Text(c, i18n.T("找不到这个文件")).TextColor(t.Danger)
				}
			})
		})
	}
}
