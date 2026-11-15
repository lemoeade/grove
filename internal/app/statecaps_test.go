package app

import (
	"testing"

	"github.com/lemoeade/grove/internal/workspace"
)

// 读 state.json 时 workspace 把几张表截到它的上限：写的地方的上限不能比它大，不然存进去的读回来少了几条。
func TestStateCaps(t *testing.T) {
	for _, c := range []struct {
		name        string
		write, read int
	}{
		{"recent", maxRecent, workspace.MaxRecent},
		{"commitMsgs", MaxCommitMsgs, workspace.MaxCommitMsgs},
		{"searchHistory", MaxSearches, workspace.MaxSearches},
	} {
		if c.write > c.read {
			t.Errorf("%s：写最多 %d 条，读的时候截到 %d 条", c.name, c.write, c.read)
		}
	}
}
