package parser

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/fatih/hcl/ast"
	"github.com/fatih/hcl/token"
)

func TestType(t *testing.T) {
	var literals = []struct {
		typ token.Type
		src string
	}{
		{token.STRING, `foo = "foo"`},
		{token.NUMBER, `foo = 123`},
		{token.FLOAT, `foo = 123.12`},
		{token.FLOAT, `foo = -123.12`},
		{token.BOOL, `foo = true`},
	}

	for _, l := range literals {
		p := newParser([]byte(l.src))
		item, err := p.objectItem()
		if err != nil {
			t.Error(err)
		}

		lit, ok := item.Val.(*ast.LiteralType)
		if !ok {
			t.Errorf("node should be of type LiteralType, got: %T", item.Val)
		}

		if lit.Token.Type != l.typ {
			t.Errorf("want: %s, got: %s", l.typ, lit.Token.Type)
		}
	}
}

func TestListType(t *testing.T) {
	var literals = []struct {
		src    string
		tokens []token.Type
	}{
		{
			`foo = ["123", 123]`,
			[]token.Type{token.STRING, token.NUMBER},
		},
		{
			`foo = [123, "123",]`,
			[]token.Type{token.NUMBER, token.STRING},
		},
		{
			`foo = []`,
			[]token.Type{},
		},
		{
			`foo = ["123", 123]`,
			[]token.Type{token.STRING, token.NUMBER},
		},
	}

	for _, l := range literals {
		p := newParser([]byte(l.src))
		item, err := p.objectItem()
		if err != nil {
			t.Error(err)
		}

		list, ok := item.Val.(*ast.ListType)
		if !ok {
			t.Errorf("node should be of type LiteralType, got: %T", item.Val)
		}

		tokens := []token.Type{}
		for _, li := range list.List {
			if tp, ok := li.(*ast.LiteralType); ok {
				tokens = append(tokens, tp.Token.Type)
			}
		}

		equals(t, l.tokens, tokens)
	}
}

func TestObjectType(t *testing.T) {
	var literals = []struct {
		src      string
		nodeType []ast.Node
		itemLen  int
	}{
		{
			`foo = {}`,
			nil,
			0,
		},
		{
			`foo = {
				bar = "fatih"
			 }`,
			[]ast.Node{&ast.LiteralType{}},
			1,
		},
		{
			`foo = {
				bar = "fatih"
				baz = ["arslan"]
			 }`,
			[]ast.Node{
				&ast.LiteralType{},
				&ast.ListType{},
			},
			2,
		},
		{
			`foo = {
				bar {}
			 }`,
			[]ast.Node{
				&ast.ObjectType{},
			},
			1,
		},
		{
			`foo {
				bar {}
				foo = true
			 }`,
			[]ast.Node{
				&ast.ObjectType{},
				&ast.LiteralType{},
			},
			2,
		},
	}

	for _, l := range literals {
		p := newParser([]byte(l.src))
		// p.enableTrace = true
		item, err := p.objectItem()
		if err != nil {
			t.Error(err)
		}

		// we know that the ObjectKey name is foo for all cases, what matters
		// is the object
		obj, ok := item.Val.(*ast.ObjectType)
		if !ok {
			t.Errorf("node should be of type LiteralType, got: %T", item.Val)
		}

		// check if the total length of items are correct
		equals(t, l.itemLen, len(obj.List.Items))

		// check if the types are correct
		for i, item := range obj.List.Items {
			equals(t, reflect.TypeOf(l.nodeType[i]), reflect.TypeOf(item.Val))
		}
	}
}

func TestObjectKey(t *testing.T) {
	keys := []struct {
		exp []token.Type
		src string
	}{
		{[]token.Type{token.IDENT}, `foo {}`},
		{[]token.Type{token.IDENT}, `foo = {}`},
		{[]token.Type{token.IDENT}, `foo = bar`},
		{[]token.Type{token.IDENT}, `foo = 123`},
		{[]token.Type{token.IDENT}, `foo = "${var.bar}`},
		{[]token.Type{token.STRING}, `"foo" {}`},
		{[]token.Type{token.STRING}, `"foo" = {}`},
		{[]token.Type{token.STRING}, `"foo" = "${var.bar}`},
		{[]token.Type{token.IDENT, token.IDENT}, `foo bar {}`},
		{[]token.Type{token.IDENT, token.STRING}, `foo "bar" {}`},
		{[]token.Type{token.STRING, token.IDENT}, `"foo" bar {}`},
		{[]token.Type{token.IDENT, token.IDENT, token.IDENT}, `foo bar baz {}`},
	}

	for _, k := range keys {
		p := newParser([]byte(k.src))
		keys, err := p.objectKey()
		if err != nil {
			t.Fatal(err)
		}

		tokens := []token.Type{}
		for _, o := range keys {
			tokens = append(tokens, o.Token.Type)
		}

		equals(t, k.exp, tokens)
	}

	errKeys := []struct {
		src string
	}{
		{`foo 12 {}`},
		{`foo bar = {}`},
		{`foo []`},
		{`12 {}`},
	}

	for _, k := range errKeys {
		p := newParser([]byte(k.src))
		_, err := p.objectKey()
		if err == nil {
			t.Errorf("case '%s' should give an error", k.src)
		}
	}
}

// Official HCL tests
func TestParse(t *testing.T) {
	cases := []struct {
		Name string
		Err  bool
	}{
		{
			"assign_colon.hcl",
			true,
		},
		{
			"comment.hcl",
			false,
		},
		{
			"comment_single.hcl",
			false,
		},
		{
			"empty.hcl",
			false,
		},
		{
			"list_comma.hcl",
			false,
		},
		{
			"multiple.hcl",
			false,
		},
		{
			"structure.hcl",
			false,
		},
		{
			"structure_basic.hcl",
			false,
		},
		{
			"structure_empty.hcl",
			false,
		},
		{
			"complex.hcl",
			false,
		},
		{
			"assign_deep.hcl",
			true,
		},
		{
			"types.hcl",
			false,
		},
		{
			"array_comment.hcl",
			false,
		},
	}

	const fixtureDir = "./test-fixtures"

	for _, tc := range cases {
		d, err := ioutil.ReadFile(filepath.Join(fixtureDir, tc.Name))
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		_, err = Parse(d)
		if (err != nil) != tc.Err {
			t.Fatalf("Input: %s\n\nError: %s", tc.Name, err)
		}
	}
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
}
