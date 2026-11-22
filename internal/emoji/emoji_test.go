package emoji

import (
	"strings"
	"testing"
)

func TestReplace(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"done :tada: :+1:", "done 🎉 👍"},
		{":rocket:", "🚀"},
		{"不认识 :no_such_emoji_x: 照原样", "不认识 :no_such_emoji_x: 照原样"},
		{"时间 10:100:20 和 a:fire:b 不换", "时间 10:100:20 和 a:fire:b 不换"},
		{"行内 `:tada:` 不换，外面 :tada: 换", "行内 `:tada:` 不换，外面 🎉 换"},
		{"两个反引号 ``a ` :tada:`` :fire:", "两个反引号 ``a ` :tada:`` 🔥"},
		{"转义 \\:tada: 不换", "转义 \\:tada: 不换"},
		{"```\n:tada:\n```\n:tada:\n", "```\n:tada:\n```\n🎉\n"},
		{"~~~md\n:fire:\n~~~~\n:fire:", "~~~md\n:fire:\n~~~~\n🔥"},
		{"https://example.com:8080/x", "https://example.com:8080/x"},
		{"(:smile:)", "(😄)"},
	} {
		if got := Replace(c.in); got != c.want {
			t.Errorf("Replace(%q) = %q，要 %q", c.in, got, c.want)
		}
		if got := Replace(c.in); strings.Count(got, "\n") != strings.Count(c.in, "\n") {
			t.Errorf("Replace(%q) 改了行数", c.in)
		}
	}
	s := "没有短代码: 就是原来那个"
	if got := Replace(s); got != s {
		t.Fatal("没换时该返回原串")
	}
	if len(table) == 0 {
		t.Fatal("短代码表是空的")
	}
	if e, ok := Lookup("heart"); !ok || e != "❤️" {
		t.Errorf("heart = %q %v", e, ok)
	}
}
