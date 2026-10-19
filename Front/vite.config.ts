import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

// En développement, Vite sert l'interface sur :5173 et relaie /api vers le
// serveur Go (:8080), y compris les montées en WebSocket (/api/rooms/{code}/ws).
// En production, `npm run build` produit Front/dist que Go sert directement.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.{ts,tsx}'],
  },
});
