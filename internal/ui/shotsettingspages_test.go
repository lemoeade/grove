package ui

import (
	"os"
	"testing"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/testutil"
)

// TestScreenshotSettingsPages 是设置窗口每一页的截图（WORKBENCH_SHOT=目录 时才跑），窗口高一点好看全一页：
//
//	settings-page-0 … settings-page-7   外观 … 语言，一组一块圆角底
//	settings-page-0-vibrancy            窗口底选了通透：“材质”在“窗口底”下面缩进着出来
func TestScreenshotSettingsPages(t *testing.T) {
	dir := os.Getenv("WORKBENCH_SHOT")
	if dir == "" {
		t.Skip("设置 WORKBENCH_SHOT=目录 才渲染截图")
	}
	shop := testutil.NewRepo(t, "shop")
	shop.Write("README.md", "# Shop\n")
	shop.Commit("初始")
	a, _, _ := newUI(t, testutil.Workspace(t, shop.Root), termState(t, false))
	shot := func(name string, page int) {
		t.Helper()
		st := wui.NewTester(SettingsView(a), 640, 1100)
		st.SetScale(2)
		st.SetLocale(i18n.Locale())
		st.Frame()
		mustClick(t, st, settingsPages[page])
		writeShot(t, st, dir, name)
	}
	for p := range keysPage {
		shot("settings-page-"+string(rune('0'+p))+".png", p)
	}
	a.St.Settings.Backdrop = "vibrancy"
	shot("settings-page-0-vibrancy.png", 0)
}
