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
const platformMenu = document.querySelector('.platform-menu');
platformMenu.id = 'platform-choices';
document.getElementById('hero-download').addEventListener('click', event => { if (event.currentTarget.getAttribute('href') === '#platform-choices') { event.preventDefault(); platformMenu.open = true; platformMenu.querySelector('summary').focus(); } });
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
    if (!manuallySelected && !detected.certain) note = `${OS_NAMES[selectedOS]} · ${selectedArch === 'arm64' ? 'Apple Silicon' : 'x64'} selected — check your processor in Other platforms`;
    document.getElementById('detection-note').textContent = selectedOS === 'windows' ? `${note} · Native Windows PTY unverified` : note;
  } else {
    heroLink.href = '#platform-choices';
    document.getElementById('hero-download-label').textContent = 'Choose your download';
    document.getElementById('detection-note').textContent = missingDetectedBuild ? `${OS_NAMES[detected.os]} ${detected.arch.toUpperCase()} has no published build — choose another platform` : detected.unsupported ? '64-bit builds only · choose a compatible machine' : 'macOS, Linux & Windows · Docker required';
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
  if (Number.isInteger(data.stargazers_count) && data.stargazers_count >= 100) { const count = document.getElementById('star-count'); count.textContent = new Intl.NumberFormat('en', {notation: 'compact', maximumFractionDigits: 1}).format(data.stargazers_count); count.hidden = false; }
}).catch(() => {});

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

// Keep native selects as the no-JavaScript fallback, then enhance them.
function enhancePicker(select) {
  const field = select.closest('.picker-field');
  const label = field.querySelector('label');
  const control = document.createElement('div');
  control.className = 'custom-picker';
  const trigger = document.createElement('button');
  trigger.type = 'button';
  trigger.className = 'picker-trigger';
  trigger.id = `${select.id}-trigger`;
  label.htmlFor = trigger.id;
  trigger.setAttribute('aria-haspopup', 'listbox');
  trigger.setAttribute('aria-expanded', 'false');
  trigger.setAttribute('aria-label', label.textContent);
  const value = document.createElement('span');
  value.className = 'picker-value';
  const chevron = document.createElement('span');
  chevron.className = 'picker-chevron';
  chevron.setAttribute('aria-hidden', 'true');
  chevron.innerHTML = '<svg viewBox="0 0 20 20" fill="none"><path d="m5 7 5 5 5-5" stroke="currentColor" stroke-width="1.5"/></svg>';
  trigger.append(value, chevron);
  const list = document.createElement('div');
  list.className = 'picker-menu';
  list.id = `${select.id}-options`;
  list.setAttribute('role', 'listbox');
  list.setAttribute('aria-label', label.textContent);
  list.hidden = true;
  trigger.setAttribute('aria-controls', list.id);
  control.append(trigger, list);
  select.insertAdjacentElement('afterend', control);

  function sync() {
    value.textContent = select.selectedOptions[0]?.textContent || 'Choose an option';
    list.replaceChildren(...Array.from(select.options, option => {
      const item = document.createElement('button');
      item.type = 'button';
      item.className = 'picker-option';
      item.setAttribute('role', 'option');
      item.setAttribute('aria-selected', String(option.value === select.value));
      item.tabIndex = -1;
      const name = document.createElement('span');
      name.textContent = option.textContent;
      const check = document.createElement('span');
      check.className = 'picker-check';
      check.setAttribute('aria-hidden', 'true');
      check.textContent = '✓';
      item.append(name, check);
      item.addEventListener('click', () => {
        select.value = option.value;
        select.dispatchEvent(new Event('change', { bubbles: true }));
        close(true);
      });
      return item;
    }));
  }
  function close(returnFocus = false) {
    if (list.hidden) {
      if (returnFocus) trigger.focus();
      return;
    }
    list.hidden = true;
    trigger.setAttribute('aria-expanded', 'false');
    field.classList.remove('picker-open');
    if (returnFocus) trigger.focus();
  }
  function open(focusEdge) {
    document.querySelectorAll('.picker-field.picker-open .picker-trigger').forEach(other => {
      if (other !== trigger) other.click();
    });
    sync();
    list.hidden = false;
    field.classList.add('picker-open');
    trigger.setAttribute('aria-expanded', 'true');
    const items = [...list.querySelectorAll('[role=option]')];
    const current = list.querySelector('[aria-selected=true]');
    (focusEdge === 'first' ? items[0] : focusEdge === 'last' ? items.at(-1) : current || items[0])?.focus();
  }
  trigger.addEventListener('click', () => list.hidden ? open() : close());
  trigger.addEventListener('keydown', event => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      if (list.hidden) open(event.key === 'ArrowDown' ? 'first' : 'last');
    }
  });
  list.addEventListener('keydown', event => {
    const items = [...list.querySelectorAll('[role=option]')];
    const current = items.indexOf(document.activeElement);
    if (event.key === 'Escape') { event.preventDefault(); close(true); return; }
    if (event.key === 'Tab') { close(); return; }
    if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
      event.preventDefault();
      const next = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1 : (current + (event.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length;
      items[next]?.focus();
    }
  });
  document.addEventListener('pointerdown', event => { if (!field.contains(event.target)) close(); });
  field.addEventListener('focusout', event => { if (!field.contains(event.relatedTarget)) close(); });
  select.addEventListener('change', sync);
  // Architecture options are replaced when the selected OS changes.
  new MutationObserver(sync).observe(select, { childList: true });
  sync();
  field.classList.add('enhanced');
}

enhancePicker(osPicker);
enhancePicker(archPicker);

// Leave the layout untouched while the platform popover opens and closes.
const menuSummary = platformMenu.querySelector(':scope > summary');
let closeTimer;
function closePlatformMenu(restoreFocus = false) {
  if (!platformMenu.open || platformMenu.classList.contains('is-closing')) return;
  const finish = () => {
    platformMenu.open = false;
    platformMenu.classList.remove('is-closing');
    if (restoreFocus) menuSummary.focus();
  };
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) return finish();
  platformMenu.classList.add('is-closing');
  clearTimeout(closeTimer);
  closeTimer = setTimeout(finish, 180);
}
menuSummary.addEventListener('click', event => {
  if (!platformMenu.open) return;
  event.preventDefault();
  closePlatformMenu();
});
document.addEventListener('pointerdown', event => {
  if (!platformMenu.contains(event.target)) closePlatformMenu();
});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && platformMenu.open && !platformMenu.querySelector('.picker-open')) {
    event.preventDefault(); closePlatformMenu(true);
  }
});


// Animate each section as it enters the viewport; content stays visible if
// JavaScript or IntersectionObserver is unavailable.
if ('IntersectionObserver' in window && !matchMedia('(prefers-reduced-motion: reduce)').matches) {
  const observer = new IntersectionObserver(entries => {
    for (const entry of entries) if (entry.isIntersecting) {
      entry.target.classList.add('in-view');
      observer.unobserve(entry.target);
    }
  }, { threshold: 0.12 });
  document.querySelectorAll('.section-heading, .steps article, .setup-intro, .setup-panel, .security-note, .download-heading, .download-picker, .faq > div').forEach(element => {
    element.classList.add('reveal');
    observer.observe(element);
  });
}
