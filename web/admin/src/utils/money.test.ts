import { describe, it, expect } from 'vitest'
import { formatMoney } from './money'

describe('formatMoney', () => {
  it('formats zero-decimal currencies without decimals', () => {
    expect(formatMoney('IDR', 299000)).toBe('IDR 299,000')
    expect(formatMoney('IDR', 604000)).toBe('IDR 604,000')
    expect(formatMoney('IDR', 150000)).toBe('IDR 150,000')
    expect(formatMoney('JPY', 1200)).toBe('JPY 1,200')
    expect(formatMoney('KRW', 50000)).toBe('KRW 50,000')
    expect(formatMoney('VND', 250000)).toBe('VND 250,000')
  })

  it('formats two-decimal currencies with fixed decimals', () => {
    expect(formatMoney('USD', 299000)).toBe('USD 2,990.00')
    expect(formatMoney('USD', 5)).toBe('USD 0.05')
    expect(formatMoney('USD', 1234567)).toBe('USD 12,345.67')
  })

  it('formats zero and negative zero-decimal amounts', () => {
    expect(formatMoney('IDR', 0)).toBe('IDR 0')
    expect(formatMoney('IDR', -299000)).toBe('IDR -299,000')
    expect(formatMoney('USD', -299000)).toBe('USD -2,990.00')
  })

  it('falls back for non-finite input', () => {
    expect(formatMoney('IDR', Number.NaN)).toBe('IDR 0.00')
  })
})
