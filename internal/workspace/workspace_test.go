package workspace

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	ws := filepath.Join(dir, "a.code-workspace")
	data := "{\n  // 注释行\n  \"folders\": [{\"path\": \"x\"}, {\"path\": \"/abs/y\"}]\n}\n"
	if err := os.WriteFile(ws, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	roots, err := Load(ws)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "x"), "/abs/y"}
	if !reflect.DeepEqual(roots, want) {
		t.Errorf("roots = %v，要 %v", roots, want)
	}
	if err := os.WriteFile(ws, []byte(`{"folders": []}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(ws); err == nil {
		t.Error("没有 folders 应该报错")
	}
}

// 用户的真实工作区要能读出 5 个仓库（只读，没有就跳过）。
func TestLoadDefault(t *testing.T) {
	roots, err := Load(DefaultPath)
	if err != nil {
		t.Skipf("没有工作区文件: %v", err)
	}
	if len(roots) != 5 {
		t.Errorf("工作区有 %d 个仓库，期望 5 个", len(roots))
	}
}

func TestStateMigration(t *testing.T) {
	type want struct {
		sidebar, right, bottom bool
		mode                   string
	}
	cases := []struct {
		name, file, data string
		want             want
	}{
		{"没有文件", "", "", want{true, true, true, ModeFiles}},
		{"面板表 全开", StateFile, `{"panels":{"files":true,"changes":true,"history":true,"diff":true,"editor":false,"terminal":true}}`, want{true, true, true, ModeFiles}},
		{"面板表 只开变更", StateFile, `{"panels":{"files":false,"changes":true,"history":false,"diff":true,"editor":true,"terminal":false}}`, want{true, false, false, ModeGit}},
		{"面板表 文件和变更都关", StateFile, `{"panels":{"files":false,"changes":false,"history":true,"terminal":true}}`, want{false, true, true, ModeFiles}},
		{"面板表 缺项按开着算", StateFile, `{"panels":{"history":false}}`, want{true, false, true, ModeFiles}},
		{"四个布尔", StateFile, `{"panels":[true,false,true,false]}`, want{true, false, false, ModeFiles}},
		{"四个布尔 历史和终端开", StateFile, `{"panels":[false,true,false,true]}`, want{true, true, true, ModeFiles}},
		{"repos.json", "repos.json", `{"added":["/x"],"hidden":["/y"]}`, want{true, true, true, ModeFiles}},
		{"新版", StateFile, `{"version":2,"sidebar":false,"rightBar":true,"bottomBar":false,"sidebarMode":"search"}`, want{false, true, false, ModeSearch}},
		{"新版 模式读不懂", StateFile, `{"version":2,"sidebar":true,"rightBar":true,"bottomBar":true,"sidebarMode":"zzz"}`, want{true, true, true, ModeFiles}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.file != "" {
				if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			st, err := LoadState(dir)
			if err != nil {
				t.Fatal(err)
			}
			got := want{st.Sidebar, st.RightBar, st.BottomBar, st.SidebarMode}
			if got != tc.want {
				t.Errorf("迁移成 %+v，要 %+v", got, tc.want)
			}
			if st.Version != Version {
				t.Errorf("version = %d", st.Version)
			}
		})
	}
}

// 界面语言：5 以前的文件 lang 没写是中文，读成 "zh"；写了 "en" 的照旧；5 起没写是"按默认"，留空；没有文件也留空。
func TestStateLangMigration(t *testing.T) {
	for _, tc := range []struct{ name, data, want string }{
		{"没有文件", "", ""},
		{"原型时期", `{"panels":[true,false,true,false]}`, "zh"},
		{"4 没写", `{"version":4}`, "zh"},
		{"4 写了 en", `{"version":4,"settings":{"lang":"en"}}`, "en"},
		{"5 没写", `{"version":5}`, ""},
		{"5 写了 zh", `{"version":5,"settings":{"lang":"zh"}}`, "zh"},
		{"5 读不懂", `{"version":5,"settings":{"lang":"fr"}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.data != "" {
				if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(tc.data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			st, err := LoadState(dir)
			if err != nil {
				t.Fatal(err)
			}
			if st.Settings.Lang != tc.want {
				t.Errorf("lang = %q，要 %q", st.Settings.Lang, tc.want)
			}
			// 存回去再读：不再迁移
			if err := SaveState(dir, st); err != nil {
				t.Fatal(err)
			}
			if again, _ := LoadState(dir); again.Settings.Lang != tc.want {
				t.Errorf("存回去再读 lang = %q，要 %q", again.Settings.Lang, tc.want)
			}
		})
	}
}

// 老版本的最近搜索只记字（"searches"）：新的 searchHistory 空着时迁过来，开关都关着；存回去只写新的键。
// 两个都有（新版读过又被旧版加了几条）时只认新的。
func TestSearchHistoryMigration(t *testing.T) {
	dir := t.TempDir()
	write := func(data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"version":4,"searches":["needle","","TODO("]}`)
	st, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []SearchEntry{{Text: "needle"}, {Text: "TODO("}}; !reflect.DeepEqual(st.SearchHistory, want) {
		t.Fatalf("迁过来 %+v", st.SearchHistory)
	}
	if err := SaveState(dir, st); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, StateFile))
	if strings.Contains(string(data), `"searches"`) || !strings.Contains(string(data), `"searchHistory": [`) {
		t.Errorf("存回去：%s", data)
	}
	if again, _ := LoadState(dir); !reflect.DeepEqual(again.SearchHistory, st.SearchHistory) {
		t.Errorf("再读 %+v", again.SearchHistory)
	}

	write(`{"version":4,"searches":["old"],"searchHistory":[{"text":"new","regexp":true,"include":"*.go"}]}`)
	if st, _ = LoadState(dir); !reflect.DeepEqual(st.SearchHistory, []SearchEntry{{Text: "new", Regexp: true, Include: "*.go"}}) {
		t.Errorf("两个都有：%+v", st.SearchHistory)
	}
}

// 旧文件里的仓库列表、diff 设置和侧栏宽度要留下；存回去是新版，再读一样。
func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	old := `{"added":["/a"],"hidden":["/b"],"panels":{"files":false,"changes":true,"history":true,"terminal":false},"unified":true,"fullDiff":true,"sidebarW":300,"centerPct":60}`
	if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(st.Added, []string{"/a"}) || !reflect.DeepEqual(st.Hidden, []string{"/b"}) || !st.Unified || !st.FullDiff || st.SidebarW != 300 {
		t.Errorf("旧字段丢了: %+v", st)
	}
	st.RightW, st.BottomH = 360, 240
	if err := SaveState(dir, st); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, StateFile))
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["panels"]; ok {
		t.Error("新版不该再写 panels")
	}
	set, _ := raw["settings"].(map[string]any)
	if set["lang"] != "zh" { // 老版本默认中文，迁移时补上
		t.Errorf("老文件的语言没迁成中文：%v", set)
	}
	delete(set, "lang")
	if len(set) != 0 { // 别的设置全是默认（通透开、材质侧栏……）：一项都不写
		t.Errorf("默认设置不该写进文件：%v", set)
	}
	if raw["version"] != float64(Version) {
		t.Errorf("version = %v", raw["version"])
	}
	again, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, st) {
		t.Errorf("再读不一样:\n%+v\n%+v", again, st)
	}
}

// 第 2 版的文件（没有 sessions）照原样读进来；工作区记录（连同 Git 栏收放和分组、所有分支、提交信息历史、
// 终端光标闪烁）存了再读一样，第 3 版里收起的贮藏去掉，读不懂的标签丢掉。
func TestStateSessions(t *testing.T) {
	dir := t.TempDir()
	v2 := `{"version":2,"added":["/a"],"sidebar":true,"rightBar":false,"bottomBar":true,"sidebarMode":"git","sidebarW":280,"unified":true,"recent":["/a/x.go"]}`
	if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(v2), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Version != Version || st.RightBar || st.SidebarMode != ModeGit || st.SidebarW != 280 || !st.Unified ||
		!reflect.DeepEqual(st.Added, []string{"/a"}) || !reflect.DeepEqual(st.Recent, []string{"/a/x.go"}) || st.Sessions != nil {
		t.Fatalf("第 2 版读成了 %+v", st)
	}

	st.Sessions = map[string]*Session{
		"/w/one.code-workspace": {
			Tabs:   []Tab{{Path: "/a/x.go", Pinned: true, Line: 12, Col: 4, Top: 3}, {Path: "/a/README.md"}},
			Active: 1, Repo: "/a", Open: []string{"/a"}, Dirs: map[string][]string{"/a": {"docs", "docs/img"}},
			Git:    map[string]string{"/a": "closed", "/b": "open"},
			Groups: map[string][]string{"/a": {"stash", "untracked"}},
			Terms: map[string]*Terms{"/a": {Tabs: []TermTab{{Name: "构建", Dir: "/a/docs"}, {Sids: []string{"s1", "s2"}},
				{Split: &TermSplit{Row: true, Ratio: 0.4, A: &TermSplit{Hist: "1793001600000-3.vt"}, B: &TermSplit{Dir: "/a/src", Sid: "s3", Profile: "fish"}}},
				{InEditor: true, Num: 4}}, Active: 1}},
			Added: []string{"/x"}, Hidden: []string{"/b"},
			WtOpen: []string{"/a"},
			Layout: &Layout{Row: true, Ratio: 0.5, A: &Layout{Tabs: []int{0}}, B: &Layout{Tabs: []int{1, 2}, Sel: 1, Focus: true}},
			Panels: &Panels{Sidebar: true, BottomBar: true, SidebarMode: ModeGit, SidebarW: 300, TermW: 420, Unified: true, DiffTrim: true, HistAll: true, HistNoGraph: true, OpenEditorsFolded: true},
		},
	}
	st.Sessions["/w/one.code-workspace"].Tabs = append(st.Sessions["/w/one.code-workspace"].Tabs,
		Tab{Path: "/a", Kind: TabVersion, Repo: "/a", Change: &TabChange{Kind: "commit", Path: "a.go", Code: "M", Commit: "5fd1", Parent: "d1e9"}, Enc: "gbk", Hex: true},
		Tab{Path: "/a", Kind: TabDiff, Repo: "/a", Change: &TabChange{Kind: "unstaged", Path: "a.go", Code: "M"}},
		Tab{Path: "/a", Kind: TabTerm, Repo: "/a", TermNum: 4},
		Tab{Path: "/a/x.go", Copy: 1, Lang: "Go", Indent: "tab", Preview: true})
	st.AppVersion, st.OpenEditorsFolded, st.TermW, st.CloneDir = "0.2.0", true, 420, "/src"
	st.TaskHistory = []TaskRun{{Repo: "/a", Cmd: "make test"}}
	st.RecentWs = []string{"/w/one.code-workspace", "/w/dir"}
	st.HistNoGraph, st.DiffTrim = true, true
	st.HistBranches, st.CommitMsgs, st.Settings.TermBlink, st.Settings.TermTheme = true, []string{"修复登录", "初始提交"}, true, "Dracula"
	st.Settings.Material, st.Settings.Backdrop = "glass", "vibrancy"
	st.Settings.Accent, st.Settings.SolidSelection = "teal", true
	st.Settings.TreeNoFollow, st.Settings.TreeNoCompact, st.Settings.TreeNoGuides = true, true, true
	st.Settings.TreeNoStatusColors = true
	st.SearchHistory = []SearchEntry{{Text: "needle", Case: true, Word: true, Regexp: true, Include: "*.go", Exclude: "vendor/", Ignored: true, Big: true}, {Text: "TODO("}}
	st.Windows = []string{"/w/one.code-workspace", "", "/w/dir"}
	if err := SaveState(dir, st); err != nil {
		t.Fatal(err)
	}
	again, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, st) {
		t.Fatalf("再读不一样:\n%+v\n%+v", again.Sessions, st.Sessions)
	}

	// 第 3 版的 groups 是收起的分组：收起的贮藏现在是默认，去掉（只收了贮藏的仓库整个不记）；第 4 版的 "stash" 是点开了，照读
	v3 := `{"version":3,"sessions":{"/w/one.code-workspace":{"groups":{"/a":["stash","untracked"],"/b":["stash"]}},"/w/two.code-workspace":{"groups":{"/c":["stash"]}}}}`
	if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(v3), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, err = LoadState(dir); err != nil {
		t.Fatal(err)
	}
	if g := st.Sessions["/w/one.code-workspace"].Groups; !reflect.DeepEqual(g, map[string][]string{"/a": {"untracked"}}) {
		t.Errorf("第 3 版的分组读成了 %v", g)
	}
	if st.HistNoGraph {
		t.Error("旧文件没有 histNoGraph，应读成画提交图")
	}
	if g := st.Sessions["/w/two.code-workspace"].Groups; g != nil {
		t.Errorf("只收了贮藏的会话应不记分组: %v", g)
	}
	if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(strings.Replace(v3, `"version":3`, `"version":4`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, err = LoadState(dir); err != nil {
		t.Fatal(err)
	}
	if g := st.Sessions["/w/two.code-workspace"].Groups; !reflect.DeepEqual(g, map[string][]string{"/c": {"stash"}}) {
		t.Errorf("第 4 版的分组读成了 %v", g)
	}

	// 老版本存正面的 treeStatusColors（默认关）：写着 false 的读成关掉，true 和没写的照新的默认开着
	for data, off := range map[string]bool{
		`{"version":4,"settings":{"treeStatusColors":false}}`:  true,
		`{"version":4,"settings":{"treeStatusColors":true}}`:   false,
		`{"version":4,"settings":{}}`:                          false,
		`{"version":4,"settings":{"treeNoStatusColors":true}}`: true,
	} {
		if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		if st, err = LoadState(dir); err != nil {
			t.Fatal(err)
		}
		if st.Settings.TreeNoStatusColors != off {
			t.Errorf("%s 读成 treeNoStatusColors=%v", data, st.Settings.TreeNoStatusColors)
		}
	}

	// 老版本的 softWrap 开关（所有文件一起）：开着的读成 wrap "all"，没开、没写的照新的默认（只给文字文件换行）；
	// 已经写了 wrap 的照 wrap
	for data, want := range map[string]string{
		`{"version":4,"settings":{"softWrap":true}}`:              "all",
		`{"version":4,"settings":{"softWrap":false}}`:             "",
		`{"version":4,"settings":{}}`:                             "",
		`{"version":4,"settings":{"softWrap":true,"wrap":"off"}}`: "off",
	} {
		if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		if st, err = LoadState(dir); err != nil {
			t.Fatal(err)
		}
		if st.Settings.Wrap != want {
			t.Errorf("%s 读成 wrap=%q", data, st.Settings.Wrap)
		}
	}

	bad := `{"version":3,"sessions":{"/w/x.code-workspace":{"tabs":[{"path":"rel.go"},{"path":"/ok.go","line":-1},{"path":"/ok.go","line":2}],"active":7},"/w/nil.code-workspace":null}}`
	if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err = LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := st.Sessions["/w/x.code-workspace"]
	if len(st.Sessions) != 1 || s == nil || !reflect.DeepEqual(s.Tabs, []Tab{{Path: "/ok.go", Line: 2}}) || s.Active != -1 {
		t.Fatalf("读不懂的没丢干净：%+v", st.Sessions)
	}

	// 最近的工作区：相对路径和重复的丢掉，最多 MaxRecentWs 个；会话里加进来、移除的仓库也只认绝对路径
	recent := `"rel","/w/0","/w/0"`
	for i := range MaxRecentWs + 2 {
		recent += `,"/w/` + strconv.Itoa(i) + `"`
	}
	bad = `{"version":3,"recentWs":[` + recent + `],"sessions":{"/w/x.code-workspace":{"added":["rel","/x"],"hidden":["/b","b"]}}}`
	if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, err = LoadState(dir); err != nil {
		t.Fatal(err)
	}
	if len(st.RecentWs) != MaxRecentWs || st.RecentWs[0] != "/w/0" || st.RecentWs[1] != "/w/1" || st.RecentWs[MaxRecentWs-1] != "/w/"+strconv.Itoa(MaxRecentWs-1) {
		t.Errorf("最近的工作区没理干净：%q", st.RecentWs)
	}
	if s := st.Sessions["/w/x.code-workspace"]; !reflect.DeepEqual(s.Added, []string{"/x"}) || !reflect.DeepEqual(s.Hidden, []string{"/b"}) {
		t.Errorf("会话里的仓库：added=%q hidden=%q", s.Added, s.Hidden)
	}

	// 终端标签：相对路径的仓库根和目录丢掉，多了截掉，当前标签越界回到第一个；主题名带换行的不认
	tabs := `{"name":"t","dir":"/a"}`
	for range MaxTermTabs {
		tabs += `,{}`
	}
	bad = `{"version":3,"settings":{"termTheme":"x\ny","material":"chrome","backdrop":"blur"},"sessions":{"/w/x.code-workspace":{"terms":{"rel":{"tabs":[{}]},"/a":{"tabs":[{"dir":"rel"},` + tabs + `],"active":99},"/b":null}}}}`
	if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err = LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	ts := st.Sessions["/w/x.code-workspace"].Terms
	if len(ts) != 1 || ts["/a"] == nil || len(ts["/a"].Tabs) != MaxTermTabs || !reflect.DeepEqual(ts["/a"].Tabs[0], TermTab{Name: "t", Dir: "/a"}) || ts["/a"].Active != 0 {
		t.Fatalf("终端标签没理干净：%+v", ts)
	}
	if st.Settings.TermTheme != "" || st.Settings.Material != "" || st.Settings.Backdrop != "" {
		t.Fatalf("主题名 %q、材质 %q、窗口底 %q 应该不认", st.Settings.TermTheme, st.Settings.Material, st.Settings.Backdrop)
	}

	// 以前能选的 HUD、透明玻璃去掉了：存过的读回来是空（侧栏）
	for _, m := range []string{"hud", "glass-clear"} {
		if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(`{"version":4,"settings":{"material":"`+m+`"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if st, err = LoadState(dir); err != nil || st.Settings.Material != "" {
			t.Errorf("存的材质 %q 读成了 %q（%v）", m, st.Settings.Material, err)
		}
	}

	// 以前的通透背景开关（opaque）换成了窗口底：旧键读进来忽略，不报错，都成柔和；材质照留
	for _, o := range []string{"true", "false"} {
		if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(`{"version":4,"settings":{"opaque":`+o+`,"material":"menu"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if st, err = LoadState(dir); err != nil || st.Settings.Backdrop != "" || st.Settings.Material != "menu" {
			t.Errorf("opaque:%s 读成了窗口底 %q、材质 %q（%v）", o, st.Settings.Backdrop, st.Settings.Material, err)
		}
	}

	// 强调色：认识的照读；存成默认那个的、不认识的读回来是 ""（默认）；跟随系统（不是默认）存实名
	for v, want := range map[string]string{"teal": "teal", "steel": "steel", "graphite": "graphite", "system": "system", DefaultAccent: "", "blue": "blue", "forest": "forest", "": ""} {
		if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(`{"version":4,"settings":{"accent":"`+v+`"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if st, err = LoadState(dir); err != nil || st.Settings.Accent != want || AccentOf(st.Settings.Accent) != cmp.Or(want, DefaultAccent) {
			t.Errorf("存的强调色 %q 读成了 %q（%v）", v, st.Settings.Accent, err)
		}
	}
	if DefaultAccent != "black" || Accents[0] != "system" || Accents[1] != "" || len(Accents) != len(AccentKeys) || slices.Contains(Accents, DefaultAccent) {
		t.Errorf("强调色的选项 %q（默认 %q）", Accents, DefaultAccent)
	}
}

// 读的时候清理会话和几张表：该是绝对路径的（仓库根、工作区、文件）相对路径的丢掉，几张表截到上限；合法的原样留着。
func TestStateClamp(t *testing.T) {
	dir := t.TempDir()
	many := func(n int, f func(i int) string) string {
		var parts []string
		for i := range n {
			parts = append(parts, f(i))
		}
		return strings.Join(parts, ",")
	}
	data := `{"version":4,
	 "recent":["rel.go","/a/0.go","/a/0.go",` + many(MaxRecent+5, func(i int) string { return `"/a/` + strconv.Itoa(i) + `.go"` }) + `],
	 "commitMsgs":["", "  ",` + many(MaxCommitMsgs+5, func(i int) string { return `"m` + strconv.Itoa(i) + `"` }) + `],
	 "searchHistory":[{"text":""},` + many(MaxSearches+5, func(i int) string { return `{"text":"q` + strconv.Itoa(i) + `"}` }) + `],
	 "windows":["rel.code-workspace","","/w/one.code-workspace",""],
	 "added":["rel","/x"],
	 "sessions":{"/w/one.code-workspace":{
	   "repo":"rel","open":["rel","/a"],"wtOpen":["/a","b"],
	   "dirs":{"rel":["docs"],"/a":["docs","/abs",""],"/b":["/abs"]},
	   "git":{"rel":"open","/a":"closed","/b":"sideways"},
	   "groups":{"rel":["stash"],"/a":["stash"]}}}}`
	if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Recent) != MaxRecent || st.Recent[0] != "/a/0.go" || st.Recent[1] != "/a/1.go" {
		t.Errorf("最近的文件：%d 个 %q…", len(st.Recent), st.Recent[:min(3, len(st.Recent))])
	}
	if len(st.CommitMsgs) != MaxCommitMsgs || st.CommitMsgs[0] != "m0" {
		t.Errorf("提交信息：%d 条 %q…", len(st.CommitMsgs), st.CommitMsgs[:min(3, len(st.CommitMsgs))])
	}
	if len(st.SearchHistory) != MaxSearches || st.SearchHistory[0].Text != "q0" {
		t.Errorf("最近搜索：%d 条 %+v…", len(st.SearchHistory), st.SearchHistory[:min(3, len(st.SearchHistory))])
	}
	if !reflect.DeepEqual(st.Windows, []string{"", "/w/one.code-workspace", ""}) || !reflect.DeepEqual(st.Added, []string{"/x"}) {
		t.Errorf("窗口 %q、added %q", st.Windows, st.Added)
	}
	s := st.Sessions["/w/one.code-workspace"]
	want := &Session{Active: -1, Open: []string{"/a"}, WtOpen: []string{"/a"},
		Dirs:   map[string][]string{"/a": {"docs"}},
		Git:    map[string]string{"/a": "closed"},
		Groups: map[string][]string{"/a": {"stash"}}}
	if !reflect.DeepEqual(s, want) {
		t.Errorf("会话读成了 %+v，要 %+v", s, want)
	}

	// 合法的一点不丢：满上限的表、空工作区的窗口重复、仓库根下的目录
	full := Default()
	for i := range MaxRecent {
		full.Recent = append(full.Recent, "/a/"+strconv.Itoa(i)+".go")
	}
	for i := range MaxCommitMsgs {
		full.CommitMsgs = append(full.CommitMsgs, "修了第 "+strconv.Itoa(i)+" 个\n\n正文")
	}
	for i := range MaxSearches {
		full.SearchHistory = append(full.SearchHistory, SearchEntry{Text: "q" + strconv.Itoa(i), Regexp: i%2 == 0})
	}
	full.Windows = []string{"", "", "/w/a.code-workspace", "/w/dir"}
	full.Sessions = map[string]*Session{"/w/a.code-workspace": {Active: -1, Repo: "/a", Open: []string{"/a", "/b"}, WtOpen: []string{"/b"},
		Dirs: map[string][]string{"/a": {"docs", "docs/img"}}, Git: map[string]string{"/a": "open", "/b": "closed"},
		Groups: map[string][]string{"/b": {"staged"}}}}
	if err := SaveState(dir, full); err != nil {
		t.Fatal(err)
	}
	if again, err := LoadState(dir); err != nil || !reflect.DeepEqual(again, full) {
		t.Errorf("合法的读回来变了（%v）:\n%+v\n%+v", err, again, full)
	}
}

// docs/architecture/state.md 的 state.json 示例要写出 State、Session（连同它挂的 Tab、Terms、Layout、Panels……）的每个键：
// 加了字段却没补文档时这里报。Settings 的键太多，不要求都在示例里。
func TestStateDocKeys(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "architecture", "state.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	start := strings.Index(doc, "### state.json")
	if start < 0 {
		t.Fatal("docs/architecture/state.md 里没有 \"### state.json\" 一节")
	}
	doc = doc[start:]
	if i := strings.Index(doc, "```json"); i >= 0 {
		doc = doc[i:]
		if j := strings.Index(doc[len("```json"):], "```"); j >= 0 {
			doc = doc[:len("```json")+j]
		}
	}
	seen := map[reflect.Type]bool{}
	var walk func(typ reflect.Type, path string)
	walk = func(typ reflect.Type, path string) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] || typ == reflect.TypeOf(Settings{}) {
			return
		}
		seen[typ] = true
		for i := range typ.NumField() {
			f := typ.Field(i)
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			if !strings.Contains(doc, `"`+name+`"`) {
				t.Errorf("docs/architecture/state.md 的 state.json 示例里没有 %s.%s（%q）：补进示例", path, f.Name, name)
			}
			walk(f.Type, path+"."+f.Name)
		}
	}
	walk(reflect.TypeOf(State{}), "State")
}

func TestStateBadFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, StateFile), []byte("{坏"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(dir)
	if err == nil {
		t.Error("坏文件应该报错")
	}
	if !reflect.DeepEqual(st, Default()) {
		t.Errorf("坏文件应该回到默认: %+v", st)
	}
}

// Store：几个窗口共用的 state.json。存的是副本（State 改了不影响它）；没变就不写；没有目录不存；写出来的和 SaveState 一样。
func TestStore(t *testing.T) {
	dir := t.TempDir()
	st := Default()
	st.RecentWs, st.Sessions = []string{"/w/a"}, map[string]*Session{"/w/a": {Repo: "/a"}}
	s := NewStore(dir, st)
	st.RecentWs[0] = "/w/改了"
	got := s.State()
	got.RecentWs[0], got.Sessions["/w/b"] = "/w/也改了", &Session{}
	if again := s.State(); again.RecentWs[0] != "/w/a" || len(again.Sessions) != 1 {
		t.Fatalf("Store 拿的不是副本：%+v", again)
	}

	changed, err := s.Save(func(st *State) { st.Windows = []string{"/w/a", ""} })
	if !changed || err != nil {
		t.Fatalf("第一次存：%v %v", changed, err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, StateFile))
	other := t.TempDir()
	if err := SaveState(other, s.State()); err != nil {
		t.Fatal(err)
	}
	if want, _ := os.ReadFile(filepath.Join(other, StateFile)); string(b) != string(want) {
		t.Errorf("Store 写的和 SaveState 不一样：\n%s\n%s", b, want)
	}
	os.Remove(filepath.Join(dir, StateFile))
	if changed, _ := s.Save(func(st *State) { st.Windows = []string{"/w/a", ""} }); changed {
		t.Error("没变也写了")
	}
	if _, err := os.Stat(filepath.Join(dir, StateFile)); !os.IsNotExist(err) {
		t.Errorf("没变也写了：%v", err)
	}
	if changed, _ := NewStore("", Default()).Save(func(st *State) { st.Recent = []string{"/x"} }); changed {
		t.Error("没有目录也存了")
	}
}

// TestLoadSettings 读 .code-workspace 的 settings 和 .vscode/settings.json 的 files.exclude：JSONC（行尾注释、块注释、
// 多余的逗号），只认值为 true 的，带 when 的对象和 false 的跳过；没有 .vscode/settings.json 不算错。
func TestLoadSettings(t *testing.T) {
	dir := t.TempDir()
	ws := filepath.Join(dir, "a.code-workspace")
	os.WriteFile(ws, []byte(`{
	// 文件夹
	"folders": [{"path": "x"},],
	"settings": {
		"files.exclude": {
			"**/node_modules": true, // 行尾注释
			"build": true,
			/* 块注释 */ "dist": false,
			"**/*.js": {"when": "$(basename).ts"},
			"a//b": true,
		},
	},
}`), 0o644)
	s, err := LoadSettings(ws)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"**/node_modules", "a//b", "build"}; !slices.Equal(s.Exclude, want) {
		t.Errorf("Exclude = %q, 要 %q", s.Exclude, want)
	}
	if roots, err := Load(ws); err != nil || len(roots) != 1 {
		t.Errorf("Load = %v, %v（多余的逗号也认）", roots, err)
	}
	if s, err := LoadFolderSettings(dir); err != nil || len(s.Exclude) != 0 {
		t.Errorf("没有 .vscode/settings.json：%v, %v", s, err)
	}
	os.MkdirAll(filepath.Join(dir, ".vscode"), 0o755)
	os.WriteFile(filepath.Join(dir, ".vscode", "settings.json"), []byte("{\"files.exclude\": {\"*.log\": true, \"x\": false}}\n"), 0o644)
	if s, err := LoadFolderSettings(dir); err != nil || !slices.Equal(s.Exclude, []string{"*.log"}) {
		t.Errorf("LoadFolderSettings = %v, %v", s, err)
	}
	os.WriteFile(filepath.Join(dir, ".vscode", "settings.json"), []byte("{\"files.exclude\": "), 0o644)
	if _, err := LoadFolderSettings(dir); err == nil {
		t.Error("坏的 JSON 要报错")
	}
	if got := string(StripJSONC([]byte(`{"a": "x // y /* z */", "b": [1, 2, ], } // c`))); got != `{"a": "x // y /* z */", "b": [1, 2 ] } ` {
		t.Errorf("StripJSONC = %q", got)
	}
}

// 终端配置的 clamp：名字空的、重名的（留前面的）、命令空的、参数超过 200 字节的丢掉，环境里不是 K=V 的行去掉，
// 最多 10 个；名字和命令去掉两头空白。Profile 按名字找，"" 和没有的是 nil；Command 没参数是 -l。
func TestProfilesClamp(t *testing.T) {
	s := Settings{TermProfiles: []TermProfile{
		{Name: " fish ", Shell: " fish ", Env: []string{"A=1", "bad", "=x", "B C=2", "D="}},
		{Name: "", Shell: "/bin/zsh"},
		{Name: "fish", Shell: "/bin/bash"},
		{Name: "noshell"},
		{Name: "long", Shell: "/bin/sh", Args: []string{strings.Repeat("x", 201)}},
		{Name: "nu", Shell: "/opt/homebrew/bin/nu", Args: []string{"--login", "-e", "echo hi"}},
	}}
	for i := range 12 {
		s.TermProfiles = append(s.TermProfiles, TermProfile{Name: "p" + strconv.Itoa(i), Shell: "/bin/sh"})
	}
	s.Clamp()
	if len(s.TermProfiles) != MaxTermProfiles {
		t.Fatalf("留了 %d 个：%+v", len(s.TermProfiles), s.TermProfiles)
	}
	f := s.TermProfiles[0]
	if f.Name != "fish" || f.Shell != "fish" || !slices.Equal(f.Env, []string{"A=1", "D="}) || !slices.Equal(f.Command(), []string{"fish", "-l"}) {
		t.Fatalf("第一个：%+v", f)
	}
	if nu := s.Profile("nu"); nu == nil || !slices.Equal(nu.Command(), []string{"/opt/homebrew/bin/nu", "--login", "-e", "echo hi"}) {
		t.Fatalf("nu：%+v", nu)
	}
	if s.TermProfiles[1].Name != "nu" || s.TermProfiles[2].Name != "p0" || s.Profile("") != nil || s.Profile("long") != nil || s.Profile("p8") != nil {
		t.Fatalf("顺序、找不到的：%+v", s.TermProfiles)
	}
}
