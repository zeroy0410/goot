package typeassertion

import (
	"fmt"
	"os"
	"sort"

	"github.com/cokeBeer/goot/pkg/dataflow/golang/switcher"
	"github.com/cokeBeer/goot/pkg/dataflow/toolkits/graph"
	"github.com/cokeBeer/goot/pkg/dataflow/toolkits/scalar"
	"github.com/cokeBeer/goot/pkg/dataflow/util/entry"
	"github.com/dnote/color"

	// "github.com/cokeBeer/goot/pkg/example/dataflow/typeassertion/utils"
	"golang.org/x/tools/go/ssa"
)

// Analysis is the type assertion analysis
type TypeAssertionAnalysis struct {
	scalar.BaseFlowAnalysis
	typeAssertionSwitcher *TypeAssertionSwitcher
}

func New(g *graph.UnitGraph) *TypeAssertionAnalysis {
	typeAssertionAnalysis := new(TypeAssertionAnalysis)
	typeAssertionAnalysis.BaseFlowAnalysis = *scalar.NewBase(g)
	typeAssertionSwitcher := new(TypeAssertionSwitcher)
	typeAssertionSwitcher.BaseSwitcher = *new(switcher.BaseSwitcher)
	typeAssertionAnalysis.typeAssertionSwitcher = typeAssertionSwitcher
	typeAssertionSwitcher.typeAssertionAnalysis = typeAssertionAnalysis
	typeAssertionAnalysis.Graph.Func.WriteTo(os.Stdout)
	return typeAssertionAnalysis
}

func (a *TypeAssertionAnalysis) NewInitalFlow() *map[any]any {
	m := make(map[any]any)
	for _, v := range a.Graph.Func.Params {
		m[v.Name()] = v.Type().String()
	}
	return &m
}

func (a *TypeAssertionAnalysis) FlowThrougth(inMap *map[any]any, unit ssa.Instruction, outMap *map[any]any) {
	a.Copy(inMap, outMap)
	a.apply(inMap, unit, outMap)
}

func (a *TypeAssertionAnalysis) End(universe []*entry.Entry) {
	for _, v := range universe {
		color.Set(color.FgGreen)
		fmt.Println("type assertion analysis result:  " + (*v).Data.String())
		color.Unset()
		keys := make([]string, len(*v.OutFlow))
		i := 0
		for k := range *v.OutFlow {
			keys[i] = k.(string)
			i++
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%v=%v ", k, (*v.OutFlow)[k])
		}
		fmt.Println()
		fmt.Println()
	}
}

func (a *TypeAssertionAnalysis) apply(inMap *map[any]any, inst ssa.Instruction, outMap *map[any]any) {
	a.typeAssertionSwitcher.inMap = inMap
	a.typeAssertionSwitcher.outMap = outMap
	switcher.Apply(a.typeAssertionSwitcher, inst)
}
