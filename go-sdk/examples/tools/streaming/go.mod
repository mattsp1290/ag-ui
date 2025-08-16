module github.com/mattsp1290/ag-ui/go-sdk/examples/tools/streaming

go 1.24.4

replace github.com/mattsp1290/ag-ui/go-sdk => ../../../

replace github.com/mattsp1290/ag-ui/go-sdk/examples/tools/streaming/data-processor => ./data-processor

replace github.com/mattsp1290/ag-ui/go-sdk/examples/tools/streaming/log-streaming => ./log-streaming

require (
	github.com/mattsp1290/ag-ui/go-sdk v0.1.0
	github.com/mattsp1290/ag-ui/go-sdk/examples/tools/streaming/data-processor v0.0.0-00010101000000-000000000000
	github.com/mattsp1290/ag-ui/go-sdk/examples/tools/streaming/log-streaming v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
