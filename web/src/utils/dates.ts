import { intlLocale } from '@/i18n'

/** Today as YYYY-MM-DD (UTC), the format the date pickers and the API use. */
export const todayIso = (): string => new Date().toISOString().substring(0, 10)

/** The YYYY-MM-DD (UTC) day of a date or an ISO string. */
export const toIsoDay = (d: string | Date): string => new Date(d).toISOString().substring(0, 10)

/** Short date with the time in the current language, e.g. "02 mai, 08:30". */
export function formatDayTime(dateStr: string) {
  return new Date(dateStr).toLocaleDateString(intlLocale(), {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}
