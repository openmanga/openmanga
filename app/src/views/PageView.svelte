<script lang="ts">
  // One manga page: placed boards clipped to panels, page layers and the engine's frames/balloons PNGs,
  // with an SVG overlay for reading-order badges, panel layout, balloon editing and the panel focus view.
  import { Pencil, LayoutPanelLeft, Rows2, Columns2, Slash, Combine, Trash2, ChevronLeft, ChevronRight } from 'lucide-svelte'
  import { app, edit, fileUrl, boardByUid, boardTitle, type Pt, type Panel, type Balloon, type Placement } from '../lib/sb.svelte'
  import { tools } from '../lib/tools.svelte'
  import { bbox, inPoly, ptsAttr, ptsArg, mapBox, resizeBox, HANDLES, handlePos, placement, rotate, type Box } from '../lib/geom'
  import Stage from './Stage.svelte'
  import DrawLayer from './DrawLayer.svelte'

  let { stage = $bindable() }: { stage?: Stage } = $props()

  const page = $derived(app.page!)
  const W = $derived(page.size[0]), H = $derived(page.size[1])
  const dir = $derived(app.project!.dir)
  const layerUrl = (l: string) => fileUrl(`${dir}/images/page-${page.id}-${l}.png`)
  const has = (l: string) => page.layers.includes(l) && !app.hidden[l]
  const panelsByOrder = $derived([...page.panels].sort((a, b) => a.order - b.order))
  const focusPanel = $derived(page.panels.find((p) => p.id === app.focusPanel) ?? null)
  const blank = $derived(page.panels.length === 0 && !page.layers.some((l) => !['frames', 'balloons'].includes(l)))

  // ---- live previews while dragging (committed to sb on pointer up) ----
  let panelPts = $state<Record<string, Pt[]>>({})
  let balloonBox = $state<null | { id: string; x: number; y: number; w: number; h: number; tail?: Pt }>(null)
  let place = $state<null | Placement>(null)
  let newRect = $state<Box | null>(null)
  let svg: SVGSVGElement
  let drag: null | { kind: string; start: Pt; data?: any } = null

  const pts = (p: Panel) => panelPts[p.id] ?? p.points
  const toPage = (e: PointerEvent | MouseEvent): Pt => {
    const r = svg.getBoundingClientRect()
    return [((e.clientX - r.left) / r.width) * W, ((e.clientY - r.top) / r.height) * H]
  }
  const panelAt = (p: Pt) => panelsByOrder.find((k) => inPoly(p, k.points))
  const balloonAt = (p: Pt) => [...page.balloons].reverse().find((b) => p[0] >= b.x && p[0] <= b.x + b.w && p[1] >= b.y && p[1] <= b.y + b.h)

  function begin(e: PointerEvent, kind: string, data?: any) {
    e.stopPropagation()
    try { svg.setPointerCapture(e.pointerId) } catch {}
    drag = { kind, start: toPage(e), data }
  }

  function down(e: PointerEvent) {
    if (e.button !== 0) return
    const p = toPage(e)
    if (app.focusPanel) return
    if (app.mode === 'panel') {
      const k = panelAt(p)
      if (!k) { app.selPanel = null; app.selPanels = []; begin(e, 'new'); return }
      if (e.shiftKey && app.selPanel && app.selPanel !== k.id) { app.selPanels = [app.selPanel, k.id]; return }
      app.selPanel = k.id
      app.selPanels = [k.id]
      begin(e, 'pmove', { id: k.id, orig: k.points })
    } else if (app.mode === 'balloon') {
      const b = balloonAt(p)
      if (b) { app.selBalloon = b.id; begin(e, 'bmove', { b }); return }
      if (app.selBalloon) { app.selBalloon = null; return }
      addBalloon(p)
    }
  }

  function move(e: PointerEvent) {
    if (!drag) return
    const p = toPage(e)
    const dx = p[0] - drag.start[0], dy = p[1] - drag.start[1]
    const d = drag.data
    switch (drag.kind) {
      case 'new': newRect = [Math.min(p[0], drag.start[0]), Math.min(p[1], drag.start[1]), Math.abs(dx), Math.abs(dy)]; break
      case 'pmove': panelPts[d.id] = d.orig.map(([x, y]: Pt) => [x + dx, y + dy]); break
      case 'presize': { const a = bbox(d.orig); panelPts[d.id] = mapBox(d.orig, a, resizeBox(a, d.h[0], d.h[1], dx, dy)); break }
      case 'bmove': balloonBox = { ...d.b, x: d.b.x + dx, y: d.b.y + dy }; break
      case 'bresize': { const [x, y, w, h] = resizeBox([d.b.x, d.b.y, d.b.w, d.b.h], d.h[0], d.h[1], dx, dy); balloonBox = { ...d.b, x, y, w, h }; break }
      case 'tail': balloonBox = { ...d.b, tail: p }; break
      case 'place-move': place = { ...d.c, x: d.c.x + dx, y: d.c.y + dy }; break
      case 'place-scale': {
        const r0 = Math.hypot(drag.start[0] - d.g.cx, drag.start[1] - d.g.cy), r1 = Math.hypot(p[0] - d.g.cx, p[1] - d.g.cy)
        place = { ...d.c, scale: Math.max(0.05, (d.c.scale * r1) / (r0 || 1)) }; break
      }
      case 'place-rotate': {
        let a = (Math.atan2(p[1] - d.g.cy, p[0] - d.g.cx) * 180) / Math.PI + 90
        if (e.shiftKey) a = Math.round(a / 15) * 15
        place = { ...d.c, rotation: Math.round(((a + 540) % 360) - 180) }; break
      }
    }
  }

  async function up() {
    if (!drag) return
    const { kind, data: d } = drag
    drag = null
    const n = page.page
    try {
      if (kind === 'new' && newRect && newRect[2] > 30 && newRect[3] > 30) {
        const r = newRect.map(Math.round)
        await edit(['panel', 'add', n, '--rect', r.join(',')])
      } else if ((kind === 'pmove' || kind === 'presize') && panelPts[d.id]) {
        const moved = panelPts[d.id].some((q, i) => Math.abs(q[0] - d.orig[i][0]) + Math.abs(q[1] - d.orig[i][1]) > 1)
        if (moved) await edit(['panel', 'set', n, d.id, '--points', ptsArg(panelPts[d.id])])
      } else if (balloonBox && (kind === 'bmove' || kind === 'bresize')) {
        const b = balloonBox
        if (b.x !== d.b.x || b.y !== d.b.y || b.w !== d.b.w || b.h !== d.b.h)
          await edit(['balloon', 'set', n, b.id, '--x', Math.round(b.x), '--y', Math.round(b.y), '--w', Math.round(b.w), '--h', Math.round(b.h)])
      } else if (kind === 'tail' && balloonBox?.tail) {
        await edit(['balloon', 'set', n, balloonBox.id, '--tail', balloonBox.tail.map(Math.round).join(',')])
      } else if (kind.startsWith('place') && place && focusPanel?.content) {
        await commitPlace(place)
      }
    } finally {
      panelPts = {}
      balloonBox = null
      newRect = null
      place = null
    }
  }

  export async function commitPlace(c: Placement) {
    const k = focusPanel!
    await edit(['panel', 'place', page.page, k.id, c.board, '--x', Math.round(c.x), '--y', Math.round(c.y),
      '--scale', c.scale.toFixed(3), '--rotation', Math.round(c.rotation), '--fit', c.fit])
  }

  async function addBalloon(p: Pt) {
    const k = panelAt(p)
    const r = await edit<Balloon>(['balloon', 'add', page.page, 'Text', '--x', Math.round(p[0] - 90), '--y', Math.round(p[1] - 50),
      ...(k ? ['--tail', `${Math.round(p[0])},${Math.round(p[1] + 160)}`] : [])])
    app.selBalloon = r.id
    app.editText++
  }

  function dbl(e: MouseEvent) {
    const p = toPage(e)
    if (app.mode === 'balloon') { if (balloonAt(p)) app.editText++; return }
    openFocus(p)
  }
  function openFocus(p: Pt) {
    const k = panelAt(p)
    if (k) { app.focusPanel = k.id; app.selPanel = k.id }
  }

  // ---- panel focus navigation ----
  function stepFocus(d: number) {
    const i = panelsByOrder.findIndex((k) => k.id === app.focusPanel)
    const k = panelsByOrder[i + d]
    if (k) app.focusPanel = app.selPanel = k.id
  }

  function key(e: KeyboardEvent) {
    if ((e.target as HTMLElement).closest?.('input,textarea') || e.metaKey) return
    if (e.key === 'Escape') {
      if (app.focusPanel) app.focusPanel = null
      else { app.selPanel = null; app.selBalloon = null; app.selPanels = [] }
    } else if (e.key === 'Backspace' || e.key === 'Delete') {
      if (app.mode === 'balloon' && app.selBalloon) edit(['balloon', 'delete', page.page, app.selBalloon])
      else if (app.mode === 'panel' && app.selPanels.length && !app.focusPanel) edit(['panel', 'delete', page.page, ...app.selPanels])
      else return
      e.preventDefault()
    } else if (app.focusPanel && (e.key === 'ArrowLeft' || e.key === 'ArrowRight')) {
      stepFocus(e.key === 'ArrowRight' ? 1 : -1)
      e.preventDefault()
    }
  }

  // ---- panel layout actions (HUD and inspector call these through app state) ----
  const selected = $derived(page.panels.find((p) => p.id === app.selPanel) ?? null)
  function split(how: 'horizontal' | 'vertical' | 'diagonal') {
    if (selected) edit(['panel', 'split', page.page, selected.id, `--${how}`])
  }
  function merge() {
    if (app.selPanels.length === 2) edit(['panel', 'merge', page.page, ...app.selPanels]).then(() => (app.selPanels = app.selPanel ? [app.selPanel] : []))
  }

  const cursorFor = (h: [number, number]) => (h[0] * h[1] > 0 ? 'nwse-resize' : h[0] * h[1] < 0 ? 'nesw-resize' : h[0] ? 'ew-resize' : 'ns-resize')
  const tailBase = (b: { x: number; y: number; w: number; h: number }): Pt => [b.x + b.w / 2, b.y + b.h / 2]
  const dropPanel = $derived(app.drag ? page.panels.find((k) => k.id === app.drag!.panel) : null)
