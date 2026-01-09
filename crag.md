# 项目调用图谱 (RAG)

> 生成时间: 2026-01-10 03:24:05
> 函数节点: 127 | 调用边: 233

## 项目结构

```
├── crag/
│   ├── internal/
│   │   ├── analyzer/
│   │   ├── export/
│   │   ├── graph/
│   │   ├── impact/
│   │   ├── mcp/
│   │   ├── storage/
│   │   ├── watcher/
│   │   ├── web/
```

## 架构图

```mermaid
flowchart TB
    subgraph analyzer [静态分析层]
        BuildSSA[BuildSSA]
        GetGitChanges[GetGitChanges]
        FilterMainPackages[FilterMainPackages]
        LoadPackages[LoadPackages]
        BuildCallGraph[BuildCallGraph]
        GetChangedPackagePatterns[GetChangedPackagePatterns]
        String[String]
        GetCallGraphStats[GetCallGraphStats]
        HasChanges[HasChanges]
    end

    subgraph graph [图构建层]
        Build[Build]
        GetNodeCount[GetNodeCount]
        NewBuilder[NewBuilder]
    end

    subgraph storage [存储层]
        InsertEdge[InsertEdge]
        GetNodeByID[GetNodeByID]
        DeleteNodesByPackage[DeleteNodesByPackage]
        InsertNode[InsertNode]
        GetStats[GetStats]
        Close[Close]
        DeleteOrphanEdges[DeleteOrphanEdges]
        GetAllFunctions[GetAllFunctions]
        GetNodeByName[GetNodeByName]
        GetNodesByPackage[GetNodesByPackage]
        Open[Open]
        Conn[Conn]
        GetCallEdgesForNode[GetCallEdgesForNode]
        GetAllEdges[GetAllEdges]
        GetDownstreamCallees[GetDownstreamCallees]
        Clear[Clear]
        FindNodesByPattern[FindNodesByPattern]
        GetUpstreamCallers[GetUpstreamCallers]
        GetDirectCallers[GetDirectCallers]
        GetDirectCallees[GetDirectCallees]
    end

    subgraph impact [影响分析层]
        AnalyzeImpact[AnalyzeImpact]
        FormatMarkdown[FormatMarkdown]
        NewAnalyzer[NewAnalyzer]
        Summary[Summary]
    end

    subgraph export [导出层]
        NewExporter[NewExporter]
        ExportIncremental[ExportIncremental]
        DefaultExportOptions[DefaultExportOptions]
        Export[Export]
    end

    subgraph other [其他]
        NewServer[NewServer]
        Run[Run]
        WithOnAnalysisDone[WithOnAnalysisDone]
        Start[Start]
        WithOnError[WithOnError]
        New[New]
        WithOnAnalysisStart[WithOnAnalysisStart]
        WithDebounceDelay[WithDebounceDelay]
        Stop[Stop]
        NewServer[NewServer]
        Run[Run]
    end

    % 关键调用关系
    New --> WithOnAnalysisStart
    New --> WithDebounceDelay
    New --> WithOnError
    New --> WithOnAnalysisDone
    ExportIncremental --> GetAllFunctions
    ExportIncremental --> GetDirectCallers
    Export --> GetAllFunctions
    Export --> GetStats
    AnalyzeImpact --> GetNodeByName
    AnalyzeImpact --> FindNodesByPattern
    AnalyzeImpact --> GetDirectCallers
    AnalyzeImpact --> GetUpstreamCallers
    AnalyzeImpact --> GetDirectCallees
    AnalyzeImpact --> GetDownstreamCallees
```

---

## 模块详解

### 📦 internal/analyzer

| 函数 | 说明 | 被调用 | 调用 |
|------|------|--------|------|
| `GetChangedPackagePatterns` | GetChangedPackagePatterns r... | 0 | 0 |
| `HasChanges` | HasChanges returns true if ... | 2 | 0 |
| `String` | String returns a summary st... | 0 | 0 |
| `BuildCallGraph` | BuildCallGraph builds the c... | 3 | 0 |
| `BuildSSA` | BuildSSA builds the SSA rep... | 3 | 0 |
| `FilterMainPackages` | FilterMainPackages filters ... | 3 | 0 |
| `GetCallGraphStats` | GetCallGraphStats returns s... | 0 | 0 |
| `GetGitChanges` | GetGitChanges returns the l... | 2 | 0 |
| `LoadPackages` | LoadPackages loads all Go p... | 3 | 0 |

#### `GetChangedPackagePatterns`

- **位置**: `internal/analyzer/git.go:95`
- **签名**: `func() []string`
- **说明**: GetChangedPackagePatterns returns package patterns for go/packages.Load

#### `HasChanges`

- **位置**: `internal/analyzer/git.go:85`
- **签名**: `func() bool`
- **说明**: HasChanges returns true if there are any Go file changes
- **被调用**: `analyzeCmd`, `exportCmd`

#### `String`

- **位置**: `internal/analyzer/git.go:90`
- **签名**: `func() string`
- **说明**: String returns a summary string of the changes

#### `BuildCallGraph`

