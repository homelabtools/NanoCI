package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"runtime"

	. "github.com/homelabtools/noci/builder"
)

func main() {
	Step("", func() {})
	//Begin(Stage("my build",
	//	Step(func() error {
	//		_, _, err := SH("echo task 1; sleep 2; echo task1a; sleep 1; echo task1b")
	//		return err
	//	}),
	//	Step("echo 2", func() error {
	//		_, _, err := SH("echao task 2")
	//		return err
	//	}).NoFailOnError(), //),
	//	Step("task 3", func() error {
	//		_, e, err := SH("echo nofail 1>&2")
	//		fmt.Println(e)
	//		return err
	//	}),
	//))
	Context(func(m map[string]interface{}) {
		fmt.Println("🤘")
	})
}

// Context executes a function elsewhere
func Context(fn interface{}) {
	fnSource, imports := ext(2)
	fmt.Println(string(fnSource))
	fmt.Println(imports)
}

func ext(offset int) ([]byte, string) {
	_, file, line, ok := runtime.Caller(offset)
	if !ok {
		panic("Couldn't get caller info")
	}
	fmt.Printf("%+v, %+v, %+v\n", file, line, ok)
	source, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	lc := 1
	lineStartsAt := 0
	lineEndsAt := 0
	for i, b := range source {
		if b == '\n' {
			lc++
			if lineStartsAt > 0 {
				lineEndsAt = i
				break
			}
		}
		if lc == line && lineStartsAt == 0 {
			lineStartsAt = i
		}
	}
	//fmt.Printf("Line %d starts at offset %d and ends at %d\n", line, lineStartsAt, lineEndsAt)
	fset := token.NewFileSet()
	src, _ := ioutil.ReadFile("main.go")
	f, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		panic(err)
	}
	var funcText []byte
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			start := int(x.Pos()) - 1
			end := int(x.End())
			if start >= lineStartsAt && start < lineEndsAt {
				//fmt.Printf("%+v, %+v\n", x.Pos(), x.End())
				funcText = source[start:end]
				return false
			}
		}
		return true
	})
	importBlock := "import (\n"
	for _, i := range f.Imports {
		importBlock += "    "
		if i.Name != nil {
			importBlock += i.Name.Name + " "
		}
		importBlock += i.Path.Value + "\n"
	}
	importBlock += ")\n"
	return funcText, importBlock
}
