package main

import (
	"fmt"
	"os"

	"github.com/juju/errors"
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
	handleError(mainE())
}

func mainE() error {
	fmt.Println("Hello")
	return nil
}