- **位置**: `internal/analyzer/callgraph.go:12`
- **签名**: `func(prog *golang.org/x/tools/go/ssa.Program) (*golang.org/x/tools/go/callgraph.Graph, error)`
- **说明**: BuildCallGraph builds the call graph using VTA (Variable Type Analysis)
VTA is more precise than other algorithms for handling interface calls
- **被调用**: `analyzeCmd`, `runInitialAnalysis`, `runAnalysis`

#### `BuildSSA`

- **位置**: `internal/analyzer/ssa.go:10`
- **签名**: `func(pkgs []*golang.org/x/tools/go/packages.Package) (*golang.org/x/tools/go/ssa.Program, []*golang.org/x/tools/go/ssa.Package)`
- **说明**: BuildSSA builds the SSA representation for the given packages
- **被调用**: `analyzeCmd`, `runInitialAnalysis`, `runAnalysis`

#### `FilterMainPackages`

- **位置**: `internal/analyzer/loader.go:47`
- **签名**: `func(pkgs []*golang.org/x/tools/go/packages.Package) []*golang.org/x/tools/go/packages.Package`
- **说明**: FilterMainPackages filters packages to only include those with source files
- **被调用**: `analyzeCmd`, `runInitialAnalysis`, `runAnalysis`

#### `GetCallGraphStats`

- **位置**: `internal/analyzer/callgraph.go:29`
- **签名**: `func(cg *golang.org/x/tools/go/callgraph.Graph) github.com/zheng/crag/internal/analyzer.CallGraphStats`
- **说明**: GetCallGraphStats returns statistics about the call graph

#### `GetGitChanges`

- **位置**: `internal/analyzer/git.go:21`
- **签名**: `func(projectPath string, base string) (*github.com/zheng/crag/internal/analyzer.GitChanges, error)`
- **说明**: GetGitChanges returns the list of changed Go files since the last commit
If base is empty, it compares with HEAD (uncommitted changes)
If base is "HEAD~1", it compares with the previous commit
- **被调用**: `analyzeCmd`, `exportCmd`

#### `LoadPackages`

- **位置**: `internal/analyzer/loader.go:10`
- **签名**: `func(projectPath string) ([]*golang.org/x/tools/go/packages.Package, error)`
- **说明**: LoadPackages loads all Go packages from the given project path
- **被调用**: `analyzeCmd`, `runInitialAnalysis`, `runAnalysis`

### 📦 internal/graph

| 函数 | 说明 | 被调用 | 调用 |
|------|------|--------|------|
| `Build` | Build processes the call gr... | 3 | 5 |
| `GetNodeCount` | GetNodeCount returns the nu... | 1 | 0 |
| `NewBuilder` | NewBuilder creates a new gr... | 3 | 0 |
| `createFunctionNode` | createFunctionNode creates ... | 1 | 1 |
| `getDocComment` | getDocComment extracts the ... | 1 | 0 |
| `getParentFunctionName` | getParentFunctionName extra... | 1 | 0 |
| `isClosure` | isClosure checks if a funct... | 1 | 0 |
| `isProjectFunction` | isProjectFunction checks if... | 1 | 0 |
| `resolveToParent` | resolveToParent returns the... | 1 | 0 |

#### `Build`

- **位置**: `internal/graph/builder.go:89`
- **签名**: `func(cg *golang.org/x/tools/go/callgraph.Graph) error`
- **说明**: Build processes the call graph and stores nodes/edges
Closures are merged into their parent functions' call chains
- **被调用**: `analyzeCmd`, `runInitialAnalysis`, `runAnalysis`
- **调用**: `isProjectFunction`, `isClosure`, `getParentFunctionName`, `createFunctionNode`, `resolveToParent`

#### `GetNodeCount`

- **位置**: `internal/graph/builder.go:256`
- **签名**: `func() int`
- **说明**: GetNodeCount returns the number of nodes created
- **被调用**: `analyzeCmd`

#### `NewBuilder`

- **位置**: `internal/graph/builder.go:26`
- **签名**: `func(fset *go/token.FileSet, pkgs []*golang.org/x/tools/go/packages.Package, insertFn func(*github.com/zheng/crag/internal/graph.Node) (int64, error), edgeFn func(*github.com/zheng/crag/internal/graph.Edge) error) *github.com/zheng/crag/internal/graph.Builder`
- **说明**: NewBuilder creates a new graph builder
- **被调用**: `analyzeCmd`, `runInitialAnalysis`, `runAnalysis`

### 📦 internal/storage

