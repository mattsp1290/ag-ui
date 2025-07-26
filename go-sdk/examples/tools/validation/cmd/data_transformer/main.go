package main

import (
	"log"

	datatransformer "github.com/ag-ui/go-sdk/examples/tools/validation/data-transformer"
)

func main() {
	if err := datatransformer.RunDataTransformerExample(); err != nil {
		log.Fatalf("Data transformer example failed: %v", err)
	}
}