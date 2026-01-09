/**
 * Path finding functionality - find paths between functions
 */

// 查找路径
async function findPath() {
  if (!selectedNodeId) {
    alert('请先在左侧选择一个起始函数');
    return;
  }

  const targetInput = document.getElementById('target-func-input').value.trim();
  if (!targetInput) {
    alert('请输入目标函数名');
    return;
  }

  const direction = document.getElementById('path-direction').value;
  const startNode = allNodes.find(n => n.id === selectedNodeId);

  // 搜索匹配的目标函数（支持模糊匹配）
  const targetMatches = allNodes.filter(n =>
    n.label.toLowerCase().includes(targetInput.toLowerCase()) ||
    n.fullName.toLowerCase().includes(targetInput.toLowerCase())
  );

  if (targetMatches.length === 0) {
    alert(`未找到匹配 "${targetInput}" 的函数`);
    return;
  }

  // 显示加载状态
  const container = document.getElementById('mermaid-graph');
  container.innerHTML = '<div style="color: var(--text-secondary); padding: 40px; text-align: center;">🔍 正在查找路径...</div>';

  // 对每个匹配的目标函数查找路径
  const allPaths = [];
  for (const targetNode of targetMatches) {
    if (targetNode.id === selectedNodeId) continue; // 跳过自身

    const paths = await findPathDFS(startNode.id, targetNode.id, direction);
    if (paths.length > 0) {
      allPaths.push({
        target: targetNode,
        paths: paths
      });
    }
  }

  if (allPaths.length === 0) {
    alert(`在${direction === 'downstream' ? '下游' : '上游'}调用链中未找到到 "${targetInput}" 的路径\n尝试切换搜索方向`);
    // 恢复原来的视图
    if (currentNodeData) {
      renderMermaidGraph(currentNodeData);
    }
    return;
  }

  // 检测重名函数
  const nameCounts = {};
  allPaths.forEach(p => {
    nameCounts[p.target.label] = (nameCounts[p.target.label] || 0) + 1;
  });
  const duplicateNames = Object.entries(nameCounts)
    .filter(([_, count]) => count > 1)
    .map(([name, _]) => name);

  // 存储路径数据
  currentPathData = {
    startNode,
    targetInput,
    direction,
    paths: allPaths,
    duplicateNames
  };
  isPathMode = true;

  // 渲染路径图
  renderPathGraph(currentPathData);
}

// DFS 查找所有路径 (使用回溯法)
async function findPathDFS(startId, endId, direction) {
  const maxDepth = 10;
  const maxPaths = 20; // 最多找 20 条路径
  const foundPaths = [];
  const neighborCache = new Map(); // 缓存邻居节点避免重复请求

  // 获取邻居节点 (带缓存)
  async function getCachedNeighbors(nodeId) {
    if (neighborCache.has(nodeId)) {
      return neighborCache.get(nodeId);
    }
    const neighbors = await getNeighbors(nodeId, direction);
    neighborCache.set(nodeId, neighbors);
    return neighbors;
  }

  // DFS 递归查找
  async function dfs(currentId, path, visited) {
    if (foundPaths.length >= maxPaths) return;
    if (path.length > maxDepth) return;

    if (currentId === endId) {
      foundPaths.push([...path]);
      return;
    }

    try {
      const neighbors = await getCachedNeighbors(currentId);
      for (const neighbor of neighbors) {
        if (!visited.has(neighbor.id)) {
          visited.add(neighbor.id);
          path.push(neighbor.id);
          await dfs(neighbor.id, path, visited);
          path.pop();
          visited.delete(neighbor.id);
        }
      }
    } catch (err) {
      console.error('获取邻居节点失败:', err);
    }
  }

  const visited = new Set([startId]);
  await dfs(startId, [startId], visited);

  return foundPaths;
}

