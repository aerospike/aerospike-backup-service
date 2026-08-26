package main

import (
	"fmt"
	"os"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintf(os.Stderr, "generate OpenAPI: %v\n", err)
		os.Exit(1)
	}
}
