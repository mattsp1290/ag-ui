package main

import (
	"log"

	cachedemo "github.com/mattsp1290/ag-ui/go-sdk/examples/tools/performance/cache-demo"
)

func main() {
	if err := cachedemo.RunCacheDemoExample(); err != nil {
		log.Fatalf("Cache demo example failed: %v", err)
	}
}