package codegen

import (
	"bufio"
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"strconv"

	"github.com/juju/errors"
	"github.com/otiai10/copy"
	"github.com/spf13/afero"
)

var fs = afero.NewOsFs()

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

// ProgramizeFunction extracts the source code of an anonymous function that was passed into the
// calling function and turns it into its own, separate executable program.
func ProgramizeFunction(offset int, outputSourceDirectory string) error {
	return nil
	//filename, fnSource, err := extractClosureSource(offset + 1)
	//if err != nil {
	//	errors.Annotatef(err, "failed to extract function argument")
	//}
	//err = os.MkdirAll(outputSourceDirectory, 0755)
	//if err != nil {
	//	return errors.Trace(err)
	//}
	//outputFile := path.Join(outputSourceDirectory, "main.go")
	//file, err := os.Create(outputFile)
	//if err != nil {
	//	errors.Annotatef(err, "failed to create new source file at '%s'", outputFile)
	//}
	//defer file.Close()
	//buf := bytes.Buffer{}
	//buf.WriteString("package main\n")
	//buf.Write(importsBlock)
	//buf.WriteString("\n")
	//buf.WriteString("func main() {extractedFunc(nil)}\n\n")
	//buf.WriteString("func extractedFunc")
	//buf.Write(fnSource[4:]) // remove "func" prefix
	//buf.WriteString("\n")
	//formatted, err := imports.Process(outputFile, buf.Bytes(), nil)
	//if err != nil {
	//	return errors.Annotate(err, "failed running goimports on generated code")
	//}
	//_, err = file.Write(formatted)
	//if err != nil {
	//	return errors.Annotatef(err, "failed writing generated code to disk at '%s'", outputFile)
	//}
	//dir := path.Dir(filename)
	//modFile := path.Join(dir, "go.mod")
	//exists, err := afero.Exists(fs, modFile)
	//if err != nil {
	//	return errors.Annotatef(err, "failed trying to detect go.mod")
	//}
	//if !exists {
	//	return errors.Errorf("programization requires use of Go modules, need a go.mod file in '%s'", dir)
	//}
	//newModFile := path.Join(outputSourceDirectory, "go.mod")
	//err = processLineByLine(modFile, newModFile, func(line *string, lineNum int) *string {
	//	if lineNum == 1 {
	//		parts := strings.Split(*line, " ")
	//		newLine := fmt.Sprintf("module main\nreplace %s => ../\n", parts[1])
	//		return &newLine
	//	}
	//	return line
	//})
	//err = compile(outputSourceDirectory)
	//if err != nil {
	//	errors.Annotatef(err, "failed to compile code in '%s'", outputSourceDirectory)
	//}
	//return nil
}

func compile(sourceDirectory string) error {
	cmd := exec.Command("go", "build", ".")
	cmd.Dir = sourceDirectory
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		// TODO: logging here
		return nil
	}
	// Mapping of filename to a "set" of line numbers where there are unused imports.
	// Line numbers are a map for efficient lookup as a set
	unusedImports := map[string]map[int]interface{}{}
	re := regexp.MustCompile(`(.*):(\d)+:\d+.*imported and not used: ".*"`)
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		match := re.FindAllStringSubmatch(line, -1)
		if match != nil {
			file := match[0][1]
			lineNum, _ := strconv.Atoi(match[0][2]) // err can be ignored since regex ensures an integer
			if _, ok := unusedImports[file]; !ok {
				unusedImports[file] = map[int]interface{}{}
			}
			// Store nil since the map is only being used as a set
			unusedImports[file][lineNum] = nil
		}
	}
	// Rewrite file without unused imports
	for filename, lineNums := range unusedImports {
		srcPath := path.Join(sourceDirectory, filename)
		fileText, err := ioutil.ReadFile(srcPath)
		if err != nil {
			return errors.Annotatef(err, "failed reading generated source file '%s'", filename)
		}
		newFile, err := os.Create(srcPath)
		if err != nil {
			return errors.Annotatef(err, "failed post-processing source file '%s'", filename)
		}
		defer newFile.Close()
		scanner := bufio.NewScanner(bytes.NewReader(fileText))
		currentLineNum := 0
		for scanner.Scan() {
			currentLineNum++
			// If this line is found in the list of unused imports, skip and do not write it back out
			if _, ok := lineNums[currentLineNum]; ok {
				continue
			}
			line := scanner.Text()
			_, err := newFile.WriteString(line + "\n")
			if err != nil {
				return errors.Annotatef(err, "failed writing post-processed source file '%s', filename")
			}
		}
	}
	cmd = exec.Command("go", "build", ".")
	cmd.Dir = sourceDirectory
	err = cmd.Run()
	if err != nil {
		// TODO: logging here
		return errors.Annotatef(err, "failed to compile generated source in '%s'", sourceDirectory)
	}
	return nil
}

// CloneModule copies project source from one place to another
func CloneModule(sourceDir, destDir string) error {
	err := copy.Copy(sourceDir, destDir)
	if err != nil {
		return errors.Annotatef(err, "failed to copy CI module directory from '%s' to '%s'", sourceDir, destDir)
	}
	return nil
}
