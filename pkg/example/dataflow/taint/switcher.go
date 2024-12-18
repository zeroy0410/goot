package taint

import (
	"go/token"
	"go/types"
	"strconv"

	"github.com/zeroy0410/goot/pkg/dataflow/golang/switcher"
	"golang.org/x/tools/go/ssa"
)

// TaintSwitcher represents a switcher for taint analysis
type TaintSwitcher struct {
	switcher.BaseSwitcher
	taintAnalysis *TaintAnalysis
	inMap         *map[any]any
	outMap        *map[any]any
}

// CaseAlloc accepts a Alloc instruction
func (s *TaintSwitcher) CaseAlloc(inst *ssa.Alloc) {
	GetTaintWrapper(s.outMap, inst.Name())
}

// CaseBinOp accepts a BinOp instruction
func (s *TaintSwitcher) CaseBinOp(inst *ssa.BinOp) {
	// update new taint by both inst.X and inst.Y
	// inst.X and inst.Y both have type ssa.Value, so they have Name()
	PassTaint(s.outMap, inst.Name(), inst.X.Name(), inst.Y.Name())
}

// CaseCall accepts a Call instruction
func (s *TaintSwitcher) CaseCall(inst *ssa.Call) {
	c := s.taintAnalysis.config
	container := c.PassThroughContainer
	init := s.taintAnalysis.config.InitMap
	// try to use pointer analysis to select callee
	callGraph := s.taintAnalysis.config.CallGraph
	if c.UsePointerAnalysis && inst.Common().StaticCallee() == nil {
		node := callGraph.Nodes[inst.Parent()]
		if node != nil {
			for _, edge := range node.Out {
				if edge.Site == inst {
					if inst.Call.Method != nil {
						// invoke
						s.passMethodTaint(edge.Callee.Func, inst)
					} else {
						f := edge.Callee.Func
						if f.Signature.Recv() != nil {
							// anonymous function points to a interface method closure
							s.passMethodTaint(f, inst)
						} else {
							// anonymous function
							s.passStaticCallTaint(edge.Callee.Func, inst)
						}
					}
					return
				}
			}
		}
	}
	// try to use CHA to select callee
	switch v := (inst.Call.Value).(type) {
	case *ssa.Field:
		// caller can be a field from a struct
		// we consider it as an interface
		if inst.Call.Method == nil {
			// if it is a function, its signature information is in inst.Call.Value
			m := v.Type().Underlying().(*types.Signature)
			s.passFuncParamTaint(m, inst)
		} else {
			// we consider is as a interface
			m := inst.Call.Method
			s.passInvokeTaint(m, inst)
		}
	case *ssa.FreeVar:
		// caller can be a free var from closure
		// we consider it as an interface
		// e.g. bound$Write
		if inst.Call.Method == nil {
			// if it is a function, its signature information is in inst.Call.Value
			m := v.Type().Underlying().(*types.Signature)
			s.passFuncParamTaint(m, inst)
		} else {
			// we consider is as a interface
			m := inst.Call.Method
			s.passInvokeTaint(m, inst)
		}
	case *ssa.Lookup:
		// caller can be a value from map
		if inst.Call.Method == nil {
			// if it is a function, its signature information is in inst.Call.Value
			typ := v.X.Type().Underlying().(*types.Map).Elem()
			if p, ok := typ.Underlying().(*types.Pointer); ok {
				// anonymous function pointer
				m := p.Elem().Underlying().(*types.Signature)
				s.passFuncParamTaint(m, inst)
			} else {
				// anonymous function
				m := typ.Underlying().(*types.Signature)
				s.passFuncParamTaint(m, inst)
			}
		} else {
			// if it is an interface
			m := inst.Call.Method
			s.passInvokeTaint(m, inst)
		}
	case *ssa.MakeInterface:
		// caller can be a MakeInterface instruction
		// we consider it as an interface
		if inst.Call.Method == nil {
			// if it is a function, its signature information is in inst.Call.Value
			m := v.Type().Underlying().(*types.Signature)
			s.passFuncParamTaint(m, inst)
		} else {
			// we consider is as a interface
			m := inst.Call.Method
			s.passInvokeTaint(m, inst)
		}
	case *ssa.TypeAssert:
		// caller can be a TypeAssert instruction
		if inst.Call.Method == nil {
			// if it is a function, its signature information is in inst.Call.Value
			m := v.Type().Underlying().(*types.Signature)
			s.passFuncParamTaint(m, inst)
		} else {
			// we consider is as a interface
			m := inst.Call.Method
			s.passInvokeTaint(m, inst)
		}
	case *ssa.UnOp:
		// caller can be a UnOp instruction
		switch x := (v.X).(type) {
		case *ssa.UnOp:
			// its inst.X can be another UnOp instruction
			switch (x.X).(type) {
			case *ssa.IndexAddr:
				// this case is special
				// when use range over an interface pointer slice, it will hanppend
				// e.g. golang.org/x/tools/go/ssa/sanity.go checkBlock
				if inst.Call.Method != nil {
					// we consider is as a interface
					m := inst.Call.Method
					s.passInvokeTaint(m, inst)
				}
			default:
				if inst.Call.Method == nil {
					// if it is a function, its signature information is in inst.Call.Value
					m := v.Type().Underlying().(*types.Signature)
					s.passFuncParamTaint(m, inst)
				} else {
					// we consider is as a interface
					m := inst.Call.Method
					s.passInvokeTaint(m, inst)
				}
			}
		case *ssa.FreeVar:
			// its inst.X can be a free var
			if inst.Call.Method == nil {
				// if it is a function, its signature information is in inst.Call.Value
				typ := x.Type()
				if p, ok := typ.Underlying().(*types.Pointer); ok {
					// anonymous function pointer
					m := p.Elem().Underlying().(*types.Signature)
					s.passFuncParamTaint(m, inst)
				} else {
					// anonymous function
					m := typ.Underlying().(*types.Signature)
					s.passFuncParamTaint(m, inst)
				}
			} else {
				// if it is an interface
				m := inst.Call.Method
				s.passInvokeTaint(m, inst)
			}
		case *ssa.Global:
			// its inst.X can be a global anonymous function or a global anonymous interface
			f, ok := (*init)[x.String()]
			if ok {
				// anonymous function that has been declared in source
				s.passCallTaint(f, inst)
			} else if inst.Call.Method != nil {
				// a global anonymous interface created by function return
				// e.g. go/types/universe.go universeAny = Universe.Lookup("any")
				m := inst.Call.Method
				s.passInvokeTaint(m, inst)
			} else {
				// anonymous function in assembly code
				// or some global anonymous functios failed to be recorded
				// e.g. golang.org/x/tools/internal/imports/fix.go fixImports
				m := x.Type().(*types.Pointer).Elem().Underlying().(*types.Signature)
				s.passFuncParamTaint(m, inst)
			}
		case *ssa.Alloc:
			// its inst.X can be a local anonymous function or a local anonymous interface
			if inst.Call.Method == nil {
				// if it is a function, its signature information is in inst.Call.Value
				// we try to find its *ssa.Function in referrers first
				// e.g. runtime/mpagealloc_64bit.go sysGrow
				ref := false
				for _, v := range *x.Referrers() {
					if store, ok := v.(*ssa.Store); ok {
						if f, ok := store.Val.(*ssa.Function); ok {
							// if a function stored to inst.X
							ref = ok
							_, ok = (*container)[f.String()]
							if !ok {
								Run(f, c)
							}
							s.passCallTaint(f, inst)
						} else if closure, ok := store.Val.(*ssa.MakeClosure); ok {
							if f, ok := closure.Fn.(*ssa.Function); ok {
								// if a closure stored to inst.X, retrive its Fn
								ref = ok
								_, ok = (*container)[f.String()]
								if !ok {
									Run(f, c)
								}
								s.passCallTaint(f, inst)
							}
						}
					}
				}
				if !ref {
					// if we can't find a *ssa.Function
					typ := x.Type()
					if p, ok := typ.Underlying().(*types.Pointer); ok {
						m := p.Elem().Underlying().(*types.Signature)
						s.passFuncParamTaint(m, inst)
					}
				}
			} else {
				// interface
				m := inst.Call.Method
				s.passInvokeTaint(m, inst)
			}
		case *ssa.FieldAddr:
			// its inst.X can be a struct field, represents an anonymous function or an anonymous interface
			// the struct can comes from reveiver or parameter
			if inst.Call.Method == nil {
				field := x.X.Type().Underlying().(*types.Pointer).Elem().Underlying().(*types.Struct).Field(x.Field)
				typ := field.Type()
				if p, ok := typ.Underlying().(*types.Pointer); ok {
					// function pointer
					m := p.Elem().Underlying().(*types.Signature)
					s.passFuncParamTaint(m, inst)
				} else {
					// function
					m := typ.Underlying().(*types.Signature)
					s.passFuncParamTaint(m, inst)
				}
			} else {
				// interface
				m := inst.Call.Method
				s.passInvokeTaint(m, inst)
			}
		case *ssa.IndexAddr:
			// its inst.X can be a slice cell, represents an anonymous function or an anonymous interface
			if inst.Call.Method == nil {
				if slice, ok := x.X.Type().Underlying().(*types.Slice); ok {
					// if inst.X.X's underlying type is a slice
					typ := slice.Elem()
					if p, ok := typ.Underlying().(*types.Pointer); ok {
						// function pointer
						m := p.Elem().Underlying().(*types.Signature)
						s.passFuncParamTaint(m, inst)
					} else {
						// function
						m := typ.Underlying().(*types.Signature)
						s.passFuncParamTaint(m, inst)
					}
				}
				if pointer, ok := x.X.Type().Underlying().(*types.Pointer); ok {
					// if inst.X.X's underlying type is a pointer
					if array, ok := pointer.Elem().Underlying().(*types.Array); ok {
						// pointer points to an array
						// e.g. html/template/escape.go contextAfterText transitionFunc
						if p, ok := array.Elem().Underlying().(*types.Pointer); ok {
							// function pointer
							m := p.Elem().Underlying().(*types.Signature)
							s.passFuncParamTaint(m, inst)
						} else {
							// function
							m := array.Elem().Underlying().(*types.Signature)
							s.passFuncParamTaint(m, inst)
						}
					} else {
						// pointer pointers to a anonymous function
						m := pointer.Elem().Underlying().(*types.Signature)
						s.passFuncParamTaint(m, inst)
					}
				}
			} else {
				// interface
				m := inst.Call.Method
				s.passInvokeTaint(m, inst)
			}
		case *ssa.Extract:
			// its inst.X can be an Extract instruction
			// in this case, the function should hava more than one return value
			if inst.Call.Method == nil {
				// if it is a function, its signature information is in inst.Call.Value
				typ := x.Type()
				if p, ok := typ.Underlying().(*types.Pointer); ok {
					// function pointer
					m := p.Elem().Underlying().(*types.Signature)
					s.passFuncParamTaint(m, inst)
				} else {
					// function
					m := typ.Underlying().(*types.Signature)
					s.passFuncParamTaint(m, inst)
				}
			} else {
				// interface
				m := inst.Call.Method
				s.passInvokeTaint(m, inst)
			}
		case *ssa.Call:
			if inst.Call.Method != nil {
				// we consider is as a interface
				m := inst.Call.Method
				s.passInvokeTaint(m, inst)
			}
		default:
			if inst.Call.Method == nil {
				// if it is a function, its signature information is in inst.Call.Value
				m := v.Type().Underlying().(*types.Signature)
				s.passFuncParamTaint(m, inst)
			} else {
				// we consider is as a interface
				m := inst.Call.Method
				s.passInvokeTaint(m, inst)
			}
		}
	case *ssa.Phi:
		// caller can be a Phi instruction
		if inst.Call.Method == nil {
			// if it is a function, its signature information is in inst.Call.Value
			// we choose first edge here
			m := v.Edges[0].Type().Underlying().(*types.Signature)
			s.passFuncParamTaint(m, inst)
		} else {
			// interface
			m := inst.Call.Method
			s.passInvokeTaint(m, inst)
		}
	case *ssa.MakeClosure:
		// caller can be a MakeClosure instruction
		if inst.Call.Method == nil {
			// if it is a function, its signature information is in inst.Call.Value
			m := v.Type().Underlying().(*types.Signature)
			s.passFuncParamTaint(m, inst)
		} else {
			// interface
			m := inst.Call.Method
			s.passInvokeTaint(m, inst)
		}
	case *ssa.Call:
		// caller can be a Call instruction
		if inst.Call.Method == nil {
			// if it is a function, its signature information is in inst.Call.Value
			m := v.Type().Underlying().(*types.Signature)
			s.passFuncParamTaint(m, inst)
		} else {
			// interface
			m := inst.Call.Method
			s.passInvokeTaint(m, inst)
		}
	case *ssa.Extract:
		// caller can be a Extract instruction
		if inst.Call.Method == nil {
			// if it is a function, its signature information is in inst.Call.Value
			m := v.Type().Underlying().(*types.Signature)
			s.passFuncParamTaint(m, inst)
		} else {
			// interface
			m := inst.Call.Method
			s.passInvokeTaint(m, inst)
		}
	case *ssa.Parameter:
		// caller can be a parameter
		if inst.Call.Method == nil {
			// if it is a function, its signature information is in inst.Call.Value
			m := v.Type().Underlying().(*types.Signature)
			s.passFuncParamTaint(m, inst)
		} else {
			// interface
			m := inst.Call.Method
			s.passInvokeTaint(m, inst)
		}
	case *ssa.Builtin:
		// builtins
		b := v
		switch b.Name() {
		case "append":
			s.passAppendTaint(inst)
		case "copy":
			s.passCopyTaint(inst)
		case
			"recover",
			"complex",
			"len",
			"delete",
			"panic",
			"real",
			"imag",
			"close",
			"print",
			"println",
			"make",
			"cap",
			"ssa:wrapnilchk":
			GetTaintWrapper(s.outMap, inst.Name())
		}
	case *ssa.Function:
		// caller can be a known function
		// global function, global method and anonymous function in function itself
		f := v
		s.passCallTaint(f, inst)
	default:
		if inst.Call.Method == nil {
			// if it is a function, its signature information is in inst.Call.Value
			m := v.Type().Underlying().(*types.Signature)
			s.passFuncParamTaint(m, inst)
		} else {
			// we consider is as a interface
			m := inst.Call.Method
			s.passInvokeTaint(m, inst)
		}
	}
}

