export interface CatalogVariant {
  id?: number
  options: Record<string, string>
  sku: string
  price?: number | null
  stock: number
  weightGrams?: number | null
  imageUrl?: string
}

export interface CatalogProduct {
  contentId: number
  price: number
  salePrice?: number | null
  sku: string
  trackInventory: boolean
  stock: number
  weightGrams?: number | null
  lowStockThreshold?: number | null
  options: Record<string, string[]>
  variants: CatalogVariant[]
  updatedAt?: string
}

export interface CatalogUpsertPayload {
  price: number
  salePrice?: number | null
  sku: string
  trackInventory: boolean
  stock: number
  weightGrams?: number | null
  lowStockThreshold?: number | null
  options: Record<string, string[]>
  variants: CatalogVariant[]
}
