package main

import (
	"fmt"
	"github.com/ag-ui/go-sdk/pkg/state"
)

func main() {
	dc := state.NewDeltaComputer(state.DefaultDeltaOptions())
	
	unoptimized := state.JSONPatch{
		{Op: state.JSONPatchOpRemove, Path: "/temp"},
		{Op: state.JSONPatchOpAdd, Path: "/temp", Value: "new value"},
		{Op: state.JSONPatchOpAdd, Path: "/a", Value: 1},
		{Op: state.JSONPatchOpAdd, Path: "/b", Value: 2},
		{Op: state.JSONPatchOpRemove, Path: "/b"},
	}
	
	fmt.Println("Original operations:")
	for i, op := range unoptimized {
		fmt.Printf("%d: %s %s", i, op.Op, op.Path)
		if op.Value != nil {
			fmt.Printf(" = %v", op.Value)
		}
		fmt.Println()
	}
	
	optimized := dc.OptimizePatch(unoptimized)
	
	fmt.Println("\nOptimized operations:")
	for i, op := range optimized {
		fmt.Printf("%d: %s %s", i, op.Op, op.Path)
		if op.Value != nil {
			fmt.Printf(" = %v", op.Value)
		}
		fmt.Println()
	}
}