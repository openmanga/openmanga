<script lang="ts">
  // The Settings window (its own Tauri window, index.html#settings). Values live in pref.json via `sb prefs`.
  import { Settings as Gear, Pencil, Grid3x3, Keyboard, Globe, Search } from 'lucide-svelte'
  import { open } from '@tauri-apps/plugin-dialog'
  import { sb, PAGE_PRESETS } from '../lib/sb.svelte'
  import { tools, loadToolPrefs, TOOL_NAMES } from '../lib/tools.svelte'
  import { prefs, pref, setPref, loadPrefs, followPrefs, curve } from '../lib/prefs.svelte'
  import Toggle from '../ui/Toggle.svelte'
  import Slider from '../ui/Slider.svelte'
  import Seg from '../ui/Seg.svelte'

  type Tab = 'general' | 'drawing' | 'canvas' | 'keyboard' | 'language'
  const TABS: [Tab, string, any][] = [['general', 'General', Gear], ['drawing', 'Drawing', Pencil], ['canvas', 'Canvas', Grid3x3], ['keyboard', 'Keyboard', Keyboard], ['language', 'Language', Globe]]
  let tab = $state<Tab>('general')
  let ready = $state(false)


  // ---- drawing ----
  const sizeTools = ['light-pencil', 'pencil', 'pen', 'brush', 'tone', 'note-pen', 'eraser']
  const sizeOf = (t: string) => pref<Record<string, number>>('toolSizes')[t] ?? tools.settings[t].size
  const setSize = (t: string, v: number) => setPref('toolSizes', { ...pref('toolSizes'), [t]: Math.round(v) })
  const curvePath = $derived.by(() => {
    void prefs.pressureCurve
    return Array.from({ length: 21 }, (_, i) => `${(i / 20) * 170},${130 - curve(i / 20) * 120}`).join(' ')
  })

  // ---- guides ----
  const guides = $derived(pref<Record<string, boolean>>('guides'))
  const toggleGuide = (k: string) => setPref('guides', { ...guides, [k]: !guides[k] })

  // ---- keyboard ----
  const KEYS: [string, [string, string[]][]][] = [
    ['Tools', [['Light Pencil', ['menu:tools:light-pencil']], ['Brush', ['menu:tools:brush']], ['Tone', ['menu:tools:tone']], ['Pencil', ['menu:tools:pencil']], ['Pen', ['menu:tools:pen']], ['Note Pen', ['menu:tools:note-pen']], ['Eraser', ['menu:tools:eraser']], ['Lasso Selection', ['drawing:marquee-mode']], ['Palette Color 1 / 2 / 3', ['menu:tools:palette-color-1', 'menu:tools:palette-color-2', 'menu:tools:palette-color-3']], ['Panel Tool', ['menu:tools:panel']], ['Balloon Tool', ['menu:tools:balloon']]]],
    ['Drawing', [['Brush Size Smaller / Larger', ['drawing:brush-size:dec', 'drawing:brush-size:inc']], ['Straight Line (hold)', ['drawing:straight-line']], ['Pan (hold)', ['drawing:pan-mode']], ['Clear Layer', ['menu:tools:clear-layer']]]],
    ['Boards and view', [['New Board', ['menu:boards:new-board']], ['Duplicate Board', ['menu:boards:duplicate']], ['Previous / Next', ['menu:navigation:previous-board', 'menu:navigation:next-board']], ['Onion Skin', ['menu:view:onion-skin']]]],
    ['Edit', [['Undo', ['menu:edit:undo']], ['Redo', ['menu:edit:redo']], ['Import Images', ['menu:file:import-images']]]],
  ]
  let query = $state('')
  let recording = $state<string | null>(null)
  const pretty = (a: string | undefined) => (a ?? '').replace(/CommandOrControl\+?/g, '⌘').replace(/Shift\+?/g, '⇧').replace(/Alt\+?/g, '⌥')
    .replace(/Backspace/, '⌫').replace(/Left/, '←').replace(/Right/, '→').replace(/Space/, 'Space').replace(/^([a-z])$/, (m) => m.toUpperCase())
  function record(e: KeyboardEvent) {
    if (!recording) return
    e.preventDefault()
    if (['Meta', 'Shift', 'Alt', 'Control'].includes(e.key)) return
    if (e.key === 'Escape') { recording = null; return }
    const names: Record<string, string> = { ArrowLeft: 'Left', ArrowRight: 'Right', ArrowUp: 'Up', ArrowDown: 'Down', ' ': 'Space' }
    const k = names[e.key] ?? (e.key.length === 1 ? e.key.toLowerCase() : e.key)
    const accel = [e.metaKey && 'CommandOrControl', e.shiftKey && 'Shift', e.altKey && 'Alt', k].filter(Boolean).join('+')
    const cmd = recording
    recording = null
    sb(['keymap', 'set', cmd, accel], undefined, false).then(() => (tools.keymap[cmd] = accel))
  }

  // ---- language ----
  let langs = $state<{ fileName: string; displayName: string; builtIn: boolean; selected: boolean }[]>([])
  async function loadLangs() { langs = (await sb<any>(['lang', 'list'], undefined, false)).languages }
  const langAct = async (...a: string[]) => { await sb(['lang', ...a], undefined, false); await loadLangs() }
  const selectedLang = $derived(langs.find((l) => l.selected)?.fileName ?? 'en-US')
  async function importLang() {
    const f = await open({ filters: [{ name: 'Locale', extensions: ['json'] }] })
    if (typeof f === 'string') langAct('import', f)
  }
  async function exportLang() {
    const d = await open({ directory: true })
    if (typeof d === 'string') langAct('export', selectedLang, d)
  }

  Promise.all([loadPrefs(), loadToolPrefs(), loadLangs()]).then(() => (ready = true))
  followPrefs()
</script>

<svelte:window onkeydown={record} />

<div class="win">
  <header data-tauri-drag-region>
    <div class="title" data-tauri-drag-region>{TABS.find((t) => t[0] === tab)?.[1]}</div>
    <nav data-tauri-drag-region>
      {#each TABS as [id, name, I]}
        <button class:on={tab === id} onclick={() => (tab = id)}><I size={20} strokeWidth={1.5} /><span>{name}</span></button>
      {/each}
    </nav>
  </header>

  {#if ready}
    <div class="body">
      {#if tab === 'general'}
        <h3>Saving</h3>
        <div class="group">
          <div class="row"><span>Autosave<small>Every drawing and layout change is written to disk as you make it</small></span><Toggle on={pref('enableAutoSave')} onchange={(v) => setPref('enableAutoSave', v)} /></div>
          <div class="row"><span>Keep layer history</span><span class="muted">Last 20 versions per layer</span></div>
        </div>
        <h3>New projects</h3>
        <div class="group">
          <div class="row"><span>Default page size</span>
            <select class="popup" value={pref('mangaDefaultPageSize')} onchange={(e) => setPref('mangaDefaultPageSize', e.currentTarget.value)}>
              {#each PAGE_PRESETS as p}<option value={p.id}>{p.name} ({p.sub})</option>{/each}
            </select>
          </div>
          <div class="row"><span>Reading direction</span><Seg value={pref('mangaReadingDirection')} options={[['rtl', 'Right to left'], ['ltr', 'Left to right']]} onchange={(v) => setPref('mangaReadingDirection', v)} /></div>
          <div class="row"><span>Starting pages</span><input class="field n" type="number" min="1" max="400" value={pref('mangaStartingPages')} onchange={(e) => setPref('mangaStartingPages', +e.currentTarget.value)} /></div>
        </div>
        <h3>Interface</h3>
        <div class="group">
          <div class="row"><span>Show tooltips</span><Toggle on={pref('enableTooltips')} onchange={(v) => setPref('enableTooltips', v)} /></div>
          <div class="row"><span>Interface sounds</span><Toggle on={pref('enableUISoundEffects')} onchange={(v) => setPref('enableUISoundEffects', v)} /></div>
          <div class="row"><span>Appearance</span><Seg value={pref('appearance')} options={[['auto', 'Auto'], ['light', 'Light'], ['dark', 'Dark']]} onchange={(v) => setPref('appearance', v)} /></div>
        </div>
      {:else if tab === 'drawing'}
        <h3>Pen pressure</h3>
        <div class="group pp">
          <svg viewBox="0 0 170 130" class="curve">
            <path d="M0 0H170M0 43H170M0 86H170M57 0V130M113 0V130" stroke="#ffffff14" />
            <line x1="0" y1="130" x2="170" y2="10" stroke="#ffffff30" />
            <polyline points={curvePath} fill="none" stroke="var(--sb-accent)" stroke-width="2.5" />
          </svg>
          <div class="ppr">
            <Seg value={pref('pressureCurve')} options={[['soft', 'Soft'], ['linear', 'Linear'], ['firm', 'Firm']]} onchange={(v) => setPref('pressureCurve', v)} />
            <p class="small muted">{pref('pressureCurve') === 'soft' ? 'Light pressure already gives a visible line.' : pref('pressureCurve') === 'firm' ? 'Press harder for full width.' : 'Width follows pressure one to one.'}</p>
            <svg viewBox="0 0 300 40" class="test">
              {#each Array.from({ length: 60 }, (_, i) => i) as i}
                {@const t = i / 59}
                <circle cx={10 + t * 280} cy="20" r={Math.max(0.4, curve(Math.sin(t * Math.PI)) * 5)} fill="#333" />
              {/each}
            </svg>
          </div>
        </div>
        <h3>Strokes</h3>
        <div class="group">
          <div class="row"><span>Straight-line delay<small>Hold still to turn a stroke into a straight line</small></span>
            <span class="r"><Slider value={pref('straightLineDelayInMsecs')} min={200} max={2000} step={50} oninput={(v) => (prefs.straightLineDelayInMsecs = v)} onchange={(v) => setPref('straightLineDelayInMsecs', v)} /><span class="val">{pref('straightLineDelayInMsecs')} ms</span></span></div>
          <div class="row"><span>Tilt changes width</span><Toggle on={pref('tiltChangesWidth')} onchange={(v) => setPref('tiltChangesWidth', v)} /></div>
          <div class="row"><span>High-quality engine</span><Toggle on={pref('enableHighQualityDrawingEngine')} onchange={(v) => setPref('enableHighQualityDrawingEngine', v)} /></div>
        </div>
        <h3>Default sizes</h3>
        <div class="group">
          {#each sizeTools as t}
            <div class="row"><span>{TOOL_NAMES[t as keyof typeof TOOL_NAMES]}</span>
              <span class="r"><Slider value={Math.sqrt(sizeOf(t))} min={1} max={16} step={0.05} onchange={(v) => setSize(t, v * v)} /><span class="val">{sizeOf(t)} px</span></span></div>
          {/each}
        </div>
      {:else if tab === 'canvas'}
        <h3>Guides</h3>
        <div class="group">
          <div class="guides">
            {#each [['grid', 'Grid'], ['center', 'Center'], ['thirds', 'Thirds'], ['safe', 'Safe area']] as [k, name]}
              <button class:on={guides[k]} onclick={() => toggleGuide(k)}>
                <svg viewBox="0 0 54 76">
                  <rect width="54" height="76" fill="#FBFAF6" />
                  <g stroke="var(--sb-accent)" stroke-width="1" fill="none">
                    {#if k === 'grid'}{#each [9, 18, 27, 36, 45] as x}<line x1={x} y1="0" x2={x} y2="76" />{/each}{#each [9, 18, 27, 36, 45, 54, 63, 72] as y}<line x1="0" y1={y} x2="54" y2={y} />{/each}
                    {:else if k === 'center'}<line x1="27" y1="0" x2="27" y2="76" /><line x1="0" y1="38" x2="54" y2="38" />
                    {:else if k === 'thirds'}<line x1="18" y1="0" x2="18" y2="76" /><line x1="36" y1="0" x2="36" y2="76" /><line x1="0" y1="25" x2="54" y2="25" /><line x1="0" y1="51" x2="54" y2="51" />
                    {:else}<rect x="4" y="4" width="46" height="68" />{/if}
                  </g>
                </svg>
                <span>{name}</span>
              </button>
            {/each}
          </div>
          <div class="row"><span>Grid size</span>
            <select class="popup" value={String(pref('gridSize'))} onchange={(e) => setPref('gridSize', +e.currentTarget.value)}>
              {#each [25, 50, 100, 200] as g}<option value={String(g)}>{g} px</option>{/each}
            </select></div>
          <div class="row"><span>Guide color</span><label class="well" style="background:{pref('guideColor')}"><input type="color" value={pref('guideColor')} onchange={(e) => setPref('guideColor', e.currentTarget.value)} /></label></div>
        </div>
        <h3>Onion skin</h3>
        <div class="group">
          <div class="row"><span>Pages<small>Boards before (blue) and after (red) the current one</small></span><span class="muted">Previous and next</span></div>
          <div class="row"><span>Opacity</span><span class="r"><Slider value={pref('onionOpacity')} min={0.1} max={0.8} oninput={(v) => (prefs.onionOpacity = v)} onchange={(v) => setPref('onionOpacity', v)} /><span class="val">{Math.round(pref('onionOpacity') * 100)}%</span></span></div>
        </div>
        <h3>Desk</h3>
        <div class="group">
          <div class="row"><span>Desk color</span><Seg value={pref('deskColor')} options={[['match', 'Match appearance'], ['light', 'Light'], ['dark', 'Dark']]} onchange={(v) => setPref('deskColor', v)} /></div>
        </div>
      {:else if tab === 'keyboard'}
        <div class="kbar">
          <label class="search"><Search size={13} /><input placeholder="Search commands" bind:value={query} /></label>
        </div>
        <div class="table">
          <div class="th"><span>Command</span><span>Shortcut</span></div>
          {#each KEYS as [group, rows]}
            {@const shown = rows.filter(([n]) => !query || n.toLowerCase().includes(query.toLowerCase()))}
            {#if shown.length}
              <div class="tg">{group}</div>
              {#each shown as [name, cmds], i}
                <div class="tr" class:alt={i % 2}>
                  <span>{name}</span>
                  <span class="keys">
                    {#each cmds as c}
                      <button class="kbd" class:rec={recording === c} title="Click, then press the new shortcut" onclick={() => (recording = c)}>{recording === c ? 'Type…' : pretty(tools.keymap[c]) || '—'}</button>
                    {/each}
                  </span>
                </div>
              {/each}
            {/if}
          {/each}
        </div>
      {:else}
        <h3>Interface language</h3>
        <div class="group">
          {#each langs as l}
            <button class="row lang" onclick={() => langAct('set', l.fileName)}>
              <span class="radio" class:on={l.selected}></span>
              <span class="ln">{l.displayName}<small>{l.fileName}</small></span>
              <span class="badge">{l.builtIn ? 'Built-in' : 'Custom'}</span>
            </button>
          {/each}
        </div>
        <div class="lbtns">
          <button class="btn" onclick={() => langAct('copy', selectedLang)}>Duplicate…</button>
          <button class="btn" onclick={importLang}>Import…</button>
          <button class="btn" onclick={exportLang}>Export…</button>
          {#if !langs.find((l) => l.selected)?.builtIn}<button class="btn danger" onclick={() => langAct('remove', selectedLang)}>Remove</button>{/if}
        </div>
        <p class="small muted">The language changes immediately; no restart needed.</p>
      {/if}
    </div>
  {/if}
</div>

<style>
  .win { position: absolute; inset: 0; background: var(--sb-sheet); display: flex; flex-direction: column; }
  header { flex: none; padding-top: 8px; border-bottom: 0.5px solid var(--sb-separator); }
  .title { text-align: center; font-weight: 600; height: 22px; line-height: 22px; }
  nav { display: flex; justify-content: center; gap: 4px; padding: 6px 0 8px; }
  nav button { width: 72px; height: 50px; border-radius: 8px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 3px; font-size: 11px; color: var(--sb-label-2); }
  nav button.on { background: var(--sb-control); color: var(--sb-accent); }
  .body { flex: 1; overflow-y: auto; padding: 4px 24px 24px; }
  h3 { font-size: 13px; margin: 18px 0 8px; }
  .group { padding: 0 12px; }
  .row { min-height: 40px; }
  .row small { display: block; font-size: 11px; color: var(--sb-label-2); margin-top: 2px; }
  .r { display: flex; align-items: center; gap: 10px; }
  .val { min-width: 52px; text-align: right; color: var(--sb-label-2); font-size: 12px; }
  .n { width: 60px; }
  .pp { display: flex; gap: 16px; padding: 12px; }
  .curve { width: 170px; height: 130px; background: #0003; border-radius: 6px; flex: none; }
  .ppr { flex: 1; display: flex; flex-direction: column; gap: 10px; }
  .ppr p { margin: 0; }
  .ppr :global(.seg) { align-self: flex-start; }
  .test { background: #FBFAF6; border-radius: 6px; width: 100%; height: 52px; }
  .guides { display: flex; justify-content: center; gap: 14px; padding: 12px 0; border-bottom: 0.5px solid var(--sb-separator); }
  .guides button { display: flex; flex-direction: column; align-items: center; gap: 6px; font-size: 11px; color: var(--sb-label-2); }
  .guides svg { width: 54px; height: 76px; border-radius: 2px; outline: 2px solid transparent; }
  .guides button.on svg { outline-color: var(--sb-accent); }
  .guides button.on span { color: var(--sb-accent); }
  .well { width: 22px; height: 22px; border-radius: 6px; position: relative; overflow: hidden; }
  .well input { opacity: 0; position: absolute; inset: 0; }
  .kbar { display: flex; gap: 10px; margin: 16px 0 12px; }
  .search { flex: 1; display: flex; align-items: center; gap: 6px; height: 28px; padding: 0 10px; border-radius: 8px; background: var(--sb-control-pressed); color: var(--sb-label-3); }
  .search input { flex: 1; border: 0; background: none; outline: none; }
  .table { border-radius: 10px; background: var(--sb-group); overflow: hidden; }
  .th, .tr { display: grid; grid-template-columns: 1fr 170px; align-items: center; padding: 0 12px; height: 26px; }
  .th { color: var(--sb-label-2); font-size: 11px; font-weight: 600; border-bottom: 0.5px solid var(--sb-separator); height: 28px; }
  .tg { font-size: 11px; font-weight: 600; color: var(--sb-label-2); padding: 8px 12px 2px; }
  .tr.alt { background: #ffffff08; }
  .keys { display: flex; gap: 4px; }
  .kbd { min-width: 20px; height: 18px; padding: 0 5px; border-radius: 4px; background: var(--sb-control); font-size: 11px; }
  .kbd.rec { background: var(--sb-accent); color: #fff; }
  .lang { width: 100%; gap: 10px; justify-content: flex-start; text-align: left; }
  .radio { width: 16px; height: 16px; border-radius: 50%; background: var(--sb-control); flex: none; }
  .radio.on { background: var(--sb-accent); box-shadow: inset 0 0 0 4.5px var(--sb-accent), inset 0 0 0 9px #fff; }
  .ln { flex: 1; }
  .badge { font-size: 10px; color: var(--sb-label-3); border: 0.5px solid var(--sb-border); border-radius: 4px; padding: 1px 5px; }
  .lbtns { display: flex; gap: 8px; margin: 14px 0; }
</style>
