import type { FieldSchema } from '@/types/customfield'

function isEmpty(value: unknown): boolean {
  return value === null || value === undefined || value === ''
}

export function validateCustomField(field: FieldSchema, value: unknown): string | null {
  if (field.type === 'checkbox') return null

  if (field.required && isEmpty(value)) {
    return `${field.name} is required`
  }

  if (isEmpty(value)) return null

  switch (field.type) {
    case 'text':
    case 'textarea':
      if (field.maxLength && String(value).length > field.maxLength) {
        return `${field.name} must be ${field.maxLength} characters or less`
      }
      break
    case 'number':
      if (isNaN(Number(value))) {
        return `${field.name} must be a valid number`
      }
      if (field.min != null && Number(value) < field.min) {
        return `${field.name} must be at least ${field.min}`
      }
      if (field.max != null && Number(value) > field.max) {
        return `${field.name} must be at most ${field.max}`
      }
      break
    case 'date': {
      if (!/^\d{4}-\d{2}-\d{2}$/.test(String(value))) {
        return `${field.name} must be a valid date in YYYY-MM-DD format`
      }
      break
    }
    case 'datetime': {
      if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2})?(\.\d+)?(Z|[+-]\d{2}:\d{2})$/.test(String(value))) {
        return `${field.name} must be a valid datetime in RFC 3339 format`
      }
      break
    }
    case 'email': {
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(String(value))) {
        return `${field.name} must be a valid email address`
      }
      break
    }
    case 'url': {
      if (!/^https?:\/\/\S+$/.test(String(value))) {
        return `${field.name} must be a valid http(s) URL`
      }
      break
    }
    case 'select':
      if (field.options && !field.options.includes(String(value))) {
        return `${field.name} must be one of: ${field.options.join(', ')}`
      }
      break
  }

  return null
}

export function validateCustomFields(
  fields: FieldSchema[],
  values: Record<string, unknown>,
): Record<string, string> {
  const errors: Record<string, string> = {}
  for (const field of fields) {
    const error = validateCustomField(field, values[field.slug])
    if (error) errors[field.slug] = error
  }
  return errors
}
