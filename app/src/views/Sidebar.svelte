<script lang="ts">
  import { PanelLeft, Plus } from 'lucide-svelte'
  import { app, edit, fileUrl, setPage, boardTitle, firstPageSingle, type PageSummary } from '../lib/sb.svelte'

  const spreads = $derived.by(() => {
    const out: PageSummary[][] = []
    const seen = new Set<string>()
    for (const p of app.pages) {
      const key = p.spread.join(',')
      if (seen.has(key)) continue
      seen.add(key)
      const ps = p.spread.map((n) => app.pages[n - 1]).filter(Boolean)
      out.push(app.reading === 'rtl' ? ps.reverse() : ps)
    }
    return out
  })
  const thumbW = 73
  const thumbH = $derived(Math.round((thumbW * app.pageSize[1]) / app.pageSize[0]))
  const current = $derived(app.view === 'spread' ? (app.pages[app.pageNo - 1]?.spread ?? []) : [app.pageNo])

  function openPage(n: number) {
    if (app.view === 'board' || app.view === 'grid') app.view = 'page'
    setPage(n)
  }

  async function addPage() {
    await edit(['page', 'add'])
    if (app.view !== 'page' && app.view !== 'spread') app.view = 'page'
    setPage(app.pages.length)
  }
  async function addBoard() {
    await edit(['board', 'add', '--after', app.boardNo])
    app.boardNo = app.boardNo + 1
    app.view = 'board'
  }

  // ---- drag a board onto a panel (pointer based, works over the drawing canvas too) ----
  let press: null | { uid: string; i: number; x: number; y: number } = null
  function boardDown(e: PointerEvent, uid: string, i: number) {
    if (e.button !== 0) return
    press = { uid, i, x: e.clientX, y: e.clientY }
  }
  function winMove(e: PointerEvent) {
    if (!press) return
    if (!app.drag && Math.hypot(e.clientX - press.x, e.clientY - press.y) < 5) return
    const hit = document.elementsFromPoint(e.clientX, e.clientY).find((el) => (el as HTMLElement).dataset?.panel)
    app.drag = { uid: press.uid, x: e.clientX, y: e.clientY, panel: app.view === 'page' ? ((hit as HTMLElement)?.dataset.panel ?? null) : null }
  }
  async function winUp() {
    const p = press
    press = null
    if (!p) return
    const d = app.drag
    app.drag = null
    if (d) {
      if (d.panel && app.page) await edit(['panel', 'place', app.page.page, d.panel, d.uid])
      return
    }
    app.boardNo = p.i + 1
    if (app.view === 'board') return
  }
</script>

<svelte:window onpointermove={winMove} onpointerup={winUp} />

<aside class="sidebar glass">
  <div class="top" data-tauri-drag-region>
    <button class="icon-btn toggle" title="Hide sidebar" onclick={() => (app.sidebar = false)}><PanelLeft size={17} strokeWidth={1.5} /></button>
  </div>
  <div class="tabs seg">
    <button class:on={app.sidebarTab === 'pages'} onclick={() => (app.sidebarTab = 'pages')}>Pages</button>
    <button class:on={app.sidebarTab === 'boards'} onclick={() => (app.sidebarTab = 'boards')}>Boards</button>
  </div>

  {#if app.sidebarTab === 'pages'}
    <div class="caption small faint">
      <span>{app.pages.length} {app.pages.length === 1 ? 'page' : 'pages'} · {app.reading === 'rtl' ? 'right-to-left' : 'left-to-right'}</span>
      <span>{firstPageSingle() ? 'p1 single' : 'p1 paired'}</span>
    </div>
    <div class="list">
      {#each spreads as s}
        <div class="spread" class:single={s.length === 1} class:rtl={app.reading === 'rtl'}>
          {#each s as p (p.id)}
            {@const on = current.includes(p.page) && app.view !== 'board'}
            <button class="pg" onclick={() => openPage(p.page)} ondblclick={() => { app.view = 'page'; setPage(p.page) }}>
              <img class:on src={fileUrl(p.posterframe)} alt="" style="width:{thumbW}px;height:{thumbH}px" loading="lazy" />
              <span class:on>{p.page}</span>
            </button>
          {/each}
        </div>
      {/each}
      {#if app.pages.length === 1}
        <button class="add-tile" style="width:{thumbW}px;height:{thumbH}px" title="Add page" onclick={addPage}><Plus size={18} /></button>
      {/if}
    </div>
    <button class="bottom glass" onclick={addPage}><Plus size={15} /> Add Page</button>
  {:else}
    <div class="boards">
      {#each app.boards as b, i (b.uid)}
        {@const on = app.boardNo === i + 1}
        <button class="bd" class:dragging={app.drag?.uid === b.uid} onpointerdown={(e) => boardDown(e, b.uid, i)}
          onclick={() => { if (app.view === 'board') app.boardNo = i + 1 }}
          ondblclick={() => { app.boardNo = i + 1; app.view = 'board' }}>
          <img class:on src={fileUrl(b.posterframe)} alt="" style="aspect-ratio:{b.size[0]}/{b.size[1]}" loading="lazy" />
          <span class:on>{boardTitle(b)}</span>
        </button>
      {/each}
    </div>
    <button class="bottom glass" onclick={addBoard}><Plus size={15} /> New Board</button>
  {/if}
</aside>

{#if app.drag}
  {@const b = app.boards.find((x) => x.uid === app.drag!.uid)}
  {#if b}
    <div class="ghost" style="left:{app.drag.x}px; top:{app.drag.y}px">
      <img src={fileUrl(b.posterframe)} alt="" style="aspect-ratio:{b.size[0]}/{b.size[1]}" />
      <span class="plus">+</span>
    </div>
  {/if}
{/if}

<style>
  .sidebar { position: absolute; left: 8px; top: 8px; bottom: 8px; width: 232px; border-radius: 16px; display: flex; flex-direction: column; z-index: 5; }
  .top { height: 44px; flex: none; display: flex; justify-content: flex-end; align-items: center; padding: 0 12px; }
  .toggle { color: var(--sb-label-2); min-width: 28px; }
  .tabs { margin: 0 10px; display: flex; height: 30px; }
  .tabs button { flex: 1; height: 26px; }
  .caption { display: flex; justify-content: space-between; padding: 10px 16px 4px; }
  .list { flex: 1; overflow-y: auto; padding: 8px 0 60px; display: flex; flex-direction: column; align-items: center; gap: 8px; }
  .spread { display: flex; width: 146px; }
  .spread.single { justify-content: flex-start; }
  .spread.single.rtl { justify-content: flex-end; }
  .pg { display: flex; flex-direction: column; align-items: center; gap: 6px; font-size: 11px; color: var(--sb-label-2); }
  .spread .pg + .pg img { box-shadow: inset 1px 0 0 #0002; }
  .pg img { display: block; background: var(--sb-paper); object-fit: cover; outline: 0.5px solid #0000; }
  img.on { outline: 2px solid var(--sb-accent) !important; outline-offset: 0; position: relative; z-index: 1; }
  span.on { color: var(--sb-accent); font-weight: 600; }
  .add-tile { border: 1px solid var(--sb-separator); border-radius: 4px; color: var(--sb-label-3); display: flex; align-items: center; justify-content: center; }
  .boards { flex: 1; overflow-y: auto; padding: 12px 12px 60px; display: grid; grid-template-columns: 1fr 1fr; gap: 12px 10px; align-content: start; }
  .bd { display: flex; flex-direction: column; gap: 5px; text-align: left; font-size: 11px; color: var(--sb-label); min-width: 0; }
  .bd img { width: 100%; max-height: 120px; object-fit: cover; border-radius: 3px; background: #fff; display: block; outline-offset: 0; }
  .bd span { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .bd.dragging { opacity: 0.4; }
  .bottom { position: absolute; left: 12px; bottom: 12px; height: 36px; padding: 0 14px; border-radius: 18px; display: flex; align-items: center; gap: 6px; }
  .ghost { position: fixed; z-index: 100; pointer-events: none; transform: translate(-50%, -60%) rotate(-3deg); }
  .ghost img { width: 150px; border-radius: 6px; box-shadow: 0 0 0 2px var(--sb-accent), 0 18px 40px #0008; background: #fff; display: block; }
  .ghost .plus { position: absolute; right: -8px; top: -8px; width: 20px; height: 20px; border-radius: 50%; background: var(--sb-green); color: #fff; font-weight: 700; display: flex; align-items: center; justify-content: center; }
</style>
