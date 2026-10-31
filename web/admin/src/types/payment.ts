export const PAYMENT_STATUSES = ['pending', 'verified', 'rejected'] as const

export type PaymentStatus = (typeof PAYMENT_STATUSES)[number]

export interface Payment {
  id: number
  orderId: number
  method: string
  status: PaymentStatus
  reference: string
  amount: number
  verifiedBy?: string
  createdAt: string
  updatedAt: string
}

export interface PaymentOrderContext {
  orderNumber: string
  status: string
  totalMinor: number
}

export interface PaymentDetail extends Payment {
  order: PaymentOrderContext
}

export interface PaymentListMeta {
  pagination?: {
    nextCursor?: string
    hasMore?: boolean
  }
}
