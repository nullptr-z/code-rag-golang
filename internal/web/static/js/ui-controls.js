/**
 * UI controls and interactions
 */

// Render node list in sidebar
function renderNodeList(nodes) {
  const container = document.getElementById('node-list');
  const groups = {};

  nodes.forEach(node => {
    const group = node.group || 'other';
    if (!groups[group]) groups[group] = [];
    groups[group].push(node);
  });

  let html = '';
  Object.entries(groups).sort((a, b) => b[1].length - a[1].length).forEach(([group, groupNodes]) => {
    const sortedNodes = groupNodes.sort((a, b) => a.label.localeCompare(b.label));
    html += `
      <div class="node-group">
        <div class="node-group-header" onclick="toggleGroup(this)">
          <span class="toggle">▼</span>
          <div class="node-dot color-${group}"></div>
          ${group} (${sortedNodes.length})
        </div>
        <div class="node-items">
          ${sortedNodes.map(n => `
            <div class="node-item" data-id="${n.id}" onclick="selectNode(${n.id})">
              <div class="node-dot color-${n.group}"></div>
              <span class="node-name" title="${n.fullName}">${n.label}</span>
            </div>
          `).join('')}
        </div>
      </div>
    `;
  });

  container.innerHTML = html;
}

function toggleGroup(header) {
  header.parentElement.classList.toggle('collapsed');
}

// Set view mode
function setViewMode(mode) {
  currentViewMode = mode;

  // Update buttons
  document.getElementById('btn-tree').classList.toggle('active', mode === 'tree');
  document.getElementById('btn-mermaid').classList.toggle('active', mode === 'mermaid');

  // Show/hide mermaid tools
  document.getElementById('mermaid-tools').classList.toggle('hidden', mode === 'tree');

  // Re-render if we have data
  if (currentNodeData) {
    renderCurrentView();
  }
}

// Update depth and re-render
function updateDepth(value) {
  const newDepth = Math.max(1, Math.min(5, parseInt(value) || 1));
  currentDepth = newDepth;
  document.getElementById('depth-input').value = newDepth;

  // Re-fetch and render if we have a selected node
  if (selectedNodeId) {
    selectNode(selectedNodeId);
  }
}

// Select a node and render view
async function selectNode(nodeId) {
  selectedNodeId = nodeId;
  // 清空终端节点记录（切换节点时重置状态）
  terminalNodes.clear();
  // 退出路径模式（选择新函数时应显示该函数的调用图）
  isPathMode = false;
  currentPathData = null;
  document.getElementById('target-func-input').value = '';

  // Update sidebar selection
  document.querySelectorAll('.node-item').forEach(el => {
    el.classList.toggle('selected', el.dataset.id == nodeId);
  });

  // Load node details with hierarchical call chain data
  try {
    const chainData = await fetchCallChain(nodeId, currentDepth);

    // Use hierarchical data structure
    currentNodeData = {
      node: chainData.target,
      callers: chainData.callers || [],
      callees: chainData.callees || []
    };

    showNodeDetail(currentNodeData);
    renderCurrentView();
  } catch (err) {
    console.error('Failed to load node details:', err);
  }
}

// Render current view based on mode
function renderCurrentView() {
  if (!currentNodeData) return;

  document.getElementById('empty-state').style.display = 'none';

  if (currentViewMode === 'mermaid') {
    document.getElementById('mermaid-container').style.display = 'flex';
    document.getElementById('tree-view').style.display = 'none';
    // Reset zoom when switching nodes
    resetZoom();
    renderMermaidGraph(currentNodeData);
  } else {
    document.getElementById('mermaid-container').style.display = 'none';
    document.getElementById('tree-view').style.display = 'block';
    renderTreeView(currentNodeData);
  }
}

