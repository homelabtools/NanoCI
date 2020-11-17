package codegen

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"

	"github.com/homelabtools/nanoci/mirror"
	"github.com/juju/errors"
	"github.com/rs/zerolog/log"
)

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
func (p *Program) Run(args interface{}) error {
	argData, err := json.Marshal(args)
	if err != nil {
		return errors.Annotatef(err, "failed to marshal args for generated program")
	}
	cmd := exec.Command(p.FullPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = bytes.NewReader(argData)
	return cmd.Run()
}
