import { createI18n } from 'vue-i18n'

export const SUPPORTED_LOCALES = ['en', 'fr'] as const
export type AppLocale = (typeof SUPPORTED_LOCALES)[number]

export const DEFAULT_LOCALE: AppLocale = 'en'
const STORAGE_KEY = 'teslacost_locale'

// Locale tag handed to Intl (dates, numbers) and to the date picker for each application language.
const INTL_LOCALES: Record<AppLocale, string> = { en: 'en-GB', fr: 'fr-FR' }

const isSupported = (value: string | null | undefined): value is AppLocale =>
  !!value && (SUPPORTED_LOCALES as readonly string[]).includes(value)

/** The stored choice first, then the first browser language the application speaks, then English. */
export function detectLocale(stored: string | null | undefined, browserLanguages: readonly string[]): AppLocale {
  if (isSupported(stored)) return stored
  for (const tag of browserLanguages) {
    const base = tag.toLowerCase().split('-')[0]
    if (isSupported(base)) return base
  }
  return DEFAULT_LOCALE
}

// Catalogs live in locales/<language>/<namespace>.json; a message key is "<namespace>.<key>".
type Messages = Record<string, Record<string, unknown>>
function loadCatalogs(): Record<AppLocale, Messages> {
  const modules = import.meta.glob('../locales/*/*.json', { eager: true, import: 'default' }) as Record<string, Record<string, unknown>>
  const catalogs = { en: {}, fr: {} } as Record<AppLocale, Messages>
  for (const [path, content] of Object.entries(modules)) {
    const match = path.match(/locales\/([^/]+)\/([^/]+)\.json$/)
    if (!match || !isSupported(match[1])) continue
    catalogs[match[1]][match[2]] = content
  }
  return catalogs
}

const storage = typeof localStorage === 'undefined' ? null : localStorage
const browserLanguages = typeof navigator === 'undefined' ? [] : navigator.languages ?? [navigator.language]

export const i18n = createI18n({
  legacy: false,
  locale: detectLocale(storage?.getItem(STORAGE_KEY), browserLanguages),
  fallbackLocale: DEFAULT_LOCALE,
  messages: loadCatalogs(),
})

/** Translate outside a component (utils, stores). Reactive when called while a component renders. */
export const t = i18n.global.t

export const currentLocale = (): AppLocale => i18n.global.locale.value as AppLocale

/** Locale tag for Intl and date-fns formatting in the current language. */
export const intlLocale = (): string => INTL_LOCALES[currentLocale()]

function applyDocumentLanguage(locale: AppLocale) {
  if (typeof document !== 'undefined') document.documentElement.lang = locale
}

export function setLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  storage?.setItem(STORAGE_KEY, locale)
  applyDocumentLanguage(locale)
}

applyDocumentLanguage(currentLocale())
