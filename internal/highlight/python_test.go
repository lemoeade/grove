package highlight

import (
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

const pySample = `"""模块说明。"""
from sqlalchemy.orm import Session


@router.get("/notes", response_model=List[Note])
def list_notes(db: Session = Depends(get_db), skip: int = 0) -> List[Note]:
    """第一行

    还在说明里
    """
    q = db.query(models.Note).filter(models.Note.owner_id == user.id)
    if not user.is_admin:
        q = q.filter(x := 1)
    return q.all()


class S:
    def __init__(self, n: int = 2) -> None:
        self.n = n + 1
        s = """不是 docstring"""
        t = f(
            """括号里的不是 docstring"""
        )
`

func TestPythonTokens(t *testing.T) {
	py := LanguageFor("a.py")
	if py == nil || py != Python() || py.Name != "Python" {
		t.Fatalf("LanguageFor(a.py) = %v", py)
	}
	lines, toks := vueLines(t, py.Tokenizer, pySample)
	for _, c := range []struct {
		line int
		word string
		nth  int
		want editor.TokenKind
	}{
		{0, `"""`, 0, editor.TokenComment}, // 模块的 docstring
		{1, "from", 0, editor.TokenKeyword},
		{1, "orm", 0, editor.TokenVariable}, // 导入路径不是属性
		{1, "Session", 0, editor.TokenType},
		{4, "@router", 0, editor.TokenMeta},
		{4, "=", 0, editor.TokenPunctuation}, // 关键字参数
		{5, ":", 0, editor.TokenPunctuation}, // 注解
		{5, "=", 0, editor.TokenPunctuation}, // 默认值
		{5, "->", 0, editor.TokenPunctuation},
		{5, ":", 2, editor.TokenPunctuation}, // 行尾的 ]:
		{5, "Depends", 0, editor.TokenFunction},
		{5, "0", 0, editor.TokenNumber},
		{6, "第一行", 0, editor.TokenComment},
		{7, "", 0, editor.TokenText},
		{8, "还在", 0, editor.TokenComment}, // 跨行的 docstring
		{9, `"""`, 0, editor.TokenComment},
		{10, "query", 0, editor.TokenFunction},
		{10, "owner_id", 0, editor.TokenVariable},
		{10, "==", 0, editor.TokenOperator}, // 真的运算符还是运算符
		{11, "not", 0, editor.TokenKeyword},
		{11, "is_admin", 0, editor.TokenVariable},
		{11, ":", 0, editor.TokenPunctuation},
		{12, ":=", 0, editor.TokenOperator},
		{17, "self", 0, editor.TokenEmphasis},
		{17, "None", 0, editor.TokenConstant},
		{18, "self", 0, editor.TokenEmphasis},
		{18, "+", 0, editor.TokenOperator},
		{19, `"""不是`, 0, editor.TokenString}, // 赋值的三引号字符串
		{21, `"""括号`, 0, editor.TokenString},
		{22, ")", 0, editor.TokenPunctuation},
	} {
		if c.word == "" {
			continue
		}
		if got := kindOf(t, lines, toks, c.line, c.word, c.nth); got != c.want {
			t.Errorf("第 %d 行 %q：%v，应为 %v（%q）", c.line+1, c.word, got, c.want, lines[c.line])
		}
	}
	// 改过类以后挨着的同类词接成了一个
	for i, row := range toks {
		for j := 1; j < len(row); j++ {
			if row[j].Kind == row[j-1].Kind {
				t.Errorf("第 %d 行有挨着的同类词：%v", i+1, row)
				break
			}
		}
	}
}

// Grove 的那几位之外，状态和 mygo 的一样；docstring 那一位不留到字符串后面。
func TestPythonStateLikeMygo(t *testing.T) {
	base := editor.LanguageNamed("Python").Tokenizer
	var a, b uint64
	for _, l := range splitLines(pySample) {
		_, a = Python().Tokenizer.Tokenize(l, a, nil)
		_, b = base.Tokenize(l, b, nil)
		if a&^pyBits != b {
			t.Fatalf("%q 之后状态 %x，mygo 是 %x", l, a, b)
		}
	}
	if a&pyDoc != 0 {
		t.Errorf("文件末尾还在 docstring 里：%x", a)
	}
}

func TestPythonLanguage(t *testing.T) {
	py := Python()
	base := editor.LanguageNamed("Python")
	if py == base || py.Tokenizer == base.Tokenizer || py.TabSize != base.TabSize || py.LineComment != base.LineComment || py.IncreaseIndent != base.IncreaseIndent {
		t.Error("Grove 的 Python 应只换了分词器")
	}
	for _, name := range []string{"x.pyi", "SConstruct"} {
		if LanguageFor(name) != py {
			t.Errorf("LanguageFor(%q) 不是 Grove 的 Python", name)
		}
	}
	if LanguageNamed("python") != py || LanguageNamed("py") != py {
		t.Error("LanguageNamed 认不出 Grove 的 Python")
	}
	n := 0
	for _, l := range Languages() {
		if l.Name == "Python" {
			n++
			if l != py {
				t.Error("Languages 里是 mygo 的 Python")
			}
		}
	}
	if n != 1 {
		t.Errorf("Languages 里 Python 有 %d 个", n)
	}
	if w := Wrap(py); w == py || w.Highlighter == nil {
		t.Errorf("Wrap(Python) = %+v", w)
	}
}

func splitLines(s string) []string {
	var out []string
	for len(s) > 0 {
		i := 0
		for i < len(s) && s[i] != '\n' {
			i++
		}
		out = append(out, s[:i])
		if i < len(s) {
			i++
		}
		s = s[i:]
	}
	return out
}

// docstring 只在文件开头（前面只有注释）和以 : 结尾的那行下面；一般的语句后面单独一行的三引号字符串不算。
func TestPythonDocstringPlace(t *testing.T) {
	src := "#!/usr/bin/env python\n'''脚本说明'''\nx = 1\n'''不算'''\nclass A(\n    B,\n):\n\n    r\"\"\"类说明\"\"\"\n"
	lines, toks := vueLines(t, Python().Tokenizer, src)
	for _, c := range []struct {
		line int
		word string
		want editor.TokenKind
	}{
		{1, "脚本", editor.TokenComment},
		{3, "不算", editor.TokenString},
		{8, "类说明", editor.TokenComment},
	} {
		if got := kindOf(t, lines, toks, c.line, c.word, 0); got != c.want {
			t.Errorf("第 %d 行 %q：%v，应为 %v", c.line+1, c.word, got, c.want)
		}
	}
}