// 渲染路径图
async function renderPathGraph(pathData) {
  const container = document.getElementById('mermaid-graph');
  const { startNode, paths, duplicateNames, direction } = pathData;

  // 为重名函数分配颜色索引
  const duplicateColorMap = {};
  duplicateNames.forEach((name, idx) => {
    duplicateColorMap[name] = (idx % 4) + 1;
  });

  // 收集所有节点和边
  const nodeSet = new Set();
  const edges = [];
  const targetNodes = new Set(); // 目标节点

  paths.forEach((pathResult, pathIdx) => {
    targetNodes.add(pathResult.target.id);
    pathResult.paths.forEach(path => {
      path.forEach(nodeId => nodeSet.add(nodeId));
      for (let i = 0; i < path.length - 1; i++) {
        edges.push({ from: path[i], to: path[i + 1], pathIdx });
      }
    });
  });

  // 构建 Mermaid 代码
  let lines = [];
  lines.push(`flowchart ${currentDirection}`);

  // 样式定义
  lines.push(`  %% Path styles`);
  lines.push(`  classDef startNode fill:#0ea5e9,stroke:#38bdf8,color:#fff,stroke-width:3px,rx:12,ry:12`);
  lines.push(`  classDef endNode fill:#10b981,stroke:#34d399,color:#fff,stroke-width:3px,rx:12,ry:12`);
  lines.push(`  classDef pathNode fill:#6366f1,stroke:#818cf8,color:#fff,stroke-width:2px,rx:8,ry:8`);
  // 重名节点的不同颜色
  lines.push(`  classDef dup1 fill:#f59e0b,stroke:#fbbf24,color:#fff,stroke-width:2px,rx:8,ry:8`);
  lines.push(`  classDef dup2 fill:#a855f7,stroke:#c084fc,color:#fff,stroke-width:2px,rx:8,ry:8`);
  lines.push(`  classDef dup3 fill:#ef4444,stroke:#f87171,color:#fff,stroke-width:2px,rx:8,ry:8`);
  lines.push(`  classDef dup4 fill:#06b6d4,stroke:#22d3ee,color:#fff,stroke-width:2px,rx:8,ry:8`);

  // 添加节点
  const nodeInfoMap = new Map();
  nodeSet.forEach(nodeId => {
    const node = allNodes.find(n => n.id === nodeId);
    if (!node) return;

    const mermaidId = `n${nodeId}`;
    const label = escapeLabel(node.label);
    nodeInfoMap.set(mermaidId, node);

    let styleClass = 'pathNode';
    if (nodeId === startNode.id) {
      styleClass = 'startNode';
      lines.push(`  ${mermaidId}(["🚀 ${label}"]):::${styleClass}`);
    } else if (targetNodes.has(nodeId)) {
      // 检查是否是重名目标
      if (duplicateNames.includes(node.label)) {
        const colorIdx = duplicateColorMap[node.label];
        styleClass = `dup${colorIdx}`;
        lines.push(`  ${mermaidId}(["🎯 ${label}"]):::${styleClass}`);
      } else {
        styleClass = 'endNode';
        lines.push(`  ${mermaidId}(["🎯 ${label}"]):::${styleClass}`);
      }
    } else {
      // 检查路径中间节点是否重名
      if (duplicateNames.includes(node.label)) {
        const colorIdx = duplicateColorMap[node.label];
        styleClass = `dup${colorIdx}`;
      }
      lines.push(`  ${mermaidId}["${label}"]:::${styleClass}`);
    }
  });

  // 去重边
  // 上游方向时反转箭头：B 调用 A，所以箭头从 B 指向 A（caller → callee）
  const edgeSet = new Set();
  edges.forEach(e => {
    const key = direction === 'upstream' ? `${e.to}-${e.from}` : `${e.from}-${e.to}`;
    if (!edgeSet.has(key)) {
      edgeSet.add(key);
      if (direction === 'upstream') {
        // 上游：反转箭头，表示调用方向（谁调用了谁）
        lines.push(`  n${e.to} --> n${e.from}`);
      } else {
        // 下游：正常方向
        lines.push(`  n${e.from} --> n${e.to}`);
      }
    }
  });

  // 添加高亮边样式
  lines.push(`  linkStyle default stroke:#818cf8,stroke-width:2px`);

  currentMermaidCode = lines.join('\n');

  // 渲染
  try {
    const { svg } = await mermaid.render('mermaid-path-svg', currentMermaidCode);
    container.innerHTML = svg;

    const svgEl = container.querySelector('svg');
    if (svgEl) {
      svgEl.style.maxWidth = 'none';
      svgEl.style.height = 'auto';

      // 绑定节点点击事件
      const tooltipEl = document.getElementById('node-tooltip');
      svgEl.querySelectorAll('.node').forEach(nodeEl => {
        const match = nodeEl.id.match(/flowchart-(n\d+)-/);
        if (match) {
          const mermaidId = match[1];
          const info = nodeInfoMap.get(mermaidId);

          nodeEl.style.cursor = 'pointer';

          // 单击显示 tooltip
          nodeEl.addEventListener('click', (e) => {
            e.preventDefault();
            e.stopPropagation();
            if (info) {
              showNodeTooltip(tooltipEl, nodeEl, info);
            }
          });

          // 双击导航到该节点
          nodeEl.addEventListener('dblclick', (e) => {
            e.preventDefault();
            e.stopPropagation();
            if (info) {
              exitPathMode();
              selectNode(info.id);
            }
          });
        }
      });

      // 自动适配缩放
      setTimeout(() => autoFitZoom(), 50);
    }

    // 更新显示数量
    document.getElementById('visible-count').textContent = nodeSet.size;

    // 显示路径信息
    showPathInfo(pathData);

  } catch (err) {
    console.error('Path render error:', err);
    container.innerHTML = `<div style="color: #f85149; padding: 20px;">渲染失败: ${err.message}</div>`;
  }
}

