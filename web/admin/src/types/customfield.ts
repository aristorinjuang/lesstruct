export type FieldType =
  | 'text'
  | 'textarea'
  | 'number'
  | 'date'
  | 'datetime'
  | 'email'
  | 'url'
  | 'select'
  | 'checkbox'
  | 'multiselect'
  | 'list'
  | 'gallery'

export interface GalleryItem {
  url: string
  alt: string
}

export interface FieldSchema {
  name: string
  slug: string
  type: FieldType
  required?: boolean
  options?: string[]
  min?: number
  max?: number
  maxLength?: number
  maxItems?: number
}
