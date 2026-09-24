<script lang="ts">
  import {
    ChevronDown, Eye, EyeOff, Rows2, Columns2, Slash, GripVertical, ArrowLeft,
    MessageCircle, Cloud, Zap, CircleDashed, RectangleHorizontal, Type, ArrowUpRight,
  } from 'lucide-svelte'
  import { app, sb, edit, refresh, fileUrl, curBoard, boardByUid, boardTitle, firstPageSingle, presetName, PAGE_PRESETS, presetPx, setPage, type BalloonType, type Placement } from '../lib/sb.svelte'
  import { tools, cur, toolLayer, saveTool, TOOL_NAMES } from '../lib/tools.svelte'
  import Slider from '../ui/Slider.svelte'
  import Toggle from '../ui/Toggle.svelte'
  import Seg from '../ui/Seg.svelte'

  const page = $derived(app.page)
  const board = $derived(curBoard())
  const panel = $derived(page?.panels.find((p) => p.id === (app.focusPanel ?? app.selPanel)) ?? null)
  const balloon = $derived(page?.balloons.find((b) => b.id === app.selBalloon) ?? null)
  const ordered = $derived(page ? [...page.panels].sort((a, b) => a.order - b.order) : [])
  let collapsed = $state<Record<string, boolean>>({})

  // ---- layers ----
  const DOT: Record<string, string> = { notes: '#E5484D', balloons: '#FFFFFF', frames: '#1A1A1A', ink: '#1A1A1A', pencil: '#5E5E66', tone: '#8E8E93', fill: '#E9DFC4', reference: '#8FB8E8' }
  const layerRows = $derived(app.view === 'board' ? ['notes', 'ink', 'pencil', 'tone', 'fill', 'reference'] : ['notes', 'balloons', 'frames', 'ink', 'pencil', 'tone', 'fill', 'reference'])
  const opacityOf = (l: string) => app.layerInfo[l]?.opacity ?? 1
  const layerTarget = () => (app.view === 'board' ? [curBoard()!.uid] : ['--page', String(app.pageNo)])
  function setOpacity(l: string, v: number) {
    app.layerInfo[l] = { ...(app.layerInfo[l] ?? { exists: false }), opacity: v }
    edit(['layer', 'set-opacity', ...layerTarget(), l, v.toFixed(2)])
  }

  // ---- page facts ----
  const spreadSide = $derived.by(() => {
    const s = app.pages[app.pageNo - 1]?.spread ?? []
    if (s.length < 2) return 'single'
    const left = app.reading === 'rtl' ? Math.max(...s) : Math.min(...s)
    return left === app.pageNo ? 'left page' : 'right page'
  })

  // ---- balloon text: committed on blur / ⌘↩ ----
  let text = $state('')
  let textEl: HTMLTextAreaElement | undefined = $state()
  $effect(() => { text = balloon?.text ?? '' })
  $effect(() => { if (app.editText && textEl) { textEl.focus(); textEl.select() } })
  function commitText() {
    if (balloon && text !== balloon.text) edit(['balloon', 'set', page!.page, balloon.id, '--text', text, '--fit'])
  }
  const BTYPES: [BalloonType, string, any][] = [
    ['speech', 'Speech', MessageCircle], ['thought', 'Thought', Cloud], ['shout', 'Shout', Zap],
    ['whisper', 'Whisper', CircleDashed], ['narration', 'Narration', RectangleHorizontal], ['sfx', 'SFX', Type],
  ]
  const bset = (...a: (string | number)[]) => edit(['balloon', 'set', page!.page, balloon!.id, ...a])

  // ---- placed board (focus view) ----
  let pl = $state<Placement | null>(null)
  $effect(() => { pl = panel?.content ? { ...panel.content } : null })
  const place = (c: Placement) =>
    edit(['panel', 'place', page!.page, panel!.id, c.board, '--x', Math.round(c.x), '--y', Math.round(c.y), '--scale', c.scale.toFixed(3), '--rotation', Math.round(c.rotation), '--fit', c.fit])
  const usedCount = (uid: string) => page?.panels.filter((p) => p.content?.board === uid).length ?? 0

  // ---- reading order drag ----
  let dragIdx = $state<number | null>(null)
  let overIdx = $state<number | null>(null)
  let listEl: HTMLDivElement | undefined = $state()
  function orderMove(e: PointerEvent) {
    if (dragIdx === null || !listEl) return
    const rows = [...listEl.querySelectorAll('.orow')]
    overIdx = rows.findIndex((r) => { const b = r.getBoundingClientRect(); return e.clientY < b.bottom })
    if (overIdx < 0) overIdx = rows.length - 1
  }
  async function orderUp() {
    if (dragIdx === null) return
    const from = dragIdx, to = overIdx ?? from
    dragIdx = overIdx = null
    if (from === to) return
    const ids = ordered.map((p) => p.id)
    ids.splice(to, 0, ids.splice(from, 1)[0])
    await edit(['panel', 'order', page!.page, ...ids])
  }

  // ---- board name/description ----
  let bname = $state(''), bdesc = $state('')
  $effect(() => { bname = board?.name ?? ''; bdesc = board?.description ?? '' })
  async function saveName() {
    if (board && bname !== board.name) await edit(['board', 'set-name', board.uid, bname])
  }
  async function saveDesc() {
    if (board && bdesc !== board.description) await edit(['board', 'set-description', board.uid, bdesc])
  }

  let usedIn = $state<{ page: number; panel: number }[]>([])
  $effect(() => {
    const b = board
    if (app.view !== 'board' || !b) return
    void app.pages
    sb<any>(['board', 'info', b.uid]).then((r) => (usedIn = r.usedIn ?? [])).catch(() => (usedIn = []))
  })

  const pickColor = (c: string) => { cur().color = c; saveTool(tools.current) }
