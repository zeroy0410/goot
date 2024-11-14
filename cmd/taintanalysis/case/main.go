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
	runCmd(fileName, "hehe")
	runCmd(fileName+"hahahah", "hehe")
}

func runCmd(fileName string, hahaName string) {
	exec.Command("go", "run", fileName)
	exec.Command("go", hahaName)
}
