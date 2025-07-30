module github.com/ag-ui/go-sdk/examples/tools/basic

go 1.23.0

toolchain go1.24.4

replace github.com/ag-ui/go-sdk => ../../../

replace github.com/ag-ui/go-sdk/examples/tools/calculator => ../calculator

replace github.com/ag-ui/go-sdk/examples/tools/greeting => ../greeting

replace github.com/ag-ui/go-sdk/examples/tools/file-operations => ../file-operations

require (
	github.com/ag-ui/go-sdk/examples/tools/calculator v0.0.0-00010101000000-000000000000
	github.com/ag-ui/go-sdk/examples/tools/file-operations v0.0.0-00010101000000-000000000000
	github.com/ag-ui/go-sdk/examples/tools/greeting v0.0.0-00010101000000-000000000000
)

require (
	github.com/ag-ui/go-sdk v0.0.0-00010101000000-000000000000 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/text v0.23.0 // indirect
)
