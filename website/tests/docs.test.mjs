import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,existsSync} from 'node:fs';
import {renderMarkdown} from '../dist/docs/render.js';
const root = new URL('../../docs/', import.meta.url);
const page = readFileSync(new URL('../dist/docs/index.html',import.meta.url),'utf8');
const manifest = JSON.parse(readFileSync(new URL('index.json',root),'utf8'));
test('each document has both source formats and an indexable fallback', () => {
 for (const item of manifest.sections.flatMap(section => section.documents)) {
  assert.ok(existsSync(new URL(`${item.id}.md`,root)));
  assert.ok(existsSync(new URL(`${item.id}.rst`,root)));
  assert.ok(page.includes(`id="${item.id}"`));
 }
 assert.match(page, /rel="canonical" href="https:\/\/sshbox\.constlm\.xyz\/docs\/"/);
});
test('remote Markdown is escaped before rendering', () => {
 const html = renderMarkdown('# Hello\n\n<script>alert(1)</script>\n\n[bad](javascript:alert(1))');
 assert.ok(!html.includes('<script>'));
 assert.ok(!html.includes('href="javascript:'));
 assert.ok(html.includes('&lt;script&gt;'));
});
