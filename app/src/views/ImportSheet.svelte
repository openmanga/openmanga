<script lang="ts">
  import { ImageDown, CircleX, GripVertical, ArrowRight } from 'lucide-svelte'
  import { open } from '@tauri-apps/plugin-dialog'
  import { getCurrentWebview } from '@tauri-apps/api/webview'
  import { convertFileSrc } from '@tauri-apps/api/core'
  import { app, edit, setPage } from '../lib/sb.svelte'

  let files = $state<{ path: string; dims: string }[]>([])
  let over = $state(false)
  let target = $state<'pages' | 'panel' | 'boards'>(app.selPanel ? 'panel' : 'pages')
  let fit = $state<'fit' | 'fill'>('fit')
  let busy = $state(false)
  let err = $state('')

  const after = app.pageNo
  const panel = $derived(app.page?.panels.find((p) => p.id === (app.selPanel ?? app.focusPanel)) ?? app.page?.panels.slice().sort((a, b) => a.order - b.order)[0])
  const name = (p: string) => p.split('/').pop()!

  function add(paths: string[]) {
    for (const p of paths) {
      if (files.some((f) => f.path === p)) continue
      const f = { path: p, dims: '' }
      files.push(f)
      if (/\.(png|jpe?g)$/i.test(p)) {
        const img = new Image()
        img.onload = () => { const x = files.find((y) => y.path === p); if (x) x.dims = `${img.naturalWidth} × ${img.naturalHeight}` }
        img.src = convertFileSrc(p)
      } else f.dims = 'folder'
    }
  }

  ;(window as any).__importAdd = add // dev UI script hook (same path as a drop)

  $effect(() => {
    const un = getCurrentWebview().onDragDropEvent((e) => {
      if (e.payload.type === 'over' || e.payload.type === 'enter') over = true
      else if (e.payload.type === 'leave') over = false
      else if (e.payload.type === 'drop') { over = false; add(e.payload.paths) }
    })
    return () => { un.then((f) => f()) }
  })

  async function choose() {
    const f = await open({ multiple: true, filters: [{ name: 'Images', extensions: ['png', 'jpg', 'jpeg'] }] })
    if (Array.isArray(f)) add(f)
  }

  async function run() {
    busy = true
    err = ''
    try {
      const paths = files.map((f) => f.path)
      if (target === 'pages') {
        await edit(['import', 'images', ...paths, '--pages', '--after', after, '--fit', fit])
        await setPage(after + 1)
      } else if (target === 'panel' && panel) {
        await edit(['import', 'images', paths[0], '--page', app.pageNo, '--panel', panel.id])
      } else {
        await edit(['import', 'images', ...paths])
      }
      app.sheet = null
    } catch (e: any) {
      err = e.message
    } finally {
      busy = false
    }
  }
</script>

<div class="dim">
  <div class="sheet">
    <h2>Import Images</h2>
    <p class="small muted sub">Scans, photos or sketches become pages, or go into a panel as reference.</p>

    <button class="drop" class:over onclick={choose}>
      <ImageDown size={26} strokeWidth={1.5} />
      <b>Drop images or folders here</b>
      <span class="small muted">PNG or JPG · folders are imported recursively · or <u>Choose Files…</u></span>
    </button>

    {#if files.length}
      <div class="list">
        {#each files as f, i (f.path)}
          <div class="file">
            <GripVertical size={14} class="faint" />
            {#if f.dims !== 'folder'}<img src={convertFileSrc(f.path)} alt="" />{:else}<span class="ph"></span>{/if}
            <span class="fn"><span>{name(f.path)}</span><span class="small muted">{f.dims}</span></span>
            <span class="dest small muted">
              {#if target === 'pages'}<ArrowRight size={12} /> page {after + 1 + i}{:else if target === 'panel'}{i === 0 ? `→ Panel ${panel?.order}` : 'skipped'}{:else}<ArrowRight size={12} /> new board{/if}
            </span>
            <button class="x" onclick={() => files.splice(i, 1)}><CircleX size={15} /></button>
          </div>
        {/each}
      </div>
    {/if}

    <div class="form">
      <span class="lbl">Import as:</span>
      <div class="opts">
        <label><input type="radio" bind:group={target} value="pages" /> <span>New pages after page {after}<small>Each image becomes one page on its reference layer</small></span></label>
        <label class:disabled={!panel}><input type="radio" bind:group={target} value="panel" disabled={!panel} /> <span>Into Panel {panel?.order ?? '–'}’s reference layer<small>Only one image · fitted inside the panel, 75% opacity</small></span></label>
        <label><input type="radio" bind:group={target} value="boards" /> <span>New boards<small>One board per image, on its reference layer</small></span></label>
      </div>
      {#if target === 'pages'}
        <span class="lbl">Fit:</span>
        <select class="popup" bind:value={fit}><option value="fit">Fit inside page</option><option value="fill">Fill page</option></select>
      {/if}
    </div>

    {#if err}<p class="err small">{err}</p>{/if}
    <div class="actions">
      <span class="small muted">{files.length} {files.length === 1 ? 'image' : 'images'}{target === 'pages' && files.length ? ` → ${files.length} new pages after page ${after}` : ''}</span>
      <span class="grow"></span>
      <button class="btn" onclick={() => (app.sheet = null)}>Cancel</button>
      <button class="btn primary" disabled={!files.length || busy} onclick={run}>{busy ? 'Importing…' : `Import ${files.length || ''} ${files.length === 1 ? 'Image' : 'Images'}`}</button>
    </div>
  </div>
</div>

<style>
  .sheet { width: 600px; }
  .sub { margin: 0 0 16px; }
  .drop { width: 100%; height: 120px; border-radius: 12px; border: 1.5px dashed var(--sb-border); display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px; color: var(--sb-label-2); }
  .drop b { color: var(--sb-label); font-weight: 600; }
  .drop.over { border: 2px solid var(--sb-accent); background: #0a84ff33; color: var(--sb-accent); }
  .drop.over b { color: var(--sb-accent); }
  .list { margin-top: 14px; border-radius: 10px; background: var(--sb-group); max-height: 240px; overflow-y: auto; }
  .file { display: flex; align-items: center; gap: 10px; height: 56px; padding: 0 12px; }
  .file + .file { border-top: 0.5px solid var(--sb-separator); }
  .file img, .ph { width: 30px; height: 40px; object-fit: cover; background: #fff; }
  .fn { flex: 1; display: flex; flex-direction: column; min-width: 0; }
  .fn span:first-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .dest { display: flex; align-items: center; gap: 4px; }
  .x { color: var(--sb-label-3); display: flex; }
  .form { display: grid; grid-template-columns: 90px 1fr; gap: 12px 10px; margin-top: 16px; align-items: start; }
  .lbl { text-align: right; padding-top: 2px; }
  .opts { display: flex; flex-direction: column; gap: 10px; }
  .opts label { display: flex; gap: 8px; align-items: flex-start; }
  .opts label.disabled { opacity: 0.45; }
  .opts small { display: block; color: var(--sb-label-2); font-size: 11px; }
  .opts input { accent-color: var(--sb-accent); margin-top: 2px; }
  .popup { justify-self: start; }
  .actions { display: flex; align-items: center; gap: 10px; margin-top: 20px; }
  .grow { flex: 1; }
  .err { color: var(--sb-red); }
</style>
