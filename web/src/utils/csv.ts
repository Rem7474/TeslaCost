/**
 * Utility to trigger a CSV file download from tabular data.
 */
export function downloadCsv(filename: string, headers: (string | number)[], rows: (string | number)[][]) {
  const content = [headers.join(','), ...rows.map((r) => r.join(','))].join('\n')
  const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename.endsWith('.csv') ? filename : `${filename}.csv`
  a.click()
  URL.revokeObjectURL(url)
}
