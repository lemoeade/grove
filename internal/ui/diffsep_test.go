package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	wui "github.com/egoist/mygo/ui"
	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/git"
	"github.com/lemoeade/grove/internal/testutil"
)

var sepText = regexp.MustCompile(`^(\d+) 行未修改$`)

// folded 是 Diff 里折叠条上写的行数（从小到大，去重：Texts 里一条会出现不止一次）。
func folded(tt *wui.Tester) []int {
	var ns []int
	seen := map[int]bool{}
	for _, s := range tt.Texts() {
		if m := sepText.FindStringSubmatch(s); m != nil {
			n, _ := strconv.Atoi(m[1])
			if !seen[n] {
				seen[n] = true
				ns = append(ns, n)
			}
		}
	}
	sort.Ints(ns)
	return ns
}

// 折叠条是细线上的一颗胶囊：按住 ⇧ 点结尾那条一次全部展开，文件开头那条点一下往改动那边展开 100 行。
func TestDiffSeparatorExpand(t *testing.T) {
	repo := testutil.NewRepo(t, "diffsep")
	lines := make([]string, 300)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	repo.Write("p.txt", strings.Join(lines, "\n")+"\n")
	repo.Commit("初始提交")
	lines[149] = "line 150（改过）"
	repo.Write("p.txt", strings.Join(lines, "\n")+"\n")
	ws := testutil.Workspace(t, repo.Root)

	a, tt, wait := newUI(t, ws, "")
	a.St.FullDiff = false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "p.txt"}, true)
	wait("p.txt 的 diff", func() bool { return diffDoc(a, "") != nil && len(folded(tt)) == 2 })

	ns := folded(tt) // 改动上面一条（First）、下面一条（Last），都超过 100 行
	ctx := diffDoc(a, "").DiffSt.ContextLines
	if ctx == 0 {
		ctx = 3 // mygo 的默认
	}
	up, down := 149-ctx, 0
	if ns[0] != up && ns[1] != up {
		t.Fatalf("折叠条 %v，应有一条是改动上面的 %d 行", ns, up)
	}
	down = ns[0] + ns[1] - up
	if up <= 100 || down <= 100 {
		t.Fatalf("两条都应超过 100 行：%v", ns)
	}

	// 先 ⇧ 点结尾那条（开头那条展开 100 行以后它就滚出视图了）
	if err := tt.ClickWith(wui.Shift, fmt.Sprintf("展开 %d 行未修改", down)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := folded(tt); len(got) != 1 || got[0] != up {
		t.Fatalf("⇧ 点结尾那条应一次全部展开，只剩开头的 %d 行：%v", up, got)
	}

	if err := tt.Click(fmt.Sprintf("展开 %d 行未修改", up)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := folded(tt); len(got) != 1 || got[0] != up-100 {
		t.Fatalf("点开头那条应往改动那边展开 100 行，剩 %d：%v", up-100, got)
	}
}
