// Turns a User-Agent header into a short label for the list of sessions ("Firefox sur Linux").
// Only the common browsers and systems are recognised; anything else falls back to a generic label.

export function describeUserAgent(ua: string | null | undefined): string {
  if (!ua) return 'Appareil inconnu'

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

  if (browser && system) return `${browser} sur ${system}`
  return browser ?? system ?? 'Appareil inconnu'
}

const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ['day', 86_400],
  ['hour', 3_600],
  ['minute', 60],
]

// "il y a 3 heures", "à l'instant"; dates older than a week are written out.
export function describeRelativeTime(iso: string, now: Date = new Date(), locale = 'fr-FR'): string {
  const then = new Date(iso)
  const seconds = Math.round((then.getTime() - now.getTime()) / 1000)
  if (Number.isNaN(seconds)) return ''
  if (Math.abs(seconds) < 60) return "à l'instant"
  if (Math.abs(seconds) >= 7 * 86_400) return then.toLocaleDateString(locale, { day: '2-digit', month: 'short', year: 'numeric' })
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'always' })
  for (const [unit, size] of units) {
    if (Math.abs(seconds) >= size) return rtf.format(Math.trunc(seconds / size), unit)
  }
  return ''
}