// Render Tree View with hierarchical structure
function renderTreeView(data) {
  const container = document.getElementById('tree-view');
  const node = data.node;
  const callers = data.callers || [];
  const callees = data.callees || [];

  let html = '';
  let nodeCount = 1; // Start with current node

  // Section 1: Upstream callers (hierarchical)
  html += `<div class="section">`;
  html += `<div class="section-title">⬆️ 调用者 (深度 ${currentDepth})</div>`;
  if (callers.length > 0) {
    nodeCount += renderCallersTree(callers, '', true).count;
    html += renderCallersTree(callers, '', true).html;
  } else {
    html += `<span class="no-data">无上游调用者 (可能是入口函数)</span>\n`;
  }
  html += `</div>`;

  // Section 2: Current function
  html += `<div class="section">`;
  html += `<div class="section-title">📍 当前函数</div>`;
  html += `<span class="tree-current">${node.label}</span>`;
  html += `<span class="tree-file">${shortPath(node.file)}:${node.line}</span>\n`;
  if (node.signature) {
    html += `<span class="tree-line">   </span><span style="color: var(--text-secondary); font-size: 12px;">${node.signature}</span>\n`;
  }
  html += `</div>`;

  // Section 3: Downstream callees (hierarchical)
  html += `<div class="section">`;
  html += `<div class="section-title">⬇️ 被调用 (深度 ${currentDepth})</div>`;
  if (callees.length > 0) {
    nodeCount += renderCalleesTree(callees, '', true).count;
    html += renderCalleesTree(callees, '', true).html;
  } else {
    html += `<span class="no-data">无下游调用 (叶子函数)</span>\n`;
  }
  html += `</div>`;

  container.innerHTML = html;
  document.getElementById('visible-count').textContent = nodeCount;
}

// Recursively render callers tree
function renderCallersTree(nodes, prefix, isRoot) {
  let html = '';
  let count = 0;

  nodes.forEach((caller, idx) => {
    const isLast = idx === nodes.length - 1;
    const connector = isLast ? '└── ' : '├── ';
    const childPrefix = prefix + (isLast ? '    ' : '│   ');

    html += `<span class="tree-line">${prefix}${connector}</span>`;
    html += `<span class="tree-node" onclick="selectNode(${caller.id})" title="${caller.fullName}">${caller.label}</span>`;
    html += `<span class="tree-file">${shortPath(caller.file)}:${caller.line}</span>\n`;
    count++;

    // Render children recursively
    if (caller.children && caller.children.length > 0) {
      const childResult = renderCallersTree(caller.children, childPrefix, false);
      html += childResult.html;
      count += childResult.count;
    }
  });

  return { html, count };
}

// Recursively render callees tree
function renderCalleesTree(nodes, prefix, isRoot) {
  let html = '';
  let count = 0;

  nodes.forEach((callee, idx) => {
    const isLast = idx === nodes.length - 1;
    const connector = isLast ? '└── ' : '├── ';
    const childPrefix = prefix + (isLast ? '    ' : '│   ');

    html += `<span class="tree-line">${prefix}${connector}</span>`;
    html += `<span class="tree-node" onclick="selectNode(${callee.id})" title="${callee.fullName}">${callee.label}</span>`;
    html += `<span class="tree-file">${shortPath(callee.file)}:${callee.line}</span>\n`;
    count++;

    // Render children recursively
    if (callee.children && callee.children.length > 0) {
      const childResult = renderCalleesTree(callee.children, childPrefix, false);
      html += childResult.html;
      count += childResult.count;
    }
  });

  return { html, count };
}

// Direction toggle
function setDirection(dir) {
  currentDirection = dir;
  document.getElementById('btn-lr').classList.toggle('active', dir === 'LR');
  document.getElementById('btn-tb').classList.toggle('active', dir === 'TB');

  if (currentNodeData && currentViewMode === 'mermaid') {
    renderMermaidGraph(currentNodeData);
  }
}

// Show node detail panel
function showNodeDetail(data) {
  const panel = document.getElementById('detail-panel');
  const content = document.getElementById('detail-content');
  const title = document.getElementById('detail-title');

  title.textContent = data.node.label;

  let html = `
    <div class="detail-row">
      <span class="detail-label">完整路径</span>
      <span class="detail-value">${data.node.fullName}</span>
    </div>
    <div class="detail-row">
      <span class="detail-label">所属包</span>
      <span class="detail-value">${data.node.package}</span>
    </div>
    <div class="detail-row">
      <span class="detail-label">文件位置</span>
      <span class="detail-value">${data.node.file}:${data.node.line}</span>
    </div>
  `;

  if (data.node.signature) {
    html += `
      <div class="detail-row">
        <span class="detail-label">函数签名</span>
        <span class="detail-value" style="font-family: monospace; font-size: 12px;">${data.node.signature}</span>
      </div>
    `;
  }

  if (data.node.doc) {
    html += `
      <div class="detail-row">
        <span class="detail-label">文档注释</span>
        <span class="detail-value" style="white-space: pre-wrap; font-size: 12px;">${data.node.doc}</span>
      </div>
    `;
  }

  content.innerHTML = html;
  panel.style.display = 'block';
}

