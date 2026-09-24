<script lang="ts">
  import { FileText, Images, LayoutGrid } from 'lucide-svelte'
  import { save, open } from '@tauri-apps/plugin-dialog'
  import { app, sb, fileUrl, presetName } from '../lib/sb.svelte'
  import Seg from '../ui/Seg.svelte'

  let kind = $state<'pdf' | 'png' | 'sheet'>('pdf')
  let range = $state<'all' | 'current' | 'range'>('all')
  let from = $state(1), to = $state(app.pages.length)
  let dpi = $state('')
  let cropMarks = $state(false)
  let pageNumbers = $state(true)
  let notes = $state(false)
  let busy = $state(false)
  let done = $state('')
  let err = $state('')

  const pagesArg = $derived(range === 'all' ? null : range === 'current' ? `${app.pageNo}` : `${Math.min(from, to)}-${Math.max(from, to)}`)
  const count = $derived(range === 'all' ? app.pages.length : range === 'current' ? 1 : Math.abs(to - from) + 1)
  const first = $derived(range === 'range' ? Math.min(from, to) : range === 'current' ? app.pageNo : 1)
  const title = app.project!.title
  const suffix = $derived(range === 'all' ? '' : range === 'current' ? ` p${app.pageNo}` : ` p${Math.min(from, to)}-${Math.max(from, to)}`)
  let name = $state('')
  $effect(() => { name = `${title}${suffix}${kind === 'pdf' ? '.pdf' : kind === 'sheet' ? ' contact sheet.png' : ''}` })

  async function go() {
    err = ''
    const dir = app.project!.dir
    let out: string | null
    if (kind === 'png') {
      const d = await open({ directory: true, defaultPath: dir })
      out = typeof d === 'string' ? `${d}/${name}` : null
    } else {
      out = await save({ defaultPath: `${dir}/exports/${name}`, filters: [{ name: kind === 'pdf' ? 'PDF' : 'PNG', extensions: [kind === 'pdf' ? 'pdf' : 'png'] }] })
    }
    if (!out) return
    busy = true
    try {
      const pages = pagesArg ? ['--pages', pagesArg] : []
      const opts = [...(notes ? ['--include-notes'] : []), ...(pageNumbers ? ['--page-numbers'] : [])]
      const pdfOpts = [...(dpi ? ['--dpi', dpi] : []), ...(cropMarks ? ['--crop-marks'] : [])]
      if (kind === 'sheet') await sb(['pages', 'contact-sheet', '--out', out, ...pages])
      else if (kind === 'pdf') await sb(['export', 'pages', '--pdf', '--out', out, ...pages, ...opts, ...pdfOpts])
      else await sb(['export', 'pages', '--png', '--out', out, ...pages, ...opts])
      done = out
    } catch (e: any) {
      err = e.message
    } finally {
      busy = false
    }
  }
</script>