| 函数 | 说明 | 被调用 | 调用 |
|------|------|--------|------|
| `Clear` | Clear removes all data from... | 3 | 0 |
| `Close` | Close closes the database c... | 11 | 0 |
| `Conn` | Conn returns the underlying... | 0 | 0 |
| `DeleteNodesByPackage` | DeleteNodesByPackage delete... | 0 | 1 |
| `DeleteOrphanEdges` | DeleteOrphanEdges deletes e... | 0 | 0 |
| `FindNodesByPattern` | FindNodesByPattern returns ... | 8 | 1 |
| `GetAllEdges` | GetAllEdges returns all edg... | 1 | 0 |
| `GetAllFunctions` | GetAllFunctions returns all... | 6 | 1 |
| `GetCallEdgesForNode` | GetCallEdgesForNode returns... | 0 | 0 |
| `GetDirectCallees` | GetDirectCallees returns fu... | 7 | 1 |
| `GetDirectCallers` | GetDirectCallers returns fu... | 7 | 1 |
| `GetDownstreamCallees` | GetDownstreamCallees return... | 4 | 1 |
| `GetNodeByID` | GetNodeByID returns a node ... | 3 | 1 |
| `GetNodeByName` | GetNodeByName returns a nod... | 1 | 1 |
| `GetNodesByPackage` | GetNodesByPackage returns a... | 0 | 2 |
| `GetStats` | GetStats returns database s... | 5 | 0 |
| `GetUpstreamCallers` | GetUpstreamCallers returns ... | 4 | 1 |
| `InsertEdge` | InsertEdge inserts an edge ... | 0 | 0 |
| `InsertNode` | InsertNode inserts a node i... | 0 | 0 |
| `Open` | Open opens or creates a SQL... | 11 | 0 |
| `joinStrings` | - | 2 | 0 |
| `scanNode` | - | 2 | 0 |
| `scanNodes` | - | 7 | 0 |

#### `Clear`

- **位置**: `internal/storage/db.go:46`
- **签名**: `func() error`
- **说明**: Clear removes all data from the database
- **被调用**: `analyzeCmd`, `runInitialAnalysis`, `runAnalysis`

#### `Close`

- **位置**: `internal/storage/db.go:41`
- **签名**: `func() error`
- **说明**: Close closes the database connection
- **被调用**: `upstreamCmd`, `mcpCmd`, `analyzeCmd`, `exportCmd`, `listCmd`, `searchCmd`, `runInitialAnalysis`, `impactCmd`, `serveCmd`, `downstreamCmd`, `runAnalysis`

#### `Conn`

- **位置**: `internal/storage/db.go:52`
- **签名**: `func() *database/sql.DB`
- **说明**: Conn returns the underlying database connection for advanced queries

#### `DeleteNodesByPackage`

- **位置**: `internal/storage/queries.go:271`
- **签名**: `func(packages []string) (int64, error)`
- **说明**: DeleteNodesByPackage deletes all nodes belonging to the specified packages
Also deletes all edges referencing those nodes
Returns the number of deleted nodes
- **调用**: `joinStrings`

#### `DeleteOrphanEdges`

- **位置**: `internal/storage/queries.go:303`
- **签名**: `func() (int64, error)`
- **说明**: DeleteOrphanEdges deletes edges that reference non-existent nodes

#### `FindNodesByPattern`

- **位置**: `internal/storage/queries.go:51`
- **签名**: `func(pattern string) ([]*github.com/zheng/crag/internal/graph.Node, error)`
- **说明**: FindNodesByPattern returns nodes matching a name pattern (using LIKE)
- **被调用**: `toolMermaid`, `toolUpstream`, `toolDownstream`, `searchCmd`, `handleSearch`, `toolSearch`, `impactCmd`, `AnalyzeImpact`
- **调用**: `scanNodes`

#### `GetAllEdges`

- **位置**: `internal/storage/queries.go:240`
- **签名**: `func() ([]*github.com/zheng/crag/internal/graph.Edge, error)`
- **说明**: GetAllEdges returns all edges in the database
- **被调用**: `handleGraph`

#### `GetAllFunctions`

- **位置**: `internal/storage/queries.go:227`
- **签名**: `func() ([]*github.com/zheng/crag/internal/graph.Node, error)`
- **说明**: GetAllFunctions returns all function nodes
- **被调用**: `ExportIncremental`, `handleGraph`, `listCmd`, `Export`, `toolList`, `handleNodes`
- **调用**: `scanNodes`

#### `GetCallEdgesForNode`

- **位置**: `internal/storage/queries.go:196`
- **签名**: `func(nodeID int64) ([]*github.com/zheng/crag/internal/graph.Edge, error)`
- **说明**: GetCallEdgesForNode returns all call edges where the node is the caller

#### `GetDirectCallees`

- **位置**: `internal/storage/queries.go:80`
- **签名**: `func(nodeID int64) ([]*github.com/zheng/crag/internal/graph.Node, error)`
- **说明**: GetDirectCallees returns functions that the given function directly calls
- **被调用**: `handleNode`, `toolMermaid`, `writePackageSection`, `writeArchitectureDiagram`, `AnalyzeImpact`, `buildCalleesTree`, `writeImpactTable`
- **调用**: `scanNodes`

#### `GetDirectCallers`

- **位置**: `internal/storage/queries.go:64`
- **签名**: `func(nodeID int64) ([]*github.com/zheng/crag/internal/graph.Node, error)`
- **说明**: GetDirectCallers returns functions that directly call the given function
- **被调用**: `ExportIncremental`, `handleNode`, `toolMermaid`, `writePackageSection`, `buildCallersTree`, `AnalyzeImpact`, `writeImpactTable`
- **调用**: `scanNodes`

#### `GetDownstreamCallees`

