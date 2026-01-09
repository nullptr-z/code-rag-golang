/**
 * Utility functions
 */

// Shorten file path for display
function shortPath(path) {
  if (!path) return '';
  const parts = path.split('/');
  if (parts.length <= 2) return path;
  return '.../' + parts.slice(-2).join('/');
}

// Escape special characters for Mermaid labels
function escapeLabel(str) {
  return str
    .replace(/"/g, "'")
    .replace(/[<>]/g, '')
    .replace(/\(/g, '❨')
    .replace(/\)/g, '❩')
    .replace(/\[/g, '❲')
    .replace(/\]/g, '❳');
}

// Copy text to clipboard with fallback
function copyToClipboard(text, successMessage) {
  navigator.clipboard.writeText(text).then(() => {
    alert(successMessage);
  }).catch(err => {
    console.error('复制失败:', err);
    const textarea = document.createElement('textarea');
    textarea.value = text;
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand('copy');
    document.body.removeChild(textarea);
    alert(successMessage);
  });
}