// CaseChangeInterface accepts a ChangeInterface instruction
func (s *TaintSwitcher) CaseChangeInterface(inst *ssa.ChangeInterface) {
	// we drop *ssa.Global, *ssa.FreeVar and *ssa.Const
	PassTaint(s.outMap, inst.Name(), inst.X.Name())
}

// CaseChangeType accepts a ChangeType instruction
func (s *TaintSwitcher) CaseChangeType(inst *ssa.ChangeType) {
	// we drop *ssa.Global, *ssa.FreeVar and *ssa.Const
	PassTaint(s.outMap, inst.Name(), inst.X.Name())
}

// CaseConvert accepts a Convert instruction
func (s *TaintSwitcher) CaseConvert(inst *ssa.Convert) {
	// skip *ssa.Global, *ssa.FreeVar and *ssa.Const
	PassTaint(s.outMap, inst.Name(), inst.X.Name())
}

// CaseExtract accepts a Extract instruction
func (s *TaintSwitcher) CaseExtract(inst *ssa.Extract) {
	// mark the variables as "inst.Tuple.Name().i"
	// e.g. t1.0, t3.2
	mark := inst.Tuple.Name() + "." + strconv.Itoa(inst.Index)
	PassTaint(s.outMap, inst.Name(), mark)
}

