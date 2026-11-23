package ui

import (
	"testing"
	"time"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/testutil"
)

// 设置的“更新”页：没接 updater（测试、go run）时开关灰着、写“这个构建不能自己更新”；接上以后开关改的是插件里的值，
// “自动下载”跟着“自动检查”，“检查更新…”和菜单命令都走 Updates.Check。
func TestSettingsUpdates(t *testing.T) {
	t.Cleanup(func() { app.Updates = nil })
	r0 := testutil.NewRepo(t, "work")
	r0.Write("a.txt", "a\n")
	r0.Commit("初始提交")
	a, _, wait := newUI(t, testutil.Workspace(t, r0.Root), t.TempDir())

	app.Updates = nil
	if a.CanRun(app.Find("app.checkUpdates")) {
		t.Fatal("没接 updater 时“检查更新…”该灰着")
	}
	st := wui.NewTester(SettingsView(a), 640, 560)
	st.SetLocale(i18n.Locale())
	st.Frame()
	mustClick(t, st, i18n.T("更新"))
	if !st.HasText(i18n.T("这个构建不能自己更新")) {
		t.Fatalf("没接 updater 时该说不能自己更新：%q", st.Texts())
	}

	autoCheck, autoDL, checks := false, false, 0
	app.Updates = &app.UpdateHooks{
		Enabled:         func() bool { return true },
		Check:           func() { checks++ },
		AutoCheck:       func() bool { return autoCheck },
		SetAutoCheck:    func(v bool) { autoCheck = v },
		AutoDownload:    func() bool { return autoDL },
		SetAutoDownload: func(v bool) { autoDL = v },
		LastCheck:       func() time.Time { return time.Now().Add(-2 * time.Hour) },
		Available:       func() string { return "" },
	}
	st.Frame()
	if st.HasText(i18n.T("这个构建不能自己更新")) {
		t.Fatal("接上 updater 后不该再说不能自己更新")
	}
	mustClick(t, st, i18n.T("自动检查更新"))
	st.Frame()
	if !autoCheck {
		t.Fatal("点“自动检查更新”没改到插件里")
	}
	mustClick(t, st, i18n.T("自动下载并安装更新"))
	st.Frame()
	if !autoDL {
		t.Fatal("开了自动检查后点“自动下载并安装更新”没改到插件里")
	}
	mustClick(t, st, i18n.T("检查更新…"))
	st.Frame()
	wait("检查更新", func() bool { return checks == 1 })
	if !a.CanRun(app.Find("app.checkUpdates")) {
		t.Fatal("接了 updater 后“检查更新…”该能点")
	}
	a.Exec("app.checkUpdates")
	if checks != 2 {
		t.Fatalf("命令没走 Updates.Check：%d 次", checks)
	}
	app.Updates.Available = func() string { return "0.3.0" }
	st.Frame()
	if !st.HasText(i18n.Tf("%s 可以更新", "0.3.0")) {
		t.Fatalf("找到新版本该写出来：%q", st.Texts())
	}
}
