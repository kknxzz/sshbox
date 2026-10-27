import { detectPlatform, parseAssets } from './platform.js';
const REPO = 'https://api.github.com/repos/kknxzz/sshbox';
const OS_NAMES = { darwin: 'macOS', linux: 'Linux', windows: 'Windows' };
const archLabels = {darwin: {arm64: 'Apple Silicon (M1 or later)', amd64: 'Intel Mac'}, linux: {amd64: 'x64 (Intel / AMD)', arm64: 'ARM64 (64-bit Raspberry Pi, etc.)'}, windows: {amd64: 'x64 (Intel / AMD)', arm64: 'ARM64'}};
let release = await fetch('./release.json').then(r => { if (!r.ok) throw Error(); return r.json(); }).catch(() => null);
let assets = release ? parseAssets(release) : [];
let detected = detectPlatform(navigator.userAgent);
let selectedOS = detected.os || 'darwin';
let selectedArch = detected.arch || (selectedOS === 'darwin' ? 'arm64' : 'amd64');
let manuallySelected = false;
const osPicker = document.getElementById('os-picker');
const archPicker = document.getElementById('arch-picker');
const heroLink = document.getElementById('hero-download');
const downloadLink = document.getElementById('selected-download');

function updateDownload() {
  const valid = assets.filter(a => a.os === selectedOS);
  if (valid.length) {
    if (!valid.some(a => a.arch === selectedArch)) selectedArch = valid[0].arch;
    archPicker.replaceChildren(...valid.map(a => new Option(archLabels[selectedOS]?.[a.arch] || a.arch, a.arch)));
  }
  osPicker.value = selectedOS;
  archPicker.value = selectedArch;
  const asset = assets.find(a => a.os === selectedOS && a.arch === selectedArch);
  downloadLink.href = asset?.url || 'https://github.com/kknxzz/sshbox/releases/latest';
  downloadLink.querySelector('span').textContent = asset ? 'Download sshbox' : 'View releases';
  document.getElementById('asset-name').textContent = asset?.name || 'Choose a build from GitHub Releases.';
  document.getElementById('run-command').textContent = selectedOS === 'windows' ? '.\\bin\\sshbox.exe' : './bin/sshbox';
  document.getElementById('windows-note').hidden = selectedOS !== 'windows';
  const missingDetectedBuild = detected.certain && !assets.some(a => a.os === detected.os && a.arch === detected.arch);
  if (manuallySelected || (detected.os && !detected.unsupported && !missingDetectedBuild)) {
    // The selected build is always visible beside the button, including uncertain chips.
    heroLink.href = downloadLink.href;
    document.getElementById('hero-download-label').textContent = `Download for ${OS_NAMES[selectedOS]}`;
    let note = `${OS_NAMES[selectedOS]} · ${archLabels[selectedOS]?.[selectedArch] || selectedArch} · Docker required`;
    if (!manuallySelected && !detected.certain) note = `${OS_NAMES[selectedOS]} · ${selectedArch === 'arm64' ? 'Apple Silicon' : 'x64'} selected — check your processor below`;
    document.getElementById('detection-note').textContent = note;
  } else {
    heroLink.href = '#downloads';
    document.getElementById('hero-download-label').textContent = 'Choose your download';
    document.getElementById('detection-note').textContent = missingDetectedBuild ? `${OS_NAMES[detected.os]} ${detected.arch.toUpperCase()} has no published build — see other platforms` : detected.unsupported ? '64-bit builds only · choose a compatible machine below' : 'macOS, Linux & Windows · Docker required';
  }
  if (release) document.getElementById('release-version').textContent = release.tag_name;
}
function updateReleaseLinks() {
  if (!release) return;
  const list = document.getElementById('download-list');
  const nodes = assets.map(asset => {
    const a = document.createElement('a'); a.href = asset.url;
    a.append(`${OS_NAMES[asset.os]} · ${archLabels[asset.os]?.[asset.arch] || asset.arch} `);
    const ext = document.createElement('span'); ext.textContent = `${asset.name.endsWith('.zip') ? '.zip' : '.tar.gz'} ↗`; a.append(ext); return a;
  });
  const source = release.assets.find(a => /^sshbox_.*_source\.tar\.gz$/.test(a.name));
  if (source && safeAsset(source.browser_download_url)) { const a = document.createElement('a'); a.href = source.browser_download_url; a.textContent = 'Build from source ↗'; nodes.push(a); }
  list.replaceChildren(...nodes);
  const checksum = release.assets.find(a => a.name === 'checksums.txt');
  const checkLink = document.getElementById('checksums-link');
  checkLink.href = checksum && safeAsset(checksum.browser_download_url) ? checksum.browser_download_url : release.html_url;
}
function safeAsset(url) { try { const u = new URL(url); return u.origin === 'https://github.com' && u.pathname.startsWith('/kknxzz/sshbox/releases/download/'); } catch { return false; } }
osPicker.addEventListener('change', () => { manuallySelected = true; selectedOS = osPicker.value; selectedArch = selectedOS === 'darwin' ? 'arm64' : 'amd64'; updateDownload(); });
archPicker.addEventListener('change', () => { manuallySelected = true; selectedArch = archPicker.value; updateDownload(); });
updateDownload(); updateReleaseLinks();