// CaseField accepts a Field instruction
func (s *TaintSwitcher) CaseField(inst *ssa.Field) {
	// we drop *ssa.Global, *ssa.FreeVar and *ssa.Const
	PassTaint(s.outMap, inst.Name(), inst.X.Name())
}

// CaseFieldAddr accepts a FieldAddr instruction
func (s *TaintSwitcher) CaseFieldAddr(inst *ssa.FieldAddr) {
	// we drop *ssa.Global, *ssa.FreeVar and *ssa.Const
	PassTaint(s.outMap, inst.Name(), inst.X.Name())
}

// CaseIndex accepts an Index instruction
func (s *TaintSwitcher) CaseIndex(inst *ssa.Index) {
	// we drop *ssa.Global, *ssa.FreeVar and *ssa.Const
	PassTaint(s.outMap, inst.Name(), inst.X.Name())
}

// CaseIndexAddr accepts an IndexAddr instruction
func (s *TaintSwitcher) CaseIndexAddr(inst *ssa.IndexAddr) {
	// we drop *ssa.Global, *ssa.FreeVar and *ssa.Const
	PassTaint(s.outMap, inst.Name(), inst.X.Name())
}

// CaseLookup accepts a Lookup instruction
func (s *TaintSwitcher) CaseLookup(inst *ssa.Lookup) {
	// pass taint in index and map
	if inst.CommaOk {
		// if needs an ok, mark two variables, and the first one inherits taint
		PassTaint(s.outMap, inst.Name()+".0", inst.Index.Name(), inst.X.Name())
		GetTaintWrapper(s.outMap, inst.Name()+".1")
	} else {
		PassTaint(s.outMap, inst.Name(), inst.Index.Name(), inst.X.Name())
	}
}

