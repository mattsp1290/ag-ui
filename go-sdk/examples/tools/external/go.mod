module github.com/mattsp1290/ag-ui/go-sdk/examples/tools/external

go 1.24.4

replace github.com/mattsp1290/ag-ui/go-sdk => ../../..

replace github.com/mattsp1290/ag-ui/go-sdk/examples/tools/external/rest-client => ./rest-client

replace github.com/mattsp1290/ag-ui/go-sdk/examples/tools/external/weather-api => ./weather-api

require (
	github.com/mattsp1290/ag-ui/go-sdk/examples/tools/external/rest-client v0.0.0-00010101000000-000000000000
	github.com/mattsp1290/ag-ui/go-sdk/examples/tools/external/weather-api v0.0.0-00010101000000-000000000000
)

require (
	github.com/mattsp1290/ag-ui/go-sdk v0.1.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/text v0.25.0 // indirect
)
