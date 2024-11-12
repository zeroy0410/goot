package defer_exectution_001_T

import (
	"fmt"
	"os"
)

func main(file_name string) {
	__taint_src, _ := os.ReadFile(file_name)
	defer_exectution_001_T(string(__taint_src))
}

func defer_exectution_001_T(__taint_src string) {
	___taint_sink(__taint_src)
}

func ___taint_sink(o interface{}) {
	fmt.Println(o)
}
