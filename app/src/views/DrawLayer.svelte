<script lang="ts">
  // Live pressure drawing over a board or page. Strokes are drawn locally while the pen is down,
  // then committed with `sb draw strokes` so the saved pixels come from the engine's renderer.
  import { sb, edit, bumpLoaded, refresh, app } from '../lib/sb.svelte'
  import { tools, cur, toolLayer } from '../lib/tools.svelte'
  import { pushEdit } from '../lib/history.svelte'
  import { curve, pref } from '../lib/prefs.svelte'

  let {
    width, height, target, kind, ondouble, ontap,
  }: { width: number; height: number; target: string[]; kind: 'board' | 'page'; ondouble?: (x: number, y: number) => void; ontap?: (x: number, y: number) => void } = $props()

  type Stroke = { straight?: boolean; pts: number[][]; tool: string; layer: string; color: string; size: number; opacity: number }

  let canvas: HTMLCanvasElement
  const scratch = document.createElement('canvas')
  let live: Stroke | null = null
  let queue: Stroke[] = [] // waiting or being committed; still shown until the new layer image is loaded
  let busy = false

  // lasso selection (boards only)
  let lasso = $state<number[][]>([])
  let lassoClosed = $state(false)
  let moveFrom: number[] | null = null
  let moveBy = $state([0, 0])

  const isLasso = () => tools.current === 'lasso' && kind === 'board'

  function toTarget(e: PointerEvent | MouseEvent): number[] {
    const r = canvas.getBoundingClientRect()
    return [((e.clientX - r.left) / r.width) * width, ((e.clientY - r.top) / r.height) * height]
  }

  function pressure(e: PointerEvent) {
    if (e.pointerType !== 'pen') return 1 // mouse/trackpad: full width, the engine still tapers the ends
    return cur().pressure ? Math.max(0.05, curve(e.pressure)) : 1
  }

  function down(e: PointerEvent) {
    if (e.button === 1 || app.drag) return
    try { canvas.setPointerCapture(e.pointerId) } catch {}
    const p = toTarget(e)
    if (isLasso()) {
      if (lassoClosed && inside(p, lasso)) { moveFrom = p; return }
      lasso = [p]; lassoClosed = false; moveBy = [0, 0]
      return
    }
    // quick erase: right button, the pen's eraser end, or Alt
    const erase = e.button === 2 || e.button === 5 || (e.buttons & 32) !== 0 || e.altKey
    const tool = erase ? 'eraser' : tools.current
    const s = tools.settings[tool]
    live = { pts: [[...p, pressure(e)]], tool, layer: toolLayer(), color: s.color, size: s.size, opacity: s.opacity }
    draw()
  }

  // holding still mid-stroke turns it into a straight line (prefs straightLineDelayInMsecs)
  let holdTimer: any
  let holdAt: number[] | null = null
  function armHold(p: number[]) {
    if (holdAt && Math.hypot(p[0] - holdAt[0], p[1] - holdAt[1]) < 4) return
    holdAt = p
    clearTimeout(holdTimer)
    holdTimer = setTimeout(() => {
      if (live && live.pts.length > 2) { live.straight = true; live.pts = [live.pts[0], live.pts[live.pts.length - 1]]; draw() }
    }, pref<number>('straightLineDelayInMsecs'))
  }

  function move(e: PointerEvent) {
    if (isLasso()) {
      if (moveFrom) { const p = toTarget(e); moveBy = [p[0] - moveFrom[0], p[1] - moveFrom[1]]; return }
      if (lasso.length && !lassoClosed && e.buttons) lasso.push(toTarget(e))
      return
    }
    if (!live) return
    const co = e.getCoalescedEvents?.()
    const evs = co?.length ? co : [e] // empty for synthetic events
    for (const ce of evs) {
      const p = [...toTarget(ce), pressure(ce)]
      if (e.shiftKey || live.straight) {
        // straight line from the first point; Alt snaps the angle to 15°
        const a = live.pts[0]
        if (e.altKey) {
          const ang = Math.round(Math.atan2(p[1] - a[1], p[0] - a[0]) / (Math.PI / 12)) * (Math.PI / 12)
          const len = Math.hypot(p[0] - a[0], p[1] - a[1])
          p[0] = a[0] + Math.cos(ang) * len
          p[1] = a[1] + Math.sin(ang) * len
        }
        live.pts = [a, p]
      } else live.pts.push(p)
    }
    if (!live.straight) armHold(live.pts[live.pts.length - 1])
    draw()
  }

  function up() {
    if (isLasso()) {
      if (moveFrom) { commitMove(); moveFrom = null; return }
      if (lasso.length > 2) lassoClosed = true
      else lasso = []
      return
    }
    clearTimeout(holdTimer)
    holdAt = null
    if (!live) return
    const s = live
    live = null
    if (s.pts.length < 2) { draw(); ontap?.(s.pts[0][0], s.pts[0][1]); return } // a click is not a stroke: it selects (double-click opens panels)
    queue.push(s)
    flush()
  }

  async function flush() {
    if (busy) return
    busy = true
    try {
      while (queue.length) {
        const s = queue[0]
        const args = ['draw', 'strokes', ...target, '--tool', s.tool, '--layer', s.layer, '-']
        const doc: any = { size: s.size, opacity: s.opacity, strokes: [{ points: s.pts.map((p) => p.map((v) => Math.round(v * 100) / 100)) }] }
        if (s.tool !== 'eraser') doc.color = s.color
        try {
          const r = await sb<any>(args, JSON.stringify(doc))
          pushEdit(target, s.layer)
          await bumpLoaded([r.file, r.posterframe, r.thumbnail])
        } catch (e: any) {
          app.error = e.message
        }
        queue.shift()
        draw()
      }
    } finally {
      busy = false
    }
    refresh()
  }

  let raf = 0
  function draw() {
    cancelAnimationFrame(raf)
    raf = requestAnimationFrame(render)
  }
  function render() {
    if (!canvas) return
    const ctx = canvas.getContext('2d')!
    ctx.clearRect(0, 0, width, height)
    const sctx = scratch.getContext('2d')!
    for (const s of live ? [...queue, live] : queue) {
      sctx.clearRect(0, 0, width, height)
      sctx.lineCap = sctx.lineJoin = 'round'
      sctx.strokeStyle = s.tool === 'eraser' ? '#ffffff' : s.color
      for (let i = 1; i < s.pts.length; i++) {
        const a = s.pts[i - 1], b = s.pts[i]
        sctx.lineWidth = Math.max(0.5, s.size * (a[2] + b[2]) / 2)
        sctx.beginPath()
        sctx.moveTo(a[0], a[1])
        sctx.lineTo(b[0], b[1])
        sctx.stroke()
      }
      ctx.globalAlpha = s.tool === 'eraser' ? 1 : Math.max(s.opacity, 0.35)
      ctx.drawImage(scratch, 0, 0)
      ctx.globalAlpha = 1
    }
  }

  $effect(() => {
    scratch.width = width
    scratch.height = height
    draw()
  })

  // ---- lasso ----
  function inside(p: number[], poly: number[][]) {
    let c = false
    for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
      const [xi, yi] = poly[i], [xj, yj] = poly[j]
      if (yi > p[1] !== yj > p[1] && p[0] < ((xj - xi) * (p[1] - yi)) / (yj - yi) + xi) c = !c
    }
    return c
  }
  const polyArg = () => lasso.map((p) => `${Math.round(p[0])},${Math.round(p[1])}`).join(' ')
  const uid = () => target[0]
  export async function lassoErase() { if (lassoClosed) { await edit(['board', 'erase-region', uid(), '--polygon', polyArg()]); lasso = []; lassoClosed = false } }
  export async function lassoFill() {
    if (!lassoClosed) return
    await edit(['board', 'fill-region', uid(), '--polygon', polyArg(), '--color', tools.settings.brush.color])
    lasso = []; lassoClosed = false
  }
  async function commitMove() {
    const [dx, dy] = moveBy.map(Math.round)
    if (!dx && !dy) return
    await edit(['board', 'move-region', uid(), '--polygon', polyArg(), '--dx', dx, '--dy', dy])
    lasso = lasso.map((p) => [p[0] + dx, p[1] + dy])
    moveBy = [0, 0]
  }
  export function lassoCancel() { lasso = []; lassoClosed = false; moveBy = [0, 0] }
  export const hasLasso = () => lassoClosed

  $effect(() => { if (tools.current !== 'lasso') lassoCancel() })

  function key(e: KeyboardEvent) {
    if (!lassoClosed || (e.target as HTMLElement).closest?.('input,textarea')) return
    if (e.key === 'Backspace' || e.key === 'Delete') { e.preventDefault(); lassoErase() }
    else if (e.key === 'f') lassoFill()
    else if (e.key === 'Escape' || e.key === 'Enter') lassoCancel()
  }
</script>

<svelte:window onkeydown={key} />

<canvas
  bind:this={canvas}
  {width}
  {height}
  class:lasso={isLasso()}
  onpointerdown={down}
  onpointermove={move}
  onpointerup={up}
  onpointercancel={up}
  oncontextmenu={(e) => e.preventDefault()}
  ondblclick={(e) => { const p = toTarget(e); ondouble?.(p[0], p[1]) }}
></canvas>

{#if lasso.length > 1}
  <svg class="lasso-svg" viewBox="0 0 {width} {height}" preserveAspectRatio="none">
    <polygon points={lasso.map((p) => `${p[0] + moveBy[0]},${p[1] + moveBy[1]}`).join(' ')} vector-effect="non-scaling-stroke" />
  </svg>
{/if}
{#if lassoClosed}
  <div class="lasso-bar glass">
    <button class="icon-btn label" onclick={lassoErase}>Erase <span class="faint">⌫</span></button>
    <button class="icon-btn label" onclick={lassoFill}>Fill <span class="faint">F</span></button>
    <button class="icon-btn label" onclick={lassoCancel}>Done</button>
  </div>
{/if}

<style>
  canvas { position: absolute; inset: 0; width: 100%; height: 100%; touch-action: none; cursor: crosshair; }
  canvas.lasso { cursor: default; }
  .lasso-svg { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; overflow: visible; }
  polygon { fill: #0a84ff14; stroke: var(--sb-accent); stroke-width: 1.5; stroke-dasharray: 5 4; }
  .lasso-bar { position: absolute; left: 50%; bottom: -56px; transform: translateX(-50%); display: flex; gap: 2px; padding: 4px; border-radius: 20px; }
</style>
