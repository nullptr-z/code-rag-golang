/**
 * RAG export functions - Generate AI-friendly output
 */

// Copy RAG data in AI-friendly format
function copyRAGJson() {
  // 如果处于路径模式，复制路径 RAG
  if (isPathMode && currentPathData) {
    copyPathRAG();
    return;
  }

  if (!currentNodeData) {
    alert('请先选择一个函数');
    return;
  }

  const ragText = generateAIFriendlyRAG(currentNodeData);
  copyToClipboard(ragText, 'RAG 上下文已复制到剪贴板');
}

// 生成 AI 友好的 RAG 格式
function generateAIFriendlyRAG(data) {
  const node = data.node;
  const callers = data.callers || [];
  const callees = data.callees || [];
  let lines = [];

  // 标题
  lines.push(`# 函数调用关系: ${node.label}`);
  lines.push('');

  // 目标函数信息
  lines.push('## 目标函数');
  lines.push(`- 函数名: ${node.label}`);
  lines.push(`- 完整路径: ${node.fullName}`);
  lines.push(`- 所属包: ${node.package}`);
  lines.push(`- 文件位置: ${node.file}:${node.line}`);
  if (node.signature) {
    lines.push(`- 函数签名: ${node.signature}`);
  }
  if (node.doc) {
    lines.push(`- 文档注释: ${node.doc}`);
  }
  lines.push('');

  // 调用关系概览
  const callerCount = countTreeNodes(callers);
  const calleeCount = countTreeNodes(callees);
  lines.push('## 调用关系概览');
  lines.push(`- 被 ${callerCount} 个函数调用 (上游调用者)`);
  lines.push(`- 调用了 ${calleeCount} 个函数 (下游被调用)`);
  lines.push(`- 分析深度: ${currentDepth} 层`);
  lines.push('');

  // 上游调用者 (谁调用了这个函数)
  lines.push('## 上游调用者 (谁调用了此函数)');
  if (callers.length > 0) {
    lines.push('```');
    generateCallTree(callers, '', lines, 'caller');
    lines.push('```');
    lines.push('');
    // 调用者详情
    lines.push('### 调用者详情');
    generateNodeDetails(callers, lines);
  } else {
    lines.push('无上游调用者 (可能是入口函数如 main)');
  }
  lines.push('');

  // 下游被调用 (这个函数调用了谁)
  lines.push('## 下游被调用 (此函数调用了哪些函数)');
  if (callees.length > 0) {
    lines.push('```');
    generateCallTree(callees, '', lines, 'callee');
    lines.push('```');
    lines.push('');
    // 被调用者详情
    lines.push('### 被调用函数详情');
    generateNodeDetails(callees, lines);
  } else {
    lines.push('无下游调用 (叶子函数，不调用其他函数)');
  }
  lines.push('');

  // 调用链路径
  lines.push('## 调用链路径');
  generateCallPaths(data, lines);

  return lines.join('\n');
}

// 统计树中节点数量
function countTreeNodes(nodes) {
  let count = 0;
  nodes.forEach(n => {
    count++;
    if (n.children && n.children.length > 0) {
      count += countTreeNodes(n.children);
    }
  });
  return count;
}

// 生成调用树的文本表示
function generateCallTree(nodes, prefix, lines, type) {
  nodes.forEach((n, idx) => {
    const isLast = idx === nodes.length - 1;
    const connector = isLast ? '└── ' : '├── ';
    const childPrefix = prefix + (isLast ? '    ' : '│   ');

    let nodeStr = `${prefix}${connector}${n.label}`;
    if (n.file) {
      nodeStr += ` (${shortPath(n.file)}:${n.line})`;
    }
    lines.push(nodeStr);

    if (n.children && n.children.length > 0) {
      generateCallTree(n.children, childPrefix, lines, type);
    }
  });
}

// 生成节点详情列表
function generateNodeDetails(nodes, lines) {
  function traverse(nodeList, depth) {
    nodeList.forEach(n => {
      const indent = '  '.repeat(depth);
      lines.push(`${indent}- **${n.label}**`);
      lines.push(`${indent}  - 位置: ${n.file}:${n.line}`);
      if (n.signature) {
        lines.push(`${indent}  - 签名: \`${n.signature}\``);
      }
      if (n.doc) {
        const docShort = n.doc.length > 100 ? n.doc.substring(0, 100) + '...' : n.doc;
        lines.push(`${indent}  - 说明: ${docShort}`);
      }
      if (n.children && n.children.length > 0) {
        traverse(n.children, depth + 1);
      }
    });
  }
  traverse(nodes, 0);
}

