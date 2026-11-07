package ui

import (
	"testing"
	"time"

	"github.com/lemoeade/grove/internal/i18n"
)

// 历史的段头和行尾时间（甲）：段头今天 / 昨天 / 本周 / 上周 / 本月 / "10 月" / 跨年的"2025 年 12 月"；
// 行尾今天昨天"15:04"，本周上周"周四 15:04"，再早"10-22"。
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
	if got, want := rowWhen(n), n.Format("15:04"); got != want {
		t.Errorf("今天的行尾 %q，要 %q", got, want)
	}
	w := n.AddDate(0, 0, -7) // 上周同一天
	if got, want := rowWhen(w), i18n.T(weekdays[w.Weekday()])+" "+w.Format("15:04"); got != want {
		t.Errorf("上周的行尾 %q，要 %q", got, want)
	}
	o := n.AddDate(0, -3, 0)
	if got, want := rowWhen(o), o.Format("01-02"); got != want {
		t.Errorf("三个月前的行尾 %q，要 %q", got, want)
	}
}
