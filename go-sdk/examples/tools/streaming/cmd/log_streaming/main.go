package main

import (
	"log"

	logstreaming "github.com/mattsp1290/ag-ui/go-sdk/examples/tools/streaming/log-streaming"
)

func main() {
	if err := logstreaming.RunLogStreamingExample(); err != nil {
		log.Fatalf("Log streaming example failed: %v", err)
	}
}