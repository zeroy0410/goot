package main

import (
	"fmt"
	"os/exec"
)

func main() {
	userInput := "ls" // Simulated user input, this is the taint source
	runCommand(userInput)
}

func runCommand(command string) {
	cmd := exec.Command(command) // This is the taint sink
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Output:", output)
}