- **位置**: `internal/storage/queries.go:147`
- **签名**: `func(nodeID int64, maxDepth int) ([]*github.com/zheng/crag/internal/graph.Node, error)`
- **说明**: GetDownstreamCallees returns all downstream callees recursively up to maxDepth
If maxDepth is 0, it returns all callees with no depth limit
- **被调用**: `handleImpact`, `toolMermaid`, `toolDownstream`, `AnalyzeImpact`
- **调用**: `scanNodes`

#### `GetNodeByID`

- **位置**: `internal/storage/queries.go:42`
- **签名**: `func(id int64) (*github.com/zheng/crag/internal/graph.Node, error)`
- **说明**: GetNodeByID returns a node by its ID
- **被调用**: `handleImpact`, `handleNode`, `handleCallChain`
- **调用**: `scanNode`

#### `GetNodeByName`

- **位置**: `internal/storage/queries.go:33`
- **签名**: `func(name string) (*github.com/zheng/crag/internal/graph.Node, error)`
- **说明**: GetNodeByName returns a node by its fully qualified name
- **被调用**: `AnalyzeImpact`
- **调用**: `scanNode`

#### `GetNodesByPackage`

- **位置**: `internal/storage/queries.go:316`
- **签名**: `func(packages []string) ([]*github.com/zheng/crag/internal/graph.Node, error)`
- **说明**: GetNodesByPackage returns all nodes in the specified packages
- **调用**: `joinStrings`, `scanNodes`

#### `GetStats`

- **位置**: `internal/storage/queries.go:338`
- **签名**: `func() (nodeCount int64, edgeCount int64, err error)`
- **说明**: GetStats returns database statistics
- **被调用**: `analyzeCmd`, `Export`, `runInitialAnalysis`, `handleStats`, `runAnalysis`

#### `GetUpstreamCallers`

- **位置**: `internal/storage/queries.go:97`
- **签名**: `func(nodeID int64, maxDepth int) ([]*github.com/zheng/crag/internal/graph.Node, error)`
- **说明**: GetUpstreamCallers returns all upstream callers recursively up to maxDepth
If maxDepth is 0, it returns all callers with no depth limit
- **被调用**: `handleImpact`, `toolMermaid`, `toolUpstream`, `AnalyzeImpact`
- **调用**: `scanNodes`

#### `InsertEdge`

- **位置**: `internal/storage/queries.go:23`
- **签名**: `func(edge *github.com/zheng/crag/internal/graph.Edge) error`
- **说明**: InsertEdge inserts an edge into the database

#### `InsertNode`

- **位置**: `internal/storage/queries.go:10`
- **签名**: `func(node *github.com/zheng/crag/internal/graph.Node) (int64, error)`
- **说明**: InsertNode inserts a node into the database and returns its ID

#### `Open`

- **位置**: `internal/storage/db.go:19`
- **签名**: `func(path string) (*github.com/zheng/crag/internal/storage.DB, error)`
- **说明**: Open opens or creates a SQLite database at the given path
- **被调用**: `upstreamCmd`, `mcpCmd`, `analyzeCmd`, `exportCmd`, `listCmd`, `searchCmd`, `runInitialAnalysis`, `impactCmd`, `serveCmd`, `downstreamCmd`, `runAnalysis`

### 📦 internal/impact

| 函数 | 说明 | 被调用 | 调用 |
|------|------|--------|------|
| `AnalyzeImpact` | AnalyzeImpact analyzes the ... | 4 | 6 |
| `FormatMarkdown` | FormatMarkdown formats the ... | 2 | 0 |
| `Summary` | FormatJSON formats the impa... | 1 | 0 |
| `NewAnalyzer` | NewAnalyzer creates a new i... | 4 | 0 |

#### `AnalyzeImpact`

- **位置**: `internal/impact/analyzer.go:31`
- **签名**: `func(funcName string, upstreamDepth int, downstreamDepth int) (*github.com/zheng/crag/internal/impact.ImpactReport, error)`
- **说明**: AnalyzeImpact analyzes the impact of changing a function
- **被调用**: `upstreamCmd`, `impactCmd`, `downstreamCmd`, `toolImpact`
- **调用**: `GetNodeByName`, `FindNodesByPattern`, `GetDirectCallers`, `GetUpstreamCallers`, `GetDirectCallees`, `GetDownstreamCallees`

#### `FormatMarkdown`

- **位置**: `internal/impact/analyzer.go:109`
- **签名**: `func() string`
- **说明**: FormatMarkdown formats the impact report as markdown
- **被调用**: `impactCmd`, `toolImpact`

#### `Summary`

- **位置**: `internal/impact/analyzer.go:175`
- **签名**: `func() string`
- **说明**: FormatJSON formats the impact report as JSON (use encoding/json for actual serialization)
- **被调用**: `impactCmd`

#### `NewAnalyzer`

- **位置**: `internal/impact/analyzer.go:17`
- **签名**: `func(db *github.com/zheng/crag/internal/storage.DB) *github.com/zheng/crag/internal/impact.Analyzer`
- **说明**: NewAnalyzer creates a new impact analyzer
- **被调用**: `upstreamCmd`, `impactCmd`, `downstreamCmd`, `toolImpact`

### 📦 internal/export

