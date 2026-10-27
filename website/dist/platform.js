export function detectPlatform(userAgent = '', hints = {}) {
  const ua = userAgent.toLowerCase();
  const platform = (hints.platform || '').toLowerCase();
  if (/android|iphone|ipad|ipod/.test(ua) || hints.mobile || (platform === 'macos' && hints.touchPoints > 1)) return { os: null, arch: null, certain: false };
  const os = platform.includes('win') || /windows/.test(ua) ? 'windows' : /mac/.test(platform) || /macintosh|mac os x/.test(ua) ? 'darwin' : /linux/.test(platform) || /linux/.test(ua) ? 'linux' : null;
  const architecture = (hints.architecture || '').toLowerCase();
  const arch = /arm|aarch64/.test(architecture) || /aarch64|arm64/.test(ua) ? 'arm64' : /x86|amd64|x64/.test(architecture) || /win64|x64|x86_64|amd64/.test(ua) ? 'amd64' : null;
  if (hints.bitness === '32' || (!arch && /i[3-6]86|armv[5-8]l|win32/.test(ua))) return { os, arch: null, certain: false, unsupported: true };
  // macOS deliberately reports Intel in legacy user-agent strings, even on Apple Silicon.
  return { os, arch, certain: Boolean(os && arch) };
}

export function parseAssets(release) {
  const results = [];
  for (const asset of release.assets || []) {
    const match = asset.name.match(/^sshbox_[v\d][\w.-]*_(linux|darwin|windows)_(amd64|arm64)\.(tar\.gz|zip)$/);
    if (!match) continue;
    try {
      const url = new URL(asset.browser_download_url);
      if (url.origin !== 'https://github.com' || !url.pathname.startsWith('/kknxzz/sshbox/releases/download/')) continue;
      results.push({ os: match[1], arch: match[2], name: asset.name, url: url.href, size: asset.size || 0 });
    } catch { /* Ignore malformed remote metadata. */ }
  }
  return results;
}
