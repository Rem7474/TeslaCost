import { i18n, t } from '@/i18n'

export interface ApiErrorBody {
  error?: string
  code?: string
  params?: Record<string, unknown>
}

/** The message of an API error in the current language: the catalog entry of its code, else the server's message. */
export function apiErrorMessage(body: ApiErrorBody | null | undefined, fallback: string): string {
  const key = body?.code ? `errors.${body.code}` : ''
  if (key && i18n.global.te(key)) return t(key, body?.params ?? {})
  return body?.error || fallback
}
