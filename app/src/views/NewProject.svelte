<script lang="ts">
  import { FileText, Pencil, ArrowLeft, ArrowRight } from 'lucide-svelte'
  import { homeDir } from '@tauri-apps/api/path'
  import { open } from '@tauri-apps/plugin-dialog'
  import { sb, openProject, edit, PAGE_PRESETS, presetPx } from '../lib/sb.svelte'
  import Seg from '../ui/Seg.svelte'

  let { onclose, importFiles = null }: { onclose: () => void; importFiles?: string[] | null } = $props()

  let title = $state('Untitled Manga')
  let start = $state<'page' | 'board'>('page')
  let preset = $state('b5')
  let customW = $state(1414), customH = $state(2000)
  let reading = $state<'rtl' | 'ltr'>('rtl')
  let firstSingle = $state(true)
  let pages = $state(1)
  let home = $state('')
  let where = $state('')
  let otherDir = $state('')
  let busy = $state(false)
  let err = $state('')

  homeDir().then((h) => { home = h.replace(/\/$/, ''); where = `${home}/Manga` })
  sb<any>(['prefs', 'get'], undefined, false).then((p) => {
    if (p.mangaDefaultPageSize) preset = p.mangaDefaultPageSize
    if (p.mangaReadingDirection) reading = p.mangaReadingDirection
    if (p.mangaStartingPages) pages = p.mangaStartingPages
  }).catch(() => {})
  if (importFiles?.length) title = "Imported Pages"

  const size = $derived(preset === 'custom' ? [customW, customH] : presetPx(PAGE_PRESETS.find((p) => p.id === preset)!))
  const ratio = $derived((size[1] / size[0]).toFixed(3).replace(/0+$/, ''))
  const dir = $derived(`${where === 'other' ? otherDir : where}/${title.trim()}`)
  const spreadsText = $derived.by(() => {
    if (pages <= 1) return 'one page'
    const rest = firstSingle ? pages - 1 : pages
    const pairs = Math.floor(rest / 2)
    const parts = [firstSingle && 'page 1', pairs && `${pairs} ${pairs === 1 ? 'spread' : 'spreads'}`, rest % 2 && `page ${pages}`].filter(Boolean)
    return parts.join(' + ')
  })

  async function chooseDir() {
    const d = await open({ directory: true })
    if (typeof d === 'string') { otherDir = d; where = 'other' } else if (!otherDir) where = `${home}/Manga`
  }

  async function create() {
    if (!title.trim()) return
    busy = true
    err = ''
    try {
      const r = await sb<any>(['project', 'new', dir, '--manga', '--page-size', `${size[0]}x${size[1]}`, '--reading', reading,
        '--pages', Math.max(1, pages), '--first-page', firstSingle ? 'single' : 'paired'], undefined, false)
      await openProject(r.file, start === 'board' ? 'board' : 'page')
      if (importFiles?.length) await edit(['import', 'images', ...importFiles, '--pages', '--after', pages])
      onclose()
    } catch (e: any) {
      err = e.message
    } finally {
      busy = false
    }
  }
</script>

