// Command stylecheck enforces the Lesstruct Go file-layout contract.
package main

import (
	"fmt"
	"os"

	"github.com/aristorinjuang/lesstruct/tools/stylecheck"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	findings, err := stylecheck.NewChecker().CheckDir(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stylecheck: %v\n", err)
		os.Exit(2)
	}
	for _, finding := range findings {
		fmt.Println(finding)
	}
	if len(findings) > 0 {
		fmt.Fprintf(os.Stderr, "stylecheck: %d violation(s)\n", len(findings))
		os.Exit(1)
	}
}
