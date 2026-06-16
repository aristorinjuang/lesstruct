<script setup lang="ts">
import { formatDate } from '@/utils/date'
import type { ApiKey } from '@/types/apiKey'

defineProps<{
  keys: ApiKey[]
  isLoading: boolean
}>()

const emit = defineEmits<{
  revoke: [id: number]
  create: []
}>()

function lastUsedLabel(key: ApiKey): string {
  // lastUsedAt is null until Story 1.4's middleware writes it. Guard against
  // empty-string / unparseable values so the cell never shows "Invalid Date".
  if (!key.lastUsedAt) return 'Never'
  const parsed = new Date(key.lastUsedAt)
  return Number.isNaN(parsed.getTime()) ? 'Never' : formatDate(key.lastUsedAt)
}
</script>

<template>
  <div class="api-key-list">
    <!-- Loading state -->
    <div v-if="isLoading && keys.length === 0" class="api-key-list__state">
      Loading API keys...
    </div>

    <!-- Empty state -->
    <div v-else-if="keys.length === 0" class="api-key-list__empty">
      <p class="api-key-list__empty-title">No API keys yet</p>
      <p class="api-key-list__empty-text">
        You don't have any API keys yet. Create one to enable programmatic access to your Lesstruct
        content.
      </p>
      <button
        type="button"
        class="api-key-list__empty-button"
        @click="emit('create')"
      >
        Create your first API key
      </button>
    </div>

    <!-- Populated list -->
    <ul v-else class="api-key-list__items">
      <li
        v-for="key in keys"
        :key="key.id"
        class="api-key-list__row"
        :class="{ 'api-key-list__row--revoked': key.revokedAt }"
      >
        <div class="api-key-list__cell api-key-list__cell--prefix">
          <span class="api-key-list__cell-label">Key</span>
          <code class="api-key-list__prefix">{{ key.prefix }}</code>
        </div>

        <div class="api-key-list__cell api-key-list__cell--name">
          <span class="api-key-list__cell-label">Name</span>
          <span class="api-key-list__name" :title="key.name">{{ key.name }}</span>
        </div>

        <div class="api-key-list__cell api-key-list__cell--last-used">
          <span class="api-key-list__cell-label">Last used</span>
          <span class="api-key-list__last-used">{{ lastUsedLabel(key) }}</span>
        </div>

        <div class="api-key-list__cell api-key-list__cell--status">
          <span
            class="api-key-list__badge"
            :class="
              key.revokedAt
                ? 'api-key-list__badge--revoked'
                : 'api-key-list__badge--active'
            "
          >
            {{ key.revokedAt ? 'Revoked' : 'Active' }}
          </span>
        </div>

        <div class="api-key-list__cell api-key-list__cell--action">
          <button
            v-if="!key.revokedAt"
            type="button"
            class="api-key-list__revoke-button"
            :aria-label="`Revoke ${key.name}`"
            @click="emit('revoke', key.id)"
          >
            Revoke
          </button>
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.api-key-list__state {
  padding: 2rem;
  text-align: center;
  color: var(--brand-dark-2);
  font-size: 0.9375rem;
}

.api-key-list__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 2.5rem 1.5rem;
  text-align: center;
  border: 1px dashed var(--brand-light-2);
  border-radius: 0.5rem;
  background-color: var(--color-background);
}

.api-key-list__empty-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--brand-dark-1);
}

.api-key-list__empty-text {
  margin: 0 0 0.75rem;
  max-width: 36ch;
  font-size: 0.875rem;
  color: var(--brand-dark-2);
  line-height: 1.5;
}

.api-key-list__empty-button {
  padding: 0.5rem 1rem;
  background-color: var(--brand-primary);
  color: white;
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  min-height: 44px;
  transition: background-color 0.2s;
}

.api-key-list__empty-button:hover {
  background-color: var(--color-interactive-hover);
}

.api-key-list__empty-button:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

.api-key-list__items {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.api-key-list__row {
  display: grid;
  grid-template-columns: minmax(160px, 1.4fr) 1fr auto auto auto;
  gap: 1rem;
  align-items: center;
  padding: 0.875rem 1rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  background-color: var(--color-background);
  transition: opacity 0.2s;
}

.api-key-list__row--revoked {
  opacity: 0.6;
}

.api-key-list__cell {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  min-width: 0;
}

.api-key-list__cell-label {
  font-size: 0.75rem;
  color: var(--brand-dark-2);
}

.api-key-list__cell--prefix {
  min-width: 0;
}

.api-key-list__prefix {
  font-family: monospace;
  font-size: 0.8125rem;
  color: var(--brand-dark-1);
  word-break: break-all;
}

.api-key-list__name {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.api-key-list__last-used {
  font-size: 0.875rem;
  color: var(--brand-dark-2);
  white-space: nowrap;
}

.api-key-list__cell--status {
  align-items: flex-start;
}

.api-key-list__badge {
  display: inline-flex;
  align-items: center;
  padding: 0.1875rem 0.5rem;
  border-radius: 0.75rem;
  font-size: 0.75rem;
  font-weight: 600;
  white-space: nowrap;
}

.api-key-list__badge--active {
  background-color: var(--color-success-bg);
  color: var(--color-success-dark);
  border: 1px solid var(--color-success-border);
}

.api-key-list__badge--revoked {
  background-color: var(--brand-light-2);
  color: var(--brand-dark-2);
  border: 1px solid var(--brand-light-2);
}

.api-key-list__cell--action {
  justify-self: end;
}

.api-key-list__revoke-button {
  padding: 0.5rem 0.875rem;
  background-color: var(--brand-light-1);
  color: var(--color-error);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 0.8125rem;
  font-weight: 500;
  cursor: pointer;
  min-height: 44px;
  transition: background-color 0.2s, color 0.2s;
}

.api-key-list__revoke-button:hover {
  background-color: rgba(239, 68, 68, 0.1);
  border-color: var(--color-error);
}

.api-key-list__revoke-button:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

/* Cell labels are only needed on mobile where cells stack. */
@media (min-width: 640px) {
  .api-key-list__cell-label {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  .api-key-list__cell {
    flex-direction: row;
    align-items: center;
  }

  .api-key-list__cell--status {
    align-items: center;
  }
}

@media (max-width: 639px) {
  .api-key-list__row {
    grid-template-columns: 1fr auto;
    grid-template-areas:
      'prefix prefix'
      'name action'
      'lastused status';
    gap: 0.5rem 1rem;
  }

  .api-key-list__cell--prefix {
    grid-area: prefix;
  }

  .api-key-list__cell--name {
    grid-area: name;
  }

  .api-key-list__cell--last-used {
    grid-area: lastused;
  }

  .api-key-list__cell--status {
    grid-area: status;
  }

  .api-key-list__cell--action {
    grid-area: action;
    justify-self: end;
    align-self: start;
  }
}
</style>
