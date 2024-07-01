package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dominikbraun/graph"
	"github.com/jamesjellow/fpm/handlers"
)

const usage = `
Usage:

fpm install        install all the dependencies in your project
fpm add <foo>      add the <foo> dependency to your project

`

var handlerInstance handlers.HandlerInterface = handlers.RealHandlers{}

func main() {
	err := run(os.Args)
	if err != nil {
		log.SetFlags(0)
		log.Fatal(err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		err := fmt.Errorf("expected 'add' or 'install' subcommand\n%s", usage)
		return err
	}

	// Initialize the dependency graph
	depGraph := graph.New(graph.StringHash, graph.Directed(), graph.PreventCycles())

	switch args[1] {
	case "add":
		return handlerInstance.HandleAdd(args, &depGraph)
	case "install":
		return handlerInstance.HandleInstall(&depGraph)
	default:
		err := fmt.Errorf("unknown subcommand: %s\n%s", strings.Join(args[1:], " "), usage)
		return err
	}
}
