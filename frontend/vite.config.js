import path from 'path'
import {defineConfig} from 'vite'
import {svelte, vitePreprocess} from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'
import wails from '@wailsio/runtime/plugins/vite'

export default defineConfig({
  base: './',
  optimizeDeps: {
    include: ['@wailsio/runtime', '@lucide/svelte']
  },
  resolve: {
    alias: {
      $lib: path.resolve('./src/lib')
    }
  },
  plugins: [
    tailwindcss(),
    svelte({
      preprocess: vitePreprocess(),
      compilerOptions: {
        runes: true
      }
    }),
    wails('./bindings')
  ]
})
