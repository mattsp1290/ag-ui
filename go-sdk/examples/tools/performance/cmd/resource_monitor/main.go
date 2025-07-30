package main

import (
	"log"

	resourcemonitor "github.com/ag-ui/go-sdk/examples/tools/performance/resource-monitor"
)

func main() {
	if err := resourcemonitor.RunResourceMonitorExample(); err != nil {
		log.Fatalf("Resource monitor example failed: %v", err)
	}
}