</script>

<svelte:window onkeydown={key} />

<Stage bind:this={stage} w={W} h={H} focus={focusPanel ? focusPanel.box : null}>
  {#snippet children(scale)}
    {@const u = 1 / scale}
    <div class="paper">
      <svg bind:this={svg} viewBox="0 0 {W} {H}" class="page" class:tool-panel={app.mode === 'panel'} class:tool-balloon={app.mode === 'balloon'}
        onpointerdown={down} onpointermove={move} onpointerup={up} ondblclick={dbl}>
        <defs>
          {#each page.panels as k (k.id)}
            <clipPath id="clip-{k.id}"><polygon points={ptsAttr(pts(k))} /></clipPath>
          {/each}
        </defs>
        <rect width={W} height={H} fill="var(--sb-paper)" />

        <!-- placed boards, clipped to their panels -->
        {#each panelsByOrder as k (k.id)}
          {@const c = (k.id === app.focusPanel && place) || k.content}
          {@const b = boardByUid(c?.board)}
          {#if c && b}
            {@const g = placement(k, c, b.size)}
            <g clip-path="url(#clip-{k.id})">
              <image href={fileUrl(b.posterframe)} x={-b.size[0] / 2} y={-b.size[1] / 2} width={b.size[0]} height={b.size[1]}
                transform="translate({g.cx} {g.cy}) rotate({g.rot}) scale({g.s})" />
            </g>
          {/if}
        {/each}

        {#each ['reference', 'fill', 'tone', 'pencil', 'ink'] as l}
          {#if has(l)}<image href={layerUrl(l)} width={W} height={H} opacity={app.layerInfo[l]?.opacity ?? 1} />{/if}
        {/each}
        {#if has('frames')}<image href={layerUrl('frames')} width={W} height={H} />{/if}
        {#if has('balloons')}<image href={layerUrl('balloons')} width={W} height={H} />{/if}
        {#if has('notes')}<image href={layerUrl('notes')} width={W} height={H} opacity={app.layerInfo.notes?.opacity ?? 1} />{/if}

        <!-- panels: hit areas, drop target, layout selection -->
        {#each panelsByOrder as k (k.id)}
          {@const sel = app.mode === 'panel' && app.selPanels.includes(k.id)}
          <polygon data-panel={k.id} points={ptsAttr(pts(k))} class="hit" class:sel class:sel-draw={app.mode === 'draw' && app.selPanel === k.id && !k.content} class:drop={dropPanel?.id === k.id} stroke-width={2 * u} />
        {/each}
        {#if newRect}<rect x={newRect[0]} y={newRect[1]} width={newRect[2]} height={newRect[3]} class="new" stroke-width={1.5 * u} />{/if}

        {#if !app.focusPanel}
          {#each panelsByOrder as k (k.id)}
            {@const bx = bbox(pts(k))}
            <g class="badge" transform="translate({bx[0] + bx[2] - 17 * u} {bx[1] + 17 * u}) scale({u})">
              <circle r="10" /><text dy="4">{k.order}</text>
            </g>
          {/each}
        {/if}

        {#if app.mode === 'panel' && selected && !app.focusPanel}
          {@const bx = bbox(pts(selected))}
          {#each HANDLES as h}
            {@const [hx, hy] = handlePos(bx, h)}
            <rect class="handle" x={hx - 4 * u} y={hy - 4 * u} width={8 * u} height={8 * u} rx={2 * u} stroke-width={1.2 * u}
              style="cursor:{cursorFor(h)}" onpointerdown={(e) => begin(e, 'presize', { id: selected.id, orig: selected.points, h })} />
          {/each}
        {/if}

        <!-- balloons -->
        {#if app.mode === 'balloon' && !app.focusPanel}
          {#each page.balloons as b (b.id)}
            {@const v = balloonBox?.id === b.id ? balloonBox : b}
            {@const sel = app.selBalloon === b.id}
            {#if sel || balloonBox?.id === b.id}
              {#if b.type === 'narration' || b.type === 'sfx'}
                <rect class="bshape" x={v.x} y={v.y} width={v.w} height={v.h} stroke-width={1.5 * u} />
              {:else}
                <ellipse class="bshape" cx={v.x + v.w / 2} cy={v.y + v.h / 2} rx={v.w / 2} ry={v.h / 2} stroke-width={1.5 * u} />
              {/if}
            {/if}
            {#if sel}
              <rect class="selbox" x={v.x} y={v.y} width={v.w} height={v.h} stroke-width={1 * u} />
              {#each HANDLES as h}
                {@const [hx, hy] = handlePos([v.x, v.y, v.w, v.h], h)}
                <rect class="handle" x={hx - 4 * u} y={hy - 4 * u} width={8 * u} height={8 * u} rx={2 * u} stroke-width={1.2 * u}
                  style="cursor:{cursorFor(h)}" onpointerdown={(e) => begin(e, 'bresize', { b, h })} />
              {/each}
              {#if b.type !== 'narration' && b.type !== 'sfx'}
                {@const t = v.tail ?? [v.x + v.w / 2, v.y + v.h + 60]}
                <line class="tailline" x1={tailBase(v)[0]} y1={tailBase(v)[1]} x2={t[0]} y2={t[1]} stroke-width={1 * u} />
                <circle class="tail" cx={t[0]} cy={t[1]} r={7 * u} stroke-width={2 * u} onpointerdown={(e) => begin(e, 'tail', { b })} />
              {/if}
            {/if}
          {/each}
        {/if}

        <!-- panel focus: dim the rest, show the whole placed board with its bounds -->
        {#if focusPanel}
          {@const c = place ?? focusPanel.content}
          {@const b = boardByUid(c?.board)}
          <path class="dim" fill-rule="evenodd" d="M0 0H{W}V{H}H0Z M{pts(focusPanel).map((p) => p.join(' ')).join(' L')}Z" />
          {#if c && b}
            {@const g = placement(focusPanel, c, b.size)}
            {@const corners = [[-1, -1], [1, -1], [1, 1], [-1, 1]].map(([sx, sy]) => rotate([g.cx + (sx * g.w) / 2, g.cy + (sy * g.h) / 2], g.rot, [g.cx, g.cy]))}
            {@const top = rotate([g.cx, g.cy - g.h / 2], g.rot, [g.cx, g.cy])}
            {@const knob = rotate([g.cx, g.cy - g.h / 2 - 32 * u], g.rot, [g.cx, g.cy])}
            <image class="ghost" href={fileUrl(b.posterframe)} x={-b.size[0] / 2} y={-b.size[1] / 2} width={b.size[0]} height={b.size[1]}
              transform="translate({g.cx} {g.cy}) rotate({g.rot}) scale({g.s})" />
            <polygon class="bounds" points={ptsAttr(corners as Pt[])} stroke-width={1.5 * u}
              onpointerdown={(e) => begin(e, 'place-move', { c: { ...c }, g })} />
            <line class="bounds-line" x1={top[0]} y1={top[1]} x2={knob[0]} y2={knob[1]} stroke-width={1.5 * u} />
            {#each corners as q}
              <rect class="handle" x={q[0] - 5 * u} y={q[1] - 5 * u} width={10 * u} height={10 * u} rx={2 * u} stroke-width={1.2 * u}
                style="cursor:nwse-resize" onpointerdown={(e) => begin(e, 'place-scale', { c: { ...c }, g })} />
            {/each}
            <circle class="rot" cx={knob[0]} cy={knob[1]} r={7 * u} stroke-width={2 * u} onpointerdown={(e) => begin(e, 'place-rotate', { c: { ...c }, g })} />
          {:else}
            {@const bx = focusPanel.box}
            <polygon class="focus-outline" points={ptsAttr(focusPanel.points)} stroke-width={2 * u} />
            <text class="placeholder" x={bx[0] + bx[2] / 2} y={bx[1] + bx[3] / 2} font-size={15 * u}>Drag a board here from the Boards library</text>
          {/if}
        {/if}
      </svg>

      {#if app.mode === 'draw' && tools.current !== 'lasso' && !app.focusPanel}
        {#key page.id}
          <DrawLayer width={W} height={H} target={['--page', String(page.page)]} kind="page" ondouble={(x, y) => openFocus([x, y])}
            ontap={(x, y) => { const k = panelAt([x, y]); app.selPanel = k?.id ?? null; app.selPanels = k ? [k.id] : [] }} />
        {/key}
      {/if}

      {#if dropPanel && app.drag}
        {@const bx = dropPanel.box}
        <div class="drop-label" style="left:{(bx[0] + bx[2] / 2) * scale}px; top:{(bx[1] + bx[3]) * scale - 36}px">
          Place “{boardTitle(boardByUid(app.drag.uid))}” in Panel {dropPanel.order}
        </div>
      {/if}

      {#if app.mode === 'panel' && selected && !app.focusPanel}
        {@const bx = bbox(pts(selected))}
        <div class="hud glass" style="left:{(bx[0] + bx[2]) * scale + 44}px; top:{bx[1] * scale + 60}px">
          <button class="icon-btn" title="Split horizontally" onclick={() => split('horizontal')}><Rows2 size={16} /></button>
          <button class="icon-btn" title="Split vertically" onclick={() => split('vertical')}><Columns2 size={16} /></button>
          <button class="icon-btn" title="Split diagonally" onclick={() => split('diagonal')}><Slash size={16} /></button>
          <button class="icon-btn" title="Merge (Shift-click a second panel)" disabled={app.selPanels.length !== 2} onclick={merge}><Combine size={16} /></button>
          <button class="icon-btn red" title="Delete panel" onclick={() => edit(['panel', 'delete', page.page, ...app.selPanels])}><Trash2 size={16} /></button>
        </div>
      {/if}

      {#if app.mode === 'draw' && selected && !selected.content && !app.focusPanel}
        {@const bx = selected.box}
        <div class="empty-panel" style="left:{bx[0] * scale}px; top:{bx[1] * scale}px; width:{bx[2] * scale}px; height:{bx[3] * scale}px">
          <Pencil size={22} strokeWidth={1.4} color="#8e8e93" />
          <b>Panel {selected.order} is empty</b>
          <span>Sketch inside it, or drop a board here from the Boards library.</span>
          <div class="cta">
            <button class="btn light" onclick={() => (app.sheet = 'import')}>Add Reference…</button>
            <button class="btn light" onclick={async () => {
              const r = await edit<Balloon>(['balloon', 'add', page.page, 'Text', '--panel', selected.id])
              app.mode = 'balloon'; app.selBalloon = r.id; app.editText++
            }}>Add Balloon</button>
          </div>
        </div>
      {/if}

      {#if blank && app.mode !== 'panel'}
        <div class="blank">
          <LayoutPanelLeft size={40} strokeWidth={1.4} color="#8e8e93" />
          <h3>Page {page.page} is blank</h3>
          <p>Start with a panel layout, or just draw — panels can come later.</p>
          <div class="tpls">
            {#each [['splash', 'Splash'], ['3-tier', '3-tier'], ['big-plus-2', 'Big + 2'], ['4-koma', '4-koma']] as [id, name]}
              <button onclick={() => edit(['page', 'template', page.page, id])}><img src="/tpl-{id}.svg" alt="" /><span>{name}</span></button>
            {/each}
          </div>
          <div class="cta">
            <button class="btn primary" onclick={() => { app.mode = 'panel'; app.popover = 'templates' }}>More Templates…</button>
            <button class="btn light" onclick={() => (app.sheet = 'import')}>Import Images…</button>
          </div>
          <p class="hint">or press 4 and start sketching with the pencil</p>
        </div>
      {/if}
    </div>
  {/snippet}
  {#snippet overlay()}
    {#if focusPanel}
      <div class="nav glass">
        <button class="icon-btn" disabled={focusPanel.order <= 1} onclick={() => stepFocus(-1)}><ChevronLeft size={16} /></button>
        <span>Panel {focusPanel.order} of {page.panels.length} · reading order</span>
        <button class="icon-btn" disabled={focusPanel.order >= page.panels.length} onclick={() => stepFocus(1)}><ChevronRight size={16} /></button>
        <button class="btn primary" onclick={() => (app.focusPanel = null)}>Done</button>
      </div>
    {/if}
  {/snippet}
</Stage>

<style>
  .paper { position: absolute; inset: 0; box-shadow: 0 0 0 0.5px #0003, 0 10px 30px #0006; }
  .page { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
  .hit { fill: transparent; stroke: none; }
  .tool-panel .hit { cursor: pointer; }
  .tool-balloon { cursor: text; }
  .hit.sel { fill: #0a84ff1f; stroke: var(--sb-accent); }
  .hit.sel-draw { stroke: var(--sb-accent); }
  .hit.drop { fill: #0a84ff38; stroke: var(--sb-accent); stroke-width: 6; }
  .new { fill: #0a84ff14; stroke: var(--sb-accent); stroke-dasharray: 6 4; }
  .badge circle { fill: var(--sb-accent); stroke: #fff; stroke-width: 1.5; }
  .badge text { fill: #fff; font: 600 11px var(--sb-font); text-anchor: middle; }
  .badge { pointer-events: none; }
  .handle { fill: #fff; stroke: var(--sb-accent); }
  .bshape { fill: #0a84ff10; stroke: var(--sb-accent); pointer-events: none; }
  .selbox { fill: none; stroke: var(--sb-accent); pointer-events: none; }
  .tailline { stroke: var(--sb-accent); pointer-events: none; }
  .tail, .rot { fill: var(--sb-accent); stroke: #fff; cursor: grab; }
  .dim { fill: #000; opacity: 0.55; pointer-events: none; }
  .ghost { opacity: 0.35; pointer-events: none; }
  .bounds { fill: transparent; stroke: var(--sb-accent); cursor: move; }
  .bounds-line { stroke: var(--sb-accent); }
  .focus-outline { fill: none; stroke: var(--sb-accent); }
  .placeholder { fill: #8e8e93; text-anchor: middle; font-family: var(--sb-font); }
  .drop-label { position: absolute; transform: translateX(-50%); background: var(--sb-accent); color: #fff; padding: 4px 12px; border-radius: 12px; font-weight: 500; white-space: nowrap; pointer-events: none; }
  .hud { position: absolute; display: flex; flex-direction: column; gap: 2px; padding: 4px; border-radius: 20px; }
  .hud .red { color: var(--sb-red); }
  .nav { position: absolute; left: 50%; bottom: 24px; transform: translateX(-50%); display: flex; align-items: center; gap: 6px; height: 40px; padding: 4px; border-radius: 20px; color: var(--sb-label-2); }
  .nav span { padding: 0 6px; }
  .empty-panel { position: absolute; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px; background: #efeee9; color: #6e6e73; font-size: 11px; text-align: center; padding: 12px; pointer-events: none; }
  .empty-panel > * { pointer-events: auto; }
  .empty-panel b { color: #1d1d1f; font-size: 14px; font-weight: 500; }
  .empty-panel .cta { margin-top: 8px; }
  .blank { position: absolute; inset: 5% 7%; border: 1px solid #0000001a; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; color: #1d1d1f; pointer-events: none; }
  .blank > * { pointer-events: auto; }
  .blank h3 { margin: 8px 0 0; font-size: 19px; }
  .blank p { margin: 0; color: #6e6e73; }
  .blank .hint { font-size: 11px; color: #a1a1a6; margin-top: 18px; }
  .tpls { display: flex; gap: 12px; margin: 16px 0; }
  .tpls button { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 8px; border-radius: 8px; background: #0000000a; font-size: 11px; color: #1d1d1f; }
  .tpls button:hover { background: #0a84ff1f; }
  .tpls img { width: 48px; height: 66px; }
  .cta { display: flex; gap: 8px; }
  .btn.light { background: #fff; color: #1d1d1f; box-shadow: 0 0 0 0.5px #0003, 0 1px 2px #0002; }
</style>
