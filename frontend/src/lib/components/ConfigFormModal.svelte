<script>
  import { locale, tValue } from '../stores/locale.js';
  import Modal from './Modal.svelte';

  let {
    open = false,
    mode = 'add',
    initialRecord = null,
    onclose = () => {},
    onsave = (rec) => {}
  } = $props();

  let processing = $state(false);
  let error = $state('');

  const defaultForm = () => ({
    name: '',
    url: '',
    type: 'remote',
    update_interval_hours: 24
  });

  let form = $state(defaultForm());

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

    const rec = {
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
      error = tValue($locale, 'configs.nameRequired', 'Name is required');
      return;
    }

    processing = true;
    try {
      await onsave(rec);
      onclose();
    } catch (err) {
      error = String(err);
    } finally {
      processing = false;
    }
  }

  function handleKeydown(event) {
    if (event.key === 'Escape') {
      onclose();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <Modal
    title={mode === 'edit'
      ? tValue($locale, 'configs.btn.edit', 'Edit config')
      : tValue($locale, 'configs.new', 'New config')}
    {onclose}
  >
    <div class="space-y-4">
      {#if error}
        <div class="rounded-xl bg-red-500/10 text-red-500 px-3 py-2 text-sm">
          {error}
        </div>
      {/if}

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <label class="block space-y-1">
          <span class="text-sm text-[var(--color-text-muted)]">
            {tValue($locale, 'configs.name', 'Name')}
          </span>
          <input
            bind:value={form.name}
            class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2"
          />
        </label>

        <label class="block space-y-1">
          <span class="text-sm text-[var(--color-text-muted)]">
            {tValue($locale, 'configs.type', 'Type')}
          </span>
          <select
            bind:value={form.type}
            class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2"
          >
            <option value="remote">{tValue($locale, 'configs.remote', 'Remote')}</option>
            <option value="local">{tValue($locale, 'configs.local', 'Local')}</option>
          </select>
        </label>

        <label class="block space-y-1 sm:col-span-2">
          <span class="text-sm text-[var(--color-text-muted)]">
            {tValue($locale, 'configs.url', 'URL / Path')}
          </span>
          <input
            bind:value={form.url}
            class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2"
          />
        </label>

        <label class="block space-y-1">
          <span class="text-sm text-[var(--color-text-muted)]">
            {tValue($locale, 'configs.interval', 'Update interval (h)')}
          </span>
          <input
            type="number"
            bind:value={form.update_interval_hours}
            min="0"
            class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2"
          />
        </label>
      </div>
    </div>

    {#snippet footer()}
      <button
        class="px-4 py-2 rounded-xl border border-[var(--color-border)] hover:bg-[var(--color-surface-variant)]"
        onclick={onclose}
        disabled={processing}
      >
        {tValue($locale, 'common.cancel', 'Cancel')}
      </button>
      <button
        class="px-4 py-2 rounded-xl bg-[var(--color-primary)] text-white disabled:opacity-50"
        onclick={submit}
        disabled={processing}
      >
        {processing
          ? tValue($locale, 'common.saving', 'Saving...')
          : tValue($locale, 'common.save', 'Save')}
      </button>
    {/snippet}
  </Modal>
{/if}
