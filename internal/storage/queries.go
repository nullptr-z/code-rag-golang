package storage

import (
	"database/sql"

	"github.com/zheng/crag/internal/graph"
)

// InsertNode inserts a node into the database and returns its ID
func (db *DB) InsertNode(node *graph.Node) (int64, error) {
	result, err := db.conn.Exec(
		`INSERT INTO nodes (kind, name, package, file, line, signature, doc)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		node.Kind, node.Name, node.Package, node.File, node.Line, node.Signature, node.Doc,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// InsertEdge inserts an edge into the database
func (db *DB) InsertEdge(edge *graph.Edge) error {
	_, err := db.conn.Exec(
		`INSERT INTO edges (from_id, to_id, kind, call_site_file, call_site_line)
		 VALUES (?, ?, ?, ?, ?)`,
		edge.FromID, edge.ToID, edge.Kind, edge.CallSiteFile, edge.CallSiteLine,
	)
	return err
}

// GetNodeByName returns a node by its fully qualified name
func (db *DB) GetNodeByName(name string) (*graph.Node, error) {
	row := db.conn.QueryRow(
		`SELECT id, kind, name, package, file, line, signature, doc FROM nodes WHERE name = ?`,
		name,
	)
	return scanNode(row)
}

// GetNodeByID returns a node by its ID
func (db *DB) GetNodeByID(id int64) (*graph.Node, error) {
	row := db.conn.QueryRow(
		`SELECT id, kind, name, package, file, line, signature, doc FROM nodes WHERE id = ?`,
		id,
	)
	return scanNode(row)
}

// FindNodesByPattern returns nodes matching a name pattern (using LIKE)
// Results are sorted by match quality: exact short name match > ends with pattern > contains pattern
func (db *DB) FindNodesByPattern(pattern string) ([]*graph.Node, error) {
	// Use a query that sorts by match quality:
	// 1. Exact match on short name (after last dot or after ").")
	// 2. Name ends with the pattern (e.g., "pkg.FuncName" matches "FuncName")
	// 3. Name contains the pattern anywhere
	rows, err := db.conn.Query(
		`SELECT id, kind, name, package, file, line, signature, doc FROM nodes
		 WHERE name LIKE ?
		 ORDER BY
			CASE
				-- Exact match on short name: name ends with ".pattern" or ").pattern"
				WHEN name LIKE '%.' || ? OR name LIKE '%).' || ? THEN 0
				-- Name ends with the pattern
				WHEN name LIKE '%' || ? THEN 1
				-- Contains pattern
				ELSE 2
			END,
			length(name) ASC`,
		"%"+pattern+"%", pattern, pattern, pattern,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetDirectCallers returns functions that directly call the given function
func (db *DB) GetDirectCallers(nodeID int64) ([]*graph.Node, error) {
	rows, err := db.conn.Query(
		`SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc
		 FROM nodes n
		 JOIN edges e ON e.from_id = n.id
		 WHERE e.to_id = ? AND e.kind = 'calls'`,
		nodeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetDirectCallees returns functions that the given function directly calls
func (db *DB) GetDirectCallees(nodeID int64) ([]*graph.Node, error) {
	rows, err := db.conn.Query(
		`SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc
		 FROM nodes n
		 JOIN edges e ON e.to_id = n.id
		 WHERE e.from_id = ? AND e.kind = 'calls'`,
		nodeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetUpstreamCallers returns all upstream callers recursively up to maxDepth
// If maxDepth is 0, it returns all callers with no depth limit
func (db *DB) GetUpstreamCallers(nodeID int64, maxDepth int) ([]*graph.Node, error) {
	var query string
	var args []interface{}

	if maxDepth == 0 {
		// No depth limit
		query = `
		WITH RECURSIVE callers(id, kind, name, package, file, line, signature, doc, depth) AS (
			SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc, 1
			FROM nodes n
			JOIN edges e ON e.from_id = n.id
			WHERE e.to_id = ? AND e.kind = 'calls'
			UNION
			SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc, c.depth + 1
			FROM nodes n
			JOIN edges e ON e.from_id = n.id
			JOIN callers c ON e.to_id = c.id
			WHERE e.kind = 'calls'
		)
		SELECT DISTINCT id, kind, name, package, file, line, signature, doc FROM callers`
		args = []interface{}{nodeID}
	} else {
		// With depth limit
		query = `
		WITH RECURSIVE callers(id, kind, name, package, file, line, signature, doc, depth) AS (
			SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc, 1
			FROM nodes n
			JOIN edges e ON e.from_id = n.id
			WHERE e.to_id = ? AND e.kind = 'calls'
			UNION
			SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc, c.depth + 1
			FROM nodes n
			JOIN edges e ON e.from_id = n.id
			JOIN callers c ON e.to_id = c.id
			WHERE e.kind = 'calls' AND c.depth < ?
		)
		SELECT DISTINCT id, kind, name, package, file, line, signature, doc FROM callers`
		args = []interface{}{nodeID, maxDepth}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetDownstreamCallees returns all downstream callees recursively up to maxDepth
// If maxDepth is 0, it returns all callees with no depth limit
func (db *DB) GetDownstreamCallees(nodeID int64, maxDepth int) ([]*graph.Node, error) {
	var query string
	var args []interface{}

	if maxDepth == 0 {
		// No depth limit
		query = `
		WITH RECURSIVE callees(id, kind, name, package, file, line, signature, doc, depth) AS (
			SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc, 1
			FROM nodes n
			JOIN edges e ON e.to_id = n.id
			WHERE e.from_id = ? AND e.kind = 'calls'
			UNION
			SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc, c.depth + 1
			FROM nodes n
			JOIN edges e ON e.to_id = n.id
			JOIN callees c ON e.from_id = c.id
			WHERE e.kind = 'calls'
		)
		SELECT DISTINCT id, kind, name, package, file, line, signature, doc FROM callees`
		args = []interface{}{nodeID}
	} else {
		// With depth limit
		query = `
		WITH RECURSIVE callees(id, kind, name, package, file, line, signature, doc, depth) AS (
			SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc, 1
			FROM nodes n
			JOIN edges e ON e.to_id = n.id
			WHERE e.from_id = ? AND e.kind = 'calls'
			UNION
			SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc, c.depth + 1
			FROM nodes n
			JOIN edges e ON e.to_id = n.id
			JOIN callees c ON e.from_id = c.id
			WHERE e.kind = 'calls' AND c.depth < ?
		)
		SELECT DISTINCT id, kind, name, package, file, line, signature, doc FROM callees`
		args = []interface{}{nodeID, maxDepth}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetCallEdgesForNode returns all call edges where the node is the caller
func (db *DB) GetCallEdgesForNode(nodeID int64) ([]*graph.Edge, error) {
	rows, err := db.conn.Query(
		`SELECT id, from_id, to_id, kind, call_site_file, call_site_line
		 FROM edges WHERE from_id = ? AND kind = 'calls'`,
		nodeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []*graph.Edge
	for rows.Next() {
		var e graph.Edge
		var callSiteFile sql.NullString
		var callSiteLine sql.NullInt64
		if err := rows.Scan(&e.ID, &e.FromID, &e.ToID, &e.Kind, &callSiteFile, &callSiteLine); err != nil {
			return nil, err
		}
		if callSiteFile.Valid {
			e.CallSiteFile = callSiteFile.String
		}
		if callSiteLine.Valid {
			e.CallSiteLine = int(callSiteLine.Int64)
		}
		edges = append(edges, &e)
	}
	return edges, rows.Err()
}

// GetAllFunctions returns all function nodes
func (db *DB) GetAllFunctions() ([]*graph.Node, error) {
	rows, err := db.conn.Query(
		`SELECT id, kind, name, package, file, line, signature, doc FROM nodes WHERE kind = 'func'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}


// GetAllEdges returns all edges in the database
func (db *DB) GetAllEdges() ([]*graph.Edge, error) {
	rows, err := db.conn.Query(
		`SELECT id, from_id, to_id, kind, call_site_file, call_site_line FROM edges`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []*graph.Edge
	for rows.Next() {
		var e graph.Edge
		var callSiteFile sql.NullString
		var callSiteLine sql.NullInt64
		if err := rows.Scan(&e.ID, &e.FromID, &e.ToID, &e.Kind, &callSiteFile, &callSiteLine); err != nil {
			return nil, err
		}
		if callSiteFile.Valid {
			e.CallSiteFile = callSiteFile.String
		}
		if callSiteLine.Valid {
			e.CallSiteLine = int(callSiteLine.Int64)
		}
		edges = append(edges, &e)
	}
	return edges, rows.Err()
}

// DeleteNodesByPackage deletes all nodes belonging to the specified packages
// Also deletes all edges referencing those nodes
// Returns the number of deleted nodes
func (db *DB) DeleteNodesByPackage(packages []string) (int64, error) {
	if len(packages) == 0 {
		return 0, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(packages))
	args := make([]interface{}, len(packages))
	for i, pkg := range packages {
		placeholders[i] = "?"
		args[i] = pkg
	}

	// First, delete edges that reference nodes in these packages
	edgeQuery := `DELETE FROM edges WHERE from_id IN (SELECT id FROM nodes WHERE package IN (` + joinStrings(placeholders, ",") + `)) OR to_id IN (SELECT id FROM nodes WHERE package IN (` + joinStrings(placeholders, ",") + `))`
	// Need to duplicate args for the two IN clauses
	edgeArgs := append(args, args...)
	_, err := db.conn.Exec(edgeQuery, edgeArgs...)
	if err != nil {
		return 0, err
	}

	// Then delete the nodes
	nodeQuery := `DELETE FROM nodes WHERE package IN (` + joinStrings(placeholders, ",") + `)`
	result, err := db.conn.Exec(nodeQuery, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteOrphanEdges deletes edges that reference non-existent nodes
func (db *DB) DeleteOrphanEdges() (int64, error) {
	result, err := db.conn.Exec(`
		DELETE FROM edges
		WHERE from_id NOT IN (SELECT id FROM nodes)
		   OR to_id NOT IN (SELECT id FROM nodes)
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetNodesByPackage returns all nodes in the specified packages
func (db *DB) GetNodesByPackage(packages []string) ([]*graph.Node, error) {
	if len(packages) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(packages))
	args := make([]interface{}, len(packages))
	for i, pkg := range packages {
		placeholders[i] = "?"
		args[i] = pkg
	}

	query := `SELECT id, kind, name, package, file, line, signature, doc FROM nodes WHERE package IN (` + joinStrings(placeholders, ",") + `)`
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetStats returns database statistics
func (db *DB) GetStats() (nodeCount, edgeCount int64, err error) {
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&nodeCount)
	if err != nil {
		return
	}
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM edges`).Scan(&edgeCount)
	return
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}

// Helper functions

func scanNode(row *sql.Row) (*graph.Node, error) {
	var n graph.Node
	var signature, doc sql.NullString
	err := row.Scan(&n.ID, &n.Kind, &n.Name, &n.Package, &n.File, &n.Line, &signature, &doc)
	if err != nil {
		return nil, err
	}
	if signature.Valid {
		n.Signature = signature.String
	}
	if doc.Valid {
		n.Doc = doc.String
	}
	return &n, nil
}

func scanNodes(rows *sql.Rows) ([]*graph.Node, error) {
	var nodes []*graph.Node
	for rows.Next() {
		var n graph.Node
		var signature, doc sql.NullString
		if err := rows.Scan(&n.ID, &n.Kind, &n.Name, &n.Package, &n.File, &n.Line, &signature, &doc); err != nil {
			return nil, err
		}
		if signature.Valid {
			n.Signature = signature.String
		}
		if doc.Valid {
			n.Doc = doc.String
		}
		nodes = append(nodes, &n)
	}
	return nodes, rows.Err()
}

// CallTreeNode represents a node in the call tree with its children
type CallTreeNode struct {
	Node     *graph.Node
	Children []*CallTreeNode
}

// GetUpstreamCallTree builds a tree of upstream callers
func (db *DB) GetUpstreamCallTree(nodeID int64, maxDepth int) ([]*CallTreeNode, error) {
	// Get direct callers
	callers, err := db.GetDirectCallers(nodeID)
	if err != nil {
		return nil, err
	}

	if maxDepth == 1 || len(callers) == 0 {
		// Convert to tree nodes without children
		result := make([]*CallTreeNode, len(callers))
		for i, c := range callers {
			result[i] = &CallTreeNode{Node: c}
		}
		return result, nil
	}

	// Recursively build tree
	result := make([]*CallTreeNode, len(callers))
	for i, c := range callers {
		children, err := db.GetUpstreamCallTree(c.ID, maxDepth-1)
		if err != nil {
			return nil, err
		}
		result[i] = &CallTreeNode{
			Node:     c,
			Children: children,
		}
	}
	return result, nil
}

// GetDownstreamCallTree builds a tree of downstream callees
func (db *DB) GetDownstreamCallTree(nodeID int64, maxDepth int) ([]*CallTreeNode, error) {
	// Get direct callees
	callees, err := db.GetDirectCallees(nodeID)
	if err != nil {
		return nil, err
	}

	if maxDepth == 1 || len(callees) == 0 {
		// Convert to tree nodes without children
		result := make([]*CallTreeNode, len(callees))
		for i, c := range callees {
			result[i] = &CallTreeNode{Node: c}
		}
		return result, nil
	}

	// Recursively build tree
	result := make([]*CallTreeNode, len(callees))
	for i, c := range callees {
		children, err := db.GetDownstreamCallTree(c.ID, maxDepth-1)
		if err != nil {
			return nil, err
		}
		result[i] = &CallTreeNode{
			Node:     c,
			Children: children,
		}
	}
	return result, nil
}

// ==================== Interface Queries ====================

// GetAllInterfaces returns all interface nodes
func (db *DB) GetAllInterfaces() ([]*graph.Node, error) {
	rows, err := db.conn.Query(
		`SELECT id, kind, name, package, file, line, signature, doc FROM nodes WHERE kind = 'interface'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// FindInterfacesByPattern returns interfaces matching a name pattern
func (db *DB) FindInterfacesByPattern(pattern string) ([]*graph.Node, error) {
	rows, err := db.conn.Query(
		`SELECT id, kind, name, package, file, line, signature, doc FROM nodes
		 WHERE kind = 'interface' AND name LIKE ?
		 ORDER BY length(name) ASC`,
		"%"+pattern+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetImplementations returns all types that implement a given interface
func (db *DB) GetImplementations(interfaceID int64) ([]*graph.Node, error) {
	rows, err := db.conn.Query(
		`SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc
		 FROM nodes n
		 JOIN edges e ON e.from_id = n.id
		 WHERE e.to_id = ? AND e.kind = 'implements'`,
		interfaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetImplementedInterfaces returns all interfaces that a type implements
func (db *DB) GetImplementedInterfaces(typeID int64) ([]*graph.Node, error) {
	rows, err := db.conn.Query(
		`SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc
		 FROM nodes n
		 JOIN edges e ON e.to_id = n.id
		 WHERE e.from_id = ? AND e.kind = 'implements'`,
		typeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetAllTypes returns all struct/type nodes
func (db *DB) GetAllTypes() ([]*graph.Node, error) {
	rows, err := db.conn.Query(
		`SELECT id, kind, name, package, file, line, signature, doc FROM nodes WHERE kind = 'struct'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// ==================== Risk Score Queries ====================

// RiskScore represents the change risk assessment for a function
type RiskScore struct {
	Node          *graph.Node
	DirectCallers int    // Number of direct callers
	TotalCallers  int    // Total upstream callers (recursive)
	MaxDepth      int    // Maximum call chain depth to entry points
	RiskLevel     string // low, medium, high, critical
}

// GetDirectCallerCount returns the number of direct callers for a node
func (db *DB) GetDirectCallerCount(nodeID int64) (int, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM edges WHERE to_id = ? AND kind = 'calls'`,
		nodeID,
	).Scan(&count)
	return count, err
}

// GetTotalCallerCount returns the total number of upstream callers (recursive)
// Limits depth to 50 to prevent infinite loops in circular call graphs
func (db *DB) GetTotalCallerCount(nodeID int64) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		WITH RECURSIVE callers(id, depth) AS (
			SELECT from_id, 1 FROM edges WHERE to_id = ? AND kind = 'calls'
			UNION
			SELECT e.from_id, c.depth + 1
			FROM edges e
			JOIN callers c ON e.to_id = c.id
			WHERE e.kind = 'calls' AND c.depth < 50
		)
		SELECT COUNT(DISTINCT id) FROM callers
	`, nodeID).Scan(&count)
	return count, err
}

// GetMaxCallDepth returns the maximum depth to reach entry points (functions with no callers)
func (db *DB) GetMaxCallDepth(nodeID int64) (int, error) {
	var maxDepth int
	err := db.conn.QueryRow(`
		WITH RECURSIVE call_chain(id, depth) AS (
			SELECT from_id, 1 FROM edges WHERE to_id = ? AND kind = 'calls'
			UNION ALL
			SELECT e.from_id, cc.depth + 1
			FROM edges e
			JOIN call_chain cc ON e.to_id = cc.id
			WHERE e.kind = 'calls' AND cc.depth < 50
		)
		SELECT COALESCE(MAX(depth), 0) FROM call_chain
	`, nodeID).Scan(&maxDepth)
	return maxDepth, err
}

// CalculateRiskLevel determines risk level based on caller metrics
func CalculateRiskLevel(directCallers, totalCallers int) string {
	// Primary factor: direct callers
	// Secondary factor: total impact
	if directCallers >= 50 || totalCallers >= 200 {
		return "critical"
	}
	if directCallers >= 20 || totalCallers >= 100 {
		return "high"
	}
	if directCallers >= 5 || totalCallers >= 30 {
		return "medium"
	}
	return "low"
}

// GetRiskScore calculates the risk score for a function
// Uses only direct callers for fast performance (recursive queries are too slow for hot functions)
func (db *DB) GetRiskScore(nodeID int64) (*RiskScore, error) {
	node, err := db.GetNodeByID(nodeID)
	if err != nil {
		return nil, err
	}

	directCallers, err := db.GetDirectCallerCount(nodeID)
	if err != nil {
		return nil, err
	}

	// Skip expensive recursive queries - use direct callers only for risk assessment
	// For functions with hundreds of callers, recursive queries can take minutes
	return &RiskScore{
		Node:          node,
		DirectCallers: directCallers,
		TotalCallers:  directCallers, // Use direct as estimate
		MaxDepth:      0,             // Skip depth calculation
		RiskLevel:     CalculateRiskLevelFast(directCallers),
	}, nil
}

// GetTopRiskyFunctions returns functions with most callers (highest risk)
// For performance, only uses direct caller count (skips expensive recursive queries)
func (db *DB) GetTopRiskyFunctions(limit int) ([]*RiskScore, error) {
	rows, err := db.conn.Query(`
		SELECT n.id, n.kind, n.name, n.package, n.file, n.line, n.signature, n.doc,
		       COUNT(e.from_id) as caller_count
		FROM nodes n
		LEFT JOIN edges e ON e.to_id = n.id AND e.kind = 'calls'
		WHERE n.kind = 'func'
		GROUP BY n.id
		ORDER BY caller_count DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*RiskScore
	for rows.Next() {
		var n graph.Node
		var signature, doc sql.NullString
		var directCallers int
		if err := rows.Scan(&n.ID, &n.Kind, &n.Name, &n.Package, &n.File, &n.Line, &signature, &doc, &directCallers); err != nil {
			return nil, err
		}
		if signature.Valid {
			n.Signature = signature.String
		}
		if doc.Valid {
			n.Doc = doc.String
		}

		// For list view, use direct callers only (fast)
		// Total callers calculated only for single function analysis
		results = append(results, &RiskScore{
			Node:          &n,
			DirectCallers: directCallers,
			TotalCallers:  directCallers, // Use direct as estimate for list
			MaxDepth:      0,
			RiskLevel:     CalculateRiskLevelFast(directCallers),
		})
	}
	return results, rows.Err()
}

// CalculateRiskLevelFast determines risk level based on direct callers only (for list view)
func CalculateRiskLevelFast(directCallers int) string {
	if directCallers >= 50 {
		return "critical"
	}
	if directCallers >= 20 {
		return "high"
	}
	if directCallers >= 5 {
		return "medium"
	}
	return "low"
}
