package main

import (
	"github.com/zeroy0410/goot/pkg/example/dataflow/constantpropagation"
)

const src = `package main

type I interface{
	// Run()
}

type A struct {
    a int
    b int
}

type B struct {
    a int
    b int
}

func Hello() int {
	var a A
	var b B
	var i I
	x := 1
	if x < 1 {
		i = a
	} else {
		i = b 
	}
	return i.(int)
}`

func main() {
	runner := constantpropagation.NewRunner(src, "Hello")
	runner.Run()
}
