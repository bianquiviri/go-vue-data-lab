import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), vueDevTools(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // Caddy (https://frontgodata.josebianco.local:8443) proxya hacia este dev
    // server: hay que escuchar en todas las interfaces y permitir el Host
    // de Caddy, o Vite responde 403 (host-check) y el contenedor no conecta.
    // El 5173 está ocupado por taskflow-node, por eso el lab usa 5174.
    // strictPort: si algo ocupa el puerto, falla en vez de saltar a otro y
    // dejar a Caddy proxyando a un servicio equivocado.
    port: 5174,
    strictPort: true,
    host: true,
    allowedHosts: [
      'frontgodata.josebianco.local',
      'apidatago.josebianco.local',
      'localhost',
      '127.0.0.1',
    ],
    // El dev server proxya la API y el swagger al backend Go (además del CORS
    // ya configurado en el backend para los orígenes lab).
    proxy: {
      '/api': {
        target: process.env.VITE_BACKEND_TARGET ?? 'http://localhost:8080',
        changeOrigin: true,
      },
      '/swagger': {
        target: process.env.VITE_BACKEND_TARGET ?? 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
