const pad = (n: number) => String(n).padStart(2, '0')

/** 2026-10-04 14:03（本地時間）。表格裡用等寬字，所有日期會對齊成一欄。 */
export function formatDateTime(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function formatTime(d: Date): string {
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 逗號、頓號、空白都能分隔，統一成小寫的 slug 陣列 */
export function parseTags(input: string): string[] {
  const seen = new Set<string>()
  for (const raw of input.split(/[,，、\s]+/)) {
    const t = raw.trim().toLowerCase()
    if (t) seen.add(t)
  }
  return [...seen]
}
