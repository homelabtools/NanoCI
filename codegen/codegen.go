package codegen

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"runtime"

	"github.com/juju/errors"
	"golang.org/x/tools/imports"
)

// extractCallerFunctionArgument extracts the source code of a function argument up the stack.
// If this function is called with offset 1, it will generate source for the function argument
// passed into the caller's method. For example, given the following code:
//
//     package main
//
//     import "fmt"
//
//     func main() {
//         Foo(func() error {
//             x := 1
//             fmt.Println(x+2)
//         })
//     }
//
//     func Foo(func() error) {
//     	   funcText, importBlock := extractCallerFunctionArgument(1)
//     }
//
// The contents of funcText would be:
//     func() error {
//         x := 1
//         fmt.Println(x+2)
//     }
//
// And the contents of importBlock would be:
//     import "fmt"
//
// Note that there is no variable capture here, it extracts source only. Any variables captured
// by the function argument will be meaningless in the extracted source. Thus, this should only
// be used for extracting functions which are self-contained. The passing of parameters should be
// done with some form of serialization, such as serializing a JSON map.
//
func extractCallerFunctionArgument(offset int) (funcText []byte, importsText []byte, err error) {
	_, file, lineNum, ok := runtime.Caller(offset + 1)
	if !ok {
		return nil, nil, errors.New("failed to get caller info")
	}
	//fmt.Printf("%+v, %+v, %+v\n", file, line, ok)
	source, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "failed to read source of '%s'", file)
	}
	// To extract the source, we have to identify which function within the AST corresponds
	// to the one on the line number of the caller. To do that, we can iterate through the source,
	// byte by byte, until the desired line number is found. Then we can keep moving until we find
	// the next line. Now we have the start and end location in the file of that line. When searching
	// through the AST below, we'll check the start position of each function and look for
	// the one that starts after lineStartsAt and before lineEndsAt.
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
		if lc == lineNum && lineStartsAt == 0 {
			lineStartsAt = i
		}
	}
	//fmt.Printf("Line %d starts at offset %d and ends at %d\n", line, lineStartsAt, lineEndsAt)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "callersource.go", source, 0)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "failed to parse source of '%s'", file)
	}
	// Given that we have the starting and ending positions in the file of the desired line of code,
	// we can search for the function whose start position is after the start position of the line
	// but before the line's ending position. This will work as long as there is only one function
	// on that line.
	numMatches := 0
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			start := int(x.Pos()) - 1
			end := int(x.End())
			if start >= lineStartsAt && start < lineEndsAt {
				numMatches++
				if numMatches > 1 {
					return false
				}
				//fmt.Printf("%+v, %+v\n", x.Pos(), x.End())
				funcText = source[start : end-1]
				return true
			}
		}
		return true
	})
	if numMatches != 1 {
		return nil, nil, errors.Errorf("expected to find only 1 function on line %d, instead found %d", lineNum, numMatches)
	}
	buf := &bytes.Buffer{}
	buf.WriteString("\nimport (\n")
	for _, i := range f.Imports {
		buf.WriteString("    ")
		if i.Name != nil {
			buf.WriteString(i.Name.Name + " ")
		}
		buf.WriteString(i.Path.Value + "\n")
	}
	buf.WriteString(")\n")
	return funcText, buf.Bytes(), nil
}

// ProgramizeFunction extracts the source code of an anonymous function that was passed into the
// calling function and turns it into its own, separate executable program.
func ProgramizeFunction(offset int, outputSourceDirectory string) error {
	fnSource, importsBlock, err := extractCallerFunctionArgument(offset + 1)
	if err != nil {
		errors.Annotatef(err, "failed to extract function argument")
	}
	path := path.Join(outputSourceDirectory, "main.go")
	file, err := os.Create(path)
	if err != nil {
		errors.Annotatef(err, "failed to create new source file at '%s'", path)
	}
	defer file.Close()
	buf := bytes.Buffer{}
	buf.WriteString("package main\n")
	buf.Write(importsBlock)
	buf.WriteString("\n")
	buf.WriteString("func main() {}\n\n")
	buf.WriteString("func extractedFunc")
	buf.Write(fnSource[4:])
	buf.WriteString("\n")
	formatted, err := imports.Process(path, buf.Bytes(), nil)
	if err != nil {
		return errors.Annotate(err, "failed running goimports on generated code")
	}
	_, err = file.Write(formatted)
	return errors.Annotatef(err, "failed writing generated code to disk at '%s'", path)
}
