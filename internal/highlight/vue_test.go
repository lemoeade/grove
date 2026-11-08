package highlight

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

// vueLines 从头分 src 的每一行，返回每行的词。
func vueLines(t *testing.T, tok editor.Tokenizer, src string) ([]string, [][]editor.Token) {
	t.Helper()
	lines := strings.Split(src, "\n")
	var state uint64
	all := make([][]editor.Token, len(lines))
	for i, l := range lines {
		var toks []editor.Token
		toks, state = tok.Tokenize(l, state, nil)
		prev := 0
		if tok != Vue().Tokenizer { // mygo 的 HTML 在 </script> 前面留一个空的词
			toks = slices.DeleteFunc(toks, func(tk editor.Token) bool { return tk.End == 0 })
		}
		for _, tk := range toks {
			if tk.End <= prev || tk.End > len(l) {
				t.Fatalf("第 %d 行 %q 的词不对：%v", i+1, l, toks)
			}
			prev = tk.End
		}
		all[i] = toks
	}
	return lines, all
}

// kindOf 是第 line 行里 word（这一行第 nth 次出现，从 0 数）第一个字节的词类。
func kindOf(t *testing.T, lines []string, toks [][]editor.Token, line int, word string, nth int) editor.TokenKind {
	t.Helper()
	l := lines[line]
	at := -1
	for from := 0; nth >= 0; nth-- {
		j := strings.Index(l[from:], word)
		if j < 0 {
			t.Fatalf("第 %d 行 %q 里没有 %q", line+1, l, word)
		}
		at = from + j
		from = at + 1
	}
	for _, tk := range toks[line] {
		if at < tk.End {
			return tk.Kind
		}
	}
	return editor.TokenText
}

const vueSample = `<script setup lang="ts">
import { ref } from 'vue'
const props = defineProps<{ value: number; label?: string }>()
const open = ref(false)
</script>

<template>
  <!-- {{ 不是插值 }} -->
  <div class="card" :class="{ active: open }" @click="toggle(1)">
    <span v-if="props.value > 0">{{ props.label ?? 'x' }}</span>
    <ElButton
      :style="{
        width: 10,
      }"
      type="primary"
    >{{ count }} &amp; 个</ElButton>
    <template #default="{ item }">{{ item.name }}</template>
  </div>
</template>

<script>
export default { name: 'A', data() { return { n: 1 } } }
const s = 'string'
</script>

<style lang="scss" scoped>
.card { color: red; }
</style>
`

func TestVueTokens(t *testing.T) {
	lines, toks := vueLines(t, Vue().Tokenizer, vueSample)
	for _, c := range []struct {
		line int
		word string
		nth  int
		want editor.TokenKind
	}{
		{0, "script", 0, editor.TokenTag},
		{0, "lang", 0, editor.TokenAttribute},
		{0, `"ts"`, 0, editor.TokenString},
		{1, "import", 0, editor.TokenKeyword},
		{2, "number", 0, editor.TokenType}, // lang="ts"：按 TypeScript 分
		{2, "string", 0, editor.TokenType},
		{3, "false", 0, editor.TokenConstant},
		{4, "script", 0, editor.TokenTag}, // </script> 回到标签
		{7, "{{", 0, editor.TokenComment}, // 注释里的不是插值
		{7, "不是插值", 0, editor.TokenComment},
		{8, "class", 0, editor.TokenAttribute},
		{8, `"card"`, 0, editor.TokenString}, // 普通属性：字符串
		{8, ":class", 0, editor.TokenAttribute},
		{8, "open", 0, editor.TokenVariable}, // 指令的值：代码
		{8, "@click", 0, editor.TokenAttribute},
		{8, "toggle", 0, editor.TokenFunction},
		{8, "1", 0, editor.TokenNumber},
		{9, "v-if", 0, editor.TokenAttribute},
		{9, "props", 0, editor.TokenVariable},
		{9, "value", 0, editor.TokenProperty},
		{9, "0", 0, editor.TokenNumber},
		{9, "{{", 0, editor.TokenPunctuation},
		{9, "label", 0, editor.TokenProperty},
		{9, "'x'", 0, editor.TokenString},
		{9, "}}", 0, editor.TokenPunctuation},
		{9, "span", 1, editor.TokenTag},
		{10, "ElButton", 0, editor.TokenTag},
		{12, "width", 0, editor.TokenVariable}, // 跨行的指令值还是代码
		{12, "10", 0, editor.TokenNumber},
		{14, "type", 0, editor.TokenAttribute}, // 指令的值结束了，回到标签
		{14, `"primary"`, 0, editor.TokenString},
		{15, "count", 0, editor.TokenVariable},
		{15, "&amp;", 0, editor.TokenEscape},
		{15, "个", 0, editor.TokenText},
		{16, "#default", 0, editor.TokenAttribute},
		{16, "item", 0, editor.TokenVariable},
		{16, "name", 0, editor.TokenProperty},
		{21, "export", 0, editor.TokenKeyword}, // 没有 lang 的 <script> 按 JavaScript
		{22, "'string'", 0, editor.TokenString},
		{26, "color", 0, editor.TokenProperty}, // <style lang="scss"> 按 CSS
		{27, "style", 0, editor.TokenTag},
	} {
		if got := kindOf(t, lines, toks, c.line, c.word, c.nth); got != c.want {
			t.Errorf("第 %d 行 %q：%v，应为 %v（%q）", c.line+1, c.word, got, c.want, lines[c.line])
		}
	}
}

