package main

import (
	"fmt"

	"github.com/homelabtools/nanoci/builder"
	. "github.com/homelabtools/nanoci/builder"
)

func main() {
	builder.BuilderMain()
	Step("", func() {})
	//Begin(Stage("my build",
	//	Step(func() error {
	//		_, _, err := SH("echo task 1; sleep 2; echo task1a; sleep 1; echo task1b")
	//		return err
	//	}),
	//	Step("echo 2", func() error {
	//		_, _, err := SH("echao task 2")
	//		return err
	//	}).NoFailOnError(), //),
	//	Step("task 3", func() error {
	//		_, e, err := SH("echo nofail 1>&2")
	//		fmt.Println(e)
	//		return err
	//	}),
	//))
	//Context(func(m map[string]interface{}) {
	//	fmt.Println("🤘")
	//})
	//Context(foo)
	//Context(A.B)
}

func foo(args map[string]interface{}) {
	fmt.Println("🤘")
}

type A struct {
}

func (A) B() {
	fmt.Println("B")
}

// Context executes a function elsewhere
func Context(fn interface{}) {
}

//// Context executes a function elsewhere
//func Context(fn interface{}) {
//	err := codegen.ProgramizeFunction(1, "gen")
//	if err != nil {
//		panic(err)
//	}
//}
