export const ORDER_STATUSES = [
  'pending',
  'paid',
  'packed',
  'shipped',
  'delivered',
  'cancelled',
  'refunded',
  'expired',
] as const

export type OrderStatus = (typeof ORDER_STATUSES)[number]

export interface ShipTo {
  street: string
  city: string
  province: string
  postalCode: string
}

export interface OrderItem {
  id: number
  productContentId: number
  variantId?: number | null
  sku?: string
  title: string
  unitPrice: number
  quantity: number
  lineTotal: number
}

export interface Order {
  id: number
  orderNumber: string
  status: OrderStatus
  customerEmail: string
  customerName: string
  customerPhone?: string
  shipTo: ShipTo
  currency: string
  subtotal: number
  shippingMinor: number
  totalMinor: number
  items: OrderItem[]
  itemCount: number
  createdAt: string
  updatedAt: string
}

export interface OrderListMeta {
  pagination?: {
    nextCursor?: string
    hasMore?: boolean
  }
}

/**
 * The admin lifecycle matrix, mirroring modules/orders/domain/status.go:
 * terminal statuses (cancelled, refunded, expired) have no outgoing moves.
 * `paid` never appears as a target: payment confirmation is the payments
 * module's proof-gated path (manual verify or gateway webhook), so no
 * admin surface can mint a payment.
 */
export const ALLOWED_TRANSITIONS: Record<OrderStatus, readonly OrderStatus[]> = {
  pending: ['cancelled', 'expired'],
  paid: ['packed', 'cancelled', 'refunded'],
  packed: ['shipped', 'cancelled', 'refunded'],
  shipped: ['delivered', 'refunded'],
  delivered: ['refunded'],
  cancelled: [],
  refunded: [],
  expired: [],
}

/** Terminal moves risk revenue or fulfilment, so they ask first. */
export const CONFIRMED_TRANSITIONS: ReadonlySet<OrderStatus> = new Set([
  'cancelled',
  'refunded',
  'expired',
])
