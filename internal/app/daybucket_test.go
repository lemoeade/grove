package app

import (
	"testing"
	"time"
)

// 提交历史的日期段：今天、昨天、本周（从周一起）、上周、本月，再早的一个月一段；跨年的月份带年。
func TestBucket(t *testing.T) {
	at := func(y, m, d, h int) time.Time { return time.Date(y, time.Month(m), d, h, 0, 0, 0, time.Local) }
	now := at(2026, 11, 20, 15) // 周五
	for _, c := range []struct {
		d     time.Time
		n     int
		start time.Time
	}{
		{at(2026, 11, 20, 0), 0, at(2026, 11, 20, 0)},
		{at(2026, 11, 21, 9), 0, at(2026, 11, 20, 0)}, // 时钟不准、未来的也算今天
		{at(2026, 11, 19, 23), 1, at(2026, 11, 19, 0)},
		{at(2026, 11, 16, 8), 2, at(2026, 11, 16, 0)}, // 这周一
		{at(2026, 11, 15, 22), 3, at(2026, 11, 9, 0)}, // 上周日
		{at(2026, 11, 9, 0), 3, at(2026, 11, 9, 0)},
		{at(2026, 11, 8, 12), 4, at(2026, 11, 1, 0)}, // 本月更早
		{at(2026, 10, 31, 12), 11, at(2026, 10, 1, 0)},
		{at(2025, 12, 3, 12), 21, at(2025, 12, 1, 0)},
	} {
		if n, start := Bucket(c.d, now); n != c.n || !start.Equal(c.start) {
			t.Errorf("Bucket(%v) = %d, %v；要 %d, %v", c.d, n, start, c.n, c.start)
		}
	}
	// 周一：昨天（周日）是昨天，不是上周
	mon := at(2026, 11, 16, 10)
	if n, _ := Bucket(at(2026, 11, 15, 10), mon); n != 1 {
		t.Errorf("周一看周日 = %d，要 1（昨天）", n)
	}
	if n, _ := Bucket(at(2026, 11, 14, 10), mon); n != 3 {
		t.Errorf("周一看上周六 = %d，要 3（上周）", n)
	}
}
