import { describe, expect, it } from 'vitest'
import { detectLocale, i18n, intlLocale, setLocale, SUPPORTED_LOCALES, t } from './index'

describe('detectLocale', () => {
  it('prefers the stored choice', () => {
    expect(detectLocale('fr', ['en-US'])).toBe('fr')
    expect(detectLocale('en', ['fr-FR'])).toBe('en')
  })

  it('ignores an unsupported stored value and reads the browser languages in order', () => {
    expect(detectLocale('de', ['de-DE', 'fr-CA', 'en'])).toBe('fr')
    expect(detectLocale(null, ['FR-fr'])).toBe('fr')
  })

  it('falls back to English', () => {
    expect(detectLocale(null, [])).toBe('en')
    expect(detectLocale(undefined, ['de-DE', 'es'])).toBe('en')
  })
})

// The catalogs are translated by hand: a missing key or a renamed placeholder would show a raw key or a blank in one language.
describe('catalogs', () => {
  const flatten = (value: unknown, prefix = ''): Record<string, string> =>
    typeof value === 'string'
      ? { [prefix]: value }
      : Object.entries(value as Record<string, unknown>).reduce(
          (acc, [key, child]) => ({ ...acc, ...flatten(child, prefix ? `${prefix}.${key}` : key) }),
          {} as Record<string, string>,
        )
  const placeholders = (message: string) => [...message.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort()

  const [reference, ...others] = SUPPORTED_LOCALES.map((l) => ({ locale: l, messages: flatten(i18n.global.getLocaleMessage(l)) }))

  it.each(others.map((o) => [o.locale, o] as const))('%s has exactly the keys and placeholders of the reference', (_name, other) => {
    expect(Object.keys(other.messages).sort()).toEqual(Object.keys(reference.messages).sort())
    for (const [key, message] of Object.entries(reference.messages)) {
      expect(placeholders(other.messages[key]), key).toEqual(placeholders(message))
    }
  })
})

describe('setLocale', () => {
  it('switches the messages, the plural forms and the Intl locale', () => {
    setLocale('en')
    expect(t('common.cancel')).toBe('Cancel')
    expect(t('shell.topBar.driveCount', 1)).toBe('+1 drive')
    expect(t('shell.topBar.driveCount', 3)).toBe('+3 drives')
    expect(intlLocale()).toBe('en-GB')
    setLocale('fr')
    expect(t('common.cancel')).toBe('Annuler')
    expect(t('shell.topBar.driveCount', 3)).toBe('+3 trajets')
    expect(intlLocale()).toBe('fr-FR')
  })
})
