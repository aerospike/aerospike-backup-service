import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { viteSingleFile } from "vite-plugin-singlefile"
import path from 'path'

export default defineConfig({
  plugins: [react(), viteSingleFile()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    assetsInlineLimit: 100000000,
    // Add this line!
    // 'inline' puts the map inside the file, perfect for your single-binary setup.
    sourcemap: 'inline',
    // Prevent minification if you want the generated code to be slightly more readable (optional)
    minify: false,
  }
})