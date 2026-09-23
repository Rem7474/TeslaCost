import { intlLocale } from '@/i18n'

// A vehicle's currency is fixed at creation (see CLAUDE.md) and travels with its own data —
// unlike the language or distance unit, it is not an account-wide preference, so every call site
// passes the currency of the vehicle the amount belongs to rather than reading a global setting.

/** Formats a cents amount as currency, e.g. formatMoney(154000, "EUR") -> "1 540,00 €". */
export function formatMoney(cents: number, currency: string): string {
  return formatAmount(cents / 100, currency)
}

/** Formats a currency amount already in main units (not cents), e.g. a €/km rate. */
export function formatAmount(amount: number, currency: string, maximumFractionDigits = 2): string {
  return new Intl.NumberFormat(intlLocale(), { style: 'currency', currency, maximumFractionDigits }).format(amount)
}
