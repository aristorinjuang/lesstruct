import type { FieldSchema } from './customfield'

export interface PostType {
  name: string
  slug: string
  description?: string
  supports: string[]
  fields?: FieldSchema[]
  systemFields?: FieldSchema[]
}

export interface PostTypesResponse {
  data: PostType[]
  error: null
  meta: {
    timestamp: string
  }
}

export interface UserFieldsResponse {
  data: {
    fields: FieldSchema[]
    systemFields: FieldSchema[]
  }
  error: null
}
