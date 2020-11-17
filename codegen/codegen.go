package codegen

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/homelabtools/nanoci/mirror"
	"github.com/homelabtools/nanoci/textfile"
	"github.com/juju/errors"
	"github.com/otiai10/copy"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
)

var fs = afero.NewOsFs()

// ProgramizeFunction extracts the source code of an anonymous function that was passed into the
// calling function and turns it into its own, separate executable program.
//func ProgramizeFunction(offset int, outputSourceDirectory string) error {
//return nil
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
//}

func compile(sourceDirectory, binName string) error {
	cmd := exec.Command("go", "build", ".", "-o", binName)
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
	err := os.RemoveAll(destDir)
	if err != nil {
		return errors.Annotatef(err, "failed to clean up generated program dir")
	}
	err = os.MkdirAll(destDir, 0777)
	if err != nil {
		return errors.Annotatef(err, "failed to create generated program dir")
	}
	sourcePath, err := filepath.EvalSymlinks(sourceDir)
	if err != nil {
		return errors.Trace(err)
	}
	destPath, err := filepath.EvalSymlinks(destDir)
	if err != nil {
		return errors.Trace(err)
	}
	if strings.Contains(destPath, sourcePath) {
		return errors.Errorf("cannot copy '%s' into '%s' as the latter is a subdirectory of the former", sourcePath, destPath)
	}
	log.Debug().Msgf("Copying '%s' to '%s", sourceDir, destDir)
	ok, err := afero.Exists(fs, path.Join(sourceDir, ".git"))
	if err != nil {
		return errors.Trace(err)
	}
	if ok {
		return errors.Errorf("builder cannot be at the root of a git repo, please place your builder code in a subdirectory of your choosing")
	}
	err = copy.Copy(sourceDir, destDir)
	if err != nil {
		return errors.Annotatef(err, "failed to copy CI module directory from '%s' to '%s'", sourceDir, destDir)
	}
	return nil
}

// CreateProgramFromFunction creates a separate program that runs the specified function.
func CreateProgramFromFunction(fi *mirror.FunctionInfo) (*Program, error) {
	dir, err := ioutil.TempDir("", "nanobuild-func-"+fi.FullName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return CreateProgramFromFunctionAt(fi, dir)
}

// CreateProgramFromFunctionAt creates a separate program that runs the specified function.
// The source is generated and built in the specified directory.
func CreateProgramFromFunctionAt(fi *mirror.FunctionInfo, dir string) (*Program, error) {
	p := &Program{}
	p.Directory = dir
	// TODO: walk the dir to find go.mod
	sourceDir := path.Dir(fi.FileName)
	err := CloneModule(sourceDir, p.Directory)
	if err != nil {
		return nil, errors.Annotatef(err, "unable to create program for function")
	}
	name := path.Join(p.Directory, path.Base(fi.FileName))
	file, err := os.OpenFile(name, os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, errors.Annotatef(err, "unable to generate program for function")
	}
	str := "\n" + "func nanofunc" + fi.Source[4:] + "\n"
	_, err = file.WriteString(str)
	if err != nil {
		return nil, errors.Trace(err)
	}
	file.Close()
	err = textfile.RewriteLineByLineInPlace(name, func(line *string, lineNum int) *string {
		if strings.Contains(*line, "BuilderMain()") {
			newLine := *line + "\n	BuilderExit(nanofunc(nil))\n"
			return &newLine
		}
		return line
	})
	if err != nil {
		return nil, errors.Annotatef(err, "failed to insert nanofunc call")
	}

	cmd := exec.Command("goimports")
	cmd.Dir = p.Directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return nil, errors.Annotatef(err, "failed to run goimports on generated source")
	}

	p.BinFileName = "nanofunc"
	p.FullPath = path.Join(p.Directory, p.BinFileName)
	err = compile(p.Directory, p.BinFileName)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to compile generated program")
	}
	log.Debug().Msgf("created function program at '%s'", p.Directory)
	return p, nil
}

// Program represents some external temporary program.
type Program struct {
	FuncInfo    *mirror.FunctionInfo
	Cmd         *exec.Cmd
	BinFileName string
	FullPath    string
	Directory   string
}

// Remove cleans up the program and deletes it from disk.
func (p *Program) Remove() {
	err := os.RemoveAll(p.Directory)
	if err != nil {
		log.Error().Msgf("failed to clean up temporary program in directory '%s'", p.Directory)
	}
}

// Run runs the program and blocks until it is completed.
func (p *Program) Run() error {
	cmd := exec.Command(p.FullPath, "--func", p.FuncInfo.FullName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
