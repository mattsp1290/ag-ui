package main

import (
	"log"

	asyncexecutor "github.com/ag-ui/go-sdk/examples/tools/performance/async-executor"
)

func main() {
	if err := asyncexecutor.RunAsyncExecutorExample(); err != nil {
		log.Fatalf("Async executor example failed: %v", err)
	}
}