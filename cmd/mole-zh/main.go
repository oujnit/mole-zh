package main

import (
	"os"

	"github.com/oujnit/mole-zh/internal/plugin"
)

func main() {
	os.Exit(plugin.Main(os.Args[1:]))
}
