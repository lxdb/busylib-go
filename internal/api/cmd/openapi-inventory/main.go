package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lxdb/busylib-go/internal/api"
)

func main() {
	input := flag.String("input", "", "OpenAPI YAML input path")
	output := flag.String("output", "", "inventory JSON output path")
	flag.Parse()

	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "-input and -output are required")
		os.Exit(2)
	}

	inventory, err := api.BuildInventoryFile(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build inventory: %v\n", err)
		os.Exit(1)
	}

	file, err := os.Create(*output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create output: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	if err := api.WriteInventory(file, inventory); err != nil {
		fmt.Fprintf(os.Stderr, "write inventory: %v\n", err)
		os.Exit(1)
	}
}
