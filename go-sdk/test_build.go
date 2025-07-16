package main

import (
	"fmt"
	"os/exec"
)

func main() {
	cmd := exec.Command("go", "build", "./pkg/core/events")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Build failed with error:")
		fmt.Println(string(output))
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Build successful!")
	}
}