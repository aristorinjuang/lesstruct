<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Button from '@/components/atoms/Button.vue'
import NumberInput from '@/components/atoms/NumberInput.vue'
import InputText from '@/components/atoms/InputText.vue'
import SwitchToggle from '@/components/atoms/SwitchToggle.vue'
import FormField from '@/components/molecules/FormField.vue'
import MediaPanel from '@/components/organisms/MediaPanel.vue'
import { useCatalogStore } from '@/stores/domain/catalog'
import type { Media } from '@/stores/domain/media'
import type { CatalogProduct, CatalogVariant, CatalogUpsertPayload } from '@/types/catalog'

interface Props {
  contentId: number
  disabled?: boolean
  legacyFields?: Record<string, unknown>
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
  legacyFields: () => ({})
})

const emit = defineEmits<{
  (e: 'show-toast', message: string, type: string): void
}>()

const catalogStore = useCatalogStore()

const price = ref<number | null>(null)
const salePrice = ref<number | null>(null)
const sku = ref('')
const trackInventory = ref(true)
const stock = ref<number | null>(0)
const weightGrams = ref<number | null>(null)
const lowStockThreshold = ref<number | null>(null)
const axesText = ref('')
const variants = ref<CatalogVariant[]>([])

const isLoaded = ref(false)
const hasData = ref(false)
const loadError = ref<string | null>(null)
const variantImagePickerIndex = ref<number | null>(null)

const loadFailed = computed(() => loadError.value !== null)
const hasVariants = computed(() => variants.value.length > 0)
const variantStockSum = computed(() =>
  variants.value.reduce((total, variant) => total + (variant.stock || 0), 0)
)

const canSave = computed(() => !props.disabled && !loadFailed.value && price.value !== null && price.value >= 0)

// Legacy commerce data carried in custom fields (the pre-catalog model):
// offered as a one-click import while the item has no catalog data yet.
const legacyPrice = computed<number | null>(() => {
  const value = props.legacyFields['price']
  return typeof value === 'number' && value >= 0 ? value : null
})
const legacySku = computed<string | null>(() => {
  const value = props.legacyFields['sku']
  return typeof value === 'string' ? value : null
})
const hasLegacyFields = computed(() => legacyPrice.value !== null && !hasData.value)

function applyProduct(stored: CatalogProduct): void {
  price.value = stored.price ?? null
  salePrice.value = stored.salePrice ?? null
  sku.value = stored.sku ?? ''
  trackInventory.value = stored.trackInventory ?? true
  stock.value = stored.stock ?? 0
  weightGrams.value = stored.weightGrams ?? null
  lowStockThreshold.value = stored.lowStockThreshold ?? null
  axesText.value = Object.entries(stored.options ?? {})
    .map(([axis, values]) => `${axis}: ${values.join(', ')}`)
    .join('\n')
  variants.value = (stored.variants ?? []).map((v) => ({ ...v }))
}

async function load(): Promise<void> {
  isLoaded.value = false
  loadError.value = null
  const stored = await catalogStore.fetchProduct(props.contentId)
  if (stored) {
    hasData.value = true
    applyProduct(stored)
  } else {
    // A 404 ("product not found") means the item simply has no catalog data
    // yet — the blank form is the correct state. Anything else is a genuine
    // load failure: surface it with a retry instead of a silent blank form.
    const message = catalogStore.error ?? ''
    if (/not found|404/i.test(message)) {
      hasData.value = false
    } else {
      loadError.value = message || 'Failed to load catalog data.'
    }
  }
  isLoaded.value = true
}

onMounted(load)

async function retry(): Promise<void> {
  await load()
}

function parseAxes(): Record<string, string[]> | null {
  const axes: Record<string, string[]> = {}
  for (const line of axesText.value.split('\n')) {
    const trimmed = line.trim()
    if (trimmed === '') continue
    const sep = trimmed.indexOf(':')
    if (sep === -1) return null
    const axis = trimmed.slice(0, sep).trim()
    const values = trimmed.slice(sep + 1).split(',').map((v) => v.trim()).filter((v) => v !== '')
    if (axis === '' || values.length === 0) return null
    axes[axis] = values
  }
  return Object.keys(axes).length > 0 ? axes : null
}

