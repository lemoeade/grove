package search

import "testing"

func TestClip(t *testing.T) {
	s := "ab中文" // 中、文各 3 字节：a b 中[2:5] 文[5:8]
	for _, c := range []struct{ i, floor, ceil int }{{0, 0, 0}, {2, 2, 2}, {3, 2, 5}, {4, 2, 5}, {5, 5, 5}, {7, 5, 8}, {8, 8, 8}} {
		if f := RuneFloor(s, c.i); f != c.floor {
			t.Errorf("RuneFloor(%d) = %d, want %d", c.i, f, c.floor)
		}
		if f := RuneCeil([]byte(s), c.i); f != c.ceil {
			t.Errorf("RuneCeil(%d) = %d, want %d", c.i, f, c.ceil)
		}
	}
	for _, c := range []struct {
		n    int
		want string
	}{{8, "ab中文"}, {9, "ab中文"}, {7, "ab中…"}, {5, "ab中…"}, {4, "ab…"}, {0, "…"}} {
		if got := Clip(s, c.n); got != c.want {
			t.Errorf("Clip(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
