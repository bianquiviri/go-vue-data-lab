# App icon & PWA (installable) setup

This folder holds the app icon and the Web App Manifest that make the frontend
**installable** (PWA) once the Vue app is scaffolded in `web/`.

## Files

| File | Purpose |
| --- | --- |
| `public/icon.svg` | Source icon (scalable, edit this one) |
| `public/favicon-16x16.png` | Browser tab (small) |
| `public/favicon-32x32.png` | Browser tab |
| `public/apple-touch-icon.png` | iOS "Add to Home Screen" (180x180) |
| `public/icon-192x192.png` | PWA / Android |
| `public/icon-512x512.png` | PWA / Android splash |
| `public/icon-maskable-512x512.png` | Android adaptive icon (safe zone) |
| `public/manifest.webmanifest` | Web App Manifest |

## Regenerate the PNGs

Edit `public/icon.svg`, then:

```bash
./web/scripts/generate-icons.sh
```

The script uses macOS `sips` (built-in). For a cross-platform / higher-quality
pipeline prefer [`@vite-pwa/assets-generator`](https://vite-pwa-org.netlify.app/assets-generator/):

```bash
npx @vite-pwa/assets-generator --preset minimal-2023 public/icon.svg
```

## Wire it into `web/index.html`

Add these tags inside `<head>`:

```html
<link rel="icon" type="image/svg+xml" href="/icon.svg">
<link rel="icon" type="image/png" sizes="32x32" href="/favicon-32x32.png">
<link rel="icon" type="image/png" sizes="16x16" href="/favicon-16x16.png">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<link rel="manifest" href="/manifest.webmanifest">
<meta name="theme-color" content="#0f172a">
```

## Making it truly installable

A manifest + icons are the minimum, but browsers require a few more things:

1. **HTTPS** — already covered locally by Caddy + mkcert
   (`https://godata.josebianco.local`).
2. **A service worker** — required for the install prompt. Add
   [`vite-plugin-pwa`](https://vite-pwa-org.netlify.app/) to `web/vite.config.ts`:

   ```ts
   import { VitePWA } from 'vite-plugin-pwa'

   export default defineConfig({
     plugins: [
       vue(),
       VitePWA({
         registerType: 'autoUpdate',
         manifest: false, // we ship our own manifest.webmanifest
         includeAssets: ['favicon-32x32.png', 'apple-touch-icon.png'],
       }),
     ],
   })
   ```

3. **`display: standalone`** — already set in `manifest.webmanifest`.

Once the service worker is registered and the manifest is linked, Chrome/Edge
show the "Install app" button and iOS Safari offers "Add to Home Screen".