| 函数 | 说明 | 被调用 | 调用 |
|------|------|--------|------|
| `Export` | Export generates a complete... | 1 | 8 |
| `ExportIncremental` | ExportIncremental generates... | 1 | 5 |
| `DefaultExportOptions` | DefaultExportOptions return... | 1 | 0 |
| `NewExporter` | NewExporter creates a new e... | 1 | 0 |
| `categorizePackages` | - | 1 | 0 |
| `writeArchitectureDiagram` | writeArchitectureDiagram wr... | 1 | 6 |
| `writeImpactTable` | writeImpactTable writes a s... | 1 | 4 |
| `writePackageSection` | writePackageSection writes ... | 1 | 8 |
| `writeProjectStructure` | writeProjectStructure write... | 1 | 0 |
| `containsPackage` | - | 0 | 1 |
| `getLayerDisplayName` | - | 1 | 0 |
| `getLayerOrder` | - | 1 | 0 |
| `getRelativePath` | - | 3 | 0 |
| `getShortDisplayName` | - | 7 | 0 |
| `getShortPackageName` | - | 1 | 0 |
| `getSortedPackageNames` | - | 1 | 1 |
| `groupByPackage` | - | 1 | 0 |
| `isExportedFunc` | - | 1 | 1 |
| `isKeyFunction` | - | 2 | 1 |
| `makeNodeID` | - | 1 | 1 |
| `matchPackageSuffix` | - | 2 | 0 |
| `truncateDoc` | - | 1 | 0 |

#### `Export`

- **位置**: `internal/export/exporter.go:44`
- **签名**: `func(w io.Writer, opts github.com/zheng/crag/internal/export.ExportOptions) error`
- **说明**: Export generates a complete RAG document
- **被调用**: `exportCmd`
- **调用**: `GetAllFunctions`, `GetStats`, `groupByPackage`, `writeProjectStructure`, `writeArchitectureDiagram`, `getSortedPackageNames`, `writePackageSection`, `writeImpactTable`

#### `ExportIncremental`

- **位置**: `internal/export/exporter.go:297`
- **签名**: `func(w io.Writer, changedPackages []string, opts github.com/zheng/crag/internal/export.ExportOptions) error`
- **说明**: ExportIncremental generates a RAG document for changed packages only
- **被调用**: `exportCmd`
- **调用**: `GetAllFunctions`, `matchPackageSuffix`, `getShortDisplayName`, `GetDirectCallers`, `getRelativePath`

#### `DefaultExportOptions`

- **位置**: `internal/export/exporter.go:34`
- **签名**: `func() github.com/zheng/crag/internal/export.ExportOptions`
- **说明**: DefaultExportOptions returns default export options
- **被调用**: `exportCmd`

#### `NewExporter`

- **位置**: `internal/export/exporter.go:21`
- **签名**: `func(db *github.com/zheng/crag/internal/storage.DB) *github.com/zheng/crag/internal/export.Exporter`
- **说明**: NewExporter creates a new exporter
- **被调用**: `exportCmd`

### 📦 zheng/crag

| 函数 | 说明 | 被调用 | 调用 |
|------|------|--------|------|
| `analyzeCmd` | - | 1 | 13 |
| `downstreamCmd` | - | 1 | 5 |
| `exportCmd` | - | 1 | 8 |
| `impactCmd` | - | 1 | 8 |
| `listCmd` | - | 1 | 3 |
| `main` | - | 0 | 10 |
| `mcpCmd` | - | 1 | 4 |
| `outputJSON` | - | 3 | 0 |
| `runInitialAnalysis` | runInitialAnalysis performs... | 1 | 10 |
| `searchCmd` | - | 1 | 3 |
| `serveCmd` | - | 1 | 4 |
| `upstreamCmd` | - | 1 | 5 |
| `watchCmd` | - | 3 | 8 |

### 📦 internal/mcp

| 函数 | 说明 | 被调用 | 调用 |
|------|------|--------|------|
| `Run` | Run starts the MCP server | 1 | 2 |
| `NewServer` | NewServer creates a new MCP... | 1 | 0 |
| `handleInitialize` | - | 1 | 1 |
| `handleRequest` | - | 1 | 4 |
| `handleToolsCall` | - | 1 | 8 |
| `handleToolsList` | - | 1 | 1 |
| `send` | - | 2 | 0 |
| `sendError` | - | 3 | 1 |
| `sendResult` | - | 3 | 1 |
| `toolDownstream` | - | 1 | 2 |
| `toolImpact` | - | 1 | 3 |
| `toolList` | - | 1 | 1 |
| `toolMermaid` | - | 1 | 7 |
| `toolSearch` | - | 1 | 1 |
| `toolUpstream` | - | 1 | 2 |
| `indexOf` | - | 1 | 0 |
| `lastIndex` | - | 1 | 0 |
| `nodeID` | - | 1 | 1 |
| `shortName` | Helper functions for Mermai... | 2 | 2 |

#### `Run`

- **位置**: `internal/mcp/server.go:113`
- **签名**: `func() error`
- **说明**: Run starts the MCP server
- **被调用**: `mcpCmd`
- **调用**: `sendError`, `handleRequest`

#### `NewServer`

- **位置**: `internal/mcp/server.go:22`
- **签名**: `func(db *github.com/zheng/crag/internal/storage.DB) *github.com/zheng/crag/internal/mcp.Server`
- **说明**: NewServer creates a new MCP server
- **被调用**: `mcpCmd`

