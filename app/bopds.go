package app

import (
	"fmt"
	"os"
	"flag"
	"net/http"
)

func CLI(args []string) int {
    var app appEnv
    err := app.fromArgs(args)
    if err != nil {
        return 2
    }
    if err = app.run(); err != nil {
        fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
        return 1
    }
    return 0
}

type appEnv struct {
	server *http.Server
	portNumber	int
	libraryPath string
}

func (app *appEnv) fromArgs(args []string) error {
    fl := flag.NewFlagSet("bopds", flag.ContinueOnError)

    fl.IntVar(&app.portNumber, "p", 3001, "Port number (default 3001)")
    fl.StringVar(&app.libraryPath, "l", "./lib", "Path to library (default ./lib)")

    if err := fl.Parse(args); err != nil {
        fl.Usage()
        return err
    }

    return nil
}

func (app *appEnv) run() error {

    return nil
}
