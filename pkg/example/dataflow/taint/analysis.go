package taint

import (
	"fmt"
	"go/types"
	"os"

	"github.com/zeroy0410/goot/pkg/dataflow/golang/switcher"
	"github.com/zeroy0410/goot/pkg/dataflow/toolkits/graph"
	"github.com/zeroy0410/goot/pkg/dataflow/toolkits/scalar"
	"github.com/zeroy0410/goot/pkg/dataflow/toolkits/solver"
	"github.com/zeroy0410/goot/pkg/dataflow/util/entry"
	"golang.org/x/tools/go/ssa"
)

// TaintAnalysis 代表一个污点分析
type TaintAnalysis struct {
	scalar.BaseFlowAnalysis
	taintSwitcher *TaintSwitcher
	passThrough   *PassThrough
	config        *TaintConfig
}

// Run 启动一个函数的污点分析
func Run(f *ssa.Function, c *TaintConfig) {
	// 如果已经在其他地方记录在 passThroughContainer 中，则跳过
	if _, ok := (*c.PassThroughContainer)[f.String()]; ok {
		return
	}

	if needNull(f, c) {
		// 如果函数是递归的或没有函数体，则初始化为空
		initNull(f, c)
		return
	}

	// 如果是目标函数，把函数信息写到标准输出
	if f.String() == c.TargetFunc {
		f.WriteTo(os.Stdout)
	}

	// 否则，执行对 *ssa.Function 的分析
	doRun(f, c)
}

// doRun 执行函数的污点分析
func doRun(f *ssa.Function, c *TaintConfig) {
	// 将函数标记为已访问以防止递归
	recordCall(f, c)

	// 创建一个新的分析
	g := graph.New(f)
	a := New(g, c)

	// 在调试模式下解决分析
	solver.Solve(a, c.Debug)
}

// recordCall 记录调用历史以防止递归
func recordCall(f *ssa.Function, c *TaintConfig) {
	(*c.History)[f.String()] = true
	c.CallStack.PushBack(f)
}

// initNull 初始化为空，表示函数没有体或是递归的
func initNull(f *ssa.Function, c *TaintConfig) {
	// 函数没有体或是递归的
	// 因此通过空 passThrough 初始化
	names := make([]string, 0)
	for _, param := range f.Params {
		names = append(names, param.Name())
	}
	recv := f.Signature.Recv() != nil
	result := f.Signature.Results().Len()
	param := f.Signature.Params().Len()
	passThrough := NewPassThrough(names, recv, result, param)
	passThroughCache := passThrough.ToCache()
	(*c.PassThroughContainer)[f.String()] = passThroughCache
	fmt.Println("end analysis for:", f.String(), ", result: ", passThroughCache)
}

// needNull 判断函数是否需要初始化为空
func needNull(f *ssa.Function, c *TaintConfig) bool {
	// 函数是否没有体？
	if f.Blocks == nil {
		return true
	}

	// 函数是否已被标记为访问过？
	if _, ok := (*c.History)[f.String()]; ok {
		// 检查调用者和被调用者的可见性及包关系
		caller := c.CallStack.Back().Value.(*ssa.Function)
		IsCallerExported := false
		IsCalleeExported := true
		IsSamePackage := false
		if caller.Object() != nil && caller.Object().Exported() {
			IsCallerExported = true
		}
		if f.Object() != nil && !f.Object().Exported() {
			IsCalleeExported = false
		}
		if caller.Pkg != nil && f.Pkg != nil && caller.Pkg.String() == f.Pkg.String() {
			IsSamePackage = true
		}
		// 如果调用者导出、被调用者不导出且在同一个包中，则返回 false
		if IsCallerExported && !IsCalleeExported && IsSamePackage {
			return false
		}
		return true
	}
	return false
}

// New 创建一个 TaintAnalysis
func New(g *graph.UnitGraph, c *TaintConfig) *TaintAnalysis {
	taintAnalysis := new(TaintAnalysis)
	taintAnalysis.BaseFlowAnalysis = *scalar.NewBase(g)
	taintSwitcher := new(TaintSwitcher)
	taintSwitcher.taintAnalysis = taintAnalysis
	taintAnalysis.taintSwitcher = taintSwitcher
	taintAnalysis.config = c

	f := taintAnalysis.Graph.Func
	names := make([]string, 0)
	for _, v := range f.Params {
		names = append(names, v.Name())
	}

	recv := f.Signature.Recv() != nil
	result := f.Signature.Results().Len()
	param := f.Signature.Params().Len()

	taintAnalysis.passThrough = NewPassThrough(names, recv, result, param)
	return taintAnalysis
}

// NewInitalFlow 返回一个新的流
func (a *TaintAnalysis) NewInitalFlow() *map[any]any {
	m := make(map[any]any)

	for _, v := range a.Graph.Func.Params {
		// 初始化参数的污点到流中
		SetTaint(&m, v.Name(), v.Name())
	}
	return &m
}

// Computations 限制流图上的计算次数
func (a *TaintAnalysis) Computations() int {
	return 3000
}

// FlowThrougth 基于 inMap 和 unit 计算 outMap
func (a *TaintAnalysis) FlowThrougth(inMap *map[any]any, unit ssa.Instruction, outMap *map[any]any) {
	a.Copy(inMap, outMap)
	a.apply(inMap, unit, outMap)
}

// apply 调用 switcher.Apply
func (a *TaintAnalysis) apply(inMap *map[any]any, inst ssa.Instruction, outMap *map[any]any) {
	a.taintSwitcher.inMap = inMap
	a.taintSwitcher.outMap = outMap
	switcher.Apply(a.taintSwitcher, inst)
}

// MergeInto 基于 unit 合并 in 到 inout
func (a *TaintAnalysis) MergeInto(unit ssa.Instruction, inout *map[any]any, in *map[any]any) {
	for name, wrapper := range *in {
		if _, ok := (*inout)[name]; ok {
			// 如果 inout 和 in 有相同的键，先合并值
			MergeTaintWrapper(inout, in, name.(string))
		} else {
			// 否则直接从 in 复制键和值到 out
			SetTaintWrapper(inout, name.(string), wrapper.(*TaintWrapper))
		}
	}
}

// End 处理分析结果
func (a *TaintAnalysis) End(universe []*entry.Entry) {
	f := a.Graph.Func
	c := a.config

	if f.Signature.Recv() != nil && false {
		// 如果是值接收器，则重置接收器的污点
		switch a.Graph.Func.Signature.Recv().Type().(type) {
		case *types.Named:
			recv := NewTaintWrapper(a.Graph.Func.Params[0].Name())
			a.passThrough.Recv = recv
		}
	}

	// 保存 passThrough 到 passThroughContainer
	passThroughCache := a.passThrough.ToCache()
	(*c.PassThroughContainer)[f.String()] = passThroughCache

	// 弹出调用栈
	c.CallStack.Remove(c.CallStack.Back())

	fmt.Println("finish analysis for: "+f.String()+", result: ", passThroughCache)
}