### 📦 internal/web

| 函数 | 说明 | 被调用 | 调用 |
|------|------|--------|------|
| `Run` | Run starts the web server | 1 | 0 |
| `NewServer` | NewServer creates a new web... | 1 | 0 |
| `buildCalleesTree` | buildCalleesTree recursivel... | 1 | 2 |
| `buildCallersTree` | buildCallersTree recursivel... | 1 | 2 |
| `handleCallChain` | handleCallChain returns hie... | 0 | 5 |
| `handleGraph` | handleGraph returns the com... | 0 | 4 |
| `handleImpact` | handleImpact returns impact... | 0 | 6 |
| `handleNode` | handleNode returns a single... | 0 | 6 |
| `handleNodes` | handleNodes returns all nodes | 0 | 3 |
| `handleSearch` | handleSearch searches for n... | 0 | 3 |
| `handleStats` | handleStats returns databas... | 0 | 2 |
| `getPackageGroup` | - | 1 | 0 |
| `nodeToData` | Helper functions | 8 | 2 |
| `nodesToData` | - | 3 | 1 |
| `shortName` | - | 1 | 0 |
| `writeJSON` | - | 7 | 0 |

#### `Run`

- **位置**: `internal/web/server.go:81`
- **签名**: `func() error`
- **说明**: Run starts the web server
- **被调用**: `serveCmd`

#### `NewServer`

- **位置**: `internal/web/server.go:27`
- **签名**: `func(db *github.com/zheng/crag/internal/storage.DB, port int) *github.com/zheng/crag/internal/web.Server`
- **说明**: NewServer creates a new web server
- **被调用**: `serveCmd`

### 📦 internal/watcher

| 函数 | 说明 | 被调用 | 调用 |
|------|------|--------|------|
| `Start` | Start begins watching for c... | 1 | 1 |
| `Stop` | Stop stops the watcher | 1 | 0 |
| `New` | New creates a new Watcher | 1 | 5 |
| `WithDebounceDelay` | WithDebounceDelay sets the ... | 2 | 0 |
| `WithOnAnalysisDone` | WithOnAnalysisDone sets the... | 2 | 0 |
| `WithOnAnalysisStart` | WithOnAnalysisStart sets th... | 2 | 0 |
| `WithOnError` | WithOnError sets the callba... | 2 | 0 |
| `addDirs` | addDirs recursively adds al... | 1 | 0 |
| `eventLoop` | eventLoop handles file syst... | 1 | 2 |
| `handleEvent` | handleEvent processes a sin... | 1 | 0 |
| `runAnalysis` | runAnalysis performs the ac... | 1 | 10 |
| `triggerAnalysis` | triggerAnalysis runs the an... | 0 | 2 |

#### `Start`

- **位置**: `internal/watcher/watcher.go:119`
- **签名**: `func()`
- **说明**: Start begins watching for changes
- **被调用**: `watchCmd`
- **调用**: `eventLoop`

#### `Stop`

- **位置**: `internal/watcher/watcher.go:124`
- **签名**: `func() error`
- **说明**: Stop stops the watcher
- **被调用**: `watchCmd`

#### `New`

- **位置**: `internal/watcher/watcher.go:70`
- **签名**: `func(projectPath string, dbPath string, opts ...github.com/zheng/crag/internal/watcher.WatcherOption) (*github.com/zheng/crag/internal/watcher.Watcher, error)`
- **说明**: New creates a new Watcher
- **被调用**: `watchCmd`
- **调用**: `WithOnAnalysisStart`, `WithDebounceDelay`, `WithOnError`, `WithOnAnalysisDone`, `addDirs`

#### `WithDebounceDelay`

- **位置**: `internal/watcher/watcher.go:42`
- **签名**: `func(d time.Duration) github.com/zheng/crag/internal/watcher.WatcherOption`
- **说明**: WithDebounceDelay sets the debounce delay
- **被调用**: `watchCmd`, `New`

#### `WithOnAnalysisDone`

- **位置**: `internal/watcher/watcher.go:56`
- **签名**: `func(fn func(nodeCount int64, edgeCount int64, duration time.Duration)) github.com/zheng/crag/internal/watcher.WatcherOption`
- **说明**: WithOnAnalysisDone sets the callback for when analysis completes
- **被调用**: `watchCmd`, `New`

#### `WithOnAnalysisStart`

- **位置**: `internal/watcher/watcher.go:49`
- **签名**: `func(fn func()) github.com/zheng/crag/internal/watcher.WatcherOption`
- **说明**: WithOnAnalysisStart sets the callback for when analysis starts
- **被调用**: `watchCmd`, `New`

#### `WithOnError`

- **位置**: `internal/watcher/watcher.go:63`
- **签名**: `func(fn func(error)) github.com/zheng/crag/internal/watcher.WatcherOption`
- **说明**: WithOnError sets the callback for errors
- **被调用**: `watchCmd`, `New`

---

## 修改影响速查

