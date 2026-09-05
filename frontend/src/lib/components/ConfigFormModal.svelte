<script lang="ts">
  import type { Snippet } from 'svelte';
  import { FolderSearch, Check, X } from '@lucide/svelte';
  import { useLocale } from '@he11ah0und/localengine-web';
  import type { ConfigRecord } from '../stores/appState.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import {
    ConfigNameAvailable,
    PickConfigFile
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  let {
    open = false,
    mode = 'add',
    initialRecord = null,
    onclose = () => {},
    onsave = async (_rec: ConfigRecord, _sourcePath: string) => {},
    actions
  }: {
    open?: boolean;
    mode?: 'add' | 'edit';
    initialRecord?: ConfigRecord | null;
    onclose?: () => void;
    onsave?: (rec: ConfigRecord, sourcePath: string) => void | Promise<void>;
    actions?: Snippet;
  } = $props();

  const L = useLocale([
    'common.cancel',
    'common.save',
    'common.saving',
    'configs.browse',
    'configs.btn.edit',
    'configs.interval',
    'configs.name',
    'configs.nameRequired',
    'configs.name_available',
    'configs.name_taken',
    'configs.new',
    'configs.source_file',
    'configs.type.label',
    'configs.type.local',
    'configs.type.remote',
    'configs.url'
  ]);

  let processing = $state(false);
  let error = $state('');
  let sourcePath = $state('');
  // Name availability indicator: checked in the background while typing.
  let nameTaken = $state(false);
  let nameChecked = $state(false);

  interface FormState {
    name: string;
    url: string;
    type: string;
    update_interval_hours: number;
  }

  const defaultForm = (): FormState => ({
    name: '',
    url: '',
    type: 'remote',
    update_interval_hours: 24
  });

  let form = $state<FormState>(defaultForm());

  $effect(() => {
    if (!open) {
      processing = false;
      error = '';
      return;
    }

    error = '';
    processing = false;
    sourcePath = '';

    if (mode === 'edit' && initialRecord) {
      form = {
        name: initialRecord.name ?? '',
        url: initialRecord.url ?? '',
        type: initialRecord.type || 'remote',
        update_interval_hours: initialRecord.update_interval_hours || 24
      };
    } else {
      form = defaultForm();
    }
  });

  // Debounced background check whether the typed name is free. The profile
  // being edited may keep its own name.
  $effect(() => {
    if (!open) return;
    const name = form.name.trim();
    const exclude = mode === 'edit' ? (initialRecord?.name ?? '') : '';
    nameChecked = false;
    nameTaken = false;
    if (!name) return;
    const timer = setTimeout(async () => {
      try {
        const free = await ConfigNameAvailable(name, exclude);
        nameTaken = !free;
        nameChecked = true;
      } catch {
        // The check is advisory only; the backend validates on save anyway.
      }
    }, 300);
    return () => clearTimeout(timer);
  });

  async function browse() {
    try {
      const path = await PickConfigFile();
      if (path) sourcePath = path;
    } catch {
      // Dialog unavailable; the user can still type the path manually.
    }
  }

  async function submit() {
    error = '';

    const rec: ConfigRecord = {
      name: form.name.trim(),
      url: form.type === 'local' ? '' : form.url.trim(),
      type: form.type,
      update_interval_hours: Number(form.update_interval_hours),
      last_update: null,
      parent: 'user',
      auto_update: true,
      hash: '',
      fallback_type: null
    };

    if (!rec.name) {
      error = L.configsNameRequired;
      return;
    }
    if (nameTaken) {
      error = L.configsName_taken;
      return;
    }

    processing = true;
    try {
      await onsave(rec, form.type === 'local' ? sourcePath.trim() : '');
      onclose();
    } catch {
      // The backend already reported the failure with a toast; keep the form open.
    } finally {
      processing = false;
    }
  }

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) onclose();
  }

  const typeLabel = $derived(form.type === 'local' ? L.configsTypeLocal : L.configsTypeRemote);
</script>

<Dialog.Root {open} onOpenChange={handleOpenChange}>
  <Dialog.Content class="sm:max-w-lg">
    <Dialog.Header>
      <Dialog.Title>
        {mode === 'edit'
          ? L.configsBtnEdit
          : L.configsNew}
      </Dialog.Title>
    </Dialog.Header>

    <div class="space-y-4">
      {#if error}
        <div class="rounded-xl bg-destructive/10 text-destructive px-3 py-2 text-sm">
          {error}
        </div>
      {/if}

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div class="space-y-1">
          <Label for="config-name">{L.configsName}</Label>
          <Input id="config-name" bind:value={form.name} />
          {#if form.name.trim() && nameChecked}
            <p
              class="text-xs flex items-center gap-1"
              style:color={nameTaken ? 'var(--destructive)' : 'var(--color-success)'}
            >
              {#if nameTaken}
                <X size={12} />
                {L.configsName_taken}
              {:else}
                <Check size={12} />
                {L.configsName_available}
              {/if}
            </p>
          {/if}
        </div>

        <div class="space-y-1">
          <Label for="config-type">{L.configsTypeLabel}</Label>
          <Select.Root type="single" bind:value={form.type}>
            <Select.Trigger id="config-type" class="w-full">{typeLabel}</Select.Trigger>
            <Select.Content>
              <Select.Item value="remote" label={L.configsTypeRemote} />
              <Select.Item value="local" label={L.configsTypeLocal} />
            </Select.Content>
          </Select.Root>
        </div>

        {#if form.type === 'local'}
          <div class="space-y-1 sm:col-span-2">
            <Label for="config-source">{L.configsSource_file}</Label>
            <div class="flex gap-2">
              <Input id="config-source" class="flex-1" bind:value={sourcePath} />
              <Button variant="outline" onclick={browse}>
                <FolderSearch size={16} />
                {L.configsBrowse}
              </Button>
            </div>
          </div>
        {:else}
          <div class="space-y-1 sm:col-span-2">
            <Label for="config-url">{L.configsUrl}</Label>
            <Input id="config-url" bind:value={form.url} />
          </div>

          <div class="space-y-1">
            <Label for="config-interval">{L.configsInterval}</Label>
            <Input id="config-interval" type="number" min="0" bind:value={form.update_interval_hours} />
          </div>
        {/if}
      </div>

      {#if mode === 'edit' && actions}
        {@render actions()}
      {/if}
    </div>

    <Dialog.Footer>
      <Button variant="outline" onclick={onclose} disabled={processing}>
        {L.commonCancel}
      </Button>
      <Button onclick={submit} disabled={processing || nameTaken}>
        {processing
          ? L.commonSaving
          : L.commonSave}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
