package main

import (
	"log"

	restclient "github.com/ag-ui/go-sdk/examples/tools/external/rest-client"
)

func main() {
	if err := restclient.RunRestClientExample(); err != nil {
		log.Fatalf("REST client example failed: %v", err)
	}
}