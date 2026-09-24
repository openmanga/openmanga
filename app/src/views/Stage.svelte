<script lang="ts">
  // Zoom (pinch / ⌘-wheel, 0.25–5) and pan (wheel, Space-drag) around content of w×h pixels.
  import type { Snippet } from 'svelte'
  import { app } from '../lib/sb.svelte'

  let {
    w, h, pad = 36, focus = null, children, overlay,
  }: { w: number; h: number; pad?: number; focus?: [number, number, number, number] | null; children: Snippet<[number]>; overlay?: Snippet } = $props()

  let el: HTMLDivElement
  let vw = $state(0), vh = $state(0)
  let scale = $state(1), tx = $state(0), ty = $state(0)
  let space = $state(false)
  let panning: null | { x: number; y: number; tx: number; ty: number } = null

  /** Fits the box [x,y,w,h] (content px) into the view. */
  export function fitBox(b: [number, number, number, number], p = pad) {
    if (!vw || !vh) return
    scale = Math.min((vw - p * 2) / b[2], (vh - p * 2) / b[3])
    tx = (vw - b[2] * scale) / 2 - b[0] * scale
    ty = (vh - b[3] * scale) / 2 - b[1] * scale
    app.zoom = scale
  }
  export const fit = () => fitBox([0, 0, w, h])

  $effect(() => {
    void app.fitRequest; void w; void h; void vw; void vh
    if (focus) fitBox(focus, 60)
    else fit()
  })

  function zoomAt(f: number, cx: number, cy: number) {
    const ns = Math.min(5, Math.max(0.1, scale * f))
    tx = cx - ((cx - tx) * ns) / scale
    ty = cy - ((cy - ty) * ns) / scale
    scale = ns
    app.zoom = scale
  }
  export function zoomBy(f: number) { zoomAt(f, vw / 2, vh / 2) }
  export function actualSize() { zoomAt(1 / scale, vw / 2, vh / 2) }

  function wheel(e: WheelEvent) {
    e.preventDefault()
    const r = el.getBoundingClientRect()
    if (e.ctrlKey || e.metaKey) zoomAt(Math.exp(-e.deltaY * 0.01), e.clientX - r.left, e.clientY - r.top)
    else { tx -= e.deltaX; ty -= e.deltaY }
  }

  function down(e: PointerEvent) {
    if (!(space || e.button === 1)) return
    e.stopPropagation(); e.preventDefault()
    el.setPointerCapture(e.pointerId)
    panning = { x: e.clientX, y: e.clientY, tx, ty }
  }
  function move(e: PointerEvent) {
    if (!panning) return
    e.stopPropagation()
    tx = panning.tx + e.clientX - panning.x
    ty = panning.ty + e.clientY - panning.y
  }
  function up(e: PointerEvent) { if (panning) { e.stopPropagation(); panning = null } }

  const typing = (e: KeyboardEvent) => (e.target as HTMLElement).closest?.('input,textarea')
</script>

<svelte:window
  onkeydown={(e) => { if (e.code === 'Space' && !typing(e)) { space = true; e.preventDefault() } }}
  onkeyup={(e) => { if (e.code === 'Space') space = false }}
/>

<div class="stage" class:space bind:this={el} bind:clientWidth={vw} bind:clientHeight={vh}
  onwheel={wheel} onpointerdowncapture={down} onpointermovecapture={move} onpointerupcapture={up}>
  <div class="content" style="left:{tx}px; top:{ty}px; width:{w * scale}px; height:{h * scale}px">
    {@render children(scale)}
  </div>
  {@render overlay?.()}
</div>

<style>
  .stage { position: absolute; inset: 0; overflow: hidden; }
  .stage.space, .stage.space :global(*) { cursor: grab !important; }
  .content { position: absolute; }
</style>
