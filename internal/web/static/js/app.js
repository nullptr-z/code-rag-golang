/**
 * Main application entry point
 */

// Initialize application
document.addEventListener('DOMContentLoaded', async () => {
  initMermaid();
  await loadGraph();
  setupEventListeners();
  setupPanZoom();
});
