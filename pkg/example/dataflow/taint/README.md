# Taint
## 选项
我编写了一个运行器以帮助您进行污点分析。\
您可以在运行器上直接设置选项，如下所示：

```go
runner := taint.NewRunner("relative/path/to/package")
runner.ModuleName = "module-name"
runner.PassThroughDstPath = "passthrough.json"
runner.CallGraphDstPath = "callgraph.json"
```

所有选项如下：

- `ModuleName`（必要）：目标模块的名称，通常在 go.mod 中
- `PkgPath`（必要）：目标包的相对路径，重要的是您应该在同一项目中编写分析文件。例如 `cmd/myanalysis/main.go`，以防 Go 找不到目标包
- `Debug`（可选）：设置为 true 时，输出调试信息，默认值为 `false`
- `InitOnly`（可选）：设置为 true 时，仅分析初始化函数，默认值为 `false`
- `PassThroughOnly`（可选）：设置为 true 时，仅进行通道分析，默认值为 `false`
- `PassThroughSrcPath`（可选）：通道源的路径，您可以使用它来加速分析或添加额外的通道，默认值为 `[]string{}`
- `PassThroughDstPath`（可选）：保存通道输出的路径，默认值为 `""`
- `TaintGraphDstPath`（可选）：保存污点边输出的路径，默认值为 `""`
- `Ruler`（可选）：ruler 是一个接口，用于定义如何判断一个节点是下沉节点、源节点或内部节点。您可以实现它，默认值为 [DummyRuler](ruler.go)
- `PersistToNeo4j`（可选）：设置为 true 时，将节点和边保存到 Neo4j，默认值为 `false`
- `Neo4jUsername`（可选）：Neo4j 用户名，默认值为 `""`
- `Neo4jPassword`（可选）：Neo4j 密码，默认值为 `""`
- `Neo4jURI`（可选）：Neo4j URI，默认值为 `""`
- `TargetFunc`（可选）：设置时，仅分析目标函数并输出其 SSA，默认值为 `""`
- `UsePointerAnalysis`（可选）：设置时，使用指针分析来帮助选择被调用者，默认值为 `false`。⚠️ 注意，如果设置为 true，`PkgPath` 选项只能包含主包