import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 後台掛在 /admin，正式環境由 Firebase Hosting 跟公開站放在同一個網域。
//
// 本機開發時把 /api 與 /images 轉給 Go server（:8080）。瀏覽器看到的永遠
// 是同源，所以後端不需要處理 CORS——開發和正式環境的行為一致，也不會因為
// 某天忘了關掉 CORS 而在線上放寬瀏覽器的保護。
export default defineConfig({
  base: '/admin/',
  plugins: [react()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': 'http://localhost:8080',
      '/images': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
  },
})
