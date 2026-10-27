// A deliberately small Markdown subset for the project's own documentation.
const escapeHTML = value => value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;');
function inline(text) {
  return escapeHTML(text).replace(/`([^`]+)`/g, '<code>$1</code>').replace(/\[([^\]]+)\]\((https:\/\/[^\s)]+)\)/g, '<a href="$2">$1</a>');
}
export function renderMarkdown(markdown) {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const out = []; let paragraph = [], list = [], code = null;
  const flush = () => { if (paragraph.length) { out.push(`<p>${inline(paragraph.join(' '))}</p>`); paragraph = []; } if (list.length) { out.push(`<ul>${list.map(item => `<li>${inline(item)}</li>`).join('')}</ul>`); list = []; } };
  for (const line of lines) {
    if (line.startsWith('```')) { if (code === null) { flush(); code = []; } else { out.push(`<pre><code>${escapeHTML(code.join('\n'))}</code></pre>`); code = null; } continue; }
    if (code !== null) { code.push(line); continue; }
    const heading = line.match(/^(#{1,3})\s+(.+)$/);
    if (heading) { flush(); const level = Math.min(heading[1].length + 1, 4); out.push(`<h${level}>${inline(heading[2])}</h${level}>`); continue; }
    if (line.startsWith('- ')) { if (paragraph.length) flush(); list.push(line.slice(2)); continue; }
    if (!line.trim()) { flush(); continue; }
    if (list.length) flush(); paragraph.push(line.trim());
  }
  if (code !== null) out.push(`<pre><code>${escapeHTML(code.join('\n'))}</code></pre>`);
  flush(); return out.join('\n');
}
