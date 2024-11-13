package taint

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"
)

// Imethod represents an interface method I.m.
// (There's no go/types object for it;
// a *types.Func may be shared by many interfaces due to interface embedding.)
type Imethod struct {
	I  *types.Interface
	id string
}

// InterfaceHierarchy represents implemetation relations
type InterfaceHierarchy struct {
	funcsBySig    *typeutil.Map
	methodsMemo   *map[Imethod][]*ssa.Function
	methodsByName *map[string][]*ssa.Function
}

// LookupMethods returns an interface method's implemetations
func (i *InterfaceHierarchy) LookupMethods(I *types.Interface, m *types.Func) []*ssa.Function {
	id := m.Id()
	methods, ok := (*i.methodsMemo)[Imethod{I, id}]
	if !ok {
		for _, f := range (*i.methodsByName)[m.Name()] {
			C := f.Signature.Recv().Type() // named or *named
			if types.Implements(C, I) {
				methods = append(methods, f)
			}
		}
		(*i.methodsMemo)[Imethod{I, id}] = methods
	}
	return methods
}

// LookupFuncs returns *ssa.Function that have same signature
func (i *InterfaceHierarchy) LookupFuncs(signature *types.Signature) []*ssa.Function {
	funcs := i.funcsBySig.At(signature)
	if funcs == nil {
		return nil
	}
	return funcs.([]*ssa.Function)
}

// NewInterfaceHierarchy returns an InterfaceHierarchy
func NewInterfaceHierarchy(allFuncs *map[*ssa.Function]bool) *InterfaceHierarchy {

	// funcsBySig contains all functions, keyed by signature.  It is
	// the effective set of address-taken functions used to resolve
	// a dynamic call of a particular signature.
	var funcsBySig typeutil.Map // value is []*ssa.Function

	// methodsByName contains all methods,
	// grouped by name for efficient lookup.
	// (methodsById would be better but not every SSA method has a go/types ID.)
	methodsByName := make(map[string][]*ssa.Function)

	// methodsMemo records, for every abstract method call I.m on
	// interface type I, the set of concrete methods C.m of all
	// types C that satisfy interface I.
	//
	// Abstract methods may be shared by several interfaces,
	// hence we must pass I explicitly, not guess from m.
	//
	// methodsMemo is just a cache, so it needn't be a typeutil.Map.
	methodsMemo := make(map[Imethod][]*ssa.Function)

	for f := range *allFuncs {
		// 遍历 allFuncs 中的每一个函数 f
		if f.Signature.Recv() == nil {
			// 如果函数没有接收者（即不是方法），则进入此分支
			// Package initializers can never be address-taken.
			if f.Name() == "init" && f.Synthetic == "package initializer" {
				// 如果函数的名字是 "init" 且其被标记为 "package initializer"（包初始化函数），则跳过此函数
				continue
			}
			// 从 funcsBySig 映射中获取与 f.Signature 对应的函数列表
			funcs, _ := funcsBySig.At(f.Signature).([]*ssa.Function)
			// 将当前函数 f 添加到该列表中
			funcs = append(funcs, f)
			// 将更新后的函数列表重新存储到 funcsBySig 中，以 f.Signature 作为键
			funcsBySig.Set(f.Signature, funcs)
		} else {
			// 如果函数有接收者（即是方法），则进入此分支
			// 将当前方法 f 添加到 methodsByName 中，以方法名 f.Name() 作为键
			methodsByName[f.Name()] = append(methodsByName[f.Name()], f)
		}
	}
	return &InterfaceHierarchy{funcsBySig: &funcsBySig, methodsMemo: &methodsMemo, methodsByName: &methodsByName}
}
