package codegen

import (
	"bufio"
	"bytes"
	"fmt"
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

// ProgramizeFunction creates a separate program that runs the specified function.
func ProgramizeFunction(fi *mirror.FunctionInfo) (*Program, error) {
	dir, err := ioutil.TempDir("", "nanobuild-func-"+fi.FullName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return ProgramizeFunctionAt(fi, dir)
}

// ProgramizeFunctionAt creates a separate program that runs the specified function.
// The source is generated and built in the specified directory.
func ProgramizeFunctionAt(fi *mirror.FunctionInfo, dir string) (*Program, error) {
	p := &Program{}
	p.Directory = dir
	// TODO: walk the dir to find go.mod
	sourceDir := path.Dir(fi.Anonymous.FileName)
	err := copyBuilderModule(sourceDir, p.Directory)
	if err != nil {
		return nil, errors.Annotatef(err, "unable to create program for function")
	}
	name := path.Join(p.Directory, path.Base(fi.Anonymous.FileName))
	file, err := os.OpenFile(name, os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, errors.Annotatef(err, "unable to generate program for function")
	}
	str := "\n" + "func " + fi.Name + fi.Anonymous.Source[4:] + "\n"
	_, err = file.WriteString(str)
	if err != nil {
		return nil, errors.Trace(err)
	}
	file.Close()
	mainCode := fmt.Sprintf(`
	// GENERATED
	import "encoding/json"
	import "os"

	func main() {
		genStdinData, genErr := ioutil.ReadAll(os.Stdin)
		if genErr != nil {
			fmt.Println("unable to read stdin for program arguments: " + genErr.Error())
			os.Exit(1)
		}
		genArgs := Args{}
		if len(genStdinData) > 0 {
			genErr = json.Unmarshal(genStdinData, &genArgs)
			if genErr != nil {
				fmt.Println("unable to unmarshal JSON data from stdin: " + genErr.Error())
				os.Exit(1)
			}
		}
		genErr = %s(genArgs)
		if genErr != nil {
			fmt.Println(genErr)
			os.Exit(1)
		}
		os.Exit(0)
		// GENERATED
	`, fi.Name)
	err = textfile.RewriteLineByLineInPlace(name, func(line *string, lineNum int) *string {
		if strings.Contains(*line, "func main()") {
			return &mainCode
		}
		return line
	})
	if err != nil {
		return nil, errors.Annotatef(err, "failed to insert nanofunc call")
	}

	p.BinFileName = fmt.Sprintf("%s_line_%d", fi.FullName, fi.Anonymous.LineNumber)
	p.FullPath = path.Join(p.Directory, p.BinFileName)
	err = compile(p.Directory, p.BinFileName)
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return nil, errors.Annotatef(err, "failed to compile function '%+v':\n%s", fi, string(err.Stderr))
		}
		return nil, errors.Annotatef(err, "failed to compile function '%+v'", fi)
	}
	log.Debug().Msgf("created function program at '%s'", p.Directory)
	return p, nil
}

func compile(sourceDirectory, binName string) error {
	cmd := exec.Command("goimports", "-w", ".")
	cmd.Dir = sourceDirectory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return errors.Annotatef(err, "failed to run goimports on generated source")
	}
	buildArgs := []string{"build", "-o", binName, "."}
	cmd = exec.Command("go", buildArgs...)
	cmd.Dir = sourceDirectory
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
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
	cmd = exec.Command("go", buildArgs...)
	cmd.Dir = sourceDirectory
	err = cmd.Run()
	if err != nil {
		// TODO: logging here
		return errors.Annotatef(err, "failed to compile generated source in '%s'", sourceDirectory)
	}
	return nil
}

// copyBuilderModule copies project source from one place to another
func copyBuilderModule(sourceDir, destDir string) error {
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
	if destPath == sourcePath {
		return errors.Errorf("cannot copy '%s' into '%s' as they are the same", sourcePath, destPath)
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