// UA Client Hints are available in Chromium; Safari only permits OS detection.
if (navigator.userAgentData?.getHighEntropyValues) {
  navigator.userAgentData.getHighEntropyValues(['architecture', 'bitness', 'platform']).then(hints => {
    if (manuallySelected) return;
    detected = detectPlatform(navigator.userAgent, { ...hints, mobile: navigator.userAgentData.mobile, touchPoints: navigator.maxTouchPoints });
    selectedOS = detected.os || selectedOS; selectedArch = detected.arch || (selectedOS === 'darwin' ? 'arm64' : 'amd64'); updateDownload();
  }).catch(() => {});
} else if (navigator.maxTouchPoints > 1 && /Macintosh/.test(navigator.userAgent)) {
  detected = {os: null, arch: null, certain: false}; updateDownload();
}

// Refresh published assets, with a bundled snapshot when GitHub is unavailable or rate-limited.
fetch(`${REPO}/releases/latest`, {signal: AbortSignal.timeout(5000)}).then(r => { if (!r.ok) throw Error(); return r.json(); }).then(latest => {
  const next = parseAssets(latest);
  if (next.length) { release = latest; assets = next; updateDownload(); updateReleaseLinks(); }
}).catch(() => {});
fetch(REPO, {signal: AbortSignal.timeout(5000)}).then(r => { if (!r.ok) throw Error(); return r.json(); }).then(data => {
  if (Number.isInteger(data.stargazers_count)) document.getElementById('star-count').textContent = new Intl.NumberFormat('en', {notation: 'compact', maximumFractionDigits: 1}).format(data.stargazers_count);
}).catch(() => { document.getElementById('star-count').title = 'Last checked: 27 October 2026'; });

let toastTimer;
for (const button of document.querySelectorAll('[data-copy]')) button.addEventListener('click', async () => {
  const status = document.getElementById('copy-status');
  try { await navigator.clipboard.writeText(button.dataset.copy); status.textContent = 'Copied to clipboard'; }
  catch { status.textContent = 'Select the command text to copy it.'; }
  status.classList.add('visible'); clearTimeout(toastTimer); toastTimer = setTimeout(() => status.classList.remove('visible'), 2500);
});
const tabs = [...document.querySelectorAll('[role=tab]')];
function activateTab(tab) {
  for (const t of tabs) {const active = t === tab; t.setAttribute('aria-selected', active); t.tabIndex = active ? 0 : -1; document.getElementById(t.dataset.panel).hidden = !active;}
}
for (const tab of tabs) {
  tab.addEventListener('click', () => activateTab(tab));
  tab.addEventListener('keydown', e => {
    if (!['ArrowLeft','ArrowRight','Home','End'].includes(e.key)) return;
    e.preventDefault(); const i = tabs.indexOf(tab);
    const next = e.key === 'Home' ? tabs[0] : e.key === 'End' ? tabs.at(-1) : tabs[(i + (e.key === 'ArrowRight' ? 1 : tabs.length - 1)) % tabs.length];
    activateTab(next); next.focus();
  });
}