// Zoom functions
function zoomIn() {
  zoomLevel = Math.min(3, zoomLevel * 1.25); // 使用乘法，每次放大25%
  applyTransform();
}

function zoomOut() {
  zoomLevel = Math.max(0.1, zoomLevel / 1.25); // 使用除法，每次缩小20%
  applyTransform();
}

function resetZoom() {
  zoomLevel = 1;
  panX = 0;
  panY = 0;
  applyTransform();
}

// 自动适配缩放，让图形适合容器并保持清晰
function autoFitZoom() {
  const container = document.getElementById('mermaid-container');
  const graph = document.getElementById('mermaid-graph');
  const svgEl = graph?.querySelector('svg');

  if (!container || !svgEl) return;

  // 获取 SVG 实际尺寸
  const svgRect = svgEl.getBoundingClientRect();
  const containerRect = container.getBoundingClientRect();

  // 计算适合容器的缩放比例（留出边距）
  const padding = 48;
  const scaleX = (containerRect.width - padding) / svgRect.width;
  const scaleY = (containerRect.height - padding) / svgRect.height;

  // 取较小值确保完全可见，但不超过 1.5 倍（避免过度放大模糊）
  // 最小值设为 0.8，确保图形足够大
  let fitScale = Math.min(scaleX, scaleY, 1.5);
  fitScale = Math.max(fitScale, 0.8);

  // 应用缩放
  zoomLevel = fitScale;
  panX = 0;
  panY = 0;
  applyTransform();
}

function applyTransform() {
  const graph = document.getElementById('mermaid-graph');
  if (graph) {
    graph.style.transform = `translate(${panX}px, ${panY}px) scale(${zoomLevel})`;
  }
  // 四舍五入到整数百分比，避免小数精度问题
  const percent = Math.round(zoomLevel * 100);
  document.getElementById('zoom-level').textContent = `${percent}%`;
}

// Setup pan and zoom handlers
function setupPanZoom() {
  const container = document.getElementById('mermaid-container');

  // Mouse wheel zoom - 使用乘法实现更平滑的缩放
  container.addEventListener('wheel', (e) => {
    if (currentViewMode !== 'mermaid') return;
    e.preventDefault();
    const factor = e.deltaY > 0 ? 0.9 : 1.1; // 滚轮缩放10%
    zoomLevel = Math.max(0.1, Math.min(3, zoomLevel * factor));
    applyTransform();
  }, { passive: false });

  // Pan with mouse drag
  container.addEventListener('mousedown', (e) => {
    if (currentViewMode !== 'mermaid') return;
    if (e.target.closest('.node')) return; // Don't pan when clicking nodes
    isPanning = true;
    startX = e.clientX - panX;
    startY = e.clientY - panY;
    container.style.cursor = 'grabbing';
  });

  document.addEventListener('mousemove', (e) => {
    if (!isPanning) return;
    panX = e.clientX - startX;
    panY = e.clientY - startY;
    applyTransform();
  });

  document.addEventListener('mouseup', () => {
    isPanning = false;
    const container = document.getElementById('mermaid-container');
    if (container) container.style.cursor = 'grab';
  });
}

// Setup event listeners
function setupEventListeners() {
  const searchInput = document.getElementById('search');

  searchInput.addEventListener('input', (e) => {
    const query = e.target.value.toLowerCase();
    const filtered = allNodes.filter(n =>
      n.label.toLowerCase().includes(query) ||
      n.fullName.toLowerCase().includes(query)
    );
    renderNodeList(filtered);
  });

  // Keyboard shortcuts
  document.addEventListener('keydown', (e) => {
    if (e.key === '/' && e.target !== searchInput) {
      e.preventDefault();
      searchInput.focus();
    }
    // Toggle view mode with 't' key
    if (e.key === 't' && e.target !== searchInput) {
      setViewMode(currentViewMode === 'tree' ? 'mermaid' : 'tree');
    }
  });
}
