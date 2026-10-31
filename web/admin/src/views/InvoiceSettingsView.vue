<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import api, { ApiError } from '@/utils/request'
import Button from '@/components/atoms/Button.vue'
import InputText from '@/components/atoms/InputText.vue'
import NumberInput from '@/components/atoms/NumberInput.vue'
import FormField from '@/components/molecules/FormField.vue'
import Toast from '@/components/molecules/Toast.vue'

interface InvoiceSettings {
  sellerName: string
  sellerAddress: string
  sellerTaxId: string
  taxRate: number
}

interface SettingsResponse {
  data: InvoiceSettings
}

const sellerName = ref('')
const sellerAddress = ref('')
const sellerTaxId = ref('')
const taxRate = ref<number | null>(12)

const isLoading = ref(true)
const isSaving = ref(false)
const isMounted = ref(true)

// Toast state (mirrors the other admin views)
const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const toastVisible = ref(false)
const toastKey = ref(0)

function displayToast(message: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = message
  toastType.value = type
  toastKey.value++
  toastVisible.value = true
}

function apply(settings: InvoiceSettings) {
  sellerName.value = settings.sellerName
  sellerAddress.value = settings.sellerAddress
  sellerTaxId.value = settings.sellerTaxId
  taxRate.value = settings.taxRate
}

async function loadSettings() {
  isLoading.value = true
  try {
    const response = await api.get<SettingsResponse>('/api/v1/orders/invoice-settings')
    apply(response.data.data)
  } catch (err) {
    if (isMounted.value) {
      displayToast((err as ApiError)?.message || 'Failed to load invoice settings', 'error')
    }
  } finally {
    isLoading.value = false
  }
}

async function save() {
  if (taxRate.value === null) {
    displayToast('Tax rate is required', 'error')
    return
  }
  isSaving.value = true
  try {
    const response = await api.put<SettingsResponse>('/api/v1/orders/invoice-settings', {
      sellerName: sellerName.value,
      sellerAddress: sellerAddress.value,
      sellerTaxId: sellerTaxId.value,
      taxRate: taxRate.value,
    })
    apply(response.data.data)
    if (isMounted.value) displayToast('Invoice settings saved')
  } catch (err) {
    if (isMounted.value) {
      displayToast((err as ApiError)?.message || 'Failed to save invoice settings', 'error')
    }
  } finally {
    isSaving.value = false
  }
}

onMounted(() => {
  loadSettings()
})

onUnmounted(() => {
  isMounted.value = false
})
</script>

<template>
  <div class="invoice-settings-view">
    <header class="page-header--stacked">
      <h1 class="page-title">Invoice Settings</h1>
      <p class="page-subtitle">
        The seller identity and PPN rate stamped on every invoice. Saving rewrites the
        [invoice] block in config.toml, so the change survives a restart and applies to the
        next rendered invoice.
      </p>
    </header>

    <Toast
      v-if="toastMessage"
      :key="toastKey"
      :message="toastMessage"
      :type="toastType"
      :visible="toastVisible"
      @dismiss="toastVisible = false"
    />

    <div v-if="isLoading" class="invoice-settings-view__loading">Loading invoice settings...</div>

    <form v-else class="invoice-settings-view__form" @submit.prevent="save">
      <FormField id="invoice-seller-name" label="Seller name">
        <InputText
          id="invoice-seller-name"
          v-model="sellerName"
          placeholder="PT Example Indonesia"
          :disabled="isSaving"
        />
      </FormField>

      <FormField id="invoice-seller-address" label="Seller address">
        <InputText
          id="invoice-seller-address"
          v-model="sellerAddress"
          placeholder="Jl. Example No. 1, Jakarta"
          multiline
          :disabled="isSaving"
        />
      </FormField>

      <FormField id="invoice-seller-tax-id" label="Tax ID (NPWP)">
        <InputText
          id="invoice-seller-tax-id"
          v-model="sellerTaxId"
          placeholder="01.234.567.8-901.234"
          :disabled="isSaving"
        />
      </FormField>

      <FormField
        id="invoice-tax-rate"
        label="PPN rate"
        label-suffix="default: statutory PPN 12%"
      >
        <NumberInput
          id="invoice-tax-rate"
          v-model="taxRate"
          :min="0"
          :max="100"
          :step="0.01"
          :disabled="isSaving"
        />
      </FormField>

      <p class="invoice-settings-view__hint">
        Set 0 for non-PKP sellers: the invoice then carries no tax line.
      </p>

      <Button type="submit" :is-loading="isSaving">Save settings</Button>
    </form>
  </div>
</template>

<style scoped>
.invoice-settings-view__form {
  max-width: 40rem;
}

.invoice-settings-view__hint {
  margin: -0.5rem 0 1.5rem;
  font-size: 0.875rem;
  color: var(--brand-dark-2);
}

.invoice-settings-view__loading {
  padding: 2rem;
  color: var(--brand-dark-2);
}
</style>
