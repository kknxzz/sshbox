import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {renderMarkdown} from '../dist/docs/render.js';
const repo = new URL('../../docs/', import.meta.url);
const dest = new URL('../dist/docs/', import.meta.url);
const manifest = JSON.parse(readFileSync(new URL('index.json',repo),'utf8'));
const origin = readFileSync(new URL('../dist/index.html',import.meta.url),'utf8').match(/rel="canonical" href="(https:\/\/[^"/]+)/)[1];
mkdirSync(dest,{recursive:true});
const esc = s => s.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;');
let nav = '', content = '';
for (const section of manifest.sections) {
  nav += `<div class="docs-group"><p>${esc(section.title)}</p>${section.documents.map(d=>`<a href="#${esc(d.id)}">${esc(d.title)}</a>`).join('')}</div>`;
  for (const doc of section.documents) {
    const md = readFileSync(new URL(`${doc.id}.md`,repo),'utf8');
    content += `<article class="docs-article" id="${esc(doc.id)}" data-document="${esc(doc.id)}">${renderMarkdown(md)}</article>`;
    const rst = md.replace(/^### (.+)$/gm, (_,s)=>`${s}\n${'~'.repeat(s.length)}`).replace(/^## (.+)$/gm, (_,s)=>`${s}\n${'-'.repeat(s.length)}`).replace(/^# (.+)$/gm, (_,s)=>`${s}\n${'='.repeat(s.length)}`).replace(/```[^\n]*\n([\s\S]*?)```/g, (_,s)=>`::\n\n${s.replace(/^/gm,'   ').trimEnd()}\n`).replace(/\[([^\]]+)\]\((https:\/\/[^)]+)\)/g,'`$1 <$2>`_').replace(/`([^`]+)`/g, (m,s)=>s.includes(' <')?m:`\`\`${s}\`\``);
    writeFileSync(new URL(`${doc.id}.rst`,repo),rst);
  }
}
const html=`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>sshbox documentation — Setup, configuration and security</title><meta name="description" content="Set up sshbox, configure Docker or Podman, understand its security model, and troubleshoot common problems."><link rel="canonical" href="${origin}/docs/"><link rel="icon" type="image/svg+xml" href="../assets/favicon.svg"><link rel="stylesheet" href="../style.css"><script type="module" src="docs.js"></script></head><body><a class="skip-link" href="#main">Skip to content</a><header class="topbar legal-topbar"><a class="wordmark" href="../" aria-label="sshbox home"><span class="brand-icon">&gt;_</span>sshbox<span class="brand-dot">.</span></a><a class="legal-back" href="../">← Back to sshbox</a></header><main id="main" class="frame docs-main"><div class="docs-title"><p class="eyebrow">SSHBOX / DOCUMENTATION</p><h1>Get up and running<span class="orange">.</span></h1><p>Setup, configuration, and the things worth knowing.</p></div><div class="docs-layout"><aside class="docs-sidebar"><label for="docs-search">Search the docs</label><input id="docs-search" type="search" placeholder="Search…" autocomplete="off"><nav class="docs-nav" aria-label="Documentation categories" id="docs-nav">${nav}</nav><p class="docs-source"><a href="https://github.com/kknxzz/sshbox/tree/main/docs">View on GitHub ↗</a></p></aside><div><p class="docs-status" id="docs-status" role="status">Showing the published documentation.</p><div id="docs-content">${content}</div><p id="docs-empty" hidden>No matching documents. Try another search.</p></div></div></main><footer class="footer frame"><a class="wordmark" href="../"><span class="brand-icon">&gt;_</span>sshbox<span class="brand-dot">.</span></a><p>Do one thing. Leave nothing behind.</p><div><a href="../privacy/">Privacy policy</a><a href="../terms/">Terms of service</a><a href="https://github.com/kknxzz/sshbox">GitHub ↗</a></div></footer></body></html>`;
writeFileSync(new URL('index.html',dest),html);
console.log('Built documentation and reStructuredText');
