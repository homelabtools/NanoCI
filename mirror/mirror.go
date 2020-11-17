package mirror

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"unicode"

	"github.com/homelabtools/nanoci/regex"
	"github.com/juju/errors"
)

var methodRegex *regexp.Regexp = regexp.MustCompile(`^([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)$`)
var functionRegex *regexp.Regexp = regexp.MustCompile(`^([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)$`)
var anonymousNameRegex *regexp.Regexp = regexp.MustCompile(`^func\d+$`)

// FunctionInfo contains detailed information about a function found with reflection.
type FunctionInfo struct {
	FullName    string
	PackageName string
	StructName  string
	Name        string
	IsAnonymous bool
	IsMethod    bool
	IsPrivate   bool
	// Source code of anonymous function, only valid if IsAnonymous is true
	Source string
	// File name of anonymous function, only valid if IsAnonymous is true
	FileName string
	// Line number of anonymous function, only valid if IsAnonymous is true
	LineNumber int
}

func (fi *FunctionInfo) String() string {
	if fi.IsAnonymous {
		return fmt.Sprintf("%s@%s:%d", fi.FullName, path.Base(fi.FileName), fi.LineNumber)
	}
	return fi.FullName
}

// NameOfFunction returns the name of a function, or an error if the argument is not a function.
func NameOfFunction(function interface{}) (string, error) {
	error := false
	name := func() string {
		defer func() {
			if err := recover(); err != nil {
				error = true
			}
		}()
		return runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name()
	}()
	if error {
		return "", errors.New("argument must be a function")
	}
	return name, nil
}

// FuncInfo retrieves function information using reflection.
func FuncInfo(function interface{}, offset int) (*FunctionInfo, error) {
	name, err := NameOfFunction(function)
	if err != nil {
		return nil, errors.Trace(err)
	}
	fi := &FunctionInfo{}
	fi.FullName = name

	var ok bool
	if fi.PackageName, fi.Name, ok = regex.Capture2(functionRegex, name); ok {
		fi.IsMethod = false
	} else if fi.PackageName, fi.StructName, fi.Name, ok = regex.Capture3(methodRegex, name); ok {
		// Anonymouss/anonymous functions take the name pkg.pkg.funcN where n is some number > 0
		if fi.PackageName == fi.StructName && anonymousNameRegex.MatchString(fi.Name) {
			fi.IsAnonymous = true
			fi.FileName, fi.LineNumber, fi.Source, err = ExtractAnonymousFuncSource(offset + 1)
			if err != nil {
				return nil, errors.Annotatef(err, "failed extracting source code of anonymous")
			}
		}
		fi.IsMethod = true
	} else {
		return nil, errors.New("unable to reflect function information")
	}
	for _, r := range fi.Name {
		if unicode.IsLower(r) {
			fi.IsPrivate = true
			break
		}
	}
	return fi, nil
}

// ExtractAnonymousFuncSource extracts the source code of an anonymous function argument.
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
//     	   funcText, importBlock := ExtractAnonymousFuncSource(1)
//     }
//
// The contents of funcText would be:
//     func() error {
//         x := 1
//         fmt.Println(x+2)
//     }
//
// Note that there is no variable capture here, it extracts source only. Any variables captured
// by the function argument will be meaningless in the extracted source. Thus, this should only
// be used for extracting functions which are self-contained. The passing of parameters should be
// done with some form of serialization, such as serializing a JSON map.
//
func ExtractAnonymousFuncSource(offset int) (filename string, lineNum int, funcText string, err error) {
	_, file, lineNum, ok := runtime.Caller(offset + 1)
	if !ok {
		return file, lineNum, "", errors.New("failed to get caller info")
	}
	// TODO: logging here
	//fmt.Printf("%+v, %+v, %+v\n", file, line, ok)
	source, err := ioutil.ReadFile(file)
	if err != nil {
		return file, lineNum, "", errors.Annotatef(err, "failed to read source of '%s'", file)
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
	// TODO: logging here
	//fmt.Printf("Line %d starts at offset %d and ends at %d\n", line, lineStartsAt, lineEndsAt)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "callersource.go", source, 0)
	if err != nil {
		return "", lineNum, "", errors.Annotatef(err, "failed to parse source of '%s'", file)
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
				// TODO: logging here
				//fmt.Printf("%+v, %+v\n", x.Pos(), x.End())
				funcText = string(source[start : end-1])
				return true
			}
		}
		return true
	})
	if numMatches != 1 {
		return file, lineNum, "", errors.Errorf("expected to find only 1 function on line %d, instead found %d", lineNum, numMatches)
	}
	return file, lineNum, funcText, nil
}
