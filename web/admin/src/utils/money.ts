/**
 * Money formatting for minor-unit amounts (the commerce payloads carry
 * integer minor units plus a currency code). Mirrors
 * modules/orders/domain/money.go and the storefront formatter:
 * zero-decimal currencies (IDR, JPY, KRW, VND) render the ledger integer
 * as-is with no decimals; every other currency divides by 100 with two
 * fixed decimals. Comma-grouped thousands.
 */
const ZERO_DECIMAL = new Set(['IDR', 'JPY', 'KRW', 'VND'])

export function formatMoney(currency: string, amount: number): string {
  if (!Number.isFinite(amount)) {
    return `${currency} 0.00`
  }
  const code = (currency || 'USD').toUpperCase()
  if (ZERO_DECIMAL.has(code)) {
    const negative = amount < 0
    const abs = Math.abs(Math.round(amount))
    const grouped = abs.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',')
    return `${currency} ${negative ? '-' : ''}${grouped}`
  }
  const negative = amount < 0
  const abs = Math.abs(Math.round(amount))
  const cents = abs % 100
  const whole = Math.floor(abs / 100)
  const grouped = whole.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  const value = `${negative ? '-' : ''}${grouped}.${String(cents).padStart(2, '0')}`
  return `${currency} ${value}`
}
