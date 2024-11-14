package graph

import "golang.org/x/tools/go/ssa"

// UnitGraph represents a graph based on ssa unit
type UnitGraph struct {
	Func        *ssa.Function
	UnitChain   []ssa.Instruction
	UnitToSuccs map[ssa.Instruction][]ssa.Instruction
	UnitToPreds map[ssa.Instruction][]ssa.Instruction
	Heads       []ssa.Instruction
	Tails       []ssa.Instruction
}

// New creates a UnitGraph
func New(f *ssa.Function) *UnitGraph {
	// 创建一个新的 UnitGraph 实例
	unitGraph := new(UnitGraph)
	unitGraph.Func = f                               // 将传入的函数保存到 UnitGraph 中
	unitGraph.UnitChain = make([]ssa.Instruction, 0) // 初始化指令链，用于存储所有的指令
	unitGraph.Heads = make([]ssa.Instruction, 0)     // 初始化头指令链，用于存储基本块的头指令

	// 如果函数有基本块
	if len(f.Blocks) != 0 {
		// 将第一个基本块的第一条指令添加到头指令链中
		unitGraph.Heads = append(unitGraph.Heads, f.Blocks[0].Instrs[0])
	}

	unitGraph.Tails = make([]ssa.Instruction, 0)                        // 初始化尾指令链，用于存储基本块的尾指令
	unitGraph.UnitToSuccs = make(map[ssa.Instruction][]ssa.Instruction) // 初始化后继指令映射
	unitGraph.UnitToPreds = make(map[ssa.Instruction][]ssa.Instruction) // 初始化前驱指令映射

	// 遍历函数的每个基本块
	for _, b := range f.Blocks {
		// 如果基本块没有指令，跳过
		if len(b.Instrs) == 0 {
			continue
		}

		// 遍历基本块中的指令（除了最后一条）
		for i := 0; i < len(b.Instrs)-1; i++ {
			// 将当前指令添加到指令链中
			unitGraph.UnitChain = append(unitGraph.UnitChain, b.Instrs[i])
			// 设置当前指令的后继为下一条指令
			unitGraph.UnitToSuccs[b.Instrs[i]] = append(unitGraph.UnitToSuccs[b.Instrs[i]], b.Instrs[i+1])
			// 设置下一条指令的前驱为当前指令
			unitGraph.UnitToPreds[b.Instrs[i+1]] = append(unitGraph.UnitToPreds[b.Instrs[i+1]], b.Instrs[i])
		}

		// 将基本块的最后一条指令添加到指令链中
		unitGraph.UnitChain = append(unitGraph.UnitChain, b.Instrs[len(b.Instrs)-1])

		// 如果基本块没有后继，说明最后一条指令是末尾指令
		if len(b.Succs) == 0 {
			// 将最后一条指令添加到尾指令链中
			unitGraph.Tails = append(unitGraph.Tails, b.Instrs[len(b.Instrs)-1])
			continue
		}

		// 遍历基本块的后继
		for _, s := range b.Succs {
			t := s
			// 找到后继基本块的第一条有效指令
			for len(t.Instrs) == 0 {
				t = t.Succs[0]
			}
			// 设置当前基本块最后一条指令的后继为后继基本块的第一条指令
			unitGraph.UnitToSuccs[b.Instrs[len(b.Instrs)-1]] = append(unitGraph.UnitToSuccs[b.Instrs[len(b.Instrs)-1]], t.Instrs[0])
			// 设置后继基本块第一条指令的前驱为当前基本块的最后一条指令
			unitGraph.UnitToPreds[t.Instrs[0]] = append(unitGraph.UnitToPreds[t.Instrs[0]], b.Instrs[len(b.Instrs)-1])
		}
	}

	// 返回构建好的 UnitGraph
	return unitGraph
}

// Size returns length of the UnitChain
func (g *UnitGraph) Size() int {
	return len(g.UnitChain)
}

// GetSuccs returns Succs of an instruction
func (g *UnitGraph) GetSuccs(inst ssa.Instruction) []ssa.Instruction {
	return g.UnitToSuccs[inst]
}

// GetPreds returns Preds of an instruction
func (g *UnitGraph) GetPreds(inst ssa.Instruction) []ssa.Instruction {
	return g.UnitToPreds[inst]
}
