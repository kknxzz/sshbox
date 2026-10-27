import {renderMarkdown} from './render.js';
const root = 'https://raw.githubusercontent.com/kknxzz/sshbox/main/docs/';
const search = document.getElementById('docs-search');
const content = document.getElementById('docs-content');
const nav = document.getElementById('docs-nav');
const status = document.getElementById('docs-status');
function filter() {
  const query = search.value.trim().toLowerCase(); let shown = 0;
  for (const article of content.querySelectorAll('.docs-article')) {
    article.hidden = query !== '' && !article.textContent.toLowerCase().includes(query);
    if (!article.hidden) shown++;
  }
  for (const group of nav.querySelectorAll('.docs-group')) {
    let matches = 0;
    for (const link of group.querySelectorAll('a')) { const article = document.getElementById(link.hash.slice(1)); link.hidden = !article || article.hidden; if (!link.hidden) matches++; }
    group.hidden = matches === 0;
  }
  document.getElementById('docs-empty').hidden = shown > 0;
}
search.addEventListener('input', filter);
async function fetchText(path) {
  const response = await fetch(root + path, {cache: 'no-cache', signal: AbortSignal.timeout(6000)});
  if (!response.ok) throw new Error('GitHub unavailable');
  return response.text();
}
try {
  const index = JSON.parse(await fetchText('index.json'));
  if (!Array.isArray(index.sections) || index.sections.length > 20) throw new Error('Invalid index');
  const sections = index.sections.map(section => {
    if (typeof section.title !== 'string' || !Array.isArray(section.documents) || section.documents.length > 50) throw new Error('Invalid section');
    return {title: section.title, documents: section.documents.map(doc => {
      if (typeof doc.title !== 'string' || !/^[a-z0-9-]+$/.test(doc.id)) throw new Error('Invalid document');
      return doc;
    })};
  });
  const docs = sections.flatMap(section => section.documents);
  if (!docs.length || new Set(docs.map(d => d.id)).size !== docs.length) throw new Error('Invalid documents');
  const texts = await Promise.all(docs.map(doc => fetchText(`${doc.id}.md`)));
  const articles = docs.map((doc, index) => {
    const article = document.createElement('article'); article.className = 'docs-article'; article.id = doc.id; article.dataset.document = doc.id;
    article.innerHTML = renderMarkdown(texts[index]); return article;
  });
  const groups = sections.map(section => {
    const group = document.createElement('div'); group.className = 'docs-group';
    const title = document.createElement('p'); title.textContent = section.title; group.append(title);
    for (const doc of section.documents) { const a = document.createElement('a'); a.href = `#${doc.id}`; a.textContent = doc.title; group.append(a); }
    return group;
  });
  content.replaceChildren(...articles); nav.replaceChildren(...groups); filter();
  status.textContent = 'Showing the latest documentation from GitHub.';
  if (location.hash) document.getElementById(decodeURIComponent(location.hash.slice(1)))?.scrollIntoView();
} catch { status.textContent = 'Showing the published copy. GitHub updates are currently unavailable.'; }
