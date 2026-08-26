package main

import (
	"fmt"
	"os"
)

func main() {
	workspace, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate OpenAPI: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) == 2 {
		workspace = os.Args[1]
	} else if len(os.Args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s [workspace]\n", os.Args[0])
		os.Exit(1)
	}

	if err := generate(workspace); err != nil {
		fmt.Fprintf(os.Stderr, "generate OpenAPI: %v\n", err)
		os.Exit(1)
	}
}