<div class="dim">
  <div class="sheet">
    <h2>Export “{title}”</h2>
    <p class="small muted sub">{app.pages.length} pages · {app.boards.length} boards · {presetName(app.pageSize)} · {app.reading === 'rtl' ? 'right-to-left' : 'left-to-right'}</p>
    <div class="kinds">
      <button class:on={kind === 'pdf'} onclick={() => (kind = 'pdf')}><FileText size={20} /><span><b>PDF</b><span class="small">One page per sheet</span></span></button>
      <button class:on={kind === 'png'} onclick={() => (kind = 'png')}><Images size={20} /><span><b>PNG</b><span class="small">One image per page</span></span></button>
      <button class:on={kind === 'sheet'} onclick={() => (kind = 'sheet')}><LayoutGrid size={20} /><span><b>Contact Sheet</b><span class="small">All pages on one sheet</span></span></button>
    </div>
    <div class="body">
      <div class="preview">
        <img src={fileUrl(app.pages[first - 1]?.posterframe)} alt="" />
        <span class="small muted">Page {first} of {count}</span>
      </div>
      <div class="form">
        <span class="lbl">Pages:</span>
        <div class="inline">
          <Seg value={range} options={[['all', 'All'], ['current', 'Current'], ['range', 'Range']]} onchange={(v) => (range = v)} />
          {#if range === 'range'}
            <input class="field n" type="number" min="1" max={app.pages.length} bind:value={from} /> <span class="muted">to</span>
            <input class="field n" type="number" min="1" max={app.pages.length} bind:value={to} />
          {/if}
        </div>
        {#if kind === 'pdf'}
          <span class="lbl">Resolution:</span>
          <select class="popup" bind:value={dpi}>
            <option value="">Page pixels ({app.pageSize[0]} × {app.pageSize[1]})</option>
            <option value="300">300 dpi</option><option value="600">600 dpi — print</option><option value="1200">1200 dpi</option>
          </select>
        {/if}
        {#if kind !== 'sheet'}
          <span class="lbl">Include:</span>
          <div class="checks">
            <label><input type="checkbox" bind:checked={notes} /> Notes layer</label>
            {#if kind === 'pdf'}<label><input type="checkbox" bind:checked={cropMarks} /> Crop marks (0.5 in margin)</label>{/if}
            <label><input type="checkbox" bind:checked={pageNumbers} /> Page numbers in footer</label>
          </div>
        {/if}
        <span class="lbl">Save as:</span><input class="field" bind:value={name} />
      </div>
    </div>
    {#if err}<p class="err small">{err}</p>{/if}
    {#if done}<p class="ok small">Exported to {done}</p>{/if}
    <div class="actions">
      <span class="small muted">{kind === 'pdf' ? 'PDF' : kind === 'png' ? 'PNG' : 'Contact sheet'} · {count} {count === 1 ? 'page' : 'pages'}</span>
      <span class="grow"></span>
      <button class="btn" onclick={() => (app.sheet = null)}>{done ? 'Close' : 'Cancel'}</button>
      <button class="btn primary" disabled={busy} onclick={go}>{busy ? 'Exporting…' : 'Export…'}</button>
    </div>
  </div>
</div>

<style>
  .sheet { width: 660px; }
  .sub { margin: 0 0 16px; }
  .kinds { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
  .kinds button { display: flex; align-items: center; gap: 12px; height: 54px; padding: 0 14px; border-radius: 10px; background: var(--sb-group); text-align: left; box-shadow: inset 0 0 0 0.5px var(--sb-separator); }
  .kinds b { display: block; font-weight: 600; }
  .kinds .small { color: var(--sb-label-2); }
  .kinds button.on { background: #0a84ff33; box-shadow: inset 0 0 0 1.5px var(--sb-accent); color: var(--sb-accent); }
  .kinds button.on b, .kinds button.on .small { color: var(--sb-accent); }
  .body { display: flex; gap: 24px; margin-top: 18px; }
  .preview { width: 164px; flex: none; background: #0003; border-radius: 10px; padding: 12px; display: flex; flex-direction: column; align-items: center; gap: 8px; }
  .preview img { width: 130px; box-shadow: 0 2px 8px #0008; background: #fff; }
  .form { flex: 1; display: grid; grid-template-columns: 90px 1fr; gap: 12px 10px; align-content: start; }
  .lbl { text-align: right; padding-top: 4px; }
  .popup { justify-self: start; }
  .checks { display: flex; flex-direction: column; gap: 6px; padding-top: 3px; }
  .checks label { display: flex; align-items: center; gap: 6px; }
  .checks input { accent-color: var(--sb-accent); margin: 0; }
  .inline { display: flex; align-items: center; gap: 8px; }
  .n { width: 56px; }
  .actions { display: flex; align-items: center; gap: 10px; margin-top: 20px; }
  .grow { flex: 1; }
  .err { color: var(--sb-red); }
  .ok { color: var(--sb-green); word-break: break-all; }
</style>
