package main

import (
	"log"

	weatherapi "github.com/ag-ui/go-sdk/examples/tools/external/weather-api"
)

func main() {
	if err := weatherapi.RunWeatherApiExample(); err != nil {
		log.Fatalf("Weather API example failed: %v", err)
	}
}