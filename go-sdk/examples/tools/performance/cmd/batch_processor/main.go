package main

import (
	"log"

	batchprocessor "github.com/ag-ui/go-sdk/examples/tools/performance/batch-processor"
)

func main() {
	if err := batchprocessor.RunBatchProcessorExample(); err != nil {
		log.Fatalf("Batch processor example failed: %v", err)
	}
}