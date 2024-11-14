package taint

import (
	"container/list"
	"github.com/zeroy0410/goot/pkg/example/dataflow/taint/rule"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
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
	cfg := &packages.Config{Mode: mode}
	initial, err := packages.Load(cfg, r.PkgPath...)

	if err != nil {
		return err
	}

	prog, _ := ssautil.AllPackages(initial, 0)

	prog.Build()

	funcs := ssautil.AllFunctions(prog)

	interfaceHierarchy := NewInterfaceHierarchy(&funcs)

	var callGraph *callgraph.Graph
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

		result := cha.CallGraph(prog)

		callGraph = result
		callGraph.DeleteSyntheticNodes()
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
		CallGraph:          callGraph,
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
