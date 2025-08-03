package main

import (
	"log"

	greeting "github.com/mattsp1290/ag-ui/go-sdk/examples/tools/greeting"
)

func main() {
	if err := greeting.RunGreetingExample(); err != nil {
		log.Fatalf("Greeting example failed: %v", err)
	}
}