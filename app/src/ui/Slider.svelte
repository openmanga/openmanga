<script lang="ts">
  let { value, min = 0, max = 1, step = 0.01, oninput, onchange, width = 110 }:
    { value: number; min?: number; max?: number; step?: number; oninput?: (v: number) => void; onchange?: (v: number) => void; width?: number } = $props()
  const pct = $derived(((value - min) / (max - min)) * 100)
</script>

<input type="range" {min} {max} {step} {value} style="width:{width}px; --p:{pct}%"
  oninput={(e) => oninput?.(+e.currentTarget.value)}
  onchange={(e) => onchange?.(+e.currentTarget.value)} />

<style>
  input { -webkit-appearance: none; appearance: none; height: 20px; background: transparent; margin: 0; }
  input::-webkit-slider-runnable-track {
    height: 4px; border-radius: 2px;
    background: linear-gradient(to right, var(--sb-accent) var(--p), #78788052 var(--p));
  }
  input::-webkit-slider-thumb {
    -webkit-appearance: none; width: 22px; height: 15px; margin-top: -5.5px; border-radius: 8px;
    background: #fff; box-shadow: 0 0.5px 2px #0006;
  }
</style>
