package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lemoeade/grove/internal/testutil"
)

// 窄右栏（300）：上周的行尾只写星期几；别的分支的胶囊只写名字最后一段；仓库名写不全时头部不写分支名
// （宽了才写）；展开的提交里两个同名文件的目录省掉中间、留下能分开它们的最后一级。
// 展开第二个不收第一个；收起再展开读过的提交时文件马上就在。HIST_NARROW_PNG=<目录> 时存一张截图看。
func TestHistNarrow(t *testing.T) {
	repo := testutil.NewRepo(t, "cppt-platform-workbench-proto")
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, time.Local)
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	lastWeek := monday.AddDate(0, 0, -5) // 上周三
	repo.Write("a.txt", "0\n")
	repo.CommitAt("docs: 初始提交", lastWeek.Add(-time.Hour))
	repo.Write("a.txt", "1\n")
	repo.CommitAt("docs: 同步成就徽章的说明和截图", lastWeek)
	repo.Git("branch", "codex/achievement-badges")
	repo.Write(".agents/skills/app-portal/SKILL.md", "a\n")
	repo.Write(".agents/skills/cppt-reports/SKILL.md", "b\n")
	repo.CommitAt("docs: 1115", today)

	a, tt, wait, r := histUI(t, repo, 3)
	const winW = 1100
	tt.SetSize(winW, 600)
	tt.SetScale(2)
	frames := func() {
		for range 3 {
			tt.Frame()
		}
	}
	// 头部的分支名：右栏里、列表上面那一条（标题栏的分支胶囊也写着 main，在右栏左边之外或更上面）
	headBranch := func() bool {
		for _, n := range accessTree(t, tt) {
			if n.Label == "main" && n.X > float64(winW-a.St.RightW) && n.Y > 40 && n.Y < 40+histHeadH {
				return true
			}
		}
		return false
	}
	a.St.RightW = 600
	frames()
	if !headBranch() {
		t.Fatalf("宽右栏头部没写分支名；texts=%q", tt.Texts())
	}

	a.St.RightW = histNarrow
	frames()
	if headBranch() {
		t.Error("窄右栏仓库名都写不全，头部还写着分支名")
	}
	if tt.HasText("codex/achievement-badges") || !tt.HasText("achievement-badges") {
		t.Errorf("窄右栏的分支胶囊该只写最后一段；texts=%q", tt.Texts())
	}
	ci := 1
	if r.Hist.Commits[ci].Subject != "docs: 同步成就徽章的说明和截图" {
		t.Fatalf("第二个提交是 %q", r.Hist.Commits[ci].Subject)
	}
	if when := rowWhen(r.Hist.Commits[ci].Date, true); !tt.HasText(when) || when != rowWhen(lastWeek, true) {
		t.Errorf("上周的行尾该是 %q；texts=%q", rowWhen(lastWeek, true), tt.Texts())
	}

	mustClickRow(t, tt, "docs: 1115")
	wait("展开", func() bool {
		return r.Hist.Expanded != "" && r.Hist.FilesFor == r.Hist.Expanded && !r.Hist.FilesLoading
	})
	frames()
	for _, d := range []string{"app-portal", "cppt-reports"} {
		if !tt.HasText(".agents/…/"+d) && !tt.HasText("…/"+d) {
			t.Errorf("目录没省掉中间、留下 %s；texts=%q", d, tt.Texts())
		}
	}
	// 再展开一个：前一个照旧开着
	mustClickRow(t, tt, "docs: 同步成就徽章的说明和截图")
	wait("展开第二个", func() bool { return !r.Hist.FilesLoading && r.Hist.FilesFor == r.Hist.Expanded })
	if len(r.Hist.Open) != 2 {
		t.Errorf("展开第二个把第一个收起了：%v", r.Hist.Open)
	}
	// 收起再展开：读过的文件从缓存拿，点下去这一帧就排好，不再转圈、不再写"读取文件…"
	mustClickTitle(t, tt, "docs: 1115")
	if r.Hist.Open[r.Hist.Commits[0].Hash] {
		t.Fatal("点开着的提交的标题没收起")
	}
	mustClickTitle(t, tt, "docs: 1115")
	if r.Hist.FilesLoading || r.Hist.FilesFor != r.Hist.Expanded || len(r.Hist.Files) != 2 {
		t.Errorf("切回读过的提交还要再读：loading=%v files=%d", r.Hist.FilesLoading, len(r.Hist.Files))
	}
	if dir := os.Getenv("HIST_NARROW_PNG"); dir != "" {
		savePNG(t, filepath.Join(dir, "hist-narrow.png"), tt.Image())
	}
}
