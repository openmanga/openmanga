<script lang="ts">
  import { app, edit, setPage, loadLayerInfo } from '../lib/sb.svelte'
  import { tools, selectTool, matches, keyFor, cur, type ToolId } from '../lib/tools.svelte'
  import { undo, redo } from '../lib/history.svelte'
  import { openSettings } from '../lib/windows'
  import { pref } from '../lib/prefs.svelte'
  import Sidebar from './Sidebar.svelte'
  import Toolbar from './Toolbar.svelte'
  import Inspector from './Inspector.svelte'
  import BoardView from './BoardView.svelte'
  import PageView from './PageView.svelte'
  import SpreadView from './SpreadView.svelte'
  import GridView from './GridView.svelte'
  import ExportSheet from './ExportSheet.svelte'
  import ImportSheet from './ImportSheet.svelte'

  const TOOL_KEYS: ToolId[] = ['light-pencil', 'brush', 'tone', 'pencil', 'pen', 'note-pen', 'eraser']

  function key(e: KeyboardEvent) {
    const t = e.target as HTMLElement
    if (t.closest?.('input,textarea,select') || app.sheet) return
    const k = (c: string) => matches(e, keyFor(c))
    if (k('menu:edit:undo')) { e.preventDefault(); undo(); return }
    if (k('menu:edit:redo')) { e.preventDefault(); redo(); return }
    if (e.metaKey && e.key === ',') { e.preventDefault(); openSettings(); return }
    if (e.metaKey && e.key === 'e') { e.preventDefault(); app.sheet = 'export'; return }
    if (k('menu:file:import-images')) { e.preventDefault(); app.sheet = 'import'; return }
    for (const id of TOOL_KEYS) if (k('menu:tools:' + id)) { selectTool(id); app.mode = 'draw'; app.popover = null; return }
    if (k('drawing:marquee-mode')) { selectTool('lasso'); app.mode = 'draw'; return }
    if (app.view !== 'board') {
      if (k('menu:tools:panel')) { app.view = 'page'; app.mode = app.mode === 'panel' ? 'draw' : 'panel'; return }
      if (k('menu:tools:balloon')) { app.view = 'page'; app.mode = app.mode === 'balloon' ? 'draw' : 'balloon'; return }
    }
    for (let i = 0; i < 3; i++) if (k(`menu:tools:palette-color-${i + 1}`)) { cur().color = cur().palette[i]; return }
    if (k('drawing:brush-size:inc')) { cur().size = Math.min(256, Math.round(cur().size * 1.25 + 0.5)); return }
    if (k('drawing:brush-size:dec')) { cur().size = Math.max(1, Math.round(cur().size / 1.25)); return }
    if (k('menu:view:onion-skin') && app.view === 'board') { app.onion = !app.onion; return }
    if (app.view === 'board') {
      if (k('menu:navigation:next-board') && app.boardNo < app.boards.length) { app.boardNo++; return }
      if (k('menu:navigation:previous-board') && app.boardNo > 1) { app.boardNo--; return }
      if (k('menu:boards:new-board')) { edit(['board', 'add', '--after', app.boardNo]).then(() => app.boardNo++); return }
    }
    if (app.view === 'page' && !app.focusPanel && !e.metaKey && (e.key === 'ArrowLeft' || e.key === 'ArrowRight')) {
      const fwd = (e.key === 'ArrowLeft') === (app.reading === 'rtl')
      const n = app.pageNo + (fwd ? 1 : -1)
      if (n >= 1 && n <= app.pages.length) setPage(n)
    }
  }

  $effect(() => {
    void app.view; void app.boardNo; void app.page?.id
    loadLayerInfo()
  })

  $effect(() => {
    if (!app.error) return
    const t = setTimeout(() => (app.error = ''), 6000)
    return () => clearTimeout(t)
  })

  // light is an alternate appearance for the editor only; the welcome window stays dark
  $effect(() => {
    const ap = pref<string>('appearance')
    const light = ap === 'light' || (ap === 'auto' && matchMedia('(prefers-color-scheme: light)').matches)
    document.documentElement.dataset.appearance = light ? 'light' : 'dark'
    return () => (document.documentElement.dataset.appearance = 'dark')
  })
</script>

<svelte:window onkeydown={key} />

<div class="editor" class:desk-light={pref('deskColor') === 'light'} class:desk-dark={pref('deskColor') === 'dark'}>
  <main style="left:{app.sidebar ? 248 : 8}px; right:{app.inspector && app.view !== 'grid' ? 296 : 8}px" class:grid={app.view === 'grid'}>
    {#if app.view === 'board'}
      {#key app.boardNo}<BoardView />{/key}
    {:else if app.view === 'spread'}
      <SpreadView />
    {:else if app.view === 'grid'}
      <GridView />
    {:else if app.page}
      <PageView />
    {/if}
  </main>
  {#if app.sidebar}<Sidebar />{/if}
  <Toolbar />
  {#if app.inspector && app.view !== 'grid'}<Inspector />{/if}
  {#if app.sheet === 'export'}<ExportSheet />{/if}
  {#if app.sheet === 'import'}<ImportSheet />{/if}
  {#if app.error}
    <button class="toast glass" onclick={() => (app.error = '')}>{app.error}</button>
  {/if}
</div>

<style>
  .editor { position: absolute; inset: 0; background: var(--sb-desk); }
  .editor.desk-light { --sb-desk: #D6D6D8; }
  .editor.desk-dark { --sb-desk: #1A1A1B; }
  main { position: absolute; top: 60px; bottom: 0; }
  main.grid { bottom: 8px; }
  .toast { position: fixed; left: 50%; bottom: 80px; transform: translateX(-50%); padding: 10px 16px; border-radius: 12px; color: var(--sb-red); max-width: 520px; z-index: 60; text-align: left; }
</style>
