import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,existsSync} from 'node:fs';
const root = new URL('../dist/',import.meta.url);
const html=readFileSync(new URL('index.html',root),'utf8');
test('static HTML contains indexable content and one H1',()=>{
 assert.equal((html.match(/<h1\b/g)||[]).length,1);
 assert.match(html,/<html lang="en">/);
 assert.match(html,/<meta name="description" content=".{80,180}">/);
 assert.ok(html.includes('disposable Linux container'));
});
test('canonical, social metadata, and sitemap use the planned domain',()=>{
 assert.match(html,/<link rel="canonical" href="https:\/\/sshbox\.constlm\.xyz\/">/);
 assert.ok(!html.includes('mightygrass2'));
 assert.ok(readFileSync(new URL('sitemap.xml',root),'utf8').includes('https://sshbox.constlm.xyz/'));
 assert.ok(readFileSync(new URL('robots.txt',root),'utf8').includes('Sitemap: https://sshbox.constlm.xyz/sitemap.xml'));
});
test('structured data is valid and contains no invented ratings',()=>{
 const json=JSON.parse(html.match(/<script type="application\/ld\+json">(.*?)<\/script>/s)[1]);
 assert.equal(json['@type'],'SoftwareApplication');
 assert.equal(json.name,'sshbox');
 assert.equal(json.offers.price,'0');
 assert.equal(json.aggregateRating,undefined);
});
test('local assets exist and no QA-only files ship',()=>{
 for(const match of html.matchAll(/(?:src|href)="((?:assets\/|style\.css|app\.js)[^"]*)"/g)) assert.ok(existsSync(new URL(match[1],root)),match[1]);
 assert.ok(existsSync(new URL('assets/social.png',root)));
 assert.ok(!existsSync(new URL('_mobile-check.html',root)));
});
test('every published platform has a download link without JavaScript',()=>{
 const release=JSON.parse(readFileSync(new URL('release.json',root),'utf8'));
 for(const a of release.assets.filter(a=>/_(darwin|linux|windows)_/.test(a.name))) assert.ok(html.includes(a.browser_download_url),a.name);
});