function generateVariants(): void {
  const axes = parseAxes()
  if (!axes) {
    emit('show-toast', 'Variant axes need one "axis: value, value" line each.', 'error')
    return
  }

  const names = Object.keys(axes)
  let combos: Record<string, string>[] = [{}]
  for (const name of names) {
    const next: Record<string, string>[] = []
    for (const combo of combos) {
      for (const value of axes[name]) {
        next.push({ ...combo, [name]: value })
      }
    }
    combos = next
  }

  variants.value = combos.map((options) => {
    const existing = variants.value.find((v) =>
      Object.entries(options).every(([axis, value]) => v.options[axis] === value)
    )
    return existing ?? { options, sku: '', price: null, stock: 0, weightGrams: null, imageUrl: '' }
  })
}

function updateVariant(index: number, patch: Partial<CatalogVariant>): void {
  variants.value = variants.value.map((v, i) => (i === index ? { ...v, ...patch } : v))
}

function removeVariant(index: number): void {
  variants.value = variants.value.filter((_, i) => i !== index)
}

function toggleVariantImagePicker(index: number): void {
  if (props.disabled || loadFailed.value) return
  variantImagePickerIndex.value = variantImagePickerIndex.value === index ? null : index
}

function handleVariantImagePicked(media: Media): void {
  const index = variantImagePickerIndex.value
  variantImagePickerIndex.value = null
  if (index === null || index >= variants.value.length) return
  updateVariant(index, { imageUrl: media.url })
}

function removeVariantImage(index: number): void {
  if (props.disabled) return
  updateVariant(index, { imageUrl: '' })
}

function buildPayload(): CatalogUpsertPayload {
  return {
    price: price.value ?? 0,
    salePrice: salePrice.value,
    sku: (sku.value ?? '').trim(),
    trackInventory: trackInventory.value,
    stock: stock.value ?? 0,
    weightGrams: weightGrams.value,
    lowStockThreshold: lowStockThreshold.value,
    options: parseAxes() ?? {},
    variants: variants.value.map((v) => ({
      options: v.options,
      sku: (v.sku ?? '').trim(),
      price: v.price ?? null,
      stock: v.stock,
      weightGrams: v.weightGrams ?? null,
      imageUrl: v.imageUrl ?? ''
    }))
  }
}

async function save(): Promise<void> {
  try {
    await catalogStore.upsertProduct(props.contentId, buildPayload())
    hasData.value = true
    // Re-read the stored row so the panel renders what the server kept
    // (variant IDs, timestamps, normalized values) instead of the request.
    const stored = await catalogStore.fetchProduct(props.contentId)
    if (stored) {
      applyProduct(stored)
    }
    emit('show-toast', 'Catalog data saved.', 'success')
  } catch {
    emit('show-toast', catalogStore.error ?? 'Failed to save catalog data.', 'error')
  }
}

async function importLegacy(): Promise<void> {
  if (legacyPrice.value === null) return
  price.value = legacyPrice.value
  sku.value = legacySku.value ?? ''
  try {
    await catalogStore.upsertProduct(props.contentId, buildPayload())
    hasData.value = true
    const stored = await catalogStore.fetchProduct(props.contentId)
    if (stored) {
      applyProduct(stored)
    }
    emit('show-toast', 'Legacy fields imported into the catalog.', 'success')
  } catch {
    emit('show-toast', catalogStore.error ?? 'Failed to import legacy fields.', 'error')
  }
}
</script>

