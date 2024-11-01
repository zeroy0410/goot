package typeassertion

import (
	// "go/token"
	// "math"
	// "fmt"
	"fmt"

	"github.com/cokeBeer/goot/pkg/dataflow/golang/switcher"
	"golang.org/x/tools/go/ssa"
)

type TypeAssertionSwitcher struct {
	switcher.BaseSwitcher
	typeAssertionAnalysis *TypeAssertionAnalysis
	inMap                 *map[any]any
	outMap                *map[any]any
}

// Helper function to check if a slice contains a particular string
func contains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func (s *TypeAssertionSwitcher) CaseTypeAssert(inst *ssa.TypeAssert) {
	assertedType := inst.AssertedType.String()
	// 获取当前的切片，如果不存在则初始化为空切片
	currentSlice, ok := (*s.outMap)[inst.Name()].([]string)
	if !ok {
		currentSlice = []string{}
	}
	// 检查切片中是否已经存在该类型信息
	if !contains(currentSlice, assertedType) {
		// 使用 append 将新的类型信息添加到切片中
		(*s.outMap)[inst.Name()] = append(currentSlice, assertedType)
	}
}

func (s *TypeAssertionSwitcher) CaseMakeInterface(inst *ssa.MakeInterface) {
	typeInfo := inst.X.Type().String()
	// 获取当前的切片，如果不存在则初始化为空切片
	currentSlice, ok := (*s.outMap)[inst.Name()].([]string)
	if !ok {
		currentSlice = []string{}
	}
	// 检查切片中是否已经存在该类型信息
	if !contains(currentSlice, typeInfo) {
		// 使用 append 将新的类型信息添加到切片中
		(*s.outMap)[inst.Name()] = append(currentSlice, typeInfo)
	}
}

func (s *TypeAssertionSwitcher) CasePhi(inst *ssa.Phi) {
	// 获取当前的切片，如果不存在则初始化为空切片
	currentSlice, ok := (*s.outMap)[inst.Name()].([]string)
	if !ok {
		currentSlice = []string{}
	}
	for _, v := range inst.Edges {
		typeInfo := (*s.outMap)[v.Name()].([]string)
		if typeInfo == nil {
			continue
		}
		// 检查切片中是否已经存在该类型信息
		for _, now := range typeInfo {
			if !contains(currentSlice, now) {
				// 使用 append 将新的类型信息添加到切片中
				currentSlice = append(currentSlice, now)
			}
		}
	}
	(*s.outMap)[inst.Name()] = currentSlice
}

func (s *TypeAssertionSwitcher) CaseCall(inst *ssa.Call) {
    fmt.Println(inst.Name())
	fmt.Println(inst.Call.Value)
	fmt.Println(inst.Call.Args)
	fmt.Println()
	fmt.Println()
}