</script>

<svelte:window onpointermove={orderMove} onpointerup={orderUp} />

<aside class="inspector glass">
  <div class="scroll">
    {#if app.view === 'board' && board}
      <div class="section-title">Board <ChevronDown size={14} class="muted" /></div>
      <input class="field wide" placeholder="Name" bind:value={bname} onblur={saveName} onkeydown={(e) => e.key === 'Enter' && e.currentTarget.blur()} />
      <textarea class="field wide desc" rows="3" placeholder="Description" bind:value={bdesc} onblur={saveDesc}></textarea>
      <div class="group">
        <div class="row"><span>Size</span><span class="muted">{board.size[0]} × {board.size[1]} px</span></div>
        <div class="row"><span>Onion skin</span><span class="r"><span class="muted small">Prev + next</span><Toggle on={app.onion} onchange={(v) => (app.onion = v)} /></span></div>
        <div class="row"><span>Used in</span>
          {#if usedIn.length}
            <button class="link" onclick={() => { app.view = 'page'; setPage(usedIn[0].page) }}>Page {usedIn[0].page} · Panel {usedIn[0].panel}{usedIn.length > 1 ? ` +${usedIn.length - 1}` : ''} <ArrowUpRight size={12} /></button>
          {:else}<span class="faint">Not placed</span>{/if}
        </div>
      </div>
    {/if}

    {#if app.focusPanel && panel && page}
      <div class="section-title">Placed board <ChevronDown size={14} /></div>
      {@const b = boardByUid(pl?.board)}
      <div class="group">
        <div class="card">
          {#if b}<img src={fileUrl(b.thumbnail)} alt="" />{:else}<div class="noimg"></div>{/if}
          <div class="cn">
            <div class="nm">{b ? boardTitle(b) : 'Empty panel'}</div>
            {#if b}<div class="small muted">Board {b.number} · {usedCount(b.uid)} {usedCount(b.uid) === 1 ? 'panel' : 'panels'}</div>{/if}
          </div>
          <select class="popup" value="" onchange={(e) => { edit(['panel', 'place', page.page, panel.id, e.currentTarget.value]); e.currentTarget.value = '' }}>
            <option value="" disabled>{b ? 'Change' : 'Place'}</option>
            {#each app.boards as x}<option value={x.uid}>{boardTitle(x)}</option>{/each}
          </select>
        </div>
      </div>
      {#if pl}
        <div class="group gap">
          <div class="row"><span>Fit</span><Seg value={pl.fit} options={[['fill', 'Fill'], ['fit', 'Fit'], ['none', 'None']]} onchange={(v) => place({ ...pl!, fit: v })} /></div>
          <div class="row"><span>Scale</span><span class="r"><Slider value={pl.scale} min={0.1} max={3} oninput={(v) => (pl!.scale = v)} onchange={() => place(pl!)} width={96} /><span class="val">{Math.round(pl.scale * 100)}%</span></span></div>
          <div class="row"><span>Offset</span><span class="r">
            <span class="faint small">X</span><input class="field num" type="number" value={Math.round(pl.x)} onchange={(e) => place({ ...pl!, x: +e.currentTarget.value })} />
            <span class="faint small">Y</span><input class="field num" type="number" value={Math.round(pl.y)} onchange={(e) => place({ ...pl!, y: +e.currentTarget.value })} />
          </span></div>
          <div class="row"><span>Rotation</span><span class="r"><Slider value={pl.rotation} min={-180} max={180} step={1} oninput={(v) => (pl!.rotation = v)} onchange={() => place(pl!)} width={96} /><span class="val">{Math.round(pl.rotation)}°</span></span></div>
          <div class="row"><span>Crop to panel</span><Toggle on={true} disabled /></div>
        </div>
        <div class="btns">
          <button class="btn primary" onclick={() => { const i = app.boards.findIndex((x) => x.uid === pl!.board); app.focusPanel = null; app.boardNo = i + 1; app.view = 'board' }}>Edit Board</button>
          <button class="btn danger" onclick={() => edit(['panel', 'clear-content', page.page, panel.id])}>Clear Panel</button>
        </div>
      {/if}
      <div class="section-title">Panel {panel.order}</div>
      <div class="group">
        <div class="row"><span>Reading order</span><span class="muted">{panel.order} of {page.panels.length}</span></div>
        {#each [page.balloons.filter((x) => x.panel === panel.id)] as bs}
          <div class="row"><span>Balloons</span><span class="muted">{bs.length}{bs.length ? ` (${[...new Set(bs.map((x) => x.type))].join(', ')})` : ''}</span></div>
        {/each}
      </div>
    {:else if app.mode === 'panel' && page && app.view === 'page'}
      {#if panel}
        <div class="section-title">Panel {panel.order} <ChevronDown size={14} /></div>
        <div class="group">
          <div class="row"><span>Split</span><span class="seg">
            <button onclick={() => edit(['panel', 'split', page.page, panel.id, '--horizontal', ...(app.gutter ? ['--gutter', app.gutter] : [])])} title="Horizontal"><Rows2 size={14} /></button>
            <button onclick={() => edit(['panel', 'split', page.page, panel.id, '--vertical', ...(app.gutter ? ['--gutter', app.gutter] : [])])} title="Vertical"><Columns2 size={14} /></button>
            <button onclick={() => edit(['panel', 'split', page.page, panel.id, '--diagonal', ...(app.gutter ? ['--gutter', app.gutter] : [])])} title="Diagonal"><Slash size={14} /></button>
          </span></div>
          <div class="row"><span>Gutter</span><span class="r"><Slider value={app.gutter || 34} min={0} max={100} step={1} oninput={(v) => (app.gutter = v)} width={96} /><span class="val">{app.gutter || 34} px</span></span></div>
          <div class="row"><span>Border</span><span class="r"><Slider value={panel.border} min={0} max={24} step={1} onchange={(v) => edit(['panel', 'set', page.page, panel.id, '--border', v])} width={96} /><span class="val">{panel.border} px</span></span></div>
          <div class="row"><span>Bleed</span><Toggle on={panel.bleed} onchange={(v) => edit(['panel', 'set', page.page, panel.id, '--bleed', String(v)])} /></div>
        </div>
      {/if}
      <div class="section-title">Reading Order <ArrowLeft size={14} class="muted" /></div>
      <div class="group">
        <div class="row">
          <span class="small">Automatic ({app.reading.toUpperCase()}, top → bottom)</span>
          <Toggle on={page.panelOrder === 'auto'} onchange={(v) => edit(['panel', 'order', page.page, ...(v ? ['--auto'] : ordered.map((p) => p.id))])} />
        </div>
        <div bind:this={listEl}>
          {#each ordered as k, i (k.id)}
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <div class="orow" class:sel={app.selPanel === k.id} class:over={overIdx === i && dragIdx !== null && dragIdx !== i} onclick={() => { app.selPanel = k.id; app.selPanels = [k.id] }}>
              <span class="num">{i + 1}</span>
              <span class="nm">{k.content ? boardTitle(boardByUid(k.content.board)) : 'Empty panel'}</span>
              <span class="grip" onpointerdown={(e) => { e.preventDefault(); dragIdx = i; overIdx = i }}><GripVertical size={14} /></span>
            </div>
          {/each}
        </div>
      </div>
    {:else if app.mode === 'balloon' && page && app.view === 'page'}
      <div class="section-title">Balloon <ChevronDown size={14} /></div>
      {#if balloon}
        <div class="types">
          {#each BTYPES as [t, name, I]}
            <button class:on={balloon.type === t} onclick={() => bset('--type', t)}><I size={18} strokeWidth={1.6} /><span>{name}</span></button>
          {/each}
        </div>
        <div class="section-title">Text</div>
        <textarea class="field wide" rows="3" bind:this={textEl} bind:value={text} onblur={commitText}
          onkeydown={(e) => { if (e.key === 'Enter' && e.metaKey) e.currentTarget.blur() }}></textarea>
        <div class="group gap">
          <div class="row"><span>Direction</span><Seg value={balloon.vertical} options={[[false, 'Horizontal'], [true, 'Vertical']]} onchange={(v) => bset('--vertical', String(v), '--fit')} /></div>
          <div class="row"><span>Size</span><span class="r"><Slider value={balloon.fontSize} min={12} max={96} step={1} onchange={(v) => bset('--size', v, '--fit')} width={96} /><span class="val">{balloon.fontSize} px</span></span></div>
          <div class="row"><span>Attach to</span><span class="muted">{balloon.panel ? `Panel ${page.panels.find((p) => p.id === balloon.panel)?.order ?? '–'}` : 'Page'}</span></div>
          <div class="row"><span>Tail</span>
            {#if balloon.tail}<button class="link" onclick={() => bset('--tail', 'none')}>Remove</button>{:else}<span class="faint">Drag the handle</span>{/if}
          </div>
        </div>
        <div class="btns"><button class="btn" onclick={() => bset('--fit')}>Fit to Text</button><button class="btn danger" onclick={() => edit(['balloon', 'delete', page.page, balloon.id])}>Delete</button></div>
      {:else}
        <p class="hint">Click on the page to add a balloon, or click a balloon to edit it.</p>
      {/if}
    {:else if app.view === 'page' || app.view === 'board'}
      {@const s = cur()}
      <button class="section-title btn-title" onclick={() => (collapsed.tool = !collapsed.tool)}>{TOOL_NAMES[tools.current]} <ChevronDown size={14} /></button>
      {#if !collapsed.tool}
        {#if tools.current === 'lasso'}
          <p class="hint">Draw around artwork, then drag it to move, press F to fill or ⌫ to erase.</p>
        {:else}
          <div class="group">
            <div class="row"><span>Size</span><span class="r"><Slider value={Math.sqrt(s.size)} min={1} max={16} step={0.05} oninput={(v) => (s.size = Math.round(v * v * 10) / 10)} width={110} /><span class="val">{s.size} pt</span></span></div>
            {#if tools.current !== 'eraser'}
              <div class="row"><span>Opacity</span><span class="r"><Slider value={s.opacity} min={0.05} max={1} oninput={(v) => (s.opacity = v)} onchange={() => saveTool(tools.current)} width={110} /><span class="val">{Math.round(s.opacity * 100)}%</span></span></div>
              <div class="row"><span>Color</span><span class="r swatches">
                {#each s.palette as c, i}
                  <button class="sw" class:on={c.toLowerCase() === s.color.toLowerCase()} style="background:{c}" title="Palette {i + 1} ({8 + i === 10 ? 0 : 8 + i}) · ⌥-click to store the current color"
                    onclick={(e) => { if (e.altKey) { s.palette[i] = s.color; saveTool(tools.current) } else pickColor(c) }}></button>
                {/each}
                <label class="wheel"><input type="color" value={s.color} onchange={(e) => pickColor(e.currentTarget.value.toUpperCase())} /></label>
              </span></div>
            {/if}
            <div class="row"><span>Pressure → Size</span><Toggle on={s.pressure} onchange={(v) => (s.pressure = v)} /></div>
          </div>
        {/if}
      {/if}

      <div class="section-title">Layers</div>
      <div class="group layers">
        {#each layerRows as l}
          {@const derived = l === 'balloons' || l === 'frames'}
          {@const sel = !derived && toolLayer() === l && tools.current !== 'eraser' && tools.current !== 'lasso'}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="lrow" class:sel class:off={app.hidden[l]} onclick={() => { if (!derived) tools.layer = l as any }}>
            <button class="eye" onclick={(e) => { e.stopPropagation(); app.hidden[l] = !app.hidden[l] }}>
              {#if app.hidden[l]}<EyeOff size={13} />{:else}<Eye size={13} />{/if}
            </button>
            <span class="dot" style="background:{DOT[l]}"></span>
            <span class="ln">{l[0].toUpperCase() + l.slice(1)}</span>
            {#if sel}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <span class="op" onclick={(e) => e.stopPropagation()}><Slider value={opacityOf(l)} min={0} max={1} width={64} oninput={(v) => (app.layerInfo[l] = { ...(app.layerInfo[l] ?? { exists: false }), opacity: v })} onchange={(v) => setOpacity(l, v)} /></span>
            {/if}
            <span class="pct">{Math.round(opacityOf(l) * 100)}%</span>
          </div>
        {/each}
      </div>
    {/if}

    {#if page && app.view === 'page'}
      <div class="section-title">Page</div>
      <div class="group">
        <div class="row"><span>Page</span><span class="muted">{app.pageNo} of {app.pages.length} · {spreadSide}</span></div>
        <div class="row"><span>Size</span>
          <select class="popup" value={presetName(app.pageSize)} onchange={(e) => { const p = PAGE_PRESETS.find((x) => x.name === e.currentTarget.value); if (p) edit(['page', 'setup', '--size', presetPx(p).join('x')]) }}>
            {#each PAGE_PRESETS as p}<option>{p.name}</option>{/each}
            {#if !PAGE_PRESETS.some((p) => p.name === presetName(app.pageSize))}<option>{presetName(app.pageSize)}</option>{/if}
          </select>
        </div>
        <div class="row"><span>Reading</span>
          <select class="popup plain" value={app.reading} onchange={(e) => edit(['page', 'setup', '--reading', e.currentTarget.value])}>
            <option value="rtl">Right to left</option><option value="ltr">Left to right</option>
          </select>
        </div>
        <div class="row"><span>Panels</span><span class="muted">{page.panels.length}</span></div>
        <div class="row"><span>First page</span>
          <select class="popup" value={firstPageSingle() ? 'single' : 'paired'} onchange={(e) => edit(['page', 'setup', '--first-page', e.currentTarget.value])}>
            <option value="single">Single</option><option value="paired">Paired</option>
          </select>
        </div>
      </div>
    {/if}
  </div>
</aside>

<style>
  .inspector { position: absolute; right: 8px; top: 60px; bottom: 8px; width: 280px; border-radius: 16px; z-index: 5; }
  .scroll { position: absolute; inset: 0; overflow-y: auto; padding: 4px 14px 16px; }
  .section-title :global(svg) { color: var(--sb-label-2); }
  .btn-title { width: 100%; font-weight: 600; }
  .r { display: flex; align-items: center; gap: 8px; }
  .val { min-width: 34px; text-align: right; color: var(--sb-label-2); font-size: 12px; }
  .wide { width: 100%; display: block; margin-bottom: 8px; }
  .desc { color: var(--sb-label-2); }
  .gap { margin-top: 8px; }
  .link { color: var(--sb-accent); display: inline-flex; align-items: center; gap: 2px; }
  .hint { color: var(--sb-label-2); font-size: 12px; margin: 4px 4px 0; line-height: 1.45; }
  .swatches { gap: 6px; }
  .sw { width: 22px; height: 22px; border-radius: 6px; box-shadow: inset 0 0 0 0.5px #fff3; }
  .sw.on { box-shadow: 0 0 0 2px var(--sb-accent); }
  .wheel { width: 22px; height: 22px; border-radius: 50%; background: conic-gradient(red, yellow, lime, cyan, blue, magenta, red); position: relative; overflow: hidden; }
  .wheel input { opacity: 0; position: absolute; inset: 0; width: 100%; height: 100%; }
  .layers { padding: 4px; }
  .lrow { display: flex; align-items: center; gap: 8px; height: 28px; padding: 0 8px; border-radius: 7px; }
  .lrow.sel { background: #0a5fd666; font-weight: 600; }
  .lrow.off .ln, .lrow.off .pct { color: var(--sb-label-3); }
  .eye { color: var(--sb-label-2); display: flex; }
  .dot { width: 10px; height: 10px; border-radius: 3px; box-shadow: inset 0 0 0 0.5px #fff4; }
  .ln { flex: 1; }
  .op { display: flex; }
  .pct { color: var(--sb-label-2); font-size: 12px; }
  .popup.plain { background-color: transparent; color: var(--sb-label-2); }
  .orow { display: flex; align-items: center; gap: 8px; height: 28px; margin: 0 -6px; padding: 0 6px; border-radius: 7px; }
  .orow.sel { background: #0a5fd666; font-weight: 600; }
  .orow.over { box-shadow: inset 0 2px 0 var(--sb-accent); }
  .orow .num { width: 18px; height: 18px; border-radius: 50%; background: var(--sb-accent); color: #fff; font-size: 11px; font-weight: 600; display: flex; align-items: center; justify-content: center; flex: none; }
  .orow .nm { flex: 1; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
  .grip { color: var(--sb-label-3); cursor: grab; display: flex; }
  .types { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; }
  .types button { height: 52px; border-radius: 10px; background: var(--sb-group); display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 4px; font-size: 11px; }
  .types button.on { background: var(--sb-accent-soft); color: var(--sb-accent); box-shadow: inset 0 0 0 1.5px var(--sb-accent); }
  .btns { display: flex; gap: 8px; margin-top: 10px; }
  .btns .btn { flex: 1; height: 30px; }
  .card { display: flex; align-items: center; gap: 10px; padding: 8px 0; }
  .card img, .noimg { width: 72px; height: 48px; object-fit: cover; border-radius: 4px; background: #fff; flex: none; }
  .cn { flex: 1; min-width: 0; }
  .cn .nm { font-weight: 600; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
  .num { width: 58px; }
  .field.num { height: 24px; padding: 0 6px; }
</style>
