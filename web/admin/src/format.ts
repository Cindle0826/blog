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

/** 跟後端 store.ValidSlug 同一條規則：小寫英數與連字號，最多 60 字 */
export function isValidSlug(s: string): boolean {
  return s.length <= 60 && /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(s)
}