// CaseMakeClosure accepts a MakeClosure instruction
func (s *TaintSwitcher) CaseMakeClosure(inst *ssa.MakeClosure) {
	GetTaintWrapper(s.outMap, inst.Name())
}

// CaseMakeChan accepts a MakeChan instruction
func (s *TaintSwitcher) CaseMakeChan(inst *ssa.MakeChan) {
	GetTaintWrapper(s.outMap, inst.Name())
}

// CaseMakeInterface accepts a MakeInterface instruction
func (s *TaintSwitcher) CaseMakeInterface(inst *ssa.MakeInterface) {
	// we drop *ssa.Global, *ssa.FreeVar and *ssa.Const
	PassTaint(s.outMap, inst.Name(), inst.X.Name())
}

// CaseMakeMap accepts a MakeMap instruction
func (s *TaintSwitcher) CaseMakeMap(inst *ssa.MakeMap) {
	GetTaintWrapper(s.outMap, inst.Name())
}

// CaseMakeSlice accepts a MakeSlice instruction
func (s *TaintSwitcher) CaseMakeSlice(inst *ssa.MakeSlice) {
	GetTaintWrapper(s.outMap, inst.Name())
}

// CaseNext accepts a Next instruction
func (s *TaintSwitcher) CaseNext(inst *ssa.Next) {
	// mark three variables, and the second and the third inherits taint
	GetTaintWrapper(s.outMap, inst.Name()+".0")
	PassTaint(s.outMap, inst.Name()+".1", inst.Iter.Name())
	PassTaint(s.outMap, inst.Name()+".2", inst.Iter.Name())
}

// CaseMapUpdate accepts a MapUpdate instruction
func (s *TaintSwitcher) CaseMapUpdate(inst *ssa.MapUpdate) {
	// pass taint in key and value
	PassTaint(s.outMap, inst.Map.Name(), inst.Key.Name(), inst.Value.Name())
}

// CasePhi accepts a Phi instruction
func (s *TaintSwitcher) CasePhi(inst *ssa.Phi) {
	// Phi is the gather of instructions
	// It may visit uninitialized register
	for _, e := range inst.Edges {
		PassTaint(s.outMap, inst.Name(), e.Name())
	}
}

// CaseRange accepts a Range instruction
func (s *TaintSwitcher) CaseRange(inst *ssa.Range) {
	PassTaint(s.outMap, inst.Name(), inst.X.Name())
}

// CaseReturn accepts a Return instruction
func (s *TaintSwitcher) CaseReturn(inst *ssa.Return) {
	passThrough := s.taintAnalysis.passThrough
	if passThrough.HasRecv() {
		// if the function has a receiver
		recv := passThrough.RecvName()
		for k := range *GetTaint(s.outMap, recv) {
			// merge receiver's taint into passthrough
			passThrough.Recv.AddTaint(k)
		}
	}
	for i := 0; i < passThrough.ResultNum(); i++ {
		result := inst.Results[i].Name()
		// skip *ssa.Global, *ssa.FreeVar and *ssa.Const
		for k := range *GetTaint(s.outMap, result) {
			// merge other results' taint
			passThrough.Results[i].AddTaint(k)
		}
	}
	for i := 0; i < s.taintAnalysis.passThrough.ParamNum(); i++ {
		arg := passThrough.ParamName(i)
		// skip *ssa.Global, *ssa.FreeVar and *ssa.Const
		for k := range *GetTaint(s.outMap, arg) {
			// merge args' taint
			passThrough.Params[i].AddTaint(k)
		}
	}
}

// CaseSend accepts a Send instruction
func (s *TaintSwitcher) CaseSend(inst *ssa.Send) {
	PassTaint(s.outMap, inst.Chan.Name(), inst.X.Name())
}

// CaseSelect accepts a Select instruction
func (s *TaintSwitcher) CaseSelect(inst *ssa.Select) {
	// mark variables as "inst.Name().i"
	// e.g. t2.0, t2.1
	GetTaintWrapper(s.outMap, inst.Name()+".0")
	GetTaintWrapper(s.outMap, inst.Name()+".1")
	for k := range inst.States {
		GetTaintWrapper(s.outMap, inst.Name()+"."+strconv.Itoa(k+2))
	}
}

// CaseSlice accepts a Slice instruction
func (s *TaintSwitcher) CaseSlice(inst *ssa.Slice) {
	PassTaint(s.outMap, inst.Name(), inst.X.Name())
}

// CaseStore accepts a Store instruction
func (s *TaintSwitcher) CaseStore(inst *ssa.Store) {
	// Store needs to visit pointer
	PassTaint(s.outMap, inst.Addr.Name(), inst.Val.Name())
	if _, ok := (inst.Addr).(*ssa.Global); ok {
		// save global anonymous function to initMap
		if f, ok := (inst.Val).(*ssa.Function); ok {
			(*s.taintAnalysis.config.InitMap)[inst.Addr.String()] = f
		}
	}
	// if inst.Addr points to struct or slice, update further
	s.passPointTaint(inst.Addr)
}

// CaseTypeAssert accepts a TypeAssert instruction
func (s *TaintSwitcher) CaseTypeAssert(inst *ssa.TypeAssert) {
	// we drop *ssa.Global, *ssa.FreeVar and *ssa.Const
	if inst.CommaOk {
		// if needs an ok, mark two variables, and the first one inherits taint
		PassTaint(s.outMap, inst.Name()+".0", inst.X.Name())
		GetTaintWrapper(s.outMap, inst.Name()+".1")
	} else {
		PassTaint(s.outMap, inst.Name(), inst.X.Name())
	}
}

// CaseUnOp accepts a UnOp instruction
func (s *TaintSwitcher) CaseUnOp(inst *ssa.UnOp) {
	if inst.Op == token.ARROW && inst.CommaOk {
		// if needs an ok, mark two variables, and the first one inherits taint
		PassTaint(s.outMap, inst.Name()+".0", inst.X.Name())
		GetTaintWrapper(s.outMap, inst.Name()+".1")
	} else {
		PassTaint(s.outMap, inst.Name(), inst.X.Name())
	}
}

// passCallTaint passes taint by *ssa.Function and a call
func (s *TaintSwitcher) passCallTaint(f *ssa.Function, inst *ssa.Call) {
	if !s.taintAnalysis.config.PassThroughOnly {
		s.collectCallEdges(f, inst)
	}
	s.passStaticCallTaint(f, inst)
}

// passStaticCallTaint passes taint by a known *ssa.Function and a call
func (s *TaintSwitcher) passStaticCallTaint(f *ssa.Function, inst *ssa.Call) {
	container := s.taintAnalysis.config.PassThroughContainer
	c := s.taintAnalysis.config
	_, ok := (*container)[f.String()]
	if !ok {
		if needNull(f, c) {
			// function is loaded from C file and has no body
			if m, ok := f.Object().(*types.Func); ok {
				s.passNullTaint(m, inst)
			} else {
				// function pointer used in recursive
				// e.g. golang.org/x/tools/go/analysis/validate.go Validate$1 visit
				// to inhibit infinite recursive, use passAnonymousTaint instead of passFuncParamTaint
				s.passAnonymousTaint(f.Signature, inst)
			}
			return
		}
		// if we can saved it, load it now
		Run(f, c)
	}

	passThroughCache := (*container)[f.String()]
	var newRecvTaint *TaintWrapper
	newResultTaints := make([]*TaintWrapper, 0)
	newParamTaints := make([]*TaintWrapper, 0)
	if passThroughCache.HasRecv() {
		newTaint := NewTaintWrapper()
		// for every parameter index in passthrough, collect arg's taint
		for _, p := range passThroughCache.Recv {
			newTaint.InheritTaint(s.outMap, inst.Call.Args[p].Name())
		}
		newRecvTaint = newTaint
	}
	for _, result := range passThroughCache.Results {
		newTaint := NewTaintWrapper()
		// for every parameter index in passthrough, collect arg's taint
		for _, p := range result {
			newTaint.InheritTaint(s.outMap, inst.Call.Args[p].Name())
		}
		newResultTaints = append(newResultTaints, newTaint)
	}
	for _, param := range passThroughCache.Params {
		newTaint := NewTaintWrapper()
		// for every parameter index in passthrough, collect arg's taint
		for _, p := range param {
			newTaint.InheritTaint(s.outMap, inst.Call.Args[p].Name())
		}
		newParamTaints = append(newParamTaints, newTaint)
	}
	if passThroughCache.HasRecv() {
		// update receiver's taint
		// the receiver may be a pointer, so update further by the pointer
		SetTaintWrapper(s.outMap, inst.Call.Args[0].Name(), newRecvTaint)
		if op, ok := (inst.Call.Args[0]).(*ssa.UnOp); ok {
			PassTaint(s.outMap, op.X.Name(), op.Name())
			s.passPointTaint(op.X)
		} else {
			s.passPointTaint(inst.Call.Args[0])
		}
	}
	for i := 0; i < passThroughCache.ResultNum(); i++ {
		if passThroughCache.ResultNum() == 1 {
			// if the function has one result
			SetTaintWrapper(s.outMap, inst.Name(), newResultTaints[i])
		} else {
			// else mark the variables as "inst.Name().X"
			// e.g. t0.1, t0.2
			SetTaintWrapper(s.outMap, inst.Name()+"."+strconv.Itoa(i), newResultTaints[i])
		}
	}
	for i := 0; i < passThroughCache.ParamNum(); i++ {
		var recv int
		if passThroughCache.HasRecv() {
			recv = 1
		} else {
			recv = 0
		}
		// update args' taint, use passPointTaint to pass back
		SetTaintWrapper(s.outMap, inst.Call.Args[recv+i].Name(), newParamTaints[i])
		s.passPointTaint(inst.Call.Args[recv+i])
	}
}

// passPointTaint passes taint by pointer
func (s *TaintSwitcher) passPointTaint(pointer ssa.Value) {
	switch addr := (pointer).(type) {
	case *ssa.Alloc:
		for _, _inst := range *addr.Referrers() {
			switch inst := _inst.(type) {
			case *ssa.Store:
				PassTaint(s.outMap, inst.Val.Name(), addr.Name())
				s.passBackCallTaint(inst.Val)
			}
		}
	case *ssa.Convert:
		// if addr is a *ssa.Convert, try use its addr.X to update further
		PassTaint(s.outMap, addr.X.Name(), addr.Name())
		s.passPointTaint(addr.X)
	case *ssa.TypeAssert:
		// if addr is a *ssa.TypeAssert, try use its addr.X to update further
		PassTaint(s.outMap, addr.X.Name(), addr.Name())
		s.passPointTaint(addr.X)
	case *ssa.ChangeType:
		// if addr is a *ssa.ChangeType, try use its addr.X to update further
		PassTaint(s.outMap, addr.X.Name(), addr.Name())
		s.passPointTaint(addr.X)
	case *ssa.ChangeInterface:
		// if addr is a *ssa.ChangeInterface, try use its addr.X to update further
		PassTaint(s.outMap, addr.X.Name(), addr.Name())
		s.passPointTaint(addr.X)
	case *ssa.MakeInterface:
		// if addr is a *ssa.MakeInterface, try use its addr.X to update further
		PassTaint(s.outMap, addr.X.Name(), addr.Name())
		s.passPointTaint(addr.X)
	case *ssa.UnOp:
		// if addr is a *ssa.UnOp, try use its addr.X to update further
		PassTaint(s.outMap, addr.X.Name(), addr.Name())
		s.passPointTaint(addr.X)
	case *ssa.FieldAddr:
		// if addr is still a *ssa.FieldAddr, update further
		PassTaint(s.outMap, addr.X.Name(), addr.Name())
		s.passPointTaint(addr.X)
	case *ssa.IndexAddr:
		// if addr is a *ssa.IndexAddr, update further
		PassTaint(s.outMap, addr.X.Name(), addr.Name())
		s.passPointTaint(addr.X)
	case *ssa.Slice:
		// if addr is a *ssa.Slice, update underlying array
		PassTaint(s.outMap, addr.X.Name(), addr.Name())
	}
}

// passBackCallTaint trys to pass back call taint from results to args
func (s *TaintSwitcher) passBackCallTaint(_call ssa.Value) {
	if call, ok := _call.(*ssa.Call); ok {
		for _, arg := range call.Call.Args {
			if _, ok := arg.(*ssa.Parameter); ok {
				PassTaint(s.outMap, arg.Name(), call.Name())
			}
		}
	}
}

// passAppendTaint passes taint by append
func (s *TaintSwitcher) passAppendTaint(inst *ssa.Call) {
	newTaint := NewTaintWrapper()
	n := len(inst.Call.Args)
	for i := 0; i < n; i++ {
		// collect taint in slices
		// need *ssa.UnOp，may be more other types
		// e.g. path/path.go Join
		// buf = append(buf, e...)
		newTaint.InheritTaint(s.outMap, inst.Call.Args[i].Name())
	}
	SetTaintWrapper(s.outMap, inst.Name(), newTaint)
	for i := 0; i < n; i++ {
		// pass taint to every slice
		PassTaint(s.outMap, inst.Call.Args[i].Name(), inst.Name())
	}
}

// passInvokeTaint passes taint by *types.Func
// actually, only interfaces use this
func (s *TaintSwitcher) passInvokeTaint(f *types.Func, inst *ssa.Call) {
	if !s.taintAnalysis.config.PassThroughOnly {
		s.collectMethodEdges(f, inst)
	}
	interfaceHierarchy := s.taintAnalysis.config.InterfaceHierarchy
	tiface := inst.Call.Value.Type().Underlying().(*types.Interface)
	methods := interfaceHierarchy.LookupMethods(tiface, f)
	if len(methods) != 0 {
		s.passMethodTaint(methods[0], inst)
	} else {
		s.passNullTaint(f, inst)
	}
}

// passMethodTaint passes taint by *ssa.Function and an invoke
func (s *TaintSwitcher) passMethodTaint(f *ssa.Function, inst *ssa.Call) {
	container := s.taintAnalysis.config.PassThroughContainer
	c := s.taintAnalysis.config
	_, ok := (*container)[f.String()]
	if !ok {
		if needNull(f, c) {
			// function is loaded from C file and has no body
			m, ok := f.Object().(*types.Func)
			if ok {
				s.passNullTaint(m, inst)
			}
			return
		}
		// if we can saved it, load it now
		Run(f, c)
	}

	passThroughCache := (*container)[f.String()]
	var newRecvTaint *TaintWrapper
	newResultTaints := make([]*TaintWrapper, 0)
	newParamTaints := make([]*TaintWrapper, 0)
	if passThroughCache.HasRecv() {
		// for every parameter index in passthrough, collect arg's taint
		for _, p := range passThroughCache.Recv {
			newTaint := NewTaintWrapper()
			if p == 0 {
				// the first arg is inst.Call.Value
				newTaint.InheritTaint(s.outMap, inst.Call.Value.Name())
			} else {
				// other args are in inst.Call.Args
				newTaint.InheritTaint(s.outMap, inst.Call.Args[p-1].Name())
			}
			newRecvTaint = newTaint
		}
	}
	for _, result := range passThroughCache.Results {
		newTaint := NewTaintWrapper()
		// for every parameter index in passthrough, collect arg's taint
		for _, p := range result {
			if p == 0 {
				// the first arg is inst.Call.Value
				newTaint.InheritTaint(s.outMap, inst.Call.Value.Name())
			} else {
				// other args are in inst.Call.Args
				newTaint.InheritTaint(s.outMap, inst.Call.Args[p-1].Name())
			}
		}
		newResultTaints = append(newResultTaints, newTaint)
	}
	for _, param := range passThroughCache.Params {
		newTaint := NewTaintWrapper()
		// for every parameter index in passthrough, collect arg's taint
		for _, p := range param {
			if p == 0 {
				// the first arg is inst.Call.Value
				newTaint.InheritTaint(s.outMap, inst.Call.Value.Name())
			} else {
				// other args are in inst.Call.Args
				newTaint.InheritTaint(s.outMap, inst.Call.Args[p-1].Name())
			}
		}
		newParamTaints = append(newParamTaints, newTaint)
	}
	if passThroughCache.HasRecv() {
		// update receiver's taint
		// the receiver may be a pointer, so update further by the pointer
		SetTaintWrapper(s.outMap, inst.Call.Value.Name(), newRecvTaint)
		if op, ok := (inst.Call.Value).(*ssa.UnOp); ok {
			PassTaint(s.outMap, op.X.Name(), op.Name())
			s.passPointTaint(op.X)
		} else {
			s.passPointTaint(inst.Call.Value)
		}
	}
	for i := 0; i < passThroughCache.ResultNum(); i++ {
		if passThroughCache.ResultNum() == 1 {
			// if the function has one result
			SetTaintWrapper(s.outMap, inst.Name(), newResultTaints[i])
		} else {
			// else mark the variables as "inst.Name().X"
			// e.g. t0.1, t0.2
			SetTaintWrapper(s.outMap, inst.Name()+"."+strconv.Itoa(i), newResultTaints[i])
		}
	}
	for i := 0; i < passThroughCache.ParamNum(); i++ {
		// update args' taint
		SetTaintWrapper(s.outMap, inst.Call.Args[i].Name(), newParamTaints[i])
	}
}

// passNullTaint passes taint when we can't know a declared function's body or have to inhibit recursive
// actually no taint will be passed
// note that this may lose some taint but help analysis keep working
func (s *TaintSwitcher) passNullTaint(f *types.Func, inst *ssa.Call) {
	signature, ok := f.Type().(*types.Signature)
	if ok {
		recv := signature.Recv() != nil
		result := signature.Results().Len()
		param := signature.Params().Len()
		//(*container)[f.String()] = NewPassThroughCache(recv, result, param)
		if recv {
			// do nothing because we don't need to update recv
		}
		for i := 0; i < result; i++ {
			if result == 1 {
				GetTaintWrapper(s.outMap, inst.Name())
			} else {
				GetTaintWrapper(s.outMap, inst.Name()+"."+strconv.Itoa(i))
			}
		}
		for i := 0; i < param; i++ {
			// do nothing because we don't need to update params
		}
	}
}

// passFuncParamTaint passes taint by *types.Signature
// actually, only functions without body use this
func (s *TaintSwitcher) passFuncParamTaint(signature *types.Signature, inst *ssa.Call) {
	if !s.taintAnalysis.config.PassThroughOnly {
		s.collectSignatureEdges(signature, inst)
	}
	interfaceHierarchy := s.taintAnalysis.config.InterfaceHierarchy
	funcs := interfaceHierarchy.LookupFuncs(signature)
	if len(funcs) != 0 {
		s.passStaticCallTaint(funcs[0], inst)
		return
	}
	s.passAnonymousTaint(signature, inst)
}

// passAnonymousTaint called by passFuncParamTaint
// it does not save passthrough to passthroughContainer
func (s *TaintSwitcher) passAnonymousTaint(signature *types.Signature, inst *ssa.Call) {
	passThrough := make([][]int, 0)
	n := signature.Results().Len()
	for i := 0; i < n; i++ {
		passThrough = append(passThrough, make([]int, 0))
	}
	n = len(passThrough)
	if n == 1 {
		GetTaintWrapper(s.outMap, inst.Name())
	} else {
		for i := 0; i < n; i++ {
			if n != 1 {
				GetTaintWrapper(s.outMap, inst.Name()+"."+strconv.Itoa(i))
			}
		}
	}
}

// passCopyTaint pass taint by copy
func (s *TaintSwitcher) passCopyTaint(inst *ssa.Call) {
	PassTaint(s.outMap, inst.Call.Args[0].Name(), inst.Call.Args[1].Name())
	GetTaintWrapper(s.outMap, inst.Name())
}

func (s *TaintSwitcher) collectCallEdges(f *ssa.Function, inst *ssa.Call) {
	taintGraph := s.taintAnalysis.config.TaintGraph
	if s.taintAnalysis.Graph.Func.Name() == "init" {
		return
	}
	for i, arg := range inst.Call.Args {
		for name := range *GetTaint(s.outMap, arg.Name()) {
			for k, v := range s.taintAnalysis.Graph.Func.Params {
				if v.Name() == name {
					edge := Edge{From: s.taintAnalysis.Graph.Func.String(), FromIndex: k, To: f.String(), ToIndex: i}
					key := s.taintAnalysis.Graph.Func.String() + "#" + strconv.Itoa(k)
					key2 := f.String() + "#" + strconv.Itoa(i)
					node := (*taintGraph.Nodes)[key]
					node2 := (*taintGraph.Nodes)[key2]
					if node.IsIntra {
						if _, ok := (*taintGraph.Edges)[key+"#"+key2]; ok {
							continue
						} else {
							(*taintGraph.Edges)[key+"#"+key2] = &edge
						}
						node.Out = append(node.Out, &edge)
						node2.In = append(node2.In, &edge)
						passProperty(node2, &edge)
					}
				}
			}
		}
	}
}

