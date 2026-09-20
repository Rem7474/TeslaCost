import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'
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

  it('writes zero in the singular in French and in the plural in English', () => {
    setLocale('fr')
    expect(t('tires.sessionCount', 0)).toBe('0 session')
    expect(t('tires.sessionCount', 2)).toBe('2 sessions')
    setLocale('en')
    expect(t('tires.sessionCount', 0)).toBe('0 sessions')
    setLocale('fr')
  })
})

// A key typed in a template or a script that no catalog defines would show up as raw text on screen.
describe('message keys used in the source', () => {
  const walk = (dir: string): string[] =>
    readdirSync(dir).flatMap((name) => {
      const path = join(dir, name)
      if (statSync(path).isDirectory()) return name === 'locales' ? [] : walk(path)
      return /\.(vue|ts)$/.test(name) && !name.endsWith('.test.ts') ? [path] : []
    })
  const catalog = i18n.global.getLocaleMessage('en') as Record<string, unknown>
  const defined = (key: string) => key.split('.').reduce<unknown>((node, part) => (node && typeof node === 'object' ? (node as Record<string, unknown>)[part] : undefined), catalog) !== undefined
  // Every namespace of the application, so a key borrowed from another area is reported even when that catalog is absent.
  const namespaces = new Set(['common', 'shell', 'auth', 'onboarding', 'account', 'dashboard', 'drives', 'expenses', 'tires', 'vehicles', 'carpool', 'manual', 'comparison', 'quickadd'])

  it('all exist in the catalogs', () => {
    const missing: string[] = []
    for (const file of walk(join(__dirname, '..'))) {
      const source = readFileSync(file, 'utf8')
      for (const match of source.matchAll(/(?<![\w.])\$?t\(\s*'([a-zA-Z]+(?:\.[a-zA-Z0-9]+)+)'/g)) {
        if (namespaces.has(match[1].split('.')[0]) && !defined(match[1])) missing.push(`${file.split('/src/')[1]}: ${match[1]}`)
      }
    }
    expect(missing).toEqual([])
  })
})