<div class="dim">
  <div class="sheet">
    <h2>New Manga Project</h2>
    <p class="small muted sub">Draw boards freely, then place them into panels on pages. Size and direction can change later.</p>

    <div class="form">
      <span class="lbl">Title:</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="field" bind:value={title} autofocus onkeydown={(e) => e.key === 'Enter' && create()} />

      <span class="lbl">Start with:</span>
      <div class="two">
        <button class="opt" class:on={start === 'page'} onclick={() => (start = 'page')}>
          <span class="oi"><FileText size={16} /></span><span><b>Blank page</b><span class="small">Lay out panels first, then place drawings</span></span>
        </button>
        <button class="opt" class:on={start === 'board'} onclick={() => (start = 'board')}>
          <span class="oi"><Pencil size={16} /></span><span><b>Blank board</b><span class="small">Just draw; build pages from boards later</span></span>
        </button>
      </div>

      <span class="lbl">Page size:</span>
      <div>
        <div class="sizes">
          {#each [...PAGE_PRESETS, { id: 'custom', name: 'Custom', sub: 'Any size', w: 1, h: 1 }] as p}
            <button class="size" class:on={preset === p.id} onclick={() => (preset = p.id)}>
              <span class="pg" style="aspect-ratio:{p.id === 'custom' ? '1/1' : `${p.w}/${p.h}`}"></span>
              <b>{p.name}</b><span class="small">{p.sub}</span>
            </button>
          {/each}
        </div>
        {#if preset === 'custom'}
          <div class="custom small">
            <input class="field" type="number" bind:value={customW} min="200" max="8000" /> × <input class="field" type="number" bind:value={customH} min="200" max="8000" /> px
          </div>
        {/if}
        <div class="small faint note">1 : {ratio} · {size[0]} × {size[1]} px</div>
      </div>

      <span class="lbl">Reading:</span>
      <div>
        <Seg value={reading} options={[['rtl', 'Right to left'], ['ltr', 'Left to right']]} onchange={(v) => (reading = v)} />
        <div class="illu small muted">
          <span class="mini">{reading === 'rtl' ? 3 : 2}</span><span class="mini">{reading === 'rtl' ? 2 : 3}</span>
          {#if reading === 'rtl'}<ArrowLeft size={14} color="var(--sb-accent)" />{:else}<ArrowRight size={14} color="var(--sb-accent)" />{/if}
          Panels and spreads read {reading === 'rtl' ? 'right → left' : 'left → right'}
        </div>
      </div>

      <span class="lbl">First page:</span>
      <div class="inline">
        <Seg value={firstSingle} options={[[true, 'Single'], [false, 'Paired']]} onchange={(v) => (firstSingle = v)} />
        <span class="minis">
          {#each (firstSingle ? [[1], [2, 3], [4, 5]] : [[1, 2], [3, 4]]) as s}
            <span class="pair">{#each reading === 'rtl' ? [...s].reverse() : s as n}<span class="mini">{n}</span>{/each}</span>
          {/each}
        </span>
      </div>

      <span class="lbl">Pages:</span>
      <div class="inline"><input class="field pages" type="number" min="1" max="400" bind:value={pages} /><span class="small muted">{spreadsText}</span></div>

      <span class="lbl">Where:</span>
      <div class="inline">
        <select class="popup" value={where} onchange={(e) => { const v = e.currentTarget.value; if (v === 'choose') chooseDir(); else where = v }}>
          <option value="{home}/Manga">Manga</option>
          <option value="{home}/Desktop">Desktop</option>
          <option value="{home}/Documents">Documents</option>
          {#if otherDir}<option value="other">{otherDir.split('/').pop()}</option>{/if}
          <option value="choose">Other…</option>
        </select>
        <span class="small muted path">{dir.replace(home, '~')}/{title.trim()}.storyboarder</span>
      </div>
    </div>

    {#if err}<p class="err small">{err}</p>{/if}
    <div class="actions">
      <button class="btn" onclick={onclose}>Cancel</button>
      <button class="btn primary" disabled={busy || !title.trim()} onclick={create}>{busy ? 'Creating…' : 'Create'}</button>
    </div>
  </div>
</div>

<style>
  .sheet { width: 600px; }
  .sub { margin: 0 0 18px; }
  .form { display: grid; grid-template-columns: 110px 1fr; gap: 14px 10px; align-items: start; }
  .lbl { text-align: right; padding-top: 5px; }
  .two { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  .opt { display: flex; gap: 10px; align-items: flex-start; padding: 10px; border-radius: 12px; background: var(--sb-control-pressed); text-align: left; box-shadow: inset 0 0 0 0.5px var(--sb-border); }
  .opt b { display: block; font-weight: 500; }
  .opt .small { color: var(--sb-label-2); }
  .opt.on { background: #0a84ff33; box-shadow: inset 0 0 0 1.5px var(--sb-accent); }
  .opt.on b { color: var(--sb-accent); }
  .oi { width: 30px; height: 30px; border-radius: 8px; background: var(--sb-control); display: flex; align-items: center; justify-content: center; flex: none; }
  .opt.on .oi { background: var(--sb-accent); color: #fff; }
  .sizes { display: grid; grid-template-columns: repeat(5, 1fr); gap: 6px; }
  .size { display: flex; flex-direction: column; align-items: center; gap: 2px; padding: 10px 4px 8px; border-radius: 10px; background: var(--sb-control-pressed); box-shadow: inset 0 0 0 0.5px var(--sb-border); }
  .size b { font-weight: 500; font-size: 12px; margin-top: 6px; }
  .size .small { color: var(--sb-label-2); font-size: 10px; }
  .size .pg { height: 42px; background: #fff; border-radius: 1px; }
  .size.on { background: #0a84ff33; box-shadow: inset 0 0 0 1.5px var(--sb-accent); }
  .size.on b, .size.on .small { color: var(--sb-accent); }
  .custom { display: flex; align-items: center; gap: 6px; margin-top: 8px; }
  .custom .field { width: 80px; }
  .note { margin-top: 6px; }
  .illu, .inline { display: flex; align-items: center; gap: 8px; margin-top: 8px; }
  .inline { margin-top: 0; }
  .minis { display: flex; gap: 6px; }
  .pair { display: flex; }
  .mini { width: 20px; height: 28px; background: #fff; color: #555; font-size: 9px; display: inline-flex; align-items: center; justify-content: center; box-shadow: 0 0 0 0.5px #0006; }
  .pages { width: 64px; }
  .path { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 320px; }
  .actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 20px; }
  .err { color: var(--sb-red); }
</style>
