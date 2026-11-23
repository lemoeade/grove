package app

import "time"

// UpdateHooks 是自动更新给界面的那几样：main 里接到 mygo 的 updater 插件（plugins/updater/native）上，
// 界面（设置的“更新”页、菜单的“检查更新…”）只经这里，不直接引插件。更新从 GitHub Releases 来，
// 用编进应用的公钥（mygo.json 的 updates.publicKey）验签，见 docs/release.md。
type UpdateHooks struct {
	Enabled         func() bool      // 这个构建能自己更新：带了更新的正式构建，装的地方写得进去（go run、开发构建不行）
	Check           func()           // 用户要检查：开更新窗口，最新、出错、开发构建不能更新都在窗口里说
	AutoCheck       func() bool      // 每天在后台检查一次
	SetAutoCheck    func(bool)       //
	AutoDownload    func() bool      // 有新版本不问就下载安装，下次启动生效
	SetAutoDownload func(bool)       //
	LastCheck       func() time.Time // 上次检查的时间，没检查过是零值
	Available       func() string    // 找到的新版本（跳过的不算），没有是空
}

// Updates 是这个进程的自动更新；测试和 go run 里为 nil（菜单项灰着，设置页写“这个构建没带自动更新”）。
var Updates *UpdateHooks

// CheckUpdates 是菜单 / 命令面板的“检查更新…”。
func (a *App) CheckUpdates() {
	if Updates != nil && Updates.Check != nil {
		Updates.Check()
	}
}
