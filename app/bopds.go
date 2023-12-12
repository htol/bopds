package app

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/htol/bopds/repo"
	"github.com/htol/bopds/scanner"
)

func CLI(args []string) int {
	var app appEnv
	if err := app.fromArgs(args); err != nil {
		fmt.Println(err)
		return 2
	}

	if err := app.run(); err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		return 1
	}
	return 0
}

type appEnv struct {
	server      *http.Server
	portNumber  int
	libraryPath string
	cmd         string
}

func (app *appEnv) fromArgs(args []string) error {
	fl := flag.NewFlagSet("bopds", flag.ContinueOnError)

	fl.IntVar(&app.portNumber, "p", 3001, "Port number (default 3001)")
	fl.StringVar(&app.libraryPath, "l", "./lib", "Path to library (default ./lib)")

	if err := fl.Parse(args); err != nil {
		fl.Usage()
		return err
	}

	if fl.NArg() < 1 {
		return fmt.Errorf("please provide a command to run")
	}

	app.cmd = fl.Arg(0)

	return nil
}

func (app *appEnv) run() error {
	switch app.cmd {
	case "scan":
		if err := scanner.ScanLibrary(app.libraryPath); err != nil {
			return err
		}
	case "serve":
		//http.ListenAndServe(fmt.Sprintf(":%d", app.portNumber), nil)
		fmt.Println("TODO: serve not implemented yet")
	case "init":
		repo.GetStorage("books.db")
	default:
		return fmt.Errorf("unknown command %s", app.cmd)
	}
	return nil
}
