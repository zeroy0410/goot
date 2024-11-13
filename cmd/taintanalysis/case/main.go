package main

import (
	"fmt"
	"os/exec"
)

func main() {
	var fileName string
	_, err := fmt.Scanf("%s", &fileName)
	if err != nil {
		return
	}
	fmt.Println(fileName)
	runCmd(fileName)
	runCmd(fileName + "hahahah")
}

func runCmd(fileName string) {
	exec.Command("go", "run", fileName)
}
