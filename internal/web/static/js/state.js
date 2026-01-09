/**
 * Global application state
 */

// Graph data
let allNodes = [];
let allEdges = [];
let selectedNodeId = null;
let nodeIdMap = {}; // maps node.id to a safe mermaid id

// View state
let currentDirection = 'TB';
let currentViewMode = 'mermaid'; // 'mermaid' or 'tree'
let currentMermaidCode = '';
let currentNodeData = null;
let currentDepth = 1; // depth for upstream/downstream traversal

// Zoom and pan state
let zoomLevel = 1;
let panX = 0;
let panY = 0;
let isPanning = false;
let startX = 0;
let startY = 0;

// 记录已确认无法展开的节点（到达末端）
let terminalNodes = new Set();

// Path finder state
let currentPathData = null;
let isPathMode = false;
