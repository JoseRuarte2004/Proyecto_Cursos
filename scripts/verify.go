package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "no pude resolver el directorio actual: %v\n", err)
		os.Exit(1)
	}

	verifier := NewVerifier(root, filepath.Join(root, "reports"))
	runErr := verifier.Run()
	verifier.PrintSummary()

	if runErr != nil || len(verifier.failures) > 0 {
		os.Exit(1)
	}
}
