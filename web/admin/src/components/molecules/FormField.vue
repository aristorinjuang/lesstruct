<script setup lang="ts">
interface Props {
  id?: string
  label?: string
  error?: string
  required?: boolean
  labelSuffix?: string
}

withDefaults(defineProps<Props>(), {
  id: '',
  label: '',
  error: '',
  required: false,
  labelSuffix: undefined
})
</script>

<template>
  <div class="form-field" :aria-invalid="!!error || undefined">
    <label v-if="label" :id="id" class="form-field__label">
      {{ label }}
      <span v-if="required" class="form-field__required">*</span>
      <span v-if="labelSuffix" class="form-field__label-suffix">{{ labelSuffix }}</span>
    </label>
    <div class="form-field__content">
      <slot />
    </div>
    <span v-if="error" class="form-field__error">{{ error }}</span>
  </div>
</template>

<style scoped>
.form-field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.form-field__label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1);
}

.form-field__required {
  color: var(--color-error);
  margin-left: 0.125rem;
}

.form-field__label-suffix {
  font-size: 0.675rem;
  text-transform: uppercase;
  color: var(--brand-dark-2);
  background-color: var(--brand-light-1);
  border-radius: 4px;
  padding: 0.125rem 0.375rem;
  margin-left: 0.5rem;
}

.form-field__error {
  font-size: 0.875rem;
  color: var(--color-error);
}
</style>
