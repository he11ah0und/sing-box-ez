<script module lang="ts">
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

<script lang="ts">
  import { Plus, Check, Trash2, Edit2 } from '@lucide/svelte';
  import { appState, type ConfigRecord } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
  import Page from '../components/Page.svelte';
  import ConfigFormModal from '../components/ConfigFormModal.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
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
  let editing = $state<string | null>(null);
  let initialRecord = $state<ConfigRecord | null>(null);

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

  function startEdit(rec: ConfigRecord) {
    editing = rec.name;
    initialRecord = rec;
    showForm = true;
  }

  function closeForm() {
    showForm = false;
    editing = null;
    initialRecord = null;
  }

  async function handleSave(rec: ConfigRecord) {
    if (editing) {
      await EditConfig(editing, rec);
    } else {
      await AddConfig(rec);
    }
    closeForm();
    await load();
  }

  async function remove(name: string) {
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
    <Button onclick={startAdd}>
      <Plus size={18} />
      {tValue($locale, 'configs.btn.add', 'Add')}
    </Button>
  {/snippet}

  <ConfigFormModal
    open={showForm}
    mode={editing ? 'edit' : 'add'}
    {initialRecord}
    onclose={closeForm}
    onsave={handleSave}
  />

  <Card.Root class="overflow-hidden py-0 gap-0">
    {#if $appState.configs.length === 0}
      <Card.Content class="py-6">
        <p class="text-muted-foreground">{tValue($locale, 'configs.empty', 'No configs yet.')}</p>
      </Card.Content>
    {:else}
      <ul class="divide-y divide-border">
        {#each $appState.configs as cfg (cfg.name)}
          <li class="p-4 flex items-center justify-between gap-4 hover:bg-accent transition">
            <div class="min-w-0">
              <p class="font-medium truncate">{cfg.name}</p>
              <p class="text-sm text-muted-foreground truncate">{cfg.type} · {cfg.update_interval_hours}h</p>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              {#if $appState.activeConfig?.name === cfg.name}
                <Badge variant="secondary" class="text-[var(--color-success)]">
                  <Check size={12} />
                  {tValue($locale, 'configs.badge.active', 'Active')}
                </Badge>
              {/if}
              <Button variant="ghost" size="icon" onclick={() => startEdit(cfg)}>
                <Edit2 size={16} />
              </Button>
              <Button variant="ghost" size="icon" class="text-destructive hover:bg-destructive/10" onclick={() => remove(cfg.name)}>
                <Trash2 size={16} />
              </Button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </Card.Root>

  {#if message}
    <p class="text-sm text-destructive">{message}</p>
  {/if}
</Page>
