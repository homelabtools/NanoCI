// Package textfile contains utilities for easily processing text files.
package textfile

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"

	"github.com/juju/errors"
)

// RewriteLineByLine rewrites text files line by line, calling the modifier function for each line.
// The modifier function should return a pointer to the new line of text, or nil if the line should
// be removed from the file.
func RewriteLineByLine(sourceFile, destFile string, modifier func(line *string, lineNum int) *string) error {
	data, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		return errors.Trace(err)
	}
	file, err := os.Create(destFile)
	if err != nil {
		return errors.Trace(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		newLine := modifier(&line, lineNum)
		if newLine == nil {
			continue
		}
		_, err := file.WriteString(*newLine + "\n")
		if err != nil {
			return errors.Annotatef(err, "failed processing file '%s'", sourceFile)
		}
	}
	return nil
}

// RewriteLineByLineInPlace is just like RewriteLineByLine but writes back out to the same file instead of a different one.
func RewriteLineByLineInPlace(sourceFile string, modifier func(line *string, lineNum int) *string) error {
	return RewriteLineByLine(sourceFile, sourceFile, modifier)
}
