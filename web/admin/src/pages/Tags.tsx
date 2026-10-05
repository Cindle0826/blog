import { useCallback, useEffect, useState } from 'react'
import { listTags, renameTag, type Tag } from '../api'
import { ErrorNotice } from '../components/ErrorNotice'
import { PageHead } from '../components/Layout'

export function Tags() {
  const [tags, setTags] = useState<Tag[] | null>(null)
  const [error, setError] = useState<unknown>(null)

  const load = useCallback(async () => {
    setError(null)
    try {
      setTags(await listTags())
    } catch (e) {
      setError(e)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  return (
    <>
      <PageHead title="標籤" />
      <p className="lede">
        標籤在寫文章時直接輸入就會建立。這裡只能改顯示名稱，slug 是網址的一部分，建立後不能改。
      </p>

      {error ? (
        <ErrorNotice error={error} onRetry={() => void load()} />
      ) : !tags ? (
        <p className="muted">載入中…</p>
      ) : tags.length === 0 ? (
        <p className="empty">還沒有標籤。</p>
      ) : (
        <table className="table table--static">
          <thead>
            <tr>
              <th>顯示名稱</th>
              <th>slug</th>
              <th className="table__num">文章數</th>
            </tr>
          </thead>
          <tbody>
            {tags.map((t) => (
              <TagRow key={t.slug} tag={t} onSaved={(saved) =>
                setTags((list) => list?.map((x) => (x.slug === saved.slug ? saved : x)) ?? null)} />
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}

function TagRow({ tag, onSaved }: { tag: Tag; onSaved: (t: Tag) => void }) {
  const [name, setName] = useState(tag.name)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const changed = name.trim() !== '' && name.trim() !== tag.name

  const save = async () => {
    if (!changed) return
    setBusy(true)
    setError(null)
    try {
      onSaved(await renameTag(tag.slug, name.trim()))
    } catch (e) {
      setError(e)
    } finally {
      setBusy(false)
    }
  }

  return (
    <tr>
      <td>
        <form className="inline-edit" onSubmit={(e) => { e.preventDefault(); void save() }}>
          <input className="input input--line" value={name} onChange={(e) => setName(e.target.value)}
            aria-label={`${tag.slug} 的顯示名稱`} />
          {changed && (
            <button type="submit" className="link-btn" disabled={busy}>{busy ? '儲存中…' : '儲存'}</button>
          )}
        </form>
        {error != null && <ErrorNotice error={error} />}
      </td>
      <td className="mono muted">{tag.slug}</td>
      <td className="table__num">{tag.count}</td>
    </tr>
  )
}
