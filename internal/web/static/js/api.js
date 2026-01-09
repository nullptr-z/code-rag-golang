/**
 * API functions for data fetching
 */

// Load graph data from API
async function loadGraph() {
  try {
    const [graphData, stats] = await Promise.all([
      fetch('/api/graph').then(r => r.json()),
      fetch('/api/stats').then(r => r.json())
    ]);

    document.getElementById('node-count').textContent = stats.nodeCount;
    document.getElementById('edge-count').textContent = stats.edgeCount;
    document.getElementById('visible-count').textContent = '-';

    allNodes = graphData.nodes;
    allEdges = graphData.edges;

    // Build node id map for mermaid (safe identifiers)
    allNodes.forEach((node, idx) => {
      nodeIdMap[node.id] = `n${node.id}`;
    });

    renderNodeList(allNodes);
    document.getElementById('loading').style.display = 'none';

    // 默认选中 main 函数
    const mainNode = allNodes.find(n => n.label === 'main' || n.fullName?.endsWith('.main'));
    if (mainNode) {
      selectNode(mainNode.id);
    }
  } catch (err) {
    console.error('Failed to load graph:', err);
    document.getElementById('loading').innerHTML = `
      <div style="color: #f85149;">加载失败: ${err.message}</div>
    `;
  }
}

// Get neighbors for a node (used by path finder)
async function getNeighbors(nodeId, direction) {
  try {
    const response = await fetch(`/api/node/${nodeId}`);
    const data = await response.json();
    // downstream: 获取 callees (A 调用了谁)
    // upstream: 获取 callers (谁调用了 A)
    return direction === 'downstream' ? (data.callees || []) : (data.callers || []);
  } catch (err) {
    return [];
  }
}

// Fetch call chain data for a node
async function fetchCallChain(nodeId, depth) {
  return fetch(`/api/chain/${nodeId}?depth=${depth}`).then(r => r.json());
}
