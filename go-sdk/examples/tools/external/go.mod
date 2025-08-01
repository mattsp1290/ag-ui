module github.com/ag-ui/go-sdk/examples/tools/external

go 1.24.4

replace github.com/ag-ui/go-sdk => ../../..

replace github.com/ag-ui/go-sdk/examples/tools/external/rest-client => ./rest-client

replace github.com/ag-ui/go-sdk/examples/tools/external/weather-api => ./weather-api

require (
	github.com/ag-ui/go-sdk/examples/tools/external/rest-client v0.0.0-00010101000000-000000000000
	github.com/ag-ui/go-sdk/examples/tools/external/weather-api v0.0.0-00010101000000-000000000000
)

require (
	github.com/ag-ui/go-sdk v0.1.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/text v0.23.0 // indirect
)
