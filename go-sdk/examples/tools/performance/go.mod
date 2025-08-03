module github.com/mattsp1290/ag-ui/go-sdk/examples/tools/performance

go 1.24.4

replace github.com/mattsp1290/ag-ui/go-sdk => ../../..

replace github.com/mattsp1290/ag-ui/go-sdk/examples/tools/performance/async-executor => ./async-executor

replace github.com/mattsp1290/ag-ui/go-sdk/examples/tools/performance/batch-processor => ./batch-processor

replace github.com/mattsp1290/ag-ui/go-sdk/examples/tools/performance/cache-demo => ./cache-demo

replace github.com/mattsp1290/ag-ui/go-sdk/examples/tools/performance/resource-monitor => ./resource-monitor

require (
	github.com/mattsp1290/ag-ui/go-sdk/examples/tools/performance/async-executor v0.0.0-00010101000000-000000000000
	github.com/mattsp1290/ag-ui/go-sdk/examples/tools/performance/batch-processor v0.0.0-00010101000000-000000000000
	github.com/mattsp1290/ag-ui/go-sdk/examples/tools/performance/cache-demo v0.0.0-00010101000000-000000000000
	github.com/mattsp1290/ag-ui/go-sdk/examples/tools/performance/resource-monitor v0.0.0-00010101000000-000000000000
)

require (
	github.com/mattsp1290/ag-ui/go-sdk v0.0.0-00010101000000-000000000000 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/text v0.23.0 // indirect
)