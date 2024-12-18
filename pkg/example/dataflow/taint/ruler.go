package taint

import (
	"fmt"
	"github.com/zeroy0410/goot/pkg/example/dataflow/taint/rule"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"strings"
)

// DummyRuler is a dummy rule.Ruler used for test
type DummyRuler struct {
	rule.BaseRuler
	moduleName []string
}

// NewDummyRuler returns a DummyRuler
func NewDummyRuler(moduleName ...string) *DummyRuler {
	dummyRuler := new(DummyRuler)
	dummyRuler.moduleName = moduleName
	return dummyRuler
}

// IsIntra returns whether a node is from target module
func (r *DummyRuler) IsIntra(_f any) bool {
	switch node := (_f).(type) {
	case *Node:
		for _, name := range r.moduleName {
			if strings.HasPrefix(node.Canonical, name) {
				return true
			} else if strings.HasPrefix(node.Canonical, "("+name) {
				return true
			} else if strings.HasPrefix(node.Canonical, "(*"+name) {
				return true
			}
		}
	}
	return false
}

// IsSink returns whether a node is a sink
func (r *DummyRuler) IsSink(_f any) bool {
	sink := make(map[string]bool)
	// cmdi
	sink["os/exec.Command"] = true
	sink["os/exec.CommandContext"] = true
	sink["syscall.Exec"] = true
	sink["syscall.ForkExec"] = true
	sink["syscall.StartProcess"] = true
	// sqli
	sink["(*database/sql.DB).Exec"] = true
	sink["(*database/sql.DB).ExecContext"] = true
	sink["(*database/sql.DB).Query"] = true
	sink["(*database/sql.DB).QueryContext"] = true
	sink["(*database/sql.DB).QueryRow"] = true
	sink["(*database/sql.DB).QueryRowContext"] = true
	sink["(*database/sql.Stmt).Exec"] = true
	sink["(*database/sql.Stmt).ExecContext"] = true
	sink["(*database/sql.Stmt).Query"] = true
	sink["(*database/sql.Stmt).QueryContext"] = true
	sink["(*database/sql.Stmt).QueryRow"] = true
	sink["(*database/sql.Stmt).QueryRowContext"] = true
	sink["(*database/sql.Tx).Exec"] = true
	sink["(*database/sql.Tx).ExecContext"] = true
	sink["(*database/sql.Tx).Query"] = true
	sink["(*database/sql.Tx).QueryContext"] = true
	sink["(*database/sql.Tx).QueryRow"] = true
	sink["(*database/sql.Tx).QueryRowContext"] = true
	sink["(*github.com/jmoiron/sqlx.DB).Select"] = true
	sink["(*github.com/jmoiron/sqlx.DB).Get"] = true
	sink["(*github.com/jmoiron/sqlx.DB).Queryx"] = true
	sink["(*github.com/jmoiron/sqlx.DB).QueryRowx"] = true
	sink["(*gorm.io/gorm.DB).Raw"] = true
	sink["(*gorm.io/gorm.DB).Where"] = true
	sink["(*gorm.io/gorm.DB).Or"] = true
	sink["(*gorm.io/gorm.DB).Order"] = true
	sink["(*xorm.io/xorm.Engine).Query"] = true
	sink["(*xorm.io/xorm.Engine).Exec"] = true
	sink["(*xorm.io/xorm.Engine).QueryString"] = true
	sink["(*xorm.io/xorm.Engine).QueryInterface"] = true
	sink["(*xorm.io/xorm.Engine).Where"] = true
	sink["(*xorm.io/xorm.Engine).OrderBy"] = true
	sink["(*xorm.io/xorm.Engine).SQL"] = true
	sink["(*xorm.io/xorm.Session).Query"] = true
	sink["(*xorm.io/xorm.Session).Exec"] = true
	sink["(*xorm.io/xorm.Session).QuerySliceString"] = true
	sink["(*xorm.io/xorm.Session).QueryInterface"] = true
	sink["(*xorm.io/xorm.Session).And"] = true
	sink["(*xorm.io/xorm.Session).Or"] = true
	sink["(*xorm.io/xorm.Session).Where"] = true
	sink["(*xorm.io/xorm.Session).OrderBy"] = true
	sink["(*xorm.io/xorm.Session).SQL"] = true
	sink["(github.com/Masterminds/squirrel.SelectBuilder).From"] = true
	sink["(github.com/Masterminds/squirrel.SelectBuilder).Where"] = true
	sink["(github.com/Masterminds/squirrel.SelectBuilder).OrderBy"] = true
	// ssrf
	sink["net/http.Get"] = true
	sink["net/http.Head"] = true
	sink["net/http.Post"] = true
	sink["net/http.PostForm"] = true
	sink["(*net/http.Client).Do"] = true
	sink["(*net/http.Client).Get"] = true
	sink["(*net/http.Client).Head"] = true
	sink["(*net/http.Client).Post"] = true
	sink["(*net/http.Client).PostForm"] = true
	sink["(*github.com/hashicorp/go-retryablehttp.Client).Do"] = true
	sink["(*github.com/hashicorp/go-retryablehttp.Client).Get"] = true
	sink["(*github.com/hashicorp/go-retryablehttp.Client).Head"] = true
	sink["(*github.com/hashicorp/go-retryablehttp.Client).Post"] = true
	sink["(*github.com/hashicorp/go-retryablehttp.Client).PostForm"] = true
	sink["(*github.com/go-resty/resty/v2.Request).Get"] = true
	sink["(*github.com/go-resty/resty/v2.Request).Post"] = true
	sink["(*github.com/go-resty/resty/v2.Request).Put"] = true
	sink["(*github.com/go-resty/resty/v2.Request).Delete"] = true
	sink["(*github.com/go-resty/resty/v2.Request).Options"] = true
	sink["(*github.com/go-resty/resty/v2.Request).Patch"] = true
	sink["(*github.com/go-resty/resty/v2.Request).Send"] = true
	sink["(*github.com/go-resty/resty/v2.Request).Execute"] = true
	sink["github.com/sethgrid/pester.Do"] = true
	sink["github.com/sethgrid/pester.Get"] = true
	sink["github.com/sethgrid/pester.Head"] = true
	sink["github.com/sethgrid/pester.Post"] = true
	sink["github.com/sethgrid/pester.PostForm"] = true
	sink["(*github.com/sethgrid/pester.Client).Do"] = true
	sink["(*github.com/sethgrid/pester.Client).Get"] = true
	sink["(*github.com/sethgrid/pester.Client).Head"] = true
	sink["(*github.com/sethgrid/pester.Client).Post"] = true
	sink["(*github.com/sethgrid/pester.Client).PostForm"] = true
	sink["(*github.com/imroc/req.Request).SetURL"] = true
	sink["(*github.com/dghubble/sling).Base"] = true
	sink["(*github.com/dghubble/sling).Get"] = true
	sink["(*github.com/dghubble/sling).Head"] = true
	sink["(*github.com/dghubble/sling).Post"] = true
	sink["(*github.com/dghubble/sling).Put"] = true
	sink["(*github.com/dghubble/sling).Patch"] = true
	sink["(*github.com/dghubble/sling).Delete"] = true
	sink["(*github.com/dghubble/sling).Options"] = true
	sink["(*github.com/dghubble/sling).Trace"] = true
	sink["(*github.com/dghubble/sling).Connect"] = true
	sink["github.com/asmcos/requests.Get"] = true
	sink["github.com/asmcos/requests.Post"] = true
	sink["github.com/asmcos/requests.PostJson"] = true
	sink["(*github.com/asmcos/requests.Request).Get"] = true
	sink["(*github.com/asmcos/requests.Request).Post"] = true
	sink["(*github.com/asmcos/requests.Request).PostJson"] = true
	sink["github.com/carlmjohnson/requests.URL"] = true
	sink["(*github.com/carlmjohnson/requests.Builder).Host"] = true
	sink["(*github.com/carlmjohnson/requests.Builder).Do"] = true
	sink["github.com/mozillazg/request.Get"] = true
	sink["github.com/mozillazg/request.Head"] = true
	sink["github.com/mozillazg/request.Post"] = true
	sink["github.com/mozillazg/request.Put"] = true
	sink["github.com/mozillazg/request.Patch"] = true
	sink["github.com/mozillazg/request.Delete"] = true
	sink["github.com/mozillazg/request.Options"] = true
	sink["(*github.com/mozillazg/request.Request).Get"] = true
	sink["(*github.com/mozillazg/request.Request).Head"] = true
	sink["(*github.com/mozillazg/request.Request).Post"] = true
	sink["(*github.com/mozillazg/request.Request).Put"] = true
	sink["(*github.com/mozillazg/request.Request).Patch"] = true
	sink["(*github.com/mozillazg/request.Request).Delete"] = true
	sink["(*github.com/mozillazg/request.Request).Options"] = true
	// traversal
	sink["os.Create"] = true
	sink["os.Open"] = true
	sink["os.OpenFile"] = true
	sink["os.ReadFile"] = true
	sink["io/ioutil.ReadFile"] = true
	sink["io/ioutil.WriteFile"] = true

	switch node := _f.(type) {
	case *Node:
		_, ok := sink[node.Canonical]
		if ok {
			return true
		}
	}
	return false
}