// 没有 lang 的 <script> 是 JavaScript：number 不是类型。
func TestVueScriptJS(t *testing.T) {
	lines, toks := vueLines(t, Vue().Tokenizer, "<script>\nlet n: number\n</script>\n<script lang=\"tsx\">\nlet n: number\n</script>")
	if got := kindOf(t, lines, toks, 1, "number", 0); got == editor.TokenType {
		t.Errorf("JavaScript 的 number 不该是类型")
	}
	if got := kindOf(t, lines, toks, 4, "number", 0); got != editor.TokenType {
		t.Errorf("lang=\"tsx\" 的 number：%v，应为类型", got)
	}
}

// 没有 Vue 语法（插值、指令、lang）的地方和 mygo 的 HTML 分得一模一样。
func TestVueLikeHTML(t *testing.T) {
	src := `<!doctype html>
<div id="app" class='x y' data-n=3>
  <p>文字 &lt; <b>粗</b></p>
  <!-- 注释
  跨行 -->
  <img src="a.png" />
</div>
<script>
const re = /a+/g; let x = 1 // 注释
</script>
<style>
.a { color: red }
</style>`
	html := editor.LanguageNamed("HTML").Tokenizer
	_, want := vueLines(t, html, src)
	_, got := vueLines(t, Vue().Tokenizer, src)
	for i := range want {
		if !slices.Equal(got[i], want[i]) {
			t.Errorf("第 %d 行：%v，HTML 是 %v", i+1, got[i], want[i])
		}
	}
}

func TestVueLanguage(t *testing.T) {
	v := Vue()
	if v == nil || v.Name != "Vue" || v.Tokenizer == nil || v.Highlighter != nil {
		t.Fatalf("Vue = %+v", v)
	}
	for _, name := range []string{"App.vue", "src/a/B.VUE"} {
		if l := LanguageFor(name); l != v {
			t.Errorf("LanguageFor(%q) = %v", name, l)
		}
	}
	if l := LanguageFor("index.html"); l == nil || l.Name != "HTML" {
		t.Errorf("index.html 还应是 HTML：%v", l)
	}
	if LanguageNamed("vue") != v || LanguageNamed("Vue") != v || LanguageNamed("go").Name != "Go" {
		t.Error("LanguageNamed 认不对")
	}
	n := 0
	for _, l := range Languages() {
		if l.Name == "Vue" {
			n++
		}
	}
	if n != 1 || len(Languages()) != len(editor.Languages())+1 {
		t.Errorf("Languages 里 Vue 有 %d 个", n)
	}
	// 缩进、括号、注释照 HTML
	html := editor.LanguageNamed("HTML")
	if v.BlockComment != html.BlockComment || v.TabSize != html.TabSize || len(v.Brackets) != len(html.Brackets) || v.IncreaseIndent != html.IncreaseIndent {
		t.Error("Vue 的缩进、括号、注释应照 HTML")
	}
	// 大文件（docs.Doc.Big）的后台高亮也能包 Vue
	if w := Wrap(v); w == v || w.Highlighter == nil || w.Name != "Vue" {
		t.Errorf("Wrap(Vue) = %+v", w)
	}
}
