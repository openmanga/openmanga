<script lang="ts">
  // All pages as spreads in reading order: multi-select, drag to reorder, delete, double-click to open.
  import { ImageIcon, Image as ImageBig } from 'lucide-svelte'
  import { app, edit, fileUrl, setPage, presetName, firstPageSingle, type PageSummary } from '../lib/sb.svelte'
  import Slider from '../ui/Slider.svelte'

  let size = $state(118)
  let sel = $state<number[]>([])
  let anchor = 0
  const h = $derived(Math.round((size * app.pageSize[1]) / app.pageSize[0]))

  const spreads = $derived.by(() => {
    const out: PageSummary[][] = []
    const seen = new Set<string>()
    for (const p of app.pages) {
      const k = p.spread.join(',')
      if (seen.has(k)) continue
      seen.add(k)
      const ps = p.spread.map((n) => app.pages[n - 1]).filter(Boolean)
      out.push(app.reading === 'rtl' ? ps.reverse() : ps)
    }
    return out
  })

  function click(e: MouseEvent, n: number) {
    if (e.metaKey) sel = sel.includes(n) ? sel.filter((x) => x !== n) : [...sel, n]
    else if (e.shiftKey) { const [a, b] = [Math.min(anchor, n), Math.max(anchor, n)]; sel = Array.from({ length: b - a + 1 }, (_, i) => a + i) }
    else sel = [n]
    if (!e.shiftKey) anchor = n
  }

  async function del() {
    if (!sel.length || sel.length >= app.pages.length) return
    await edit(['page', 'delete', ...sel])
    sel = []
  }

  // drag to reorder: drop a page on another page's slot
  let drag = $state<null | { n: number; x: number; y: number; moved: boolean; over: number | null }>(null)
  function down(e: PointerEvent, n: number) { if (e.button === 0) drag = { n, x: e.clientX, y: e.clientY, moved: false, over: null } }
  function move(e: PointerEvent) {
    if (!drag) return
    if (!drag.moved && Math.hypot(e.clientX - drag.x, e.clientY - drag.y) < 6) return
    const hit = document.elementsFromPoint(e.clientX, e.clientY).find((el) => (el as HTMLElement).dataset?.page) as HTMLElement | undefined
    drag = { ...drag, x: e.clientX, y: e.clientY, moved: true, over: hit ? +hit.dataset.page! : null }
  }
  async function up() {
    const d = drag
    drag = null
    if (d?.moved && d.over && d.over !== d.n) {
      await edit(['page', 'move', d.n, '--to', d.over])
      sel = [d.over]
    }
  }
  function key(e: KeyboardEvent) {
    if ((e.target as HTMLElement).closest?.('input,textarea')) return
    if (e.key === 'Backspace' || e.key === 'Delete') { e.preventDefault(); del() }
    if (e.key === 'a' && e.metaKey) { e.preventDefault(); sel = app.pages.map((p) => p.page) }
  }
</script>

<svelte:window onpointermove={move} onpointerup={up} onkeydown={key} />

<div class="grid-view">
  <div class="head">
    <div>
      <h1>{app.project?.title}</h1>
      <div class="small muted">{app.pages.length} pages · {app.boards.length} boards · {app.reading === 'rtl' ? 'right-to-left' : 'left-to-right'} · page 1 {firstPageSingle() ? 'single' : 'paired'} · {presetName(app.pageSize)}</div>
    </div>
    <div class="tools">
      {#if sel.length}<span class="small muted">{sel.length} {sel.length === 1 ? 'page' : 'pages'} selected</span>
        <button class="btn danger" onclick={del} disabled={sel.length >= app.pages.length}>Delete…</button>{/if}
      <ImageIcon size={12} class="muted" />
      <Slider value={size} min={70} max={220} step={1} oninput={(v) => (size = v)} width={90} />
      <ImageBig size={16} class="muted" />
    </div>
  </div>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="pages" class:rtl={app.reading === 'rtl'} onclick={(e) => { if (e.target === e.currentTarget) sel = [] }}>
    {#each spreads as s}
      <div class="spread" class:single={s.length === 1}>
        {#each s as p (p.id)}
          <button class="pg" data-page={p.page} class:sel={sel.includes(p.page)} class:over={drag?.over === p.page && drag.n !== p.page} class:ghosted={drag?.moved && drag.n === p.page}
            onpointerdown={(e) => down(e, p.page)} onclick={(e) => click(e, p.page)} ondblclick={() => { app.view = 'page'; setPage(p.page) }}>
            <img src={fileUrl(p.posterframe)} alt="" style="width:{size}px;height:{h}px" loading="lazy" />
            <span>{p.page}</span>
          </button>
        {/each}
      </div>
    {/each}
  </div>
  <div class="foot small faint">Drag to reorder · ⌫ deletes the selection · double-click a page to open it</div>
  {#if drag?.moved}
    {@const p = app.pages[drag.n - 1]}
    <img class="dragghost" src={fileUrl(p.posterframe)} alt="" style="left:{drag.x}px;top:{drag.y}px;width:{size}px;height:{h}px" />
  {/if}
</div>

<style>
  .grid-view { position: absolute; inset: 0; display: flex; flex-direction: column; background: var(--sb-content); border-radius: 16px; }
  .head { display: flex; justify-content: space-between; align-items: flex-end; padding: 18px 40px 16px; }
  h1 { font-size: 20px; margin: 0 0 4px; }
  .tools { display: flex; align-items: center; gap: 10px; }
  .pages { flex: 1; overflow-y: auto; padding: 0 40px 20px; display: flex; flex-wrap: wrap; align-content: flex-start; gap: 24px 48px; }
  .pages.rtl { flex-direction: row-reverse; }
  .spread { display: flex; }
  .pg { display: flex; flex-direction: column; align-items: center; gap: 8px; font-size: 11px; color: var(--sb-label-2); }
  .pg img { display: block; background: var(--sb-paper); box-shadow: 0 1px 3px #0006; }
  .pg.sel img { outline: 3px solid var(--sb-accent); position: relative; z-index: 1; }
  .pg.sel span { color: var(--sb-accent); font-weight: 600; }
  .pg.over img { box-shadow: -4px 0 0 var(--sb-accent); }
  .pg.ghosted img { opacity: 0.3; }
  .foot { text-align: center; padding: 10px; }
  .dragghost { position: fixed; pointer-events: none; transform: translate(-50%, -50%) rotate(-4deg); box-shadow: 0 18px 40px #0009; opacity: 0.92; z-index: 50; }
</style>
