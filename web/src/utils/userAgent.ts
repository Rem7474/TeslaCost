import { intlLocale, t } from '@/i18n'
// Turns a User-Agent header into a short label for the list of sessions ("Firefox on Linux").
// Only the common browsers and systems are recognised; anything else falls back to a generic label.

export function describeUserAgent(ua: string | null | undefined): string {
  if (!ua) return t('account.unknownDevice')

  // Order matters: Edge and Opera also announce Chrome, Chrome also announces Safari.
  const browser =
    /Edg(?:e|A|iOS)?\//.test(ua) ? 'Edge'
    : /OPR\/|Opera/.test(ua) ? 'Opera'
    : /Firefox\/|FxiOS\//.test(ua) ? 'Firefox'
    : /Chrome\/|CriOS\//.test(ua) ? 'Chrome'
    : /Safari\//.test(ua) && /Version\//.test(ua) ? 'Safari'
    : /curl\//i.test(ua) ? 'curl'
    : null

  // Android and iOS UAs also mention Linux / Mac OS X.
  const system =
    /Android/.test(ua) ? 'Android'
    : /iPhone|iPad|iPod/.test(ua) ? 'iOS'
    : /Windows/.test(ua) ? 'Windows'
    : /Mac OS X|Macintosh/.test(ua) ? 'macOS'
    : /CrOS/.test(ua) ? 'ChromeOS'
    : /Linux|X11/.test(ua) ? 'Linux'
    : null

  if (browser && system) return t('account.browserOnSystem', { browser, system })
  return browser ?? system ?? t('account.unknownDevice')
}

const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ['day', 86_400],
  ['hour', 3_600],
  ['minute', 60],
]

// "3 hours ago", "just now"; dates older than a week are written out.
export function describeRelativeTime(iso: string, now: Date = new Date(), locale = intlLocale()): string {
  const then = new Date(iso)
  const seconds = Math.round((then.getTime() - now.getTime()) / 1000)
  if (Number.isNaN(seconds)) return ''
  if (Math.abs(seconds) < 60) return t('account.justNow')
  if (Math.abs(seconds) >= 7 * 86_400) return then.toLocaleDateString(locale, { day: '2-digit', month: 'short', year: 'numeric' })
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'always' })
  for (const [unit, size] of units) {
    if (Math.abs(seconds) >= size) return rtf.format(Math.trunc(seconds / size), unit)
  }
  return ''
}
