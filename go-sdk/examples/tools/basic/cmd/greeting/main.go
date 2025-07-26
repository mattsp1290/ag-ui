package main

import (
	"log"

	greeting "github.com/ag-ui/go-sdk/examples/tools/greeting"
)

func main() {
	if err := greeting.RunGreetingExample(); err != nil {
		log.Fatalf("Greeting example failed: %v", err)
	}
}