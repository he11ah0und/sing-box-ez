import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'
import wails from '@wailsio/runtime/plugins/vite'

export default defineConfig({
  base: './',
  plugins: [
    tailwindcss(),
    svelte({
      compilerOptions: {
        runes: true
      }
    }),
    wails('./bindings')
  ]
})
