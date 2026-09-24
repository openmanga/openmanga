<script lang="ts">
  let { on = false, onchange, disabled = false }: { on?: boolean; onchange?: (v: boolean) => void; disabled?: boolean } = $props()
</script>

<button class="toggle" class:on {disabled} aria-pressed={on} onclick={() => onchange?.(!on)}><span></span></button>

<style>
  .toggle { width: 32px; height: 19px; border-radius: 10px; background: #78788052; position: relative; flex: none; transition: background 0.15s; }
  .toggle.on { background: var(--sb-accent); }
  .toggle span { position: absolute; top: 2px; left: 2px; width: 15px; height: 15px; border-radius: 50%; background: #fff; box-shadow: 0 1px 2px #0004; transition: transform 0.15s; }
  .toggle.on span { transform: translateX(13px); }
  .toggle:disabled { opacity: 0.4; }
</style>
