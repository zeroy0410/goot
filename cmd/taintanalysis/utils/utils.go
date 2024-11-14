package utils

import (
	"fmt"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
	"sort"
	"strings"
)

// CallGraphStr stringifes `g` into a list of strings where
// each entry is of the form
//
//	f: cs1 -> f1, f2, ...; ...; csw -> fx, fy, ...
//
// f is a function, cs1, ..., csw are call sites in f, and
// f1, f2, ..., fx, fy, ... are the resolved callees.
func CallGraphStr(g *callgraph.Graph) []string {
	var gs []string
	for f, n := range g.Nodes {
		c := make(map[string][]string)
		for _, edge := range n.Out {
			cs := edge.Site.String() // TODO(adonovan): handle Site=nil gracefully
			c[cs] = append(c[cs], funcName(edge.Callee.Func))
		}

		var cs []string
		for site, fs := range c {
			sort.Strings(fs)
			entry := fmt.Sprintf("%v -> %v", site, strings.Join(fs, ", "))
			cs = append(cs, entry)
		}

		sort.Strings(cs)
		entry := fmt.Sprintf("%v: %v", funcName(f), strings.Join(cs, "; "))
		gs = append(gs, removeModulePrefix(entry))
	}
	return gs
}

// funcName returns a name of the function `f`
// prefixed with the name of the receiver type.
func funcName(f *ssa.Function) string {
	recv := f.Signature.Recv()
	if recv == nil {
		return f.Name()
	}
	tp := recv.Type().String()
	return tp[strings.LastIndex(tp, ".")+1:] + "." + f.Name()
}

func removeModulePrefix(s string) string {
	return strings.ReplaceAll(s, "x.io/", "")
}
