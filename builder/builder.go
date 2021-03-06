package builder

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/homelabtools/nanoci/codegen"
	"github.com/homelabtools/nanoci/mirror"
	"github.com/juju/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Args is an alias for map[string]interface{}, which is used to pass arguments to a ContextFunc via JSON serialization
type Args = map[string]interface{}

// ContextFunc is a function that can run in some external context, such as inside a Docker container or a VM
type ContextFunc func(Args) error

// Context represents an external environment in which a ContextFunc is run, such as a Docker container or a VM
type Context struct {
	fn       ContextFunc
	TaskFunc func(Args) error
	funcInfo *mirror.FunctionInfo
}

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

// BuilderMain is the setup function that should be called be builder.go's main function
func BuilderMain() {
}

func BuilderExit(err error) {
	if err != nil {
		panic(err)
	}
	os.Exit(0)
}

// Begin starts a workflow of stages
func Begin(stages ...*Task) {
	all := Stage("root", stages...)
	all.fn()
}

// ExternalProcess runs a ContextFunc in another process
func ExternalProcess(fn ContextFunc) *Context {
	fi, err := mirror.FuncInfo(fn, 1)
	if err != nil {
		panic(err)
	}
	wd, _ := os.Getwd()
	taskFn := func(args Args) error {
		p, err := codegen.ProgramizeFunctionAt(fi, path.Join(wd, "..", "func"))
		//p, err := codegen.CreateProgramFromFunction(fi)
		if err != nil {
			return errors.Trace(err)
		}
		//defer p.Remove()
		return errors.Trace(p.Run(args))
	}
	return &Context{
		funcInfo: fi,
		fn:       fn,
		TaskFunc: taskFn,
	}
}

// Inside runs something inside another context, like a container or a VM
func Inside(context *Context, args Args) *Task {
	task := &Task{
		fn: func() error {
			return context.TaskFunc(args)
		},
	}
	return task
}

// Task is a task
type Task struct {
	callLocation  string
	name          string
	fn            func() error
	noFailOnError bool
}

// NoFailOnError indicates that a task should not fail if it returns an error
func (t *Task) NoFailOnError() *Task {
	t.noFailOnError = true
	return t
}

func (t *Task) failureString() string {
	if t.name == "" {
		return "Failure in task from " + t.callLocation
	}
	return fmt.Sprintf("Failure in task '%s' from %s", t.name, t.callLocation)
}

// Stage executes a sequence of tasks one after another, failing if any of the tasks fail.
func Stage(name string, tasks ...*Task) *Task {
	_, fn := buildTaskFunc(func() error {
		defer recoverError()
		for _, t := range tasks {
			err := t.fn()
			if err != nil && !t.noFailOnError {
				log.Error().Msgf(t.failureString()+": %s", errors.ErrorStack(err))
				return errors.Annotatef(err, "task within stage '%s' failed", name)
			} else if err != nil && t.noFailOnError {
				log.Error().Msgf(t.failureString()+" in stage '%s': %s", name, errors.ErrorStack(err))
			}
		}
		return nil
	})
	return &Task{fn: fn}
}

// Step executes a function as a task
//func Step(fn func() error) *Task {
func Step(args ...interface{}) *Task {
	defer recoverError()
	argErrorMsg := "Invalid arguments for the Task function, must be one of:\n[name string, task func() error], [name string, task func()], [task func() error] or [task func()]"
	task := &Task{}
	switch len(args) {
	case 1:
		isCorrectType, fn := buildTaskFunc(args[0])
		if !isCorrectType {
			panic(errors.New("The function argument for Task must be either `func() error` or `func()"))
		}
		task.fn = fn
	case 2:
		switch arg1 := args[0].(type) {
		case string:
			task.name = arg1
		default:
			panic(errors.New("The name argument for Task must be a string"))
		}
		isCorrectType, fn := buildTaskFunc(args[1])
		if !isCorrectType {
			panic(errors.New("The second argument for Task must be either `func() error` or `func()`"))
		}
		task.fn = fn
	default:
		panic(errors.New(argErrorMsg))
	}

	_, file, line, _ := runtime.Caller(1)
	task.callLocation = fmt.Sprintf("%s:%d", file, line)
	return task
}

func buildTaskFunc(taskFunc interface{}) (bool, func() error) {
	switch arg1 := taskFunc.(type) {
	case func() error:
		return true, arg1
	case func():
		return true, func() error {
			arg1()
			return nil
		}
	default:
		return false, nil
	}
}

// Parallel runs one or more tasks in parallel
func Parallel(tasks ...*Task) *Task {
	return &Task{fn: func() error {
		wg := sync.WaitGroup{}
		for _, t := range tasks {
			wg.Add(1)
			go func(t *Task) {
				fnErr := t.fn()
				defer func() {
					if err := recover(); err != nil {
						if e, ok := err.(error); ok {
							log.Error().Msgf("%s: %s", t.failureString(), errors.ErrorStack(e))
						} else {
							log.Error().Msgf("%s: %+v", t.failureString(), err)
						}
					}
					if fnErr != nil {
						log.Error().Msg(errors.ErrorStack(fnErr))
					}
					wg.Done()
				}()
			}(t)
		}
		wg.Wait()
		return nil
	}}
}

func recoverError() {
	if err := recover(); err != nil {
		switch err := err.(type) {
		case error:
			log.Error().Msgf("%s", errors.ErrorStack(err))
		default:
			log.Error().Msgf("%+v", err)
		}
		msg := string(debug.Stack())
		msg = msg[strings.Index(msg, "panic("):]
		msg = msg[strings.Index(msg, "panic.go"):]
		msg = msg[strings.Index(msg, "\n"):]
		log.Fatal().Msg(msg)
	}
}

func handleError(err error) {
	if err == nil {
		return
	}
	msg := errors.ErrorStack(err)
	log.Error().Msg(msg)
	os.Exit(1)
}

// SH runs an arbitrary shell commanear
func SH(shellCommand string) (string, string, error) {
	cmd := exec.Command("sh", "-c", shellCommand)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", errors.Trace(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", "", errors.Trace(err)
	}
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	stdoutTee := io.TeeReader(stdout, stdoutBuf)
	stderrTee := io.TeeReader(stderr, stderrBuf)
	err = cmd.Start()
	if err != nil {
		return "", "", errors.Trace(err)
	}
	go io.Copy(os.Stdout, stdoutTee)
	go io.Copy(os.Stderr, stderrTee)
	err = cmd.Wait()
	stdoutContents := strings.TrimSpace(string(stdoutBuf.Bytes()))
	stderrContents := strings.TrimSpace(string(stderrBuf.Bytes()))
	if err != nil {
		return "", "", errors.Trace(err)
	}
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return stdoutContents, stderrContents, errors.Annotatef(err, "Command '%s' failed with exit code %d", shellCommand, e.ProcessState.ExitCode())
		}
		return stdoutContents, stderrContents, errors.Trace(err)
	}
	return stdoutContents, stderrContents, nil
}
