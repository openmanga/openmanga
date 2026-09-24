/** @type {import("@sveltejs/vite-plugin-svelte").SvelteConfig} */
export default {
  // a desktop drawing app: SVG handles and canvases take pointer input directly
  compilerOptions: { warningFilter: (w) => !w.code.startsWith('a11y') },
}
