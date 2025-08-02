package main

import (
	"log"

	fileops "github.com/mattsp1290/ag-ui/go-sdk/examples/tools/file-operations"
)

func main() {
	if err := fileops.RunFileOperationsExample(); err != nil {
		log.Fatalf("File operations example failed: %v", err)
	}
}