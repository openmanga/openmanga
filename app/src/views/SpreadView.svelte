<script lang="ts">
  // Two facing pages in reading order (rtl: [3|2]), from the engine's page composites.
  import { app, pageInfo, fileUrl, setPage, type PageInfo } from '../lib/sb.svelte'
  import Stage from './Stage.svelte'

  const W = $derived(app.pageSize[0]), H = $derived(app.pageSize[1])
  const nums = $derived.by(() => {
    const s = [...(app.pages[app.pageNo - 1]?.spread ?? [app.pageNo])]
    return app.reading === 'rtl' ? s.reverse() : s
  })
  let infos = $state<Record<number, PageInfo>>({})
  $effect(() => {
    void app.pages
    for (const n of nums) pageInfo(n).then((p) => { infos[n] = p }).catch(() => {})
  })

  function key(e: KeyboardEvent) {
    if ((e.target as HTMLElement).closest?.('input,textarea') || e.metaKey) return
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    const fwd = (e.key === 'ArrowLeft') === (app.reading === 'rtl')
    const s = app.pages[app.pageNo - 1]?.spread ?? [app.pageNo]
    const n = fwd ? Math.max(...s) + 1 : Math.min(...s) - 1
    if (n >= 1 && n <= app.pages.length) setPage(n)
    e.preventDefault()
  }
</script>

<svelte:window onkeydown={key} />

<Stage w={W * nums.length} h={H} pad={40}>
  {#snippet children(scale)}
    <div class="spread">
      {#each nums as n, i (n)}
        {@const p = app.pages[n - 1]}
        <button class="pg" style="left:{i * W * scale}px; width:{W * scale}px" ondblclick={() => { app.view = 'page'; setPage(n) }} onclick={() => setPage(n)}>
          <img src={fileUrl(p?.posterframe)} alt="" />
          {#if infos[n]}
            <svg viewBox="0 0 {W} {H}">
              {#each infos[n].panels as k}
                <g transform="translate({k.box[0] + k.box[2] - 17 / scale} {k.box[1] + 17 / scale}) scale({1 / scale})">
                  <circle r="10" /><text dy="4">{k.order}</text>
                </g>
              {/each}
            </svg>
          {/if}
        </button>
      {/each}
      {#if nums.length === 2}<div class="fold" style="left:{W * scale}px"></div>{/if}
    </div>
  {/snippet}
</Stage>

<style>
  .spread { position: absolute; inset: 0; box-shadow: 0 10px 30px #0006; background: var(--sb-paper); }
  .pg { position: absolute; top: 0; bottom: 0; }
  .pg img, .pg svg { position: absolute; inset: 0; width: 100%; height: 100%; }
  circle { fill: var(--sb-accent); stroke: #fff; stroke-width: 1.5; }
  text { fill: #fff; font: 600 11px var(--sb-font); text-anchor: middle; }
  .fold { position: absolute; top: 0; bottom: 0; width: 1px; background: #0002; box-shadow: 0 0 18px 6px #0000000d; }
</style>
