<script lang="ts">
  import { Plus, Check, Trash2, Edit2, RefreshCw, ShieldCheck, FileText, FolderOpen, RotateCw, Copy } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState, type ConfigRecord } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
  import Page from '../components/Page.svelte';
  import ConfigFormModal from '../components/ConfigFormModal.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
  import * as Tooltip from '$lib/components/ui/tooltip/index.js';
  import {
    GetConfigs,
    GetActiveConfig,
    AddConfig,
    EditConfig,
    DeleteConfig,
    UpdateAllConfigs,
    UpdateConfigNow,
    RecreateLocalConfig,
    OpenConfigFile,
    OpenConfigDir,
    ValidateConfig,
    IsConfigHashMismatch,
    HasCachedConfig
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type {
    DeprecatedField,
    ValidationResult
  } from '../../../bindings/sing-box-ez/internal/singboxconfig/models.js';

  let processing = $state(false);
  let updatingAll = $state(false);
  let updating = $state<Record<string, boolean>>({});
  let hashMismatch = $state<Record<string, boolean>>({});
  let hasCached = $state<Record<string, boolean>>({});
  let showForm = $state(false);
  let editing = $state<string | null>(null);
  let initialRecord = $state<ConfigRecord | null>(null);
  let deleteTarget = $state<string | null>(null);
  let validation = $state<{ name: string; result: ValidationResult } | null>(null);

  async function load() {
    try {
      const [configs, active] = await Promise.all([GetConfigs(), GetActiveConfig()]);
      const list = configs ?? [];
      appState.update((s) => ({
        ...s,
        configs: list,
        activeConfig: active
      }));
      const [mismatch, cached] = await Promise.all([
        Promise.all(list.map((c) => IsConfigHashMismatch(c.name).catch(() => false))),
        Promise.all(
          list.map((c) => (c.type === 'local' ? HasCachedConfig(c.name).catch(() => true) : Promise.resolve(true)))
        )
      ]);
      const mismatchMap: Record<string, boolean> = {};
      const cachedMap: Record<string, boolean> = {};
      list.forEach((c, i) => {
        mismatchMap[c.name] = mismatch[i];
        cachedMap[c.name] = cached[i];
      });
      hashMismatch = mismatchMap;
      hasCached = cachedMap;
    } catch (err) {
      toast.error(String(err));
    }
  }

  async function updateAll() {
    updatingAll = true;
    try {
      await UpdateAllConfigs();
      toast.success(tValue($locale, 'configs.update_all.done', 'All configs updated'));
      await load();
    } catch (err) {
      toast.error(String(err));
    } finally {
      updatingAll = false;
    }
  }

  async function updateNow(name: string) {
    updating = { ...updating, [name]: true };
    try {
      await UpdateConfigNow(name);
      toast.success(tValue($locale, 'configs.update_now.done', 'Config updated'));
      await load();
    } catch (err) {
      toast.error(String(err));
    } finally {
      updating = { ...updating, [name]: false };
    }
  }

  async function openFile(name: string) {
    try {
      await OpenConfigFile(name);
    } catch (err) {
      toast.error(String(err));
    }
  }

  async function openDir(name: string) {
    try {
      await OpenConfigDir(name);
    } catch (err) {
      toast.error(String(err));
    }
  }

  async function recreate(name: string) {
    try {
      await RecreateLocalConfig(name);
      toast.success(tValue($locale, 'configs.recreate.done', 'Config file recreated'));
      await load();
    } catch (err) {
      toast.error(String(err));
    }
  }

  async function validate(name: string) {
    try {
      const result = await ValidateConfig(name);
      validation = { name, result };
    } catch (err) {
      toast.error(String(err));
    }
  }

  function fieldLine(f: DeprecatedField): string {
    const parts = [f.path];
    if (f.deprecated) parts.push(`${tValue($locale, 'validation.field.deprecated', 'deprecated in')} ${f.deprecated}`);
    if (f.removed) parts.push(`${tValue($locale, 'validation.field.removed', 'removed in')} ${f.removed}`);
    if (f.replacement) parts.push(`${tValue($locale, 'validation.field.replacement', 'replacement')}: ${f.replacement}`);
    return parts.join(' — ');
  }

  function validationText(): string {
    if (!validation) return '';
    const { result } = validation;
    const lines: string[] = [];
    if (result.errors?.length) {
      lines.push(tValue($locale, 'validation.errors_title', 'Errors (%d)').replace('%d', String(result.errors.length)));
      result.errors.forEach((f) => lines.push(`- ${fieldLine(f)}`));
    }
    if (result.warnings?.length) {
      lines.push(tValue($locale, 'validation.warnings_title', 'Warnings (%d)').replace('%d', String(result.warnings.length)));
      result.warnings.forEach((f) => lines.push(`- ${fieldLine(f)}`));
    }
    if (result.info?.length) {
      lines.push(tValue($locale, 'validation.info_title', 'Info'));
      result.info.forEach((i) => lines.push(`- ${i}`));
    }
    if (lines.length === 0) {
      lines.push(tValue($locale, 'validation.ok', 'No deprecated or removed fields detected.'));
    }
    return lines.join('\n');
  }

  async function copyValidation() {
    try {
      await navigator.clipboard.writeText(validationText());
      toast.success(tValue($locale, 'validation.copied', 'Validation result copied'));
    } catch (err) {
      toast.error(String(err));
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

  async function confirmDelete() {
    if (!deleteTarget) return;
    processing = true;
    try {
      await DeleteConfig(deleteTarget);
      await load();
    } catch (err) {
      toast.error(String(err));
    } finally {
      processing = false;
      deleteTarget = null;
    }
  }
</script>

<Page
  title={tValue($locale, 'tab.configs', 'Configs')}
  onLoad={load}
>
  {#snippet actions()}
    <Button variant="outline" disabled={updatingAll} onclick={updateAll}>
      <RefreshCw size={16} class={updatingAll ? 'animate-spin' : ''} />
      {tValue($locale, 'configs.btn.update_all', 'Update all')}
    </Button>
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
  >
    {#snippet actions()}
      {#if editing && initialRecord?.type === 'local'}
        <div class="flex flex-wrap gap-2 rounded-xl border border-border bg-background p-3">
          <Button variant="outline" size="sm" onclick={() => openFile(editing!)}>
            <FileText size={14} />
            {tValue($locale, 'configs.dialog.btn.open', 'Open')}
          </Button>
          <Button variant="outline" size="sm" onclick={() => openDir(editing!)}>
            <FolderOpen size={14} />
            {tValue($locale, 'configs.dialog.btn.open_dir', 'Open location')}
          </Button>
          {#if editing && hasCached[editing] === false}
            <Button variant="outline" size="sm" onclick={() => recreate(editing!)}>
              <RotateCw size={14} />
              {tValue($locale, 'configs.dialog.btn.create', 'Create again')}
            </Button>
          {/if}
        </div>
      {/if}
    {/snippet}
  </ConfigFormModal>

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
              {#if hashMismatch[cfg.name]}
                <Tooltip.Root>
                  <Tooltip.Trigger>
                    {#snippet child({ props })}
                      <Badge {...props} variant="secondary" class="text-[var(--color-warning)]">
                        {tValue($locale, 'configs.badge.modified', 'modified')}
                      </Badge>
                    {/snippet}
                  </Tooltip.Trigger>
                  <Tooltip.Content>
                    {tValue($locale, 'configs.badge.hash_mismatch_tooltip', 'Cached content differs from the stored hash')}
                  </Tooltip.Content>
                </Tooltip.Root>
              {/if}
              {#if cfg.type === 'remote'}
                <Tooltip.Root>
                  <Tooltip.Trigger>
                    {#snippet child({ props })}
                      <Button
                        {...props}
                        variant="ghost"
                        size="icon"
                        aria-label={tValue($locale, 'configs.dialog.btn.update_now', 'Update now')}
                        disabled={updating[cfg.name]}
                        onclick={() => updateNow(cfg.name)}
                      >
                        <RefreshCw size={16} class={updating[cfg.name] ? 'animate-spin' : ''} />
                      </Button>
                    {/snippet}
                  </Tooltip.Trigger>
                  <Tooltip.Content>{tValue($locale, 'configs.dialog.btn.update_now', 'Update now')}</Tooltip.Content>
                </Tooltip.Root>
              {/if}
              <Tooltip.Root>
                <Tooltip.Trigger>
                  {#snippet child({ props })}
                    <Button
                      {...props}
                      variant="ghost"
                      size="icon"
                      aria-label={tValue($locale, 'configs.dialog.btn.validate', 'Validate')}
                      onclick={() => validate(cfg.name)}
                    >
                      <ShieldCheck size={16} />
                    </Button>
                  {/snippet}
                </Tooltip.Trigger>
                <Tooltip.Content>{tValue($locale, 'configs.dialog.btn.validate', 'Validate')}</Tooltip.Content>
              </Tooltip.Root>
              <Tooltip.Root>
                <Tooltip.Trigger>
                  {#snippet child({ props })}
                    <Button
                      {...props}
                      variant="ghost"
                      size="icon"
                      aria-label={tValue($locale, 'configs.btn.edit', 'Edit config')}
                      onclick={() => startEdit(cfg)}
                    >
                      <Edit2 size={16} />
                    </Button>
                  {/snippet}
                </Tooltip.Trigger>
                <Tooltip.Content>{tValue($locale, 'configs.btn.edit', 'Edit config')}</Tooltip.Content>
              </Tooltip.Root>
              <Tooltip.Root>
                <Tooltip.Trigger>
                  {#snippet child({ props })}
                    <Button
                      {...props}
                      variant="ghost"
                      size="icon"
                      class="text-destructive hover:bg-destructive/10"
                      aria-label={tValue($locale, 'configs.btn.delete', 'Delete')}
                      onclick={() => (deleteTarget = cfg.name)}
                    >
                      <Trash2 size={16} />
                    </Button>
                  {/snippet}
                </Tooltip.Trigger>
                <Tooltip.Content>{tValue($locale, 'configs.btn.delete', 'Delete')}</Tooltip.Content>
              </Tooltip.Root>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </Card.Root>

  <AlertDialog.Root open={deleteTarget != null} onOpenChange={(open) => { if (!open) deleteTarget = null; }}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{tValue($locale, 'configs.btn.delete', 'Delete')}</AlertDialog.Title>
        <AlertDialog.Description>
          {tValue($locale, 'configs.confirmDelete', 'Delete this config?')}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{tValue($locale, 'common.cancel', 'Cancel')}</AlertDialog.Cancel>
        <AlertDialog.Action
          class="bg-destructive text-white hover:bg-destructive/90"
          disabled={processing}
          onclick={confirmDelete}
        >
          {tValue($locale, 'startup.continue', 'Continue')}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <Dialog.Root open={validation != null} onOpenChange={(open) => { if (!open) validation = null; }}>
    <Dialog.Content class="sm:max-w-2xl">
      <Dialog.Header>
        <Dialog.Title>
          {tValue($locale, 'configs.dialog.btn.validate', 'Validate')} — {validation?.name ?? ''}
        </Dialog.Title>
      </Dialog.Header>
      {#if validation}
        <div class="space-y-4 max-h-[60vh] overflow-auto text-sm">
          {#if !validation.result.errors?.length && !validation.result.warnings?.length && !validation.result.info?.length}
            <p class="text-[var(--color-success)]">
              {tValue($locale, 'validation.ok', 'No deprecated or removed fields detected.')}
            </p>
          {:else}
            {#if validation.result.errors?.length}
              <div>
                <p class="font-medium text-destructive mb-1">
                  {tValue($locale, 'validation.errors_title', 'Errors (%d)').replace('%d', String(validation.result.errors.length))}
                </p>
                <ul class="space-y-1">
                  {#each validation.result.errors as field (field.path)}
                    <li class="whitespace-pre-wrap break-words">{fieldLine(field)}</li>
                  {/each}
                </ul>
              </div>
            {/if}
            {#if validation.result.warnings?.length}
              <div>
                <p class="font-medium text-[var(--color-warning)] mb-1">
                  {tValue($locale, 'validation.warnings_title', 'Warnings (%d)').replace('%d', String(validation.result.warnings.length))}
                </p>
                <ul class="space-y-1">
                  {#each validation.result.warnings as field (field.path)}
                    <li class="whitespace-pre-wrap break-words">{fieldLine(field)}</li>
                  {/each}
                </ul>
              </div>
            {/if}
            {#if validation.result.info?.length}
              <div>
                <p class="font-medium mb-1">{tValue($locale, 'validation.info_title', 'Info')}</p>
                <ul class="space-y-1">
                  {#each validation.result.info as line}
                    <li class="whitespace-pre-wrap break-words text-muted-foreground">{line}</li>
                  {/each}
                </ul>
              </div>
            {/if}
          {/if}
        </div>
      {/if}
      <Dialog.Footer>
        <Button variant="outline" onclick={copyValidation}>
          <Copy size={16} />
          {tValue($locale, 'configs.dialog.btn.copy', 'Copy')}
        </Button>
      </Dialog.Footer>
    </Dialog.Content>
  </Dialog.Root>
</Page>
