import { useCallback, useEffect, useState, type KeyboardEvent } from 'react'
import { createTag, listTags, type Tag } from '../api'
import { isValidSlug } from '../format'
import { ErrorNotice } from './ErrorNotice'

/**
 * 文章的標籤只能從已經建立的標籤裡選（API.md「標籤」）。
 * 要用新標籤就在這裡建立，建立完重新拉一次列表再勾起來。
 */
export function TagPicker({ value, onChange, labelId }: {
  value: string[]
  onChange: (slugs: string[]) => void
  labelId: string
}) {
  const [tags, setTags] = useState<Tag[] | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [adding, setAdding] = useState(false)

  const load = useCallback(async () => {
    setError(null)
    try {
      setTags(await listTags())
    } catch (e) {
      setError(e)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  if (error) return <ErrorNotice error={error} onRetry={() => void load()} />
  if (!tags) return <p className="muted tag-picker__state">載入標籤中…</p>

  const toggle = (slug: string, on: boolean) =>
    onChange(on ? [...value, slug] : value.filter((s) => s !== slug))

  // 文章上有、但標籤列表裡沒有的：舊資料才會發生。照樣列出來讓人取消勾選，
  // 不然存檔時後端回 400，畫面上卻看不出是哪一個。
  const missing = value.filter((s) => !tags.some((t) => t.slug === s))

  return (
    <>
      <div className="tag-picker" role="group" aria-labelledby={labelId}>
        {tags.map((t) => (
          <label key={t.slug} className="tag-picker__item" title={t.slug}>
            <input type="checkbox" checked={value.includes(t.slug)}
              onChange={(e) => toggle(t.slug, e.target.checked)} />
            <span>{t.name}</span>
          </label>
        ))}
        {missing.map((slug) => (
          <label key={slug} className="tag-picker__item tag-picker__item--missing" title="這個標籤不存在，存檔前要取消勾選">
            <input type="checkbox" checked onChange={() => toggle(slug, false)} />
            <span className="mono">{slug}</span>
            <span>（不存在）</span>
          </label>
        ))}
        {!adding && (
          <button type="button" className="link-btn tag-picker__add" onClick={() => setAdding(true)}>新增標籤</button>
        )}
      </div>
      {adding && (
        <NewTagForm
          onCancel={() => setAdding(false)}
          onCreated={async (tag) => {
            setAdding(false)
            await load()
            if (!value.includes(tag.slug)) onChange([...value, tag.slug])
          }} />
      )}
    </>
  )
}

/**
 * 建立標籤的欄位。故意不用 <form>：它會放在文章編輯頁的 <form> 裡面，
 * 巢狀 form 不合法，Enter 也會變成送出整篇文章。
 */
export function NewTagForm({ onCreated, onCancel }: {
  onCreated: (tag: Tag) => void | Promise<void>
  onCancel?: () => void
}) {
  const [slug, setSlug] = useState('')
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)

  const trimmed = slug.trim()
  const slugOk = isValidSlug(trimmed)
  const canCreate = slugOk && !busy

  const create = async () => {
    if (!canCreate) return
    setBusy(true)
    setError(null)
    try {
      // 顯示名稱留空就先用 slug，之後可以在標籤頁改
      const tag = await createTag(trimmed, name.trim() || trimmed)
      setSlug('')
      setName('')
      await onCreated(tag)
    } catch (e) {
      setError(e)
    } finally {
      setBusy(false)
    }
  }

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.nativeEvent.isComposing) return
    if (e.key === 'Enter') {
      e.preventDefault()
      void create()
    } else if (e.key === 'Escape' && onCancel) {
      onCancel()
    }
  }

  return (
    <div className="tag-new">
      <div className="tag-new__row">
        <input className="input input--mono tag-new__slug" value={slug} autoFocus spellCheck={false}
          onChange={(e) => { setSlug(e.target.value); setError(null) }} onKeyDown={onKeyDown}
          placeholder="slug，例如 rust" aria-label="新標籤的 slug" />
        <input className="input tag-new__name" value={name}
          onChange={(e) => { setName(e.target.value); setError(null) }} onKeyDown={onKeyDown}
          placeholder="顯示名稱，留空用 slug" aria-label="新標籤的顯示名稱" />
        <button type="button" className="btn" onClick={() => void create()} disabled={!canCreate}>
          {busy ? '建立中…' : '建立'}
        </button>
        {onCancel && <button type="button" className="link-btn" onClick={onCancel}>取消</button>}
      </div>
      {trimmed !== '' && !slugOk && (
        <span className="field__hint">slug 只能用小寫英數與連字號，最多 60 字。它是標籤頁的網址，建立後不能改。</span>
      )}
      {error != null && <ErrorNotice error={error} />}
    </div>
  )
}