// 显示路径信息面板
function showPathInfo(pathData) {
  const panel = document.getElementById('detail-panel');
  const content = document.getElementById('detail-content');
  const title = document.getElementById('detail-title');

  title.textContent = `🔗 路径: ${pathData.startNode.label} → ${pathData.targetInput}`;

  let html = `
    <div class="detail-row">
      <span class="detail-label">方向</span>
      <span class="detail-value">${pathData.direction === 'downstream' ? '下游 (A调用B)' : '上游 (B调用A)'}</span>
    </div>
    <div class="detail-row">
      <span class="detail-label">找到</span>
      <span class="detail-value">${pathData.paths.length} 个目标, ${pathData.paths.reduce((sum, p) => sum + p.paths.length, 0)} 条路径</span>
    </div>
  `;

  // 显示每个目标的路径摘要
  pathData.paths.forEach((p, idx) => {
    const isDup = pathData.duplicateNames.includes(p.target.label);
    html += `
      <div class="detail-row" style="margin-top: 8px;">
        <span class="detail-label">目标 ${idx + 1}</span>
        <span class="detail-value" style="${isDup ? 'color: var(--accent-orange);' : ''}">${p.target.label}${isDup ? ' ⚠️' : ''}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label"></span>
        <span class="detail-value" style="font-size: 11px; color: var(--text-secondary);">${p.target.file}:${p.target.line}</span>
      </div>
    `;
  });

  // 添加操作按钮
  html += `
    <div style="margin-top: 12px; display: flex; gap: 8px;">
      <button class="toolbar-btn" onclick="copyPathRAG()" style="flex: 1;">📋 复制路径</button>
      <button class="toolbar-btn" onclick="exitPathMode()" style="flex: 1;">✖️ 退出路径</button>
    </div>
  `;

  content.innerHTML = html;
  panel.style.display = 'block';
}

// 退出路径模式
function exitPathMode() {
  isPathMode = false;
  currentPathData = null;
  document.getElementById('target-func-input').value = '';

  // 恢复原来的视图
  if (currentNodeData) {
    renderMermaidGraph(currentNodeData);
    showNodeDetail(currentNodeData);
  }
}