// IsSource returns whether a node is a source
func (r *DummyRuler) IsSource(_f any) bool {
	source := make(map[string]bool)
	source["os.ReadFile"] = true
	switch node := _f.(type) {
	case *Node:
		_, ok := source[node.Canonical]
		if ok {
			fmt.Println("is source!")
			fmt.Println(node.Canonical)
			fmt.Println()
			return true
		}
		if node.Function != nil {
			flag := false
			f := node.Function
			flag = flag || checkTrivalHandler(f) || checkBeegoHandler(f) || checkGinHandler(f)
			if flag {
				fmt.Println("is source!")
				fmt.Println(node.Canonical)
				fmt.Println()
				return true
			}
		}
	}
	return false
}

func checkTrivalHandler(f *ssa.Function) bool {
	hit := 0
	for _, param := range f.Params {
		if param.Type().String() == "net/http.ResponseWriter" {
			hit++
		}
		if param.Type().String() == "*net/http.Request" {
			hit++
		}
		if hit >= 2 {
			return true
		}
	}
	return false
}

func checkBeegoHandler(f *ssa.Function) bool {
	if f.Signature.Recv() != nil {
		recv := f.Signature.Recv().Type()
		if pointer, ok := recv.(*types.Pointer); ok {
			recv = pointer.Elem()
		}
		if named, ok := recv.(*types.Named); ok {
			name := named.Obj().Name()
			if f.Pkg != nil {
				if member, ok := f.Pkg.Members[name]; ok {
					if typ, ok := member.Type().Underlying().(*types.Struct); ok {
						n := typ.NumFields()
						for i := 0; i < n; i++ {
							field := typ.Field(i).Type().String()
							if field == "github.com/beego/beego/v2/server/web.Controller" {
								return true
							}
							if field == "github.com/beego/beego/beego.Controller" {
								return true
							}
							if field == "github.com/astaxie/beego/beego.Controller" {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

func checkGinHandler(f *ssa.Function) bool {
	hit := 0
	for _, param := range f.Params {
		if param.Type().String() == "*github.com/gin-gonic/gin.Context" {
			hit++
		}
		if hit >= 1 {
			return true
		}
	}
	return false
}

// passPropertry pass properties from a node to an edge
func passProperty(node *Node, edge *Edge) {
	if node.IsMethod {
		edge.ToIsMethod = true
	} else if node.IsStatic {
		edge.ToIsStatic = true
	} else if node.IsSignature {
		edge.ToIsSignature = true
	}
	if node.IsSink {
		edge.ToIsSink = true
	}
}

// decideProperty decide a node's properties by a ruler
func decidePropertry(node *Node, ruler rule.Ruler) {
	if ruler.IsIntra(node) {
		node.IsIntra = true
	}
	if ruler.IsSource(node) {
		node.IsSource = true
	}
	if ruler.IsSink(node) {
		node.IsSink = true
	}
}
