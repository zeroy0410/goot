package main

import (
	"github.com/cokeBeer/goot/pkg/example/dataflow/typeassertion"
)

const src = `package main

import (
	"fmt"
	// "errors"
)

// 定义一个基础接口
type Describer interface {
	Describe() string
}

// 定义另一个接口，嵌入了 Describer
type DetailedDescriber interface {
	Describer
	Details() string
}

// 定义一个结构体，实现 Describer 接口
type Person struct {
	Name string
	Age  int
}

func (p Person) Describe() string {
	return fmt.Sprintf("Name: %s, Age: %d", p.Name, p.Age)
}

// 定义另一个结构体，实现 DetailedDescriber 接口
type Employee struct {
	Person
	Position string
}

func (e Employee) Describe() string {
	return fmt.Sprintf("%s, Position: %s", e.Person.Describe(), e.Position)
}

func (e Employee) Details() string {
	return fmt.Sprintf("Employee Details - Name: %s, Age: %d, Position: %s", e.Name, e.Age, e.Position)
}

// 定义一个函数，通过接口进行多态处理
func ProcessDescriber(d Describer) {
	fmt.Println("Description:", d.Describe())

	// 类型断言为 DetailedDescriber
	if detailed, ok := d.(DetailedDescriber); ok {
		fmt.Println("Details:", detailed.Details())
	} else {
		fmt.Println("No detailed description available.")
	}

	// 类型断言为 *Person
	if person, ok := d.(*Person); ok {
		fmt.Println("Person's Name:", person.Name)
	} else {
		fmt.Println("Not a Person type.")
	}
}

// 定义一个接口返回函数
func GetDescriber(asDetailed bool) Describer {
	if asDetailed {
		return Employee{
			Person:   Person{Name: "Alice", Age: 30},
			Position: "Engineer",
		}
	}
	return &Person{Name: "Bob", Age: 25}
}

// 定义一个函数，实现 interface{} 的处理
func InterfaceConversion(i interface{}) {
	switch v := i.(type) {
	case Describer:
		fmt.Println("This is a Describer:", v.Describe())
	case *Person:
		fmt.Println("This is a Person pointer:", v.Name)
	case string:
		fmt.Println("This is a string:", v)
	default:
		fmt.Println("Unknown type")
	}
}

// Main 函数
func main() {
	// 创建一个 Employee 实例
	emp := Employee{
		Person:   Person{Name: "John", Age: 40},
		Position: "Manager",
	}
	// 类型转换和接口处理
	InterfaceConversion(emp)
}
`

func main() {
	runner := typeassertion.NewRunner(src, "main")
	runner.Run()
}
