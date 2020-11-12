package main

import (
	"fmt"

	. "github.com/homelabtools/noci/builder"
)

func main() {
	Begin(Stage("my build",
		Step(func() error {
			_, _, err := SH("echo task 1; sleep 2; echo task1a; sleep 1; echo task1b")
			return err
		}),
		Step("echo 2", func() error {
			_, _, err := SH("echao task 2")
			return err
		}).NoFailOnError(), //),
		Step("task 3", func() error {
			_, e, err := SH("echo nofail 1>&2")
			fmt.Println(e)
			return err
		}),
	))
}
