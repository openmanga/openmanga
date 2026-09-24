<script lang="ts">
  import { app } from './lib/sb.svelte'
  import { loadToolPrefs, tools } from './lib/tools.svelte'
  import { loadPrefs, followPrefs, pref } from './lib/prefs.svelte'
  import Welcome from './views/Welcome.svelte'
  import Editor from './views/Editor.svelte'

  const applySizes = () => {
    for (const [t, s] of Object.entries<number>(pref('toolSizes'))) if (tools.settings[t]) tools.settings[t].size = s
  }
  loadToolPrefs().catch(() => {})
  loadPrefs().then(applySizes).catch(() => {})
  followPrefs((k) => { if (k === 'toolSizes') applySizes() })
</script>

{#if app.project}
  <Editor />
{:else}
  <Welcome />
{/if}
