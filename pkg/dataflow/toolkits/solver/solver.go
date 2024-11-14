package solver

import (
	"log"
	"math"
	"reflect"

	"github.com/dnote/color"
	"github.com/zeroy0410/goot/pkg/dataflow/toolkits/graph"
	"github.com/zeroy0410/goot/pkg/dataflow/toolkits/scalar"
	"github.com/zeroy0410/goot/pkg/dataflow/util"
	"github.com/zeroy0410/goot/pkg/dataflow/util/deque"
	"github.com/zeroy0410/goot/pkg/dataflow/util/entry"
	"github.com/zeroy0410/goot/pkg/dataflow/util/queue"
	"golang.org/x/tools/go/ssa"
)

// Solver 表示一个流分析求解器
// 用于执行数据流分析，分析程序中数据的传播路径
type Solver struct {
	Analysis scalar.FlowAnalysis // 数据流分析的具体实现
	Debug    bool                // 是否输出调试信息
}

// Solve 构造一个 Solver 并调用 Solver.DoAnalysis
// a: 数据流分析的实例，debug: 是否启用调试模式
func Solve(a scalar.FlowAnalysis, debug bool) {
	s := new(Solver)
	s.Analysis = a  // 设置分析实例
	s.Debug = debug // 设置调试标志
	s.DoAnalysis()  // 执行分析
}

// DoAnalysis 执行数据流分析
// 返回值为执行的计算次数
func (s *Solver) DoAnalysis() int {
	a := s.Analysis
	// 创建分析用的宇宙结构，包含图的所有节点
	universe := s.newUniverse(a.GetGraph(), a.EntryInitalFlow(), a.IsForward())

	// 用于存储每个节点的输入和输出流
	inFlow := make(map[any]any)
	outFlow := make(map[any]any)

	// 初始化流的状态
	s.initFlow(universe, &inFlow, &outFlow)

	// 创建处理队列，将所有节点加入队列中
	q := queue.Of(&universe)

	// numComputations 记录计算的次数
	for numComputations := 0; ; numComputations++ {
		e := q.Poll() // 获取队列中的下一个节点
		if e == nil { // 如果队列为空，分析结束
			a.End(universe)        // 结束分析
			return numComputations // 返回总计算次数
		}

		// 计算当前节点的输入流
		s.meetFlows(e)

		// 通过流函数更新流状态，判断是否有变化
		hasChanged := s.flowThrougth(e)

		// 如果流状态发生变化，将后继节点加入队列
		if hasChanged {
			for _, o := range e.Out {
				q.Add(o)
			}
		}

		// 检查是否超过最大计算次数
		if numComputations > a.Computations() {
			if s.Debug {
				color.Set(color.FgYellow)
				log.Println("has computed", a.GetGraph().Func.String(), "more than max computations, skip")
				color.Unset()
			}
			a.End(universe)
			return numComputations
		}
	}
}

// 检查两个映射是否相等
// src: 源映射，dst: 目标映射
func equal(src map[any]any, dst map[any]any) bool {
	if len(src) != len(dst) {
		return false
	}
	for k, v := range src {
		u, ok := dst[k]
		if !ok || !reflect.DeepEqual(v, u) {
			return false
		}
	}
	return true
}

// 更新一个入口的输出流，返回值指示输出流是否发生变化
// d: 当前处理的入口
func (s *Solver) flowThrougth(d *entry.Entry) bool {
	if d.InFlow == d.OutFlow {
		return true
	}
	if d.IsRealStronglyConnected {
		// 如果这个节点是强连通分量的一部分，创建新的输出流
		out := s.Analysis.NewInitalFlow()
		s.Analysis.FlowThrougth(d.InFlow, d.Data, out)
		// 如果新旧输出流相等，返回 false
		if equal(*out, *d.OutFlow) {
			return false
		}
		s.Analysis.Copy(out, d.OutFlow) // 复制新输出流到 d.OutFlow
		return true
	}
	// 对非强连通分量的节点，直接更新输出流
	s.Analysis.FlowThrougth(d.InFlow, d.Data, d.OutFlow)
	return true
}

// 合并多个输入流到当前入口的输入流
// e: 当前处理的入口
func (s *Solver) meetFlows(e *entry.Entry) {
	if len(e.In) > 1 {
		copy := true
		for _, o := range e.In {
			if copy {
				copy = false
				s.Analysis.Copy(o.OutFlow, e.InFlow) // 初始化输入流为第一个输入的输出流
			} else {
				s.Analysis.MergeInto(e.Data, e.InFlow, o.OutFlow) // 合并其他输入的输出流
			}
		}
	}
}

// 初始化每个入口的输入流和输出流
// universe: 所有入口的集合，in: 输入流映射，out: 输出流映射
func (s *Solver) initFlow(universe []*entry.Entry, in *map[any]any, out *map[any]any) {
	for _, n := range universe {
		if len(n.In) > 1 {
			n.InFlow = s.Analysis.NewInitalFlow() // 创建新的输入流
		} else if len(n.In) == 1 {
			n.InFlow = n.In[0].OutFlow // 如果只有一个输入，直接使用该输入的输出流
		}
		n.OutFlow = s.Analysis.NewInitalFlow() // 初始化输出流
		(*in)[n.Data] = n.InFlow
		(*out)[n.Data] = n.OutFlow
	}
}

