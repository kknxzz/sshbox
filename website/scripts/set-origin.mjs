// Set the canonical origin before moving the page to another host or domain.
import {readFileSync,writeFileSync} from 'node:fs';
const url = new URL(process.argv[2]);
if(url.protocol !== 'https:' || url.pathname !== '/' || url.search || url.hash) throw Error('Use an HTTPS origin, e.g. https://sshbox.example');
const files=['index.html','robots.txt','sitemap.xml'];
const homepage=new URL('../dist/index.html',import.meta.url);
const old=readFileSync(homepage,'utf8').match(/rel="canonical" href="(https:\/\/[^"/]+)\//)?.[1];
if(!old) throw Error('Missing canonical URL');
for(const name of files) {
 const path=new URL(`../dist/${name}`,import.meta.url);
 writeFileSync(path,readFileSync(path,'utf8').replaceAll(old,url.origin));
}
console.log(`Canonical, sitemap, and social URLs now use ${url.origin}`);
