<script lang="ts">
  import {
    PencilLine, Brush, Grip, Pencil, PenTool, Highlighter, Eraser, Lasso, LayoutPanelLeft, MessageCircle,
    ZoomIn, File, BookOpen, LayoutGrid, Share, PanelRight, PanelLeft, Undo2, Redo2, Layers,
  } from 'lucide-svelte'
  import { app, edit, curBoard, boardTitle, type View } from '../lib/sb.svelte'
  import { tools, selectTool, TOOL_NAMES, type ToolId } from '../lib/tools.svelte'
  import { hist, undo, redo } from '../lib/history.svelte'
  import Toggle from '../ui/Toggle.svelte'

  const TOOL_ICONS: [ToolId, any][] = [
    ['light-pencil', PencilLine], ['brush', Brush], ['tone', Grip], ['pencil', Pencil],
    ['pen', PenTool], ['note-pen', Highlighter], ['eraser', Eraser], ['lasso', Lasso],
  ]
  const TEMPLATES = [['splash', 'Splash'], ['2-tier', '2-tier'], ['3-tier', '3-tier'], ['4-koma', '4-koma'], ['big-plus-2', 'Big + 2'], ['grid-3x2', 'Grid']]

  const board = $derived(curBoard())
  const title = $derived(app.view === 'board' ? boardTitle(board) : app.project?.title)
  const focus = $derived(app.page?.panels.find((p) => p.id === app.focusPanel))
  const subtitle = $derived.by(() => {
    if (app.view === 'board') return board ? `Board ${app.boardNo} of ${app.boards.length} · ${board.size[0]} × ${board.size[1]} px` : ''
    if (app.view === 'grid') return `All pages · ${app.pages.length}`
    if (app.view === 'spread') return `Spread ${[...(app.pages[app.pageNo - 1]?.spread ?? [])].join('–')}`
    if (focus) return `Page ${app.pageNo} · Panel ${focus.order} of ${app.page?.panels.length}`
    return `Page ${app.pageNo}` + (app.mode === 'panel' ? ' · Layout' : app.mode === 'balloon' ? ' · Balloon' : '')
  })
  const drawingOff = $derived(app.view === 'grid' || app.view === 'spread')

  function pickTool(t: ToolId) {
    selectTool(t)
    app.mode = 'draw'
    app.popover = null
    if (app.view === 'spread' || app.view === 'grid') app.view = 'page'
  }
  function pickMode(m: 'panel' | 'balloon') {
    if (app.view !== 'page') app.view = 'page'
    if (m === 'panel') app.popover = app.mode === 'panel' && app.popover ? null : 'templates'
    else app.popover = null
    app.mode = app.mode === m ? 'draw' : m
    if (app.mode !== 'panel') app.popover = null
  }
  function setView(v: View) {
    app.view = v
    app.focusPanel = null
    app.popover = null
  }

  let gutter = $state('')
  let bleed = $state(false)
  async function applyTemplate(id: string) {
    const n = app.pageNo
    await edit(['page', 'template', n, id, ...(gutter ? ['--gutter', gutter] : [])])
    if (bleed) for (const k of app.page?.panels ?? []) await edit(['panel', 'set', n, k.id, '--bleed', 'true'])
    app.popover = null
  }
</script>

