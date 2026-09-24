<script lang="ts" generics="T">
  let { value, options, onchange }: { value: T; options: [T, string][]; onchange: (v: any) => void } = $props()
</script>

<div class="seg">
  {#each options as [v, label]}
    <button class:on={v === value} onclick={() => onchange(v)}>{label}</button>
  {/each}
</div>
