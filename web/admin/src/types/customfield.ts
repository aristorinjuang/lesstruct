export type FieldType = 'text' | 'textarea' | 'number' | 'date' | 'select' | 'checkbox'

export interface FieldSchema {
  name: string
  slug: string
  type: FieldType
  required?: boolean
  options?: string[]
  min?: number
  max?: number
  maxLength?: number
}
