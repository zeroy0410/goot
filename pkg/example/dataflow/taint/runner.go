package taint

import (
	"container/list"
	"fmt"
	"github.com/zeroy0410/goot/pkg/example/dataflow/taint/rule"
	"go/types"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"strings"
)

// Runner represents a analysis runner
type Runner struct {
	ModuleName         string
	PkgPath            []string
	UsePointerAnalysis bool
	Debug              bool
	InitOnly           bool
	PassThroughOnly    bool
	PassThroughSrcPath []string
	PassThroughDstPath string
	TaintGraphDstPath  string
	Ruler              rule.Ruler
	PersistToNeo4j     bool
	Neo4jUsername      string
	Neo4jPassword      string
	Neo4jURI           string
	TargetFunc         string
	PassBack           bool
}

func getTypes(t types.Type) (types.Type, string) {
	switch tt := t.(type) {
	case *types.Pointer:
		_, kind := getTypes(tt.Elem())
		return tt, "pointer to " + kind
	case *types.Named:
		return tt.Underlying(), tt.Underlying().String()
	case *types.Interface:
		return tt, "interface"
	case *types.Struct:
		return tt, "struct"
	default:
		return tt, fmt.Sprintf("%T", tt)
	}
}

func extractInterfaceFromPointer(t types.Type) (*types.Interface, bool) {
	ptrType, ok := t.(*types.Pointer)
	if !ok {
		return nil, false
	}

	elemType := ptrType.Elem()
	for {
		switch tt := elemType.(type) {
		case *types.Named:
			elemType = tt.Underlying()
		case *types.Interface:
			return tt, true
		default:
			return nil, false
		}
	}
}

func PrintAssertionsInfo(resultTypes map[*ssa.TypeAssert][]types.Type) {
	for node, possibleTypes := range resultTypes {
		parentFunc := node.X.Parent()
		if pkg := parentFunc.Package(); pkg != nil {
			fmt.Println("Package: ", pkg)
		}
		fmt.Println("Function: ", parentFunc.Name())
		fmt.Println("Node: ", node.X)
		fmt.Print("Assertion: ", node.AssertedType)

		assertedType, assertedTypeStr := getTypes(node.AssertedType)
		fmt.Print("    ", assertedTypeStr)
		fmt.Println()
		fmt.Println("Possible Types: ")

		for _, typ := range possibleTypes {
			fmt.Print("    ", typ)
			actualType, typeStr := getTypes(typ)
			fmt.Print("    ", typeStr)

			if strings.Contains(assertedTypeStr, "pointer to struct") && strings.Contains(typeStr, "pointer to interface") {
				interfaceType, ok := extractInterfaceFromPointer(actualType)
				if !ok {
					fmt.Print("    Unable to extract interface type")
					continue
				}

				interfaceType.Complete()
				implements := types.Implements(assertedType, interfaceType)
				fmt.Print("    Implements: ", implements)
			}
			fmt.Println()
		}
		fmt.Println("-----------------------")
	}
}

// NewRunner returns a *taint.Runner
func NewRunner(PkgPath ...string) *Runner {
	return &Runner{PkgPath: PkgPath, ModuleName: "",
		PassThroughSrcPath: nil, PassThroughDstPath: "",
		TaintGraphDstPath: "", Ruler: nil,
		Debug: false, InitOnly: false, PassThroughOnly: false,
		PersistToNeo4j: false, Neo4jURI: "", Neo4jUsername: "", Neo4jPassword: "",
		TargetFunc: "", PassBack: false,
		UsePointerAnalysis: false}
}

// Run kick off an analysis
func (r *Runner) Run() error {
	mode := packages.NeedName |
		packages.NeedFiles |
		packages.NeedCompiledGoFiles |
		packages.NeedSyntax |
		packages.NeedTypesInfo |
		packages.NeedImports |
		packages.NeedTypesSizes |
		packages.NeedTypes |
		packages.NeedDeps
	cfg := &packages.Config{Mode: mode, Dir: "C:/Users/zeroy/Documents/Code/goot/cmd/taintanalysis/nilaway/"}
	initial, err := packages.Load(cfg, r.PkgPath...)

	if err != nil {
		return err
	}

	prog, _ := ssautil.AllPackages(initial, 0)

	prog.Build()

	funcs := ssautil.AllFunctions(prog)

	interfaceHierarchy := NewInterfaceHierarchy(&funcs)

	var cg *callgraph.Graph
	if r.UsePointerAnalysis {
		mainFuncs := make([]*ssa.Function, 0)
		for _, pkg := range initial {
			mainPkg := prog.Package(pkg.Types)
			if mainPkg != nil && mainPkg.Pkg.Name() == "main" && mainPkg.Func("main") != nil {
				mainFuncs = append(mainFuncs, mainPkg.Func("main"))
			}
		}
		if len(mainFuncs) == 0 {
			return new(NoMainPkgError)
		}

		result := vta.CallGraph(ssautil.AllFunctions(prog), nil)
		resultTypes := vta.GetTypeAsserts(ssautil.AllFunctions(prog), nil)
		PrintAssertionsInfo(resultTypes)

		cg = result
		cg.DeleteSyntheticNodes()
	}

	var ruler rule.Ruler
	if r.Ruler != nil {
		ruler = r.Ruler
	} else {
		ruler = NewDummyRuler(r.ModuleName)
	}
	taintGraph := NewTaintGraph(&funcs, ruler)

	passThroughContainter := make(map[string]*PassThroughCache)
	if r.PassThroughSrcPath != nil {
		err := FetchPassThrough(&passThroughContainter, r.PassThroughSrcPath)
		if err != nil {
			return err
		}
	}

	initMap := make(map[string]*ssa.Function)
	history := make(map[string]bool)

	c := &TaintConfig{PassThroughContainer: &passThroughContainter,
		InitMap:            &initMap,
		History:            &history,
		CallStack:          list.New().Init(),
		InterfaceHierarchy: interfaceHierarchy,
		TaintGraph:         taintGraph,
		UsePointerAnalysis: r.UsePointerAnalysis,
		CallGraph:          cg,
		Ruler:              ruler,
		PassThroughOnly:    r.PassThroughOnly,
		Debug:              r.Debug,
		TargetFunc:         r.TargetFunc,
		PassBack:           r.PassBack}

	for f := range funcs {
		if f.Name() == "init" {
			Run(f, c)
		}
	}

	if !r.InitOnly {
		for f := range funcs {
			if f.String() != "init" {
				if r.TargetFunc != "" && f.String() != r.TargetFunc {
					continue
				}
				Run(f, c)
			}
		}
	}

	if r.PassThroughDstPath != "" {
		PersistPassThrough(&passThroughContainter, r.PassThroughDstPath)
	}
	if r.TaintGraphDstPath != "" {
		PersistTaintGraph(taintGraph.Edges, r.TaintGraphDstPath)
	}
	if !r.PassThroughOnly && r.PersistToNeo4j {
		PersistToNeo4j(taintGraph.Nodes, taintGraph.Edges, r.Neo4jURI, r.Neo4jUsername, r.Neo4jPassword)
	}
	return nil
}
