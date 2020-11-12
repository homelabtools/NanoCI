package main

import (
	"fmt"
	"os"

	. "github.com/homelabtools/noci/builder"
	"github.com/juju/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func handleError(err error) {
	if err == nil {
		return
	}
	msg := errors.ErrorStack(err)
	log.Error().Msg(msg)
	os.Exit(1)
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	handleError(mainE())
}

func mainE() error {
	Stage( //Parallel(
		Task(func() error {
			_, _, err := SH("echo task 1; sleep 2; echo task1a; sleep 1; echo task1b")
			return err
		}),
		Task("echo 2", func() error {
			_, _, err := SH("echao task 2")
			return err
		}).NoFailOnError(), //),
		Task("task 3", func() error {
			_, e, err := SH("echo nofail 1>&2")
			fmt.Println(e)
			return err
		}),
	)
	return nil
}