// 生成调用链路径
function generateCallPaths(data, lines) {
  const node = data.node;

  // 上游路径
  if (data.callers.length > 0) {
    lines.push('### 入口到目标函数的路径');
    function getCallerPaths(nodes, currentPath) {
      const paths = [];
      nodes.forEach(n => {
        const newPath = [n.label, ...currentPath];
        if (n.children && n.children.length > 0) {
          paths.push(...getCallerPaths(n.children, newPath));
        } else {
          paths.push(newPath);
        }
      });
      return paths;
    }
    const callerPaths = getCallerPaths(data.callers, [node.label]);
    callerPaths.slice(0, 10).forEach(path => {
      lines.push(`- ${path.join(' → ')}`);
    });
    if (callerPaths.length > 10) {
      lines.push(`- ... 还有 ${callerPaths.length - 10} 条路径`);
    }
    lines.push('');
  }

  // 下游路径
  if (data.callees.length > 0) {
    lines.push('### 目标函数到叶子的路径');
    function getCalleePaths(nodes, currentPath) {
      const paths = [];
      nodes.forEach(n => {
        const newPath = [...currentPath, n.label];
        if (n.children && n.children.length > 0) {
          paths.push(...getCalleePaths(n.children, newPath));
        } else {
          paths.push(newPath);
        }
      });
      return paths;
    }
    const calleePaths = getCalleePaths(data.callees, [node.label]);
    calleePaths.slice(0, 10).forEach(path => {
      lines.push(`- ${path.join(' → ')}`);
    });
    if (calleePaths.length > 10) {
      lines.push(`- ... 还有 ${calleePaths.length - 10} 条路径`);
    }
  }
}

// 复制路径 RAG
function copyPathRAG() {
  if (!currentPathData) {
    alert('没有可复制的路径');
    return;
  }

  const { startNode, paths, direction } = currentPathData;
  let lines = [];

  lines.push(`# 函数调用路径分析`);
  lines.push('');
  lines.push(`## 起始函数: ${startNode.label}`);
  lines.push(`- 完整路径: ${startNode.fullName}`);
  lines.push(`- 文件位置: ${startNode.file}:${startNode.line}`);
  if (startNode.signature) lines.push(`- 函数签名: ${startNode.signature}`);
  if (startNode.doc) lines.push(`- 文档: ${startNode.doc}`);
  lines.push('');
  lines.push(`## 搜索方向: ${direction === 'downstream' ? '下游 (A 调用 B)' : '上游 (B 调用 A)'}`);
  lines.push('');

  paths.forEach((result, idx) => {
    lines.push(`## 目标函数 ${idx + 1}: ${result.target.label}`);
    lines.push(`- 完整路径: ${result.target.fullName}`);
    lines.push(`- 文件位置: ${result.target.file}:${result.target.line}`);
    if (result.target.signature) lines.push(`- 函数签名: ${result.target.signature}`);
    if (result.target.doc) lines.push(`- 文档: ${result.target.doc}`);
    lines.push('');

    lines.push(`### 调用路径 (共 ${result.paths.length} 条)`);
    result.paths.forEach((path, pathIdx) => {
      const pathNodes = path.map(id => allNodes.find(n => n.id === id)).filter(Boolean);
      const pathLabels = pathNodes.map(n => n.label);
      lines.push(`**路径 ${pathIdx + 1}**: ${pathLabels.join(' → ')}`);
      lines.push('');
      lines.push('节点详情:');
      pathNodes.forEach((node, nodeIdx) => {
        lines.push(`- [${nodeIdx + 1}] **${node.label}**`);
        lines.push(`  - 位置: ${node.file}:${node.line}`);
        if (node.signature) lines.push(`  - 签名: \`${node.signature}\``);
        if (node.doc) lines.push(`  - 说明: ${node.doc}`);
      });
      lines.push('');
    });
  });

  const ragText = lines.join('\n');
  copyToClipboard(ragText, '路径 RAG 已复制到剪贴板');
}

// Copy Mermaid code
function copyMermaidCode() {
  if (!currentMermaidCode) {
    alert('请先选择一个函数');
    return;
  }
  copyToClipboard(currentMermaidCode, 'Mermaid 代码已复制到剪贴板');
}

// Export SVG
function exportSVG() {
  const svg = document.querySelector('#mermaid-graph svg');
  if (!svg) {
    alert('请先选择一个函数');
    return;
  }

  const svgData = new XMLSerializer().serializeToString(svg);
  const blob = new Blob([svgData], { type: 'image/svg+xml' });
  const url = URL.createObjectURL(blob);

  const a = document.createElement('a');
  a.href = url;
  a.download = `call-graph-${selectedNodeId}.svg`;
  a.click();

  URL.revokeObjectURL(url);
}
