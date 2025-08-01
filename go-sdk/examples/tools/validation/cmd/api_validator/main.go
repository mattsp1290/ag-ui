package main

import (
	"log"

	apivalidator "github.com/ag-ui/go-sdk/examples/tools/validation/api-validator"
)

func main() {
	if err := apivalidator.RunApiValidatorExample(); err != nil {
		log.Fatalf("API validator example failed: %v", err)
	}
}