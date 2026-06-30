<script module>
  import { List } from '@lucide/svelte';
  export const pageMeta = {
    id: 'configs',
    key: 'tab.configs',
    icon: List,
    nav: true,
    bottomNav: true,
    order: 1
  };
</script>

<script>
  import { Plus, Check, Trash2, Edit2 } from '@lucide/svelte';
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
  import Page from '../components/Page.svelte';
  import ConfigFormModal from '../components/ConfigFormModal.svelte';
  import {
    GetConfigs,
    GetActiveConfig,
    AddConfig,
    EditConfig,
    DeleteConfig
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  let processing = $state(false);
  let message = $state('');
  let showForm = $state(false);
  let editing = $state(null);
  let initialRecord = $state(null);

  async function load() {
    try {
      const [configs, active] = await Promise.all([GetConfigs(), GetActiveConfig()]);
      appState.update((s) => ({
        ...s,
        configs: configs ?? [],
        activeConfig: active
      }));
    } catch (err) {
      message = String(err);
    }
  }

  function startAdd() {
    editing = null;
    initialRecord = null;
    showForm = true;
  }

  function startEdit(rec) {
    editing = rec.name;
    initialRecord = rec;
    showForm = true;
  }

  function closeForm() {
    showForm = false;
    editing = null;
    initialRecord = null;
  }

  async function handleSave(rec) {
    if (editing) {
      await EditConfig(editing, rec);
    } else {
      await AddConfig(rec);
    }
    closeForm();
    await load();
  }

  async function remove(name) {
    if (!confirm(tValue($locale, 'configs.confirmDelete', 'Delete this config?'))) return;
    processing = true;
    try {
      await DeleteConfig(name);
      await load();
    } catch (err) {
      message = String(err);
    } finally {
      processing = false;
    }
  }
</script>

<Page
  title={tValue($locale, 'tab.configs', 'Configs')}
  onLoad={load}
>
  {#snippet actions()}
    <button
      class="flex items-center gap-2 px-3 py-2 rounded-xl bg-[var(--color-primary)] text-white hover:opacity-90 transition"
      onclick={startAdd}
    >
      <Plus size={18} />
      {tValue($locale, 'configs.btn.add', 'Add')}
    </button>
  {/snippet}

  <ConfigFormModal
    open={showForm}
    mode={editing ? 'edit' : 'add'}
    {initialRecord}
    onclose={closeForm}
    onsave={handleSave}
  />

  <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-sm overflow-hidden">
    {#if $appState.configs.length === 0}
      <p class="p-6 text-[var(--color-text-muted)]">{tValue($locale, 'configs.empty', 'No configs yet.')}</p>
    {:else}
      <ul class="divide-y divide-[var(--color-border)]">
        {#each $appState.configs as cfg (cfg.name)}
          <li class="p-4 flex items-center justify-between gap-4 hover:bg-[var(--color-bg)] transition">
            <div class="min-w-0">
              <p class="font-medium truncate">{cfg.name}</p>
              <p class="text-sm text-[var(--color-text-muted)] truncate">{cfg.type} · {cfg.update_interval_hours}h</p>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              {#if $appState.activeConfig?.name === cfg.name}
                <span class="flex items-center gap-1 text-sm text-green-500"><Check size={16} /> {tValue($locale, 'configs.badge.active', 'Active')}</span>
              {/if}
              <button
                class="p-2 rounded-lg hover:bg-[var(--color-surface-variant)]"
                onclick={() => startEdit(cfg)}
              >
                <Edit2 size={16} />
              </button>
              <button
                class="p-2 rounded-lg hover:bg-[var(--color-danger)] hover:text-white text-[var(--color-danger)] transition"
                onclick={() => remove(cfg.name)}
              >
                <Trash2 size={16} />
              </button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  {#if message}
    <p class="text-sm text-[var(--color-danger)]">{message}</p>
  {/if}
</Page>