// collectMethodsEdges records node only use type information
func (s *TaintSwitcher) collectMethodEdges(f *types.Func, inst *ssa.Call) {
	signature, ok := f.Type().(*types.Signature)
	ruler := s.taintAnalysis.config.Ruler
	taintGraph := s.taintAnalysis.config.TaintGraph
	if ok {
		for name := range *GetTaint(s.outMap, inst.Call.Value.Name()) {
			// contruct taint edge from receiver to arg
			for k, v := range s.taintAnalysis.Graph.Func.Params {
				if v.Name() == name {
					edge := Edge{From: s.taintAnalysis.Graph.Func.String(), FromIndex: k, To: f.String(), ToIndex: 0}
					key := s.taintAnalysis.Graph.Func.String() + "#" + strconv.Itoa(k)
					key2 := f.String() + "#" + strconv.Itoa(0)
					node := (*taintGraph.Nodes)[key]
					if node.IsIntra {
						node.Out = append(node.Out, &edge)
						if _, ok := (*taintGraph.Edges)[key+"#"+key2]; ok {
							continue
						} else {
							(*taintGraph.Edges)[key+"#"+key2] = &edge
						}
						if node2, ok := (*taintGraph.Nodes)[key2]; ok {
							node2.In = append(node2.In, &edge)
							passProperty(node2, &edge)
						} else {
							node2 := &Node{Canonical: signature.String(), Index: 0, Out: make([]*Edge, 0), In: make([]*Edge, 0), IsSignature: false, IsMethod: true, IsStatic: false}
							decidePropertry(node2, ruler)
							node2.In = append(node2.In, &edge)
							(*taintGraph.Nodes)[f.String()] = node2
							passProperty(node2, &edge)
						}
					}
				}
			}
		}
		n := signature.Params().Len()
		for i := 0; i < n; i++ {
			// contruct taint edge from param to arg
			for name := range *GetTaint(s.outMap, inst.Call.Args[i].Name()) {
				for k, v := range s.taintAnalysis.Graph.Func.Params {
					if v.Name() == name {
						edge := Edge{From: s.taintAnalysis.Graph.Func.String(), FromIndex: k, To: f.String(), ToIndex: i + 1}
						key := s.taintAnalysis.Graph.Func.String() + "#" + strconv.Itoa(k)
						key2 := f.String() + "#" + strconv.Itoa(0)
						node := (*taintGraph.Nodes)[key]
						if node.IsIntra {
							node.Out = append(node.Out, &edge)
							if _, ok := (*taintGraph.Edges)[key+"#"+key2]; ok {
								continue
							} else {
								(*taintGraph.Edges)[key+"#"+key2] = &edge
							}
							if node2, ok := (*taintGraph.Nodes)[key2]; ok {
								node2.In = append(node2.In, &edge)
								passProperty(node2, &edge)
							} else {
								node2 := &Node{Canonical: signature.String(), Index: 0, Out: make([]*Edge, 0), In: make([]*Edge, 0), IsSignature: false, IsMethod: true, IsStatic: false}
								decidePropertry(node2, ruler)
								node2.In = append(node2.In, &edge)
								(*taintGraph.Nodes)[f.String()] = node2
								passProperty(node2, &edge)
							}
						}
					}
				}
			}
		}
	}
}

// collectSignatureEdges records node only use signature information
func (s *TaintSwitcher) collectSignatureEdges(signature *types.Signature, inst *ssa.Call) {
	ruler := s.taintAnalysis.config.Ruler
	taintGraph := s.taintAnalysis.config.TaintGraph
	n := signature.Params().Len()
	for i := 0; i < n; i++ {
		for name := range *GetTaint(s.outMap, inst.Call.Args[i].Name()) {
			for k, v := range s.taintAnalysis.Graph.Func.Params {
				if v.Name() == name {
					edge := Edge{From: s.taintAnalysis.Graph.Func.String(), FromIndex: k, To: signature.String(), ToIndex: i}
					key := s.taintAnalysis.Graph.Func.String() + "#" + strconv.Itoa(k)
					key2 := signature.String() + "#" + strconv.Itoa(0)
					node := (*taintGraph.Nodes)[key]
					if node.IsIntra {
						node.Out = append(node.Out, &edge)
						if _, ok := (*taintGraph.Edges)[key+"#"+key2]; ok {
							continue
						} else {
							(*taintGraph.Edges)[key+"#"+key2] = &edge
						}
						if node2, ok := (*taintGraph.Nodes)[key2]; ok {
							node2.In = append(node2.In, &edge)
							passProperty(node2, &edge)
						} else {
							node2 := &Node{Canonical: signature.String(), Index: 0, Out: make([]*Edge, 0), In: make([]*Edge, 0), IsSignature: true, IsMethod: false, IsStatic: false}
							decidePropertry(node2, ruler)
							node2.In = append(node2.In, &edge)
							(*taintGraph.Nodes)[signature.String()] = node2
							passProperty(node2, &edge)
						}
					}
				}
			}
		}
	}
}
