package main

import (
	"log"

	calculator "github.com/ag-ui/go-sdk/examples/tools/calculator"
)

func main() {
	if err := calculator.RunCalculatorExample(); err != nil {
		log.Fatalf("Calculator example failed: %v", err)
	}
}