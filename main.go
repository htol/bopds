package main

import (
	"os"

	"github.com/htol/bopds/app"
)

func main() {
	os.Exit(app.CLI(os.Args[1:]))
}
