package mirror

import (
	"reflect"
	"regexp"
	"runtime"
	"unicode"

	"github.com/homelabtools/nanoci/codegen"
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
			fi.FileName, fi.LineNumber, fi.Source, err = codegen.ExtractAnonymousFuncSource(offset + 1)
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
