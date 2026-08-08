<script lang="ts">
  import { Plus, Check, Trash2, Edit2, RefreshCw, ShieldCheck, FileText, FolderOpen, RotateCw, Copy } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState, type ConfigRecord } from '../stores/appState.js';
  import { useLocale } from '../stores/locale.js';
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
    HasCachedConfig,
    CopyValidationReport
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type {
    DeprecatedField,
    ValidationResult
  } from '../../../bindings/sing-box-ez/internal/singboxconfig/models.js';

  const commonCancel = useLocale('common.cancel');
  const configsBadgeActive = useLocale('configs.badge.active');
  const configsBadgeHashMismatchTooltip = useLocale('configs.badge.hash_mismatch_tooltip');
  const configsBadgeModified = useLocale('configs.badge.modified');
  const configsBtnAdd = useLocale('configs.btn.add');
  const configsBtnDelete = useLocale('configs.btn.delete');
  const configsBtnEdit = useLocale('configs.btn.edit');
  const configsBtnUpdateAll = useLocale('configs.btn.update_all');
  const configsConfirmDelete = useLocale('configs.confirmDelete');
  const configsDialogBtnCopy = useLocale('configs.dialog.btn.copy');
  const configsDialogBtnCreate = useLocale('configs.dialog.btn.create');
  const configsDialogBtnOpen = useLocale('configs.dialog.btn.open');
  const configsDialogBtnOpenDir = useLocale('configs.dialog.btn.open_dir');
  const configsDialogBtnUpdateNow = useLocale('configs.dialog.btn.update_now');
  const configsDialogBtnValidate = useLocale('configs.dialog.btn.validate');
  const configsEmpty = useLocale('configs.empty');
  const startupContinue = useLocale('startup.continue');
  const tabConfigs = useLocale('tab.configs');
  const validationErrorsTitle = useLocale('validation.errors_title');
  const validationInfoTitle = useLocale('validation.info_title');
  const validationOk = useLocale('validation.ok');
  const validationWarningsTitle = useLocale('validation.warnings_title');
  const validationFieldDeprecated = useLocale('validation.field.deprecated');
  const validationFieldRemoved = useLocale('validation.field.removed');
  const validationFieldReplacement = useLocale('validation.field.replacement');

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
    if (f.deprecated) parts.push(`${$validationFieldDeprecated} ${f.deprecated}`);
    if (f.removed) parts.push(`${$validationFieldRemoved} ${f.removed}`);
    if (f.replacement) parts.push(`${$validationFieldReplacement}: ${f.replacement}`);
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
  title={$tabConfigs}
  onLoad={load}