<template>
  <div class="catalog-panel">
    <p v-if="loadError" class="catalog-panel__error" role="alert">
      {{ loadError }}
      <Button size="small" variant="neutral" @click="retry">Retry</Button>
    </p>

    <hr class="catalog-panel__divider" />
    <p class="catalog-panel__separate">
      Catalog data is saved separately from the content. Saving the content does
      not save the catalog — use “Save catalog data” below.
    </p>

    <p v-if="hasLegacyFields" class="catalog-panel__legacy">
      This product still uses custom fields for pricing.
      <Button size="small" variant="neutral" :disabled="disabled || loadFailed" @click="importLegacy">
        Import {{ legacyPrice }} {{ legacySku ? `· ${legacySku}` : '' }} into the catalog
      </Button>
    </p>

    <FormField label="Price">
      <NumberInput v-model="price" :min="0" :disabled="disabled || loadFailed" />
    </FormField>
    <FormField label="Sale price">
      <NumberInput v-model="salePrice" :min="0" :disabled="disabled || loadFailed" />
    </FormField>
    <FormField label="SKU">
      <InputText v-model="sku" :disabled="disabled || loadFailed" placeholder="TSHIRT" />
    </FormField>
    <FormField label="Track inventory">
      <SwitchToggle v-model="trackInventory" :disabled="disabled || loadFailed" />
    </FormField>
    <FormField label="Stock">
      <NumberInput v-if="!hasVariants" v-model="stock" :min="0" :disabled="disabled || loadFailed || !trackInventory" />
      <p v-else class="catalog-panel__sum">Sum of variant stock: {{ variantStockSum }}</p>
      <p v-if="lowStockThreshold !== null && trackInventory" class="catalog-panel__hint">
        Low-stock alert when {{ hasVariants ? 'the variant sum' : 'stock' }} reaches {{ lowStockThreshold }}.
      </p>
    </FormField>
    <FormField label="Weight (grams)">
      <NumberInput v-model="weightGrams" :min="0" :disabled="disabled || loadFailed" />
    </FormField>
    <FormField label="Low stock threshold">
      <NumberInput v-model="lowStockThreshold" :min="0" :disabled="disabled || loadFailed || !trackInventory" />
    </FormField>

    <FormField label="Variant axes">
      <textarea
        v-model="axesText"
        rows="2"
        placeholder="size: S, M, L"
        class="catalog-panel__axes"
        :disabled="disabled || loadFailed"
      />
      <p class="catalog-panel__hint">One axis per line: name, colon, comma-separated values.</p>
      <Button size="small" variant="neutral" :disabled="disabled || loadFailed" @click="generateVariants">
        Generate variants
      </Button>
    </FormField>

    <div v-if="variants.length" class="catalog-panel__variants">
      <div v-for="(variant, index) in variants" :key="index" class="catalog-panel__variant">
        <span class="catalog-panel__variant-options">
          {{ Object.values(variant.options).join(' · ') }}
        </span>
        <InputText v-model="variant.sku" :disabled="disabled || loadFailed" placeholder="SKU" />
        <NumberInput v-model="variant.price" :min="0" :disabled="disabled || loadFailed" placeholder="Price override" />
        <NumberInput v-model="variant.stock" :min="0" :disabled="disabled || loadFailed" />
        <div class="catalog-panel__variant-image">
          <img
            v-if="variant.imageUrl"
            :src="variant.imageUrl"
            alt="Variant image preview"
            class="catalog-panel__variant-thumb"
          />
          <Button
            size="small"
            variant="neutral"
            :disabled="disabled || loadFailed"
            @click="toggleVariantImagePicker(index)"
          >
            {{ variantImagePickerIndex === index ? 'Hide media' : variant.imageUrl ? 'Change' : 'Add image' }}
          </Button>
          <Button
            v-if="variant.imageUrl"
            size="small"
            variant="danger"
            :disabled="disabled || loadFailed"
            @click="removeVariantImage(index)"
          >
            Remove
          </Button>
        </div>
        <Button size="small" variant="danger" :disabled="disabled || loadFailed" @click="removeVariant(index)">
          Remove
        </Button>
      </div>
    </div>

    <div class="catalog-panel__save">
      <Button variant="primary" :disabled="!canSave" @click="save">Save catalog data</Button>
    </div>

    <MediaPanel
      :is-open="variantImagePickerIndex !== null"
      @insert-image="handleVariantImagePicked"
      @show-toast="(msg: string, type: string) => emit('show-toast', msg, type)"
    />
  </div>
</template>

<style scoped>
.catalog-panel {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.catalog-panel__divider {
  border: none;
  border-top: 1px solid var(--color-border);
  margin: 0.25rem 0 0;
}

.catalog-panel__separate {
  color: var(--brand-dark-2);
  font-size: 0.8125rem;
  margin: 0;
}

.catalog-panel__error {
  color: var(--color-error);
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0;
}

.catalog-panel__legacy {
  margin: 0;
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.catalog-panel__axes {
  width: 100%;
  border: 1px solid var(--color-border);
  border-radius: 0.375rem;
  padding: 0.5rem;
  font-size: 0.875rem;
  margin-bottom: 0.5rem;
}

.catalog-panel__hint {
  color: var(--brand-dark-2);
  font-size: 0.75rem;
  margin: 0.25rem 0 0.5rem;
}

.catalog-panel__sum {
  font-size: 0.875rem;
  font-weight: 600;
  margin: 0;
}

.catalog-panel__variants {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.catalog-panel__variant {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem;
  border: 1px solid var(--btn-neutral-border);
  border-radius: 0.375rem;
  background: var(--btn-neutral-bg);
}

.catalog-panel__variant-options {
  font-size: 0.875rem;
  min-width: 8rem;
}

.catalog-panel__variant-image {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.catalog-panel__variant-thumb {
  width: 48px;
  height: 48px;
  object-fit: cover;
  border-radius: 0.375rem;
  border: 1px solid var(--color-border);
}

.catalog-panel__save {
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--color-border);
}
</style>
