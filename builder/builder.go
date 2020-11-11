package builder

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/juju/errors"
)

// SH runs an arbitrary shell command
func SH(shellCommand string) (string, error) {
	cmd := exec.Command("sh", "-c", shellCommand)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", errors.Trace(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", errors.Trace(err)
	}
	err = cmd.Start()
	if err != nil {
		return "", errors.Trace(err)
	}
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)
	err = cmd.Wait()
	out, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", errors.Trace(err)
	}
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return string(out), errors.Annotatef(err, "Command '%s' failed with exit code %d", shellCommand, e.ProcessState.ExitCode())
		}
		return "", errors.Trace(err)
	}
	return string(out), nil
}