>
  {#snippet actions()}
    <Button variant="outline" disabled={updatingAll} onclick={updateAll}>
      <RefreshCw size={16} class={updatingAll ? 'animate-spin' : ''} />
      {$configsBtnUpdateAll}
    </Button>
    <Button onclick={startAdd}>
      <Plus size={18} />
      {$configsBtnAdd}
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
            {$configsDialogBtnOpen}
          </Button>
          <Button variant="outline" size="sm" onclick={() => openDir(editing!)}>
            <FolderOpen size={14} />
            {$configsDialogBtnOpenDir}
          </Button>
          {#if editing && hasCached[editing] === false}
            <Button variant="outline" size="sm" onclick={() => recreate(editing!)}>
              <RotateCw size={14} />
              {$configsDialogBtnCreate}
            </Button>
          {/if}
        </div>
      {/if}
    {/snippet}
  </ConfigFormModal>

  <Card.Root class="overflow-hidden py-0 gap-0">
    {#if $appState.configs.length === 0}
      <Card.Content class="py-6">
        <p class="text-muted-foreground">{$configsEmpty}</p>
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
                  {$configsBadgeActive}
                </Badge>
              {/if}
              {#if hashMismatch[cfg.name]}
                <Tooltip.Root>
                  <Tooltip.Trigger>
                    {#snippet child({ props })}
                      <Badge {...props} variant="secondary" class="text-[var(--color-warning)]">
                        {$configsBadgeModified}
                      </Badge>
                    {/snippet}
                  </Tooltip.Trigger>
                  <Tooltip.Content>
                    {$configsBadgeHashMismatchTooltip}
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
                        aria-label={$configsDialogBtnUpdateNow}
                        disabled={updating[cfg.name]}
                        onclick={() => updateNow(cfg.name)}
                      >
                        <RefreshCw size={16} class={updating[cfg.name] ? 'animate-spin' : ''} />
                      </Button>
                    {/snippet}
                  </Tooltip.Trigger>
                  <Tooltip.Content>{$configsDialogBtnUpdateNow}</Tooltip.Content>
                </Tooltip.Root>
              {/if}
              <Tooltip.Root>
                <Tooltip.Trigger>
                  {#snippet child({ props })}
                    <Button
                      {...props}
                      variant="ghost"
                      size="icon"
                      aria-label={$configsDialogBtnValidate}
                      onclick={() => validate(cfg.name)}
                    >
                      <ShieldCheck size={16} />
                    </Button>
                  {/snippet}
                </Tooltip.Trigger>
                <Tooltip.Content>{$configsDialogBtnValidate}</Tooltip.Content>
              </Tooltip.Root>
              <Tooltip.Root>
                <Tooltip.Trigger>
                  {#snippet child({ props })}
                    <Button
                      {...props}
                      variant="ghost"
                      size="icon"
                      aria-label={$configsBtnEdit}
                      onclick={() => startEdit(cfg)}
                    >
                      <Edit2 size={16} />
                    </Button>
                  {/snippet}
                </Tooltip.Trigger>
                <Tooltip.Content>{$configsBtnEdit}</Tooltip.Content>
              </Tooltip.Root>
              <Tooltip.Root>
                <Tooltip.Trigger>
                  {#snippet child({ props })}
                    <Button
                      {...props}
                      variant="ghost"
                      size="icon"
                      class="text-destructive hover:bg-destructive/10"
                      aria-label={$configsBtnDelete}
                      onclick={() => (deleteTarget = cfg.name)}
                    >
                      <Trash2 size={16} />
                    </Button>
                  {/snippet}
                </Tooltip.Trigger>
                <Tooltip.Content>{$configsBtnDelete}</Tooltip.Content>
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
        <AlertDialog.Title>{$configsBtnDelete}</AlertDialog.Title>
        <AlertDialog.Description>
          {$configsConfirmDelete}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{$commonCancel}</AlertDialog.Cancel>
        <AlertDialog.Action
          class="bg-destructive text-white hover:bg-destructive/90"
          disabled={processing}
          onclick={confirmDelete}
        >
          {$startupContinue}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <Dialog.Root open={validation != null} onOpenChange={(open) => { if (!open) validation = null; }}>
    <Dialog.Content class="sm:max-w-2xl">
      <Dialog.Header>
        <Dialog.Title>
          {$configsDialogBtnValidate} — {validation?.name ?? ''}
        </Dialog.Title>
      </Dialog.Header>
      {#if validation}
        <div class="space-y-4 max-h-[60vh] overflow-auto text-sm">
          {#if !validation.result.errors?.length && !validation.result.warnings?.length && !validation.result.info?.length}
            <p class="text-[var(--color-success)]">
              {$validationOk}
            </p>
          {:else}
            {#if validation.result.errors?.length}
              <div>
                <p class="font-medium text-destructive mb-1">
                  {$validationErrorsTitle.replace('%d', String(validation.result.errors.length))}
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
                  {$validationWarningsTitle.replace('%d', String(validation.result.warnings.length))}
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
                <p class="font-medium mb-1">{$validationInfoTitle}</p>
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
          {$configsDialogBtnCopy}
        </Button>
      </Dialog.Footer>
    </Dialog.Content>
  </Dialog.Root>
</Page>
