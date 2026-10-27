import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';

const pages = [
  {slug:'privacy', name:'Privacy policy', source:'privacy-policy.md'},
  {slug:'terms', name:'Terms of service', source:'terms-of-service.md'},
];
const root = new URL('../',import.meta.url);
function escape(text) { return text.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;'); }
function inline(text) {
  return escape(text).replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2">$1</a>');
}
function body(markdown) {
  return markdown.trim().split(/\n\s*\n/).map(part => {
    if (part.startsWith('# ')) return `<h1>${inline(part.slice(2).trim())}</h1>`;
    if (part.startsWith('## ')) return `<h2>${inline(part.slice(3).trim())}</h2>`;
    return `<p>${inline(part.replaceAll('\n',' ').trim())}</p>`;
  }).join('\n');
}
for (const page of pages) {
  const text=readFileSync(new URL(`legal-output/${page.source}`,root),'utf8');
  const origin=readFileSync(new URL('dist/index.html',root),'utf8').match(/rel="canonical" href="(https:\/\/[^"/]+)\//)?.[1];
  if(!origin) throw Error('Missing canonical origin');
  const html=`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#f7f7f2"><title>${page.name} — sshbox</title><meta name="description" content="${page.name} for the sshbox website."><link rel="canonical" href="${origin}/${page.slug}/"><link rel="icon" type="image/svg+xml" href="../assets/favicon.svg"><link rel="stylesheet" href="../style.css"></head>
<body><a class="skip-link" href="#main">Skip to content</a><header class="topbar legal-topbar"><a href="../" class="wordmark" aria-label="sshbox home"><span class="brand-icon">&gt;_</span>sshbox<span class="brand-dot">.</span></a><a class="legal-back" href="../">← Back to sshbox</a></header>
<main id="main" class="legal-main frame"><div class="legal-heading"><p class="eyebrow">SSHBOX / LEGAL</p></div><article class="legal-content">${body(text)}</article></main>
<footer class="footer frame"><a class="wordmark" href="../"><span class="brand-icon">&gt;_</span>sshbox<span class="brand-dot">.</span></a><p>Do one thing. Leave nothing behind.</p><div><a href="../privacy/">Privacy policy</a><a href="../terms/">Terms of service</a><a href="https://github.com/kknxzz/sshbox">GitHub ↗</a></div></footer></body></html>\n`;
  const directory=new URL(`dist/${page.slug}/`,root); mkdirSync(directory,{recursive:true});writeFileSync(new URL('index.html',directory),html);
}
