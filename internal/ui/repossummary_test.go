package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/git"
)

// 状态栏左边是所有仓库的总况：几个仓库、几个有改动 / 要推送、谁在合并，读取失败的单独红字；
// 只有一个仓库时是它自己的改动；不是 git 仓库的文件夹不算。
func TestReposSummary(t *testing.T) {
	repo := func(name string, st git.Status) *app.Repo {
		r := &app.Repo{Name: name, Loaded: true}
		r.Status = st
		return r
	}
	clean := repo("docs", git.Status{Branch: "main"})
	dirty := repo("api", git.Status{Branch: "main", Count: 2, Unstaged: 2})
	ahead := repo("web", git.Status{Branch: "main", Ahead: 1})
	merging := repo("infra", git.Status{Branch: "main", Count: 1, Conflicts: 1})
	merging.Extra.Ongoing = git.Merging
	broken := &app.Repo{Name: "old", Err: errors.New("坏了")}
	plain := &app.Repo{Name: "notes", Plain: true}

	text, bad, tip := reposSummary([]*app.Repo{clean, dirty, ahead, merging, broken, plain})
	if want := "5 个仓库 · 2 个有改动 · 1 个要推送 · infra 合并中"; text != want {
		t.Errorf("总况 %q，应是 %q", text, want)
	}
	if bad != "1 个读取失败" {
		t.Errorf("读取失败 %q", bad)
	}
	for _, s := range []string{"api  未暂存 2", "web  工作区干净 · ↑1", "infra  冲突 1 · 合并中", "old  git status 失败"} {
		if !strings.Contains(tip, s) {
			t.Errorf("提示里应有 %q：%q", s, tip)
		}
	}
	if strings.Contains(tip, "notes") {
		t.Error("不是 git 仓库的文件夹不该算进去")
	}

	if text, _, _ := reposSummary([]*app.Repo{clean, ahead}); text != "2 个仓库 · 都干净 · 1 个要推送" {
		t.Errorf("都干净：%q", text)
	}
	if text, _, _ := reposSummary([]*app.Repo{dirty, plain}); text != "未暂存 2" {
		t.Errorf("只有一个仓库时写它自己的改动：%q", text)
	}
	if text, bad, _ := reposSummary([]*app.Repo{plain}); text != "" || bad != "" {
		t.Errorf("没有 git 仓库时什么都不写：%q %q", text, bad)
	}
}
