package main

import "fmt"

func processValue(v interface{}) {
	switch v.(type) {
	case string:
		fmt.Println("String value:", v)
	case int:
		fmt.Println("Integer value:", v)
	default:
		fmt.Println("Unknown type")
	}
}

func main() {
	var i interface{} = true

	// 我们没有处理 bool 类型的情况
	processValue(i)
}
