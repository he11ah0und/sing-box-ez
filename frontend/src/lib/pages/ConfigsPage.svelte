<script lang="ts">
  import { Plus, Trash2, Edit2, RefreshCw, ShieldCheck, FileText, FolderOpen, RotateCw, Copy, EllipsisVertical } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState, type ConfigRecord } from '../stores/appState.js';
  import { useLocale, formatValue } from '../stores/locale.svelte.js';
  import Page from '../components/Page.svelte';
  import ConfigFormModal from '../components/ConfigFormModal.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
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
    HasCachedConfig,
    CopyValidationReport,
    GetConfigMeta
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type { ConfigMeta } from '../../../bindings/sing-box-ez/internal/gui/wails/models.js';
  import type {
    DeprecatedField,
    ValidationResult
  } from '../../../bindings/sing-box-ez/internal/singboxconfig/models.js';

  const L = useLocale([
    'common.cancel',
    'configs.actions',
    'configs.badge.hash_mismatch_tooltip',
    'configs.badge.modified',
    'configs.btn.add',
    'configs.btn.delete',
    'configs.btn.edit',
    'configs.btn.update_all',
    'configs.confirmDelete',
    'configs.dialog.btn.copy',
    'configs.dialog.btn.create',
    'configs.dialog.btn.open',
    'configs.dialog.btn.open_dir',
    'configs.dialog.btn.update_now',
    'configs.dialog.btn.validate',
    'configs.empty',
    'configs.style.client',
    'configs.style.server',
    'configs.type.local',
    'configs.type.remote',
    'configs.update.overdue',
    'duration.ago',
    'duration.in',
    'startup.continue',
    'tab.configs',
    'validation.errors_title',
    'common.info',
    'validation.ok',
    'validation.warnings_title',
    'validation.field.deprecated',
    'validation.field.removed',
    'validation.field.replacement'
  ]);

  let processing = $state(false);
  let updatingAll = $state(false);
  let updating = $state<Record<string, boolean>>({});
  let hashMismatch = $state<Record<string, boolean>>({});
  let hasCached = $state<Record<string, boolean>>({});
  let configMeta = $state<Record<string, ConfigMeta | undefined>>({});

  const typeLabels = $derived<Record<string, string>>({
    remote: L.configsTypeRemote,
    local: L.configsTypeLocal
  });
  const styleLabels = $derived<Record<string, string>>({
    client: L.configsStyleClient,
    server: L.configsStyleServer
  });
  // Type and style carry the colors the legacy gio UI used: remote is the
  // info accent, client is the success green, server the warning accent.
  const typeColors: Record<string, string> = {
    remote: 'var(--color-info)',
    local: 'var(--foreground)'
  };
  const styleColors: Record<string, string> = {
    client: 'var(--color-success)',
    server: 'var(--color-status-warning, var(--color-warning))'
  };

  // cardColor mirrors the gio card background semantics: cache state ×
  // auto-update flag mapped onto the theme's card-* palette. A remote
  // profile counts as cached only when it was actually downloaded once
  // (lastPlain non-empty), same as the legacy UI did.
  function cardColor(cfg: ConfigRecord): string {
    const meta = configMeta[cfg.name];
    const cached = (hasCached[cfg.name] ?? true) && (cfg.type === 'local' || !!meta?.lastPlain);
    const auto = cfg.auto_update !== false;
    if (auto) {
      return cached
        ? 'var(--color-card-cached, #28643C)'
        : 'var(--color-card-uncached, #783C28)';
    }
    return cached
      ? 'var(--color-card-cached-no-auto-update, #0F2E18)'
      : 'var(--color-card-uncached-no-auto-update, #3A1A0F)';
  }
  let showForm = $state(false);
  let editing = $state<string | null>(null);
  let initialRecord = $state<ConfigRecord | null>(null);
  let deleteTarget = $state<string | null>(null);
  let validation = $state<{ name: string; result: ValidationResult } | null>(null);

  async function load() {
    try {
      const [configs, active, meta] = await Promise.all([GetConfigs(), GetActiveConfig(), GetConfigMeta()]);
      const list = configs ?? [];
      configMeta = meta ?? {};
      appState.update((s) => ({
        ...s,
        configs: list,
        activeConfig: active
      }));
      const [mismatch, cached] = await Promise.all([
        Promise.all(list.map((c) => IsConfigHashMismatch(c.name).catch(() => false))),
        Promise.all(list.map((c) => HasCachedConfig(c.name).catch(() => true)))
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
      // Result and error toasts are emitted by the backend.
      await UpdateAllConfigs();
      await load();
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      updatingAll = false;
    }
  }

  async function updateNow(name: string) {
    updating = { ...updating, [name]: true };
    try {
      await UpdateConfigNow(name);
      await load();
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      updating = { ...updating, [name]: false };
    }
  }

  async function openFile(name: string) {
    await OpenConfigFile(name).catch(() => {});
  }

  async function openDir(name: string) {
    await OpenConfigDir(name).catch(() => {});
  }

  async function recreate(name: string) {
    try {
      await RecreateLocalConfig(name);
      await load();
    } catch {
      // The backend already reported the failure with a toast.
    }
  }

  async function validate(name: string) {
    try {
      const result = await ValidateConfig(name);
      validation = { name, result };
    } catch {
      // The backend already reported the failure with a toast.
    }
  }

  // Display-only rendering of one validation field in the dialog.
  function fieldLine(f: DeprecatedField): string {
    const parts = [f.path];
    if (f.deprecated) parts.push(`${L.validationFieldDeprecated} ${f.deprecated}`);
    if (f.removed) parts.push(`${L.validationFieldRemoved} ${f.removed}`);
    if (f.replacement) parts.push(`${L.validationFieldReplacement}: ${f.replacement}`);
    return parts.join(' — ');
  }

  async function copyValidation() {
    if (!validation) return;
    // Formatting, clipboard and the result toast live in the backend.
    await CopyValidationReport(validation.name).catch(() => {});
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
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      processing = false;
      deleteTarget = null;
    }
  }
</script>

<Page
  title={L.tabConfigs}
  onLoad={load}
>
  {#snippet actions()}
    <Button variant="outline" disabled={updatingAll} onclick={updateAll}>
      <RefreshCw size={16} class={updatingAll ? 'animate-spin' : ''} />
      {L.configsBtnUpdate_all}
    </Button>
    <Button onclick={startAdd}>
      <Plus size={18} />
      {L.configsBtnAdd}
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
            {L.configsDialogBtnOpen}
          </Button>
          <Button variant="outline" size="sm" onclick={() => openDir(editing!)}>
            <FolderOpen size={14} />
            {L.configsDialogBtnOpen_dir}
          </Button>
          {#if editing && hasCached[editing] === false}
            <Button variant="outline" size="sm" onclick={() => recreate(editing!)}>
              <RotateCw size={14} />
              {L.configsDialogBtnCreate}
            </Button>
          {/if}
        </div>
      {/if}
    {/snippet}
  </ConfigFormModal>

  <Card.Root class="overflow-hidden py-0 gap-0">
    {#if $appState.configs.length === 0}
      <Card.Content class="py-6">
        <p class="text-muted-foreground">{L.configsEmpty}</p>
      </Card.Content>
    {:else}
      <ul class="divide-y divide-border">
        {#each $appState.configs as cfg (cfg.name)}
          {@const meta = configMeta[cfg.name]}
          <li
            class="p-4 flex items-center justify-between gap-4 transition hover:brightness-125"
            style:background-color={cardColor(cfg)}
          >
            <div class="min-w-0">
              <p class="font-medium truncate">{cfg.name}</p>
              <p class="text-sm text-muted-foreground break-words">
                <span style:color={typeColors[cfg.type]}>{typeLabels[cfg.type] ?? ''}</span>
                {#if meta?.style}
                  <span> · </span><span style:color={styleColors[meta.style]}>{styleLabels[meta.style] ?? ''}</span>
                {/if}
                {#if meta?.lastPlain}
                  <span> · {formatValue(L.durationAgo, [meta.lastPlain])}</span>
                {/if}
                {#if meta?.overdue}
                  <span style:color="var(--color-warning)"> · {L.configsUpdateOverdue}</span>
                {:else if meta?.nextPlain}
                  <span> · {formatValue(L.durationIn, [meta.nextPlain])}</span>
                {/if}
              </p>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              {#if hashMismatch[cfg.name]}
                <Tooltip.Root>
                  <Tooltip.Trigger>
                    {#snippet child({ props })}
                      <Badge {...props} variant="secondary" class="text-[var(--color-warning)]">
                        {L.configsBadgeModified}
                      </Badge>
                    {/snippet}
                  </Tooltip.Trigger>
                  <Tooltip.Content>
                    {L.configsBadgeHash_mismatch_tooltip}
                  </Tooltip.Content>
                </Tooltip.Root>
              {/if}
              <!-- Desktop: the full set of icon buttons. -->
              <div class="hidden sm:flex items-center gap-2">
                {#if cfg.type === 'remote'}
                  <Tooltip.Root>
                    <Tooltip.Trigger>
                      {#snippet child({ props })}
                        <Button
                          {...props}
                          variant="ghost"
                          size="icon"
                          aria-label={L.configsDialogBtnUpdate_now}
                          disabled={updating[cfg.name]}
                          onclick={() => updateNow(cfg.name)}
                        >
                          <RefreshCw size={16} class={updating[cfg.name] ? 'animate-spin' : ''} />
                        </Button>
                      {/snippet}
                    </Tooltip.Trigger>
                    <Tooltip.Content>{L.configsDialogBtnUpdate_now}</Tooltip.Content>
                  </Tooltip.Root>
                {/if}
                <Tooltip.Root>
                  <Tooltip.Trigger>
                    {#snippet child({ props })}
                      <Button
                        {...props}
                        variant="ghost"
                        size="icon"
                        aria-label={L.configsDialogBtnValidate}
                        onclick={() => validate(cfg.name)}
                      >
                        <ShieldCheck size={16} />
                      </Button>
                    {/snippet}
                  </Tooltip.Trigger>
                  <Tooltip.Content>{L.configsDialogBtnValidate}</Tooltip.Content>
                </Tooltip.Root>
                <Tooltip.Root>
                  <Tooltip.Trigger>
                    {#snippet child({ props })}
                      <Button
                        {...props}
                        variant="ghost"
                        size="icon"
                        aria-label={L.configsBtnEdit}
                        onclick={() => startEdit(cfg)}
                      >
                        <Edit2 size={16} />
                      </Button>
                    {/snippet}
                  </Tooltip.Trigger>
                  <Tooltip.Content>{L.configsBtnEdit}</Tooltip.Content>
                </Tooltip.Root>
                <Tooltip.Root>
                  <Tooltip.Trigger>
                    {#snippet child({ props })}
                      <Button
                        {...props}
                        variant="ghost"
                        size="icon"
                        class="text-destructive hover:bg-destructive/10"
                        aria-label={L.configsBtnDelete}
                        onclick={() => (deleteTarget = cfg.name)}
                      >
                        <Trash2 size={16} />
                      </Button>
                    {/snippet}
                  </Tooltip.Trigger>
                  <Tooltip.Content>{L.configsBtnDelete}</Tooltip.Content>
                </Tooltip.Root>
              </div>
              <!-- Mobile: the same actions collapsed into one dropdown menu. -->
              <div class="sm:hidden">
                <DropdownMenu.Root>
                  <DropdownMenu.Trigger>
                    {#snippet child({ props })}
                      <Button {...props} variant="ghost" size="icon" aria-label={L.configsActions}>
                        <EllipsisVertical size={16} />
                      </Button>
                    {/snippet}
                  </DropdownMenu.Trigger>
                  <DropdownMenu.Content align="end">
                    {#if cfg.type === 'remote'}
                      <DropdownMenu.Item
                        disabled={updating[cfg.name]}
                        onclick={() => updateNow(cfg.name)}
                      >
                        <RefreshCw size={14} class={updating[cfg.name] ? 'animate-spin' : ''} />
                        {L.configsDialogBtnUpdate_now}
                      </DropdownMenu.Item>
                    {/if}
                    <DropdownMenu.Item onclick={() => validate(cfg.name)}>
                      <ShieldCheck size={14} />
                      {L.configsDialogBtnValidate}
                    </DropdownMenu.Item>
                    <DropdownMenu.Item onclick={() => startEdit(cfg)}>
                      <Edit2 size={14} />
                      {L.configsBtnEdit}
                    </DropdownMenu.Item>
                    <DropdownMenu.Separator />
                    <DropdownMenu.Item
                      class="text-destructive"
                      onclick={() => (deleteTarget = cfg.name)}
                    >
                      <Trash2 size={14} />
                      {L.configsBtnDelete}
                    </DropdownMenu.Item>
                  </DropdownMenu.Content>
                </DropdownMenu.Root>
              </div>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </Card.Root>

  <AlertDialog.Root open={deleteTarget != null} onOpenChange={(open) => { if (!open) deleteTarget = null; }}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{L.configsBtnDelete}</AlertDialog.Title>
        <AlertDialog.Description>
          {L.configsConfirmDelete}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{L.commonCancel}</AlertDialog.Cancel>
        <AlertDialog.Action
          class="bg-destructive text-white hover:bg-destructive/90"
          disabled={processing}
          onclick={confirmDelete}
        >
          {L.startupContinue}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <Dialog.Root open={validation != null} onOpenChange={(open) => { if (!open) validation = null; }}>
    <Dialog.Content class="sm:max-w-2xl">
      <Dialog.Header>
        <Dialog.Title>
          {L.configsDialogBtnValidate} — {validation?.name ?? ''}
        </Dialog.Title>
      </Dialog.Header>
      {#if validation}
        <div class="space-y-4 max-h-[60vh] overflow-auto text-sm">
          {#if !validation.result.errors?.length && !validation.result.warnings?.length && !validation.result.info?.length}
            <p class="text-[var(--color-success)]">
              {L.validationOk}
            </p>
          {:else}
            {#if validation.result.errors?.length}
              <div>
                <p class="font-medium text-destructive mb-1">
                  {L.validationErrors_title.replace('%d', String(validation.result.errors.length))}
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
                  {L.validationWarnings_title.replace('%d', String(validation.result.warnings.length))}
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
                <p class="font-medium mb-1">{L.commonInfo}</p>
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
          {L.configsDialogBtnCopy}
        </Button>
      </Dialog.Footer>
    </Dialog.Content>
  </Dialog.Root>
</Page>