<header class="toolbar" data-tauri-drag-region style="left:{app.sidebar ? 248 : 0}px">
  <div class="title" data-tauri-drag-region style="padding-left:{app.sidebar ? 8 : 84}px">
    {#if !app.sidebar}
      <button class="icon-btn show-sb" title="Show sidebar" onclick={() => (app.sidebar = true)}><PanelLeft size={17} strokeWidth={1.5} /></button>
    {/if}
    <div data-tauri-drag-region>
      <div class="t" data-tauri-drag-region>{title}</div>
      <div class="s small muted" data-tauri-drag-region>{subtitle}</div>
    </div>
  </div>

  <div class="center">
    <div class="capsule glass" class:faded={drawingOff}>
      {#each TOOL_ICONS as [id, I]}
        <button class="icon-btn" class:on={tools.current === id && app.mode === 'draw' && !drawingOff}
          title="{TOOL_NAMES[id]}" onclick={() => pickTool(id)}><I size={17} strokeWidth={1.5} /></button>
      {/each}
    </div>
    {#if app.view !== 'board'}
      <div class="capsule glass mode">
        <button class="icon-btn" class:on={app.mode === 'panel' && !drawingOff} title="Panels (P)" onclick={() => pickMode('panel')}><LayoutPanelLeft size={17} strokeWidth={1.5} /></button>
        <button class="icon-btn" class:on={app.mode === 'balloon' && !drawingOff} title="Balloons (B)" onclick={() => pickMode('balloon')}><MessageCircle size={17} strokeWidth={1.5} /></button>
        {#if app.popover === 'templates'}
          <div class="popover glass">
            <div class="ph"><b>Page Templates</b><span class="small muted">Replaces panels on page {app.pageNo}</span></div>
            <div class="grid">
              {#each TEMPLATES as [id, name]}
                <button class="tpl" onclick={() => applyTemplate(id)}>
                  <img src="/tpl-{id.startsWith('grid') ? 'grid' : id}.svg" alt="" /><span>{name}</span>
                </button>
              {/each}
            </div>
            <div class="pf">
              <span class="muted">Gutter</span>
              <select class="popup" bind:value={gutter}>
                <option value="">Default</option>
                {#each [12, 24, 36, 48, 64] as g}<option value={String(g)}>{g} px</option>{/each}
              </select>
              <span class="muted" style="margin-left:12px">Bleed</span>
              <Toggle on={bleed} onchange={(v) => (bleed = v)} />
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </div>

  <div class="right">
    {#if app.view === 'board'}
      <div class="capsule glass">
        <button class="icon-btn" title="Undo (⌘Z)" disabled={!hist.canUndo} onclick={undo}><Undo2 size={17} strokeWidth={1.5} /></button>
        <button class="icon-btn" title="Redo (⇧⌘Z)" disabled={!hist.canRedo} onclick={redo}><Redo2 size={17} strokeWidth={1.5} /></button>
      </div>
      <div class="capsule glass">
        <button class="icon-btn label" title="Zoom to fit" onclick={() => app.fitRequest++}><ZoomIn size={17} strokeWidth={1.5} /> {Math.round(app.zoom * 100)}%</button>
      </div>
      <div class="capsule glass">
        <button class="icon-btn label" class:onion={app.onion} title="Onion skin (O)" onclick={() => (app.onion = !app.onion)}><Layers size={17} strokeWidth={1.5} /> Onion Skin</button>
      </div>
    {:else}
      <div class="capsule glass">
        <button class="icon-btn label" onclick={() => app.fitRequest++}><ZoomIn size={17} strokeWidth={1.5} /> Fit</button>
      </div>
      <div class="capsule glass">
        <button class="icon-btn" class:on2={app.view === 'page'} title="Page" onclick={() => setView('page')}><File size={17} strokeWidth={1.5} /></button>
        <button class="icon-btn" class:on2={app.view === 'spread'} title="Spread" onclick={() => setView('spread')}><BookOpen size={17} strokeWidth={1.5} /></button>
        <button class="icon-btn" class:on2={app.view === 'grid'} title="All pages" onclick={() => setView('grid')}><LayoutGrid size={17} strokeWidth={1.5} /></button>
      </div>
    {/if}
    <div class="capsule glass">
      <button class="icon-btn label" onclick={() => (app.sheet = 'export')}><Share size={17} strokeWidth={1.5} /> Export</button>
    </div>
    <div class="capsule glass round">
      <button class="icon-btn" class:acc={app.inspector} title="Inspector" onclick={() => (app.inspector = !app.inspector)}><PanelRight size={17} strokeWidth={1.5} /></button>
    </div>
  </div>
</header>

<style>
  .toolbar { position: absolute; top: 0; right: 0; height: 60px; display: flex; align-items: center; justify-content: space-between; padding-right: 8px; z-index: 4; }
  .title { display: flex; align-items: center; gap: 8px; min-width: 0; }
  .t { font-weight: 700; font-size: 13px; }
  .s { margin-top: 1px; }
  .show-sb { color: var(--sb-label-2); }
  .center { position: fixed; left: 50%; top: 10px; transform: translateX(-50%); display: flex; gap: 10px; }
  .faded { opacity: 0.45; }
  .mode { position: relative; }
  .right { display: flex; gap: 10px; align-items: center; }
  .round { padding: 4px; }
  .round .icon-btn { min-width: 32px; padding: 0; }
  .icon-btn.acc { color: var(--sb-accent); }
  .icon-btn.on2 { background: var(--sb-control); }
  .icon-btn.onion { background: var(--sb-accent-soft); color: var(--sb-accent); }
  .popover { position: absolute; top: 50px; left: -180px; width: 392px; border-radius: 16px; padding: 14px; z-index: 20; background: color-mix(in srgb, var(--sb-popover) 94%, transparent); }
  .ph { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 12px; }
  .grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
  .tpl { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 10px 6px; border-radius: 10px; font-size: 12px; }
  .tpl:hover { background: var(--sb-accent-soft); color: var(--sb-accent); box-shadow: inset 0 0 0 1px var(--sb-accent); }
  .tpl img { width: 64px; height: 88px; box-shadow: 0 1px 3px #0005; }
  .pf { display: flex; align-items: center; gap: 8px; border-top: 0.5px solid var(--sb-separator); margin-top: 12px; padding-top: 12px; }
</style>
