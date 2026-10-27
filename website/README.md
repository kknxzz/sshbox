# sshbox website

Static HTML, CSS, and JavaScript. No install or build step.

From this directory:

```
python3 -m http.server 4173 --directory dist
node --test tests/*.test.mjs
```

Serve `dist/` with any static host. The download picker uses GitHub's latest release API, with `dist/release.json` as a fallback. The top-right star count refreshes from GitHub; its fallback was checked on 27 October 2026. OS/processor detection uses browser client hints when available. A Mac's legacy user-agent cannot identify its chip, so the picker stays visible.

The canonical URL, sitemap, and social metadata use https://sshbox.constlm.xyz. If you change domains, update the SEO URLs:

```
node scripts/set-origin.mjs https://your-domain.example
```

Page text, metadata, platform links, and structured data are in the initial HTML so they remain accessible without JavaScript. The site code does not include an analytics script or set tracking cookies. Cloudflare and the server may process request data as described in the privacy policy. ENDLESS is self-hosted; its source and usage notes are in `dist/assets/FONT-LICENSE.txt`.

Privacy and terms source lives in `legal-output/`. After editing either document, run `node scripts/build-legal.mjs` to update the public pages. Set a server-log retention period and review the documents against the deployed setup before publication.