| 函数 | 位置 | 被调用次数 | 调用次数 | 风险 |
|------|------|-----------|----------|------|
| `Close` | internal/storage/db.go:41 | 11 | 0 | 🔴 高 |
| `Open` | internal/storage/db.go:19 | 11 | 0 | 🔴 高 |
| `FindNodesByPattern` | internal/storage/queries.go:51 | 8 | 1 | 🔴 高 |
| `nodeToData` | internal/web/server.go:351 | 8 | 2 | 🔴 高 |
| `getShortDisplayName` | internal/export/exporter.go:452 | 7 | 0 | 🔴 高 |
| `scanNodes` | internal/storage/queries.go:376 | 7 | 0 | 🔴 高 |
| `GetDirectCallees` | internal/storage/queries.go:80 | 7 | 1 | 🔴 高 |
| `GetDirectCallers` | internal/storage/queries.go:64 | 7 | 1 | 🔴 高 |
| `writeJSON` | internal/web/server.go:427 | 7 | 0 | 🔴 高 |
| `GetAllFunctions` | internal/storage/queries.go:227 | 6 | 1 | 🔴 高 |
| `GetStats` | internal/storage/queries.go:338 | 5 | 0 | 🔴 高 |
| `AnalyzeImpact` | internal/impact/analyzer.go:31 | 4 | 6 | 🟡 中 |
| `GetUpstreamCallers` | internal/storage/queries.go:97 | 4 | 1 | 🟡 中 |
| `NewAnalyzer` | internal/impact/analyzer.go:17 | 4 | 0 | 🟡 中 |
| `GetDownstreamCallees` | internal/storage/queries.go:147 | 4 | 1 | 🟡 中 |
| `FilterMainPackages` | internal/analyzer/loader.go:47 | 3 | 0 | 🟡 中 |
| `Clear` | internal/storage/db.go:46 | 3 | 0 | 🟡 中 |
| `sendResult` | internal/mcp/server.go:659 | 3 | 1 | 🟡 中 |
| `Build` | internal/graph/builder.go:89 | 3 | 5 | 🟡 中 |
| `sendError` | internal/mcp/server.go:668 | 3 | 1 | 🟡 中 |
| `nodesToData` | internal/web/server.go:365 | 3 | 1 | 🟡 中 |
| `BuildCallGraph` | internal/analyzer/callgraph.go:12 | 3 | 0 | 🟡 中 |
| `LoadPackages` | internal/analyzer/loader.go:10 | 3 | 0 | 🟡 中 |
| `GetNodeByID` | internal/storage/queries.go:42 | 3 | 1 | 🟡 中 |
| `NewBuilder` | internal/graph/builder.go:26 | 3 | 0 | 🟡 中 |
| `getRelativePath` | internal/export/exporter.go:505 | 3 | 0 | 🟡 中 |
| `outputJSON` | main.go:433 | 3 | 0 | 🟡 中 |
| `BuildSSA` | internal/analyzer/ssa.go:10 | 3 | 0 | 🟡 中 |
| `watchCmd` | main.go:530 | 3 | 8 | 🟡 中 |
| `WithDebounceDelay` | internal/watcher/watcher.go:42 | 2 | 0 | 🟢 |
| `isKeyFunction` | internal/export/exporter.go:516 | 2 | 1 | 🟢 |
| `matchPackageSuffix` | internal/export/exporter.go:571 | 2 | 0 | 🟢 |
| `send` | internal/mcp/server.go:677 | 2 | 0 | 🟢 |
| `joinStrings` | internal/storage/queries.go:347 | 2 | 0 | 🟢 |
| `GetGitChanges` | internal/analyzer/git.go:21 | 2 | 0 | 🟢 |
| `WithOnAnalysisStart` | internal/watcher/watcher.go:49 | 2 | 0 | 🟢 |
| `WithOnError` | internal/watcher/watcher.go:63 | 2 | 0 | 🟢 |
| `WithOnAnalysisDone` | internal/watcher/watcher.go:56 | 2 | 0 | 🟢 |
| `scanNode` | internal/storage/queries.go:360 | 2 | 0 | 🟢 |
| `FormatMarkdown` | internal/impact/analyzer.go:109 | 2 | 0 | 🟢 |
| `HasChanges` | internal/analyzer/git.go:85 | 2 | 0 | 🟢 |
| `shortName` | internal/mcp/server.go:587 | 2 | 2 | 🟢 |
| `Summary` | internal/impact/analyzer.go:175 | 1 | 0 | 🟢 |
| `getDocComment` | internal/graph/builder.go:230 | 1 | 0 | 🟢 |
| `downstreamCmd` | main.go:222 | 1 | 5 | 🟢 |
| `Run` | internal/mcp/server.go:113 | 1 | 2 | 🟢 |
| `isClosure` | internal/graph/builder.go:62 | 1 | 0 | 🟢 |
| `isExportedFunc` | internal/export/exporter.go:540 | 1 | 1 | 🟢 |
| `DefaultExportOptions` | internal/export/exporter.go:34 | 1 | 0 | 🟢 |
| `GetNodeByName` | internal/storage/queries.go:33 | 1 | 1 | 🟢 |
| `New` | internal/watcher/watcher.go:70 | 1 | 5 | 🟢 |
| `handleToolsList` | internal/mcp/server.go:165 | 1 | 1 | 🟢 |
| `makeNodeID` | internal/export/exporter.go:494 | 1 | 1 | 🟢 |
| `searchCmd` | main.go:397 | 1 | 3 | 🟢 |
| `toolMermaid` | internal/mcp/server.go:460 | 1 | 7 | 🟢 |
| `Run` | internal/web/server.go:81 | 1 | 0 | 🟢 |
| `nodeID` | internal/mcp/server.go:627 | 1 | 1 | 🟢 |
| `mcpCmd` | main.go:503 | 1 | 4 | 🟢 |
| `writePackageSection` | internal/export/exporter.go:170 | 1 | 8 | 🟢 |
| `toolUpstream` | internal/mcp/server.go:320 | 1 | 2 | 🟢 |
| `handleEvent` | internal/watcher/watcher.go:154 | 1 | 0 | 🟢 |
| `getPackageGroup` | internal/web/server.go:406 | 1 | 0 | 🟢 |
| `NewServer` | internal/mcp/server.go:22 | 1 | 0 | 🟢 |
| `toolDownstream` | internal/mcp/server.go:360 | 1 | 2 | 🟢 |
| `impactCmd` | main.go:285 | 1 | 8 | 🟢 |
| `ExportIncremental` | internal/export/exporter.go:297 | 1 | 5 | 🟢 |
| `addDirs` | internal/watcher/watcher.go:99 | 1 | 0 | 🟢 |
| `handleRequest` | internal/mcp/server.go:136 | 1 | 4 | 🟢 |
| `analyzeCmd` | main.go:56 | 1 | 13 | 🟢 |
| `Start` | internal/watcher/watcher.go:119 | 1 | 1 | 🟢 |
| `Export` | internal/export/exporter.go:44 | 1 | 8 | 🟢 |
| `categorizePackages` | internal/export/exporter.go:364 | 1 | 0 | 🟢 |
| `runInitialAnalysis` | main.go:608 | 1 | 10 | 🟢 |
| `getLayerOrder` | internal/export/exporter.go:408 | 1 | 0 | 🟢 |
| `runAnalysis` | internal/watcher/watcher.go:227 | 1 | 10 | 🟢 |
| `listCmd` | main.go:358 | 1 | 3 | 🟢 |
| `handleInitialize` | internal/mcp/server.go:151 | 1 | 1 | 🟢 |
| `GetNodeCount` | internal/graph/builder.go:256 | 1 | 0 | 🟢 |
| `isProjectFunction` | internal/graph/builder.go:52 | 1 | 0 | 🟢 |
| `NewServer` | internal/web/server.go:27 | 1 | 0 | 🟢 |
| `shortName` | internal/web/server.go:373 | 1 | 0 | 🟢 |
| `GetAllEdges` | internal/storage/queries.go:240 | 1 | 0 | 🟢 |
| `writeImpactTable` | internal/export/exporter.go:252 | 1 | 4 | 🟢 |
| `getParentFunctionName` | internal/graph/builder.go:69 | 1 | 0 | 🟢 |
| `exportCmd` | main.go:439 | 1 | 8 | 🟢 |
| `serveCmd` | main.go:658 | 1 | 4 | 🟢 |
| `writeArchitectureDiagram` | internal/export/exporter.go:117 | 1 | 6 | 🟢 |
| `NewExporter` | internal/export/exporter.go:21 | 1 | 0 | 🟢 |
| `toolList` | internal/mcp/server.go:425 | 1 | 1 | 🟢 |
| `truncateDoc` | internal/export/exporter.go:559 | 1 | 0 | 🟢 |
| `toolSearch` | internal/mcp/server.go:400 | 1 | 1 | 🟢 |
| `buildCalleesTree` | internal/web/server.go:292 | 1 | 2 | 🟢 |
| `getShortPackageName` | internal/export/exporter.go:444 | 1 | 0 | 🟢 |
| `resolveToParent` | internal/graph/builder.go:79 | 1 | 0 | 🟢 |
| `groupByPackage` | internal/export/exporter.go:388 | 1 | 0 | 🟢 |
| `writeProjectStructure` | internal/export/exporter.go:82 | 1 | 0 | 🟢 |
| `lastIndex` | internal/mcp/server.go:641 | 1 | 0 | 🟢 |
| `buildCallersTree` | internal/web/server.go:259 | 1 | 2 | 🟢 |
| `getSortedPackageNames` | internal/export/exporter.go:396 | 1 | 1 | 🟢 |
| `Stop` | internal/watcher/watcher.go:124 | 1 | 0 | 🟢 |
| `eventLoop` | internal/watcher/watcher.go:130 | 1 | 2 | 🟢 |
| `getLayerDisplayName` | internal/export/exporter.go:425 | 1 | 0 | 🟢 |
| `upstreamCmd` | main.go:159 | 1 | 5 | 🟢 |
| `toolImpact` | internal/mcp/server.go:305 | 1 | 3 | 🟢 |
| `handleToolsCall` | internal/mcp/server.go:271 | 1 | 8 | 🟢 |
| `indexOf` | internal/mcp/server.go:650 | 1 | 0 | 🟢 |
| `createFunctionNode` | internal/graph/builder.go:198 | 1 | 1 | 🟢 |
