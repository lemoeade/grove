package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/lemoeade/grove/internal/i18n"
)

// 历史的段头和行尾时间（甲）：段头今天 / 昨天 / 本周 / 上周 / 本月 / "10 月" / 跨年的"2025 年 12 月"；
// 行尾今天昨天"15:04"，本周上周"周四 15:04"（窄栏只写"周四"），再早"10-22"。
func TestHistWhen(t *testing.T) {
	at := func(y, m, d, h int) time.Time { return time.Date(y, time.Month(m), d, h, 0, 0, 0, time.Local) }
	now := at(2026, 11, 20, 15) // 周五
	for _, c := range []struct {
		d    time.Time
		want string
	}{
		{at(2026, 11, 20, 9), "今天"},
		{at(2026, 11, 19, 9), "昨天"},
		{at(2026, 11, 17, 9), "本周"},
		{at(2026, 11, 12, 9), "上周"},
		{at(2026, 11, 3, 9), "本月"},
		{at(2026, 10, 3, 9), i18n.Tf("%d 月", 10)},
		{at(2025, 12, 3, 9), i18n.Tf("%d 年 %d 月", 2025, 12)},
	} {
		if got := groupLabel(c.d, now); got != i18n.T(c.want) && got != c.want {
			t.Errorf("groupLabel(%v) = %q，要 %q", c.d, got, c.want)
		}
	}
	n := time.Now()
	if got, want := rowWhen(n, false), n.Format("15:04"); got != want {
		t.Errorf("今天的行尾 %q，要 %q", got, want)
	}
	w := n.AddDate(0, 0, -7) // 上周同一天
	if got, want := rowWhen(w, false), i18n.T(weekdays[w.Weekday()])+" "+w.Format("15:04"); got != want {
		t.Errorf("上周的行尾 %q，要 %q", got, want)
	}
	if got, want := rowWhen(w, true), i18n.T(weekdays[w.Weekday()]); got != want {
		t.Errorf("窄栏上周的行尾 %q，要 %q（段头说了是上周，只写星期几）", got, want)
	}
	if got, want := rowWhen(n, true), n.Format("15:04"); got != want {
		t.Errorf("窄栏今天的行尾 %q，要 %q", got, want)
	}
	o := n.AddDate(0, -3, 0)
	if got, want := rowWhen(o, false), o.Format("01-02"); got != want {
		t.Errorf("三个月前的行尾 %q，要 %q", got, want)
	}
}

// midCut：目录放不下时省掉中间、留最后一级，前面能多留几级就多留几级。按字数假装量宽。
func TestMidCut(t *testing.T) {
	for _, c := range []struct {
		dir  string
		room int
		want string
	}{
		{".agents/skills/app-foo", 18, ".agents/…/app-foo"},
		{".agents/skills/app-foo", 10, "…/app-foo"},
		{".agents/skills/app-foo", 5, "…/app-foo"}, // 最后一级都放不下：交给省略号从尾巴截
		{"a/b/c/d/e", 8, "a/b/…/e"},
	} {
		segs := strings.Split(c.dir, "/")
		fits := func(s string) bool { return len([]rune(s)) <= c.room }
		if got := midCut(segs, fits); got != c.want {
			t.Errorf("midCut(%q, %d) = %q，要 %q", c.dir, c.room, got, c.want)
		}
	}
}
