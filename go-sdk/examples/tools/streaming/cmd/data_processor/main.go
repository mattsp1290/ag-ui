package main

import (
	"log"

	dataprocessor "github.com/mattsp1290/ag-ui/go-sdk/examples/tools/streaming/data-processor"
)

func main() {
	if err := dataprocessor.RunDataProcessorExample(); err != nil {
		log.Fatalf("Data processor example failed: %v", err)
	}
}