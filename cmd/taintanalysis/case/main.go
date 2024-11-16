package main

import "fmt"

type Person interface {
	Run()
}

type Stu struct {
	Name string
	Age  int
}

func (a *Stu) Run() {
	fmt.Println(a.Name, "is running")
}

type Tea struct {
	Name string
	Age  int
}

func (t *Tea) Run() {
	fmt.Println(t.Name, "is running")
}

func main() {
	// 创建 Stu 和 Tea 的实例
	stu1 := &Stu{Name: "Alice", Age: 20}
	tea1 := &Stu{Name: "Bob", Age: 30}

	// 创建一个 Person 类型的切片，存储 Stu 和 Tea 的实例
	persons := []Person{stu1, tea1}

	// 遍历切片并调用 Run 方法
	for _, person := range persons {
		person.Run()
	}

	// 进一步测试类型断言
	for _, person := range persons {
		switch p := person.(type) {
		case *Stu:
			fmt.Println("This is a student:", p.Name)
		case *Tea:
			fmt.Println("This is a teacher:", p.Name)
		}
	}
}