// 构建图的宇宙表示，返回入口的集合
// g: 单元图，entryFlow: 初始流，isForward: 是否为前向分析
func (s *Solver) newUniverse(g *graph.UnitGraph, entryFlow *map[any]any, isForward bool) []*entry.Entry {
	n := g.Size()                                     // 图的大小
	universe := make([]*entry.Entry, 0)               // 创建入口集合
	q := deque.New()                                  // 用于处理强连通分量的双端队列
	visited := make(map[ssa.Instruction]*entry.Entry) // 记录访问过的入口
	superEntry := entry.New(nil, nil)                 // 创建一个超级入口
	var entries []ssa.Instruction                     // 实际的入口指令集合
	var actualEntries []ssa.Instruction

	// 根据分析方向选择入口
	if isForward {
		actualEntries = g.Heads
	} else {
		actualEntries = g.Tails
	}

	// 如果有实际入口，直接使用它们
	if len(actualEntries) != 0 {
		entries = actualEntries
	} else {
		// 否则，根据分析方向处理没有入口的情况
		if isForward {
			if s.Debug {
				color.Set(color.FgYellow)
				log.Println("error: no entry point for method in forward analysis")
				color.Unset()
			}
		} else {
			// 在后向分析中，构建入口列表
			entries = make([]ssa.Instruction, 0)
			head := g.Heads[0]
			visitedNodes := make(map[any]any)
			worklist := make([]ssa.Instruction, 0)
			worklist = append(worklist, head)
			var current ssa.Instruction
			for len(worklist) != 0 {
				current = worklist[0]
				worklist = worklist[1:]
				visitedNodes[current] = true
				switch node := current.(type) {
				case *ssa.Jump:
					entries = append(entries, node)
				}
				for _, next := range g.GetSuccs(current) {
					if util.Collision(&visitedNodes, next) {
						continue
					}
					worklist = append(worklist, next)
				}
			}
			// 如果没有找到入口，抛出错误
			if len(entries) == 0 {
				log.Fatal("error: backward analysis on an empty entry set.")
			}
		}
	}

	// 初始化超级入口与实际入口的连接
	visitEntry(visited, superEntry, entries)
	superEntry.InFlow = entryFlow
	superEntry.OutFlow = entryFlow

	// 用于跟踪节点的访问顺序
	sv := make([]*entry.Entry, n)
	si := make([]int, n)
	index := 0
	i := 0
	v := superEntry

	// 深度优先搜索处理图中的节点
	for {
		if i < len(v.Out) {
			w := v.Out[i]
			i++
			if w.Number == math.MinInt {
				w.Number = q.Len()
				q.AddLast(w)
				if isForward {
					visitEntry(visited, w, g.GetSuccs(w.Data))
				} else {
					visitEntry(visited, w, g.GetPreds(w.Data))
				}
				si[index] = i
				sv[index] = v
				index++
				i = 0
				v = w
			}
		} else {
			if index == 0 {
				for i, j := 0, len(universe)-1; i < j; i, j = i+1, j-1 {
					universe[i], universe[j] = universe[j], universe[i]
				}
				return universe
			}
			universe = append(universe, v)
			sccPop(q, v) // 处理强连通分量
			index--
			v = sv[index]
			i = si[index]
		}
	}
}

// 访问入口并建立与后继节点的连接
// visited: 已访问的节点集合，v: 当前入口，out: 当前入口的后继节点集合
func visitEntry(visited map[ssa.Instruction]*entry.Entry, v *entry.Entry, out []ssa.Instruction) []*entry.Entry {
	n := len(out)
	a := make([]*entry.Entry, n)
	for i := 0; i < n; i++ {
		a[i] = getEntryOf(visited, out[i], v) // 获取或创建后继节点的入口
	}
	v.Out = a
	return a
}

// 获取或创建一个入口节点
// visited: 已访问的节点集合，d: 指令，v: 当前入口
func getEntryOf(visited map[ssa.Instruction]*entry.Entry, d ssa.Instruction, v *entry.Entry) *entry.Entry {
	newEntry := entry.New(d, v) // 创建一个新入口
	var oldEntry *entry.Entry
	if _, ok := visited[d]; ok {
		oldEntry = visited[d] // 如果已访问，使用旧入口
	} else {
		visited[d] = newEntry // 否则将新入口加入已访问集合
		oldEntry = nil
	}
	if oldEntry == nil {
		return newEntry // 返回新创建的入口
	}
	if oldEntry == v {
		oldEntry.IsRealStronglyConnected = true // 标记为强连通分量
	}
	oldEntry.In = append(oldEntry.In, v) // 将当前入口加入旧入口的前驱
	return oldEntry
}

// 处理强连通分量
// s: 双端队列，v: 当前入口
func sccPop(s *deque.Deque, v *entry.Entry) {
	min := v.Number
	for _, e := range v.Out {
		if e.Number < min {
			min = e.Number // 找到最小编号
		}
	}
	if min != v.Number {
		v.Number = min
		return
	}

	// SCC 的出栈处理
	w := s.PollLast()
	w.Number = math.MaxInt
	if w == v {
		return
	}
	w.IsRealStronglyConnected = true
	for {
		w = s.PollLast()
		w.IsRealStronglyConnected = true
		w.Number = math.MaxInt
		if w == v {
			return
		}
	}
}
