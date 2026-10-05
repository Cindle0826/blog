import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { listPosts, type PostKind, type PostPage, type PostStatus } from '../api'
import { ErrorNotice } from '../components/ErrorNotice'
import { PageHead } from '../components/Layout'
import { formatDateTime } from '../format'

const PER_PAGE = 20

const STATUS_TABS: { value: PostStatus | 'all'; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'draft', label: '草稿' },
  { value: 'published', label: '已發布' },
]

const KIND_TABS: { value: PostKind | 'all'; label: string }[] = [
  { value: 'all', label: '全部類型' },
  { value: 'post', label: '文章' },
  { value: 'note', label: '筆記' },
]

export const KIND_LABEL: Record<PostKind, string> = { post: '文章', note: '筆記' }
export const STATUS_LABEL: Record<PostStatus, string> = { draft: '草稿', published: '已發布' }

export function PostList() {
  // 篩選條件放在網址上：重新整理、上一頁、把網址傳給自己都會停在同一個畫面。
  const [params, setParams] = useSearchParams()
  const status = (params.get('status') as PostStatus | null) ?? 'all'
  const kind = (params.get('kind') as PostKind | null) ?? 'all'
  const search = params.get('search') ?? ''
  const page = Math.max(1, Number(params.get('page')) || 1)

  const [data, setData] = useState<PostPage | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setData(await listPosts({ status, kind, search, page, limit: PER_PAGE }))
    } catch (e) {
      setError(e)
    } finally {
      setLoading(false)
    }
  }, [status, kind, search, page])

  useEffect(() => { void load() }, [load])

  const update = (patch: Record<string, string | null>) => {
    const next = new URLSearchParams(params)
    for (const [k, v] of Object.entries(patch)) {
      if (v === null || v === '' || v === 'all') next.delete(k)
      else next.set(k, v)
    }
    // 換篩選條件時回到第 1 頁，不然可能停在一個不存在的頁碼
    if (!('page' in patch)) next.delete('page')
    setParams(next)
  }

  const onSearch = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const value = new FormData(e.currentTarget).get('search')
    update({ search: typeof value === 'string' ? value.trim() : '' })
  }

  return (
    <>
      <PageHead title="文章">
        <Link className="btn btn--primary" to="/posts/new">新增</Link>
      </PageHead>

      <div className="filters">
        <Tabs items={STATUS_TABS} value={status} onChange={(v) => update({ status: v })} />
        <Tabs items={KIND_TABS} value={kind} onChange={(v) => update({ kind: v })} />
        <form className="search" onSubmit={onSearch} role="search">
          <input key={search} name="search" type="search" className="input input--line" defaultValue={search}
            placeholder="搜尋標題" aria-label="搜尋標題" />
        </form>
      </div>

      {error ? (
        <ErrorNotice error={error} onRetry={() => void load()} />
      ) : !data ? (
        <p className="muted">載入中…</p>
      ) : data.items.length === 0 ? (
        <Empty filtered={status !== 'all' || kind !== 'all' || search !== ''} />
      ) : (
        <>
          <PostTable data={data} dimmed={loading} />
          <Pager page={data.page} totalPages={data.totalPages} total={data.total}
            onChange={(p) => update({ page: String(p) })} />
        </>
      )}
    </>
  )
}

function Tabs<T extends string>({ items, value, onChange }: {
  items: { value: T; label: string }[]
  value: T
  onChange: (v: T) => void
}) {
  return (
    <div className="tabs" role="group">
      {items.map((it) => (
        <button key={it.value} type="button" className="tabs__item"
          aria-pressed={it.value === value} onClick={() => onChange(it.value)}>
          {it.label}
        </button>
      ))}
    </div>
  )
}

function PostTable({ data, dimmed }: { data: PostPage; dimmed: boolean }) {
  const navigate = useNavigate()

  return (
    <table className={`table${dimmed ? ' is-dimmed' : ''}`}>
      <thead>
        <tr>
          <th>標題</th>
          <th className="table__narrow">類型</th>
          <th className="table__narrow">狀態</th>
          <th className="table__date">最後修改</th>
        </tr>
      </thead>
      <tbody>
        {data.items.map((p) => (
          // 整列可點，但真正的連結在標題上：鍵盤和螢幕閱讀器才有東西可以聚焦
          <tr key={p.id} onClick={() => navigate(`/posts/${p.id}`)}>
            <td>
              <Link className="table__title" to={`/posts/${p.id}`} onClick={(e) => e.stopPropagation()}>
                {p.title}
              </Link>
              {p.pinned && <span className="tag-label">置頂</span>}
              <div className="table__slug">/posts/{p.slug}</div>
            </td>
            <td className="table__narrow">{KIND_LABEL[p.kind]}</td>
            <td className="table__narrow">
              <span className={`status status--${p.status}`}>{STATUS_LABEL[p.status]}</span>
            </td>
            <td className="table__date">{formatDateTime(p.updatedAt)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function Pager({ page, totalPages, total, onChange }: {
  page: number
  totalPages: number
  total: number
  onChange: (p: number) => void
}) {
  if (totalPages <= 1) {
    return <p className="pager"><span className="pager__state">共 {total} 篇</span></p>
  }
  return (
    <nav className="pager" aria-label="分頁">
      <button type="button" className="link-btn" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        ← 上一頁
      </button>
      <span className="pager__state">{page} / {Math.max(totalPages, 1)} · 共 {total} 篇</span>
      <button type="button" className="link-btn" disabled={page >= totalPages} onClick={() => onChange(page + 1)}>
        下一頁 →
      </button>
    </nav>
  )
}

function Empty({ filtered }: { filtered: boolean }) {
  return filtered ? (
    <p className="empty">沒有符合條件的文章。</p>
  ) : (
    <p className="empty">還沒有文章。<Link to="/posts/new">寫第一篇</Link></p>
  )
}
