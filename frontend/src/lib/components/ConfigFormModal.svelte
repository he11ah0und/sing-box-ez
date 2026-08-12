<script lang="ts">
  import type { Snippet } from 'svelte';
  import { useLocale } from '@he11ah0und/localengine-web';
  import type { ConfigRecord } from '../stores/appState.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import { Button } from '$lib/components/ui/button/index.js';

  let {
    open = false,
    mode = 'add',
    initialRecord = null,
    onclose = () => {},
    onsave = async (_rec: ConfigRecord) => {},
    actions
  }: {
    open?: boolean;
    mode?: 'add' | 'edit';
    initialRecord?: ConfigRecord | null;
    onclose?: () => void;
    onsave?: (rec: ConfigRecord) => void | Promise<void>;
    actions?: Snippet;
  } = $props();

  const L = useLocale([
    'common.cancel',
    'common.save',
    'common.saving',
    'configs.btn.edit',
    'configs.interval',
    'configs.name',
    'configs.nameRequired',
    'configs.new',
    'configs.type.label',
    'configs.type.local',
    'configs.type.remote',
    'configs.url'
  ]);

  let processing = $state(false);
  let error = $state('');

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

  async function submit() {
    error = '';

    const rec: ConfigRecord = {
      name: form.name.trim(),
      url: form.url.trim(),
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

    processing = true;
    try {
      await onsave(rec);
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

        <div class="space-y-1 sm:col-span-2">
          <Label for="config-url">{L.configsUrl}</Label>
          <Input id="config-url" bind:value={form.url} />
        </div>

        <div class="space-y-1">
          <Label for="config-interval">{L.configsInterval}</Label>
          <Input id="config-interval" type="number" min="0" bind:value={form.update_interval_hours} />
        </div>
      </div>

      {#if mode === 'edit' && actions}
        {@render actions()}
      {/if}
    </div>

    <Dialog.Footer>
      <Button variant="outline" onclick={onclose} disabled={processing}>
        {L.commonCancel}
      </Button>
      <Button onclick={submit} disabled={processing}>
        {processing
          ? L.commonSaving
          : L.commonSave}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
