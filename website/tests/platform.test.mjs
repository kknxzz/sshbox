import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {detectPlatform, parseAssets} from '../dist/platform.js';

test('detects 64-bit Linux and Windows user agents', () => {
  assert.deepEqual(detectPlatform('Mozilla/5.0 (X11; Linux x86_64)'), {os:'linux',arch:'amd64',certain:true});
  assert.deepEqual(detectPlatform('Mozilla/5.0 (Windows NT 10.0; Win64; x64)'), {os:'windows',arch:'amd64',certain:true});
  assert.deepEqual(detectPlatform('Mozilla/5.0 (X11; Linux aarch64)'), {os:'linux',arch:'arm64',certain:true});
});
test('does not claim that a Mac legacy Intel user-agent identifies its chip', () => {
  assert.deepEqual(detectPlatform('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)'), {os:'darwin',arch:null,certain:false});
});
test('uses architecture client hints where supported', () => {
  assert.deepEqual(detectPlatform('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)', {platform:'macOS',architecture:'arm',bitness:'64'}), {os:'darwin',arch:'arm64',certain:true});
});
test('does not offer desktop downloads as a detected mobile build', () => {
  for (const ua of ['Mozilla/5.0 (iPhone; CPU iPhone OS)', 'Mozilla/5.0 (Linux; Android 15; arm64)']) assert.equal(detectPlatform(ua).os,null);
  assert.equal(detectPlatform('Mozilla/5.0 (Macintosh; Intel Mac OS X)',{platform:'macOS',touchPoints:5}).os,null);
});
test('identifies unsupported 32-bit environments', () => {
  assert.equal(detectPlatform('Mozilla/5.0 (X11; Linux i686)').unsupported,true);
  assert.equal(detectPlatform('Windows',{platform:'Windows',architecture:'x86',bitness:'32'}).unsupported,true);
});
test('uses the actual five published binary assets and excludes sources/checksums', () => {
  const release=JSON.parse(readFileSync(new URL('../dist/release.json',import.meta.url)));
  const assets=parseAssets(release);
  assert.equal(assets.length,5);
  assert.deepEqual(assets.map(a=>`${a.os}/${a.arch}`).sort(), ['darwin/amd64','darwin/arm64','linux/amd64','linux/arm64','windows/amd64']);
});
test('ignores malformed metadata and download URLs outside the repository', () => {
  assert.deepEqual(parseAssets({assets:[{name:'sshbox_2.0.0_linux_amd64.tar.gz',browser_download_url:'javascript:alert(1)'},{name:'sshbox_2.0.0_linux_amd64.tar.gz',browser_download_url:'https://example.com/download'}]}),[]);
});
