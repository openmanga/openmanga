<script lang="ts">
  // The focused drawing space for one board: stacked layer PNGs, onion skin, live drawing on top.
  import { ChevronLeft, ChevronRight } from 'lucide-svelte'
  import { app, fileUrl, curBoard, LAYERS } from '../lib/sb.svelte'
  import Stage from './Stage.svelte'
  import DrawLayer from './DrawLayer.svelte'
  import { pref } from '../lib/prefs.svelte'

  const g = $derived(pref<Record<string, boolean>>('guides'))

  let { stage = $bindable() }: { stage?: Stage } = $props()

  const board = $derived(curBoard())
  const prev = $derived(app.boards[app.boardNo - 2])
  const next = $derived(app.boards[app.boardNo])

  const missing = $state<Record<string, boolean>>({})
</script>

{#if board}
  <Stage bind:this={stage} w={board.size[0]} h={board.size[1]} pad={48}>
    {#snippet children()}
      <div class="paper">
        {#if app.onion}
          {#if prev}<img class="onion prev" style="opacity:{pref('onionOpacity')}" src={fileUrl(prev.posterframe)} alt="" />{/if}
          {#if next}<img class="onion next" style="opacity:{pref('onionOpacity')}" src={fileUrl(next.posterframe)} alt="" />{/if}
        {/if}
        {#each LAYERS as l (l)}
          {@const src = fileUrl(`${board.base}-${l}.png`)}
          <img {src} alt="" class:missing={missing[src]} onerror={() => (missing[src] = true)} onload={() => (missing[src] = false)}
            style="opacity:{app.hidden[l] ? 0 : (app.layerInfo[l]?.opacity ?? 1)}" />
        {/each}
        {#if g.grid || g.center || g.thirds || g.safe}
          {@const [w, h] = board.size}
          {@const step = pref<number>('gridSize')}
          <svg class="guides" viewBox="0 0 {w} {h}" stroke={pref('guideColor')} fill="none" stroke-width={w / 900}>
            {#if g.grid}
              {#each Array.from({ length: Math.floor(w / step) }, (_, i) => (i + 1) * step) as x}<line x1={x} y1="0" x2={x} y2={h} opacity="0.35" />{/each}
              {#each Array.from({ length: Math.floor(h / step) }, (_, i) => (i + 1) * step) as y}<line x1="0" y1={y} x2={w} y2={y} opacity="0.35" />{/each}
            {/if}
            {#if g.center}<line x1={w / 2} y1="0" x2={w / 2} y2={h} /><line x1="0" y1={h / 2} x2={w} y2={h / 2} />{/if}
            {#if g.thirds}{#each [1, 2] as k}<line x1={(w * k) / 3} y1="0" x2={(w * k) / 3} y2={h} /><line x1="0" y1={(h * k) / 3} x2={w} y2={(h * k) / 3} />{/each}{/if}
            {#if g.safe}<rect x={w * 0.05} y={h * 0.05} width={w * 0.9} height={h * 0.9} />{/if}
          </svg>
        {/if}
        {#key board.uid}
          <DrawLayer width={board.size[0]} height={board.size[1]} target={[board.uid]} kind="board" />
        {/key}
      </div>
    {/snippet}
    {#snippet overlay()}
      <div class="nav glass">
        <button class="icon-btn" disabled={!prev} onclick={() => app.boardNo--}><ChevronLeft size={16} /></button>
        <span>Board {app.boardNo} of {app.boards.length}</span>
        <button class="icon-btn" disabled={!next} onclick={() => app.boardNo++}><ChevronRight size={16} /></button>
      </div>
    {/snippet}
  </Stage>
{/if}

<svg width="0" height="0" style="position:absolute">
  <!-- onion skin: dark lines become tinted, paper becomes transparent -->
  <filter id="onion-blue"><feColorMatrix type="matrix" values="0 0 0 0 0.1  0 0 0 0 0.35  0 0 0 0 1  -0.33 -0.33 -0.33 0 1" /></filter>
  <filter id="onion-red"><feColorMatrix type="matrix" values="0 0 0 0 1  0 0 0 0 0.2  0 0 0 0 0.2  -0.33 -0.33 -0.33 0 1" /></filter>
</svg>

<style>
  .paper { position: absolute; inset: 0; background: #fff; box-shadow: 0 0 0 0.5px #0003, 0 10px 30px #0006; }
  .paper img { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; }
  .paper img.missing { visibility: hidden; }
  .guides { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; }
  .onion.prev { filter: url(#onion-blue); }
  .onion.next { filter: url(#onion-red); }
  .nav { position: absolute; left: 50%; bottom: 24px; transform: translateX(-50%); display: flex; align-items: center; gap: 6px; height: 40px; padding: 4px; border-radius: 20px; color: var(--sb-label-2); }
  .nav span { padding: 0 4px; }
</style>
