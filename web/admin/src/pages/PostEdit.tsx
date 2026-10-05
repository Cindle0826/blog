import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Link, useBlocker, useLocation, useNavigate, useParams } from 'react-router-dom'
import {
  createPost, deletePost, getPost, setPublished, updatePost,
  type PostDetail, type PostKind, type PostPatch,
} from '../api'
import { ErrorNotice } from '../components/ErrorNotice'
import { formatDateTime, formatTime, parseTags } from '../format'
import { KIND_LABEL, STATUS_LABEL } from './PostList'

interface Form {
  title: string
  slug: string
  kind: PostKind
  tags: string
  summary: string
  markdown: string
  pinned: boolean
}

const EMPTY: Form = { title: '', slug: '', kind: 'post', tags: '', summary: '', markdown: '', pinned: false }

const toForm = (p: PostDetail): Form => ({
  title: p.title,
  slug: p.slug,
  kind: p.kind,
  tags: p.tagSlugs.join(', '),
  summary: p.summary,
  markdown: p.markdown,
  pinned: p.pinned,
})

/** 只送有改動的欄位——契約規定 PATCH 沒送的欄位要維持原狀 */
function diff(form: Form, base: Form): PostPatch {
  const patch: PostPatch = {}
  if (form.title !== base.title) patch.title = form.title.trim()
  if (form.slug !== base.slug) patch.slug = form.slug.trim()
  if (form.kind !== base.kind) patch.kind = form.kind
  if (form.summary !== base.summary) patch.summary = form.summary
  if (form.markdown !== base.markdown) patch.markdown = form.markdown
  if (form.pinned !== base.pinned) patch.pinned = form.pinned
  const tags = parseTags(form.tags)
  if (tags.join(',') !== parseTags(base.tags).join(',')) patch.tagSlugs = tags
  return patch
}

export function PostEdit() {
  const { id } = useParams()
  const isNew = id === undefined
  const navigate = useNavigate()
  const location = useLocation()

  // 剛建立完跳過來時，POST 的回應已經是完整物件，不用再 GET 一次
  const passed = (location.state as { post?: PostDetail } | null)?.post
  const initial = passed && passed.id === id ? passed : null

  const [post, setPost] = useState<PostDetail | null>(initial)
  const [form, setForm] = useState<Form>(initial ? toForm(initial) : EMPTY)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [saveError, setSaveError] = useState<unknown>(null)
  const [busy, setBusy] = useState<null | 'save' | 'publish' | 'delete'>(null)
  const [savedAt, setSavedAt] = useState<Date | null>(null)
  const deletedRef = useRef(false) // 刪除後跳回列表，不要再問「確定要離開嗎」

  const load = useCallback(async () => {
    if (isNew || initial) return
    setLoadError(null)
    try {
      const p = await getPost(id)
      setPost(p)
      setForm(toForm(p))
    } catch (e) {
      setLoadError(e)
    }
  }, [id, isNew, initial])

  useEffect(() => { void load() }, [load])

  const base = useMemo(() => (post ? toForm(post) : EMPTY), [post])
  const patch = useMemo(() => diff(form, base), [form, base])
  const dirty = Object.keys(patch).length > 0
  const canSave = form.title.trim() !== '' && (isNew || dirty) && busy === null

  const set = <K extends keyof Form>(key: K, value: Form[K]) => {
    setForm((f) => ({ ...f, [key]: value }))
    setSaveError(null)
  }

  const save = useCallback(async (): Promise<PostDetail | null> => {
    if (form.title.trim() === '') return null
    setBusy('save')
    setSaveError(null)
    try {
      if (isNew) {
        const created = await createPost({
          title: form.title.trim(),
          kind: form.kind,
          slug: form.slug.trim(),
          tagSlugs: parseTags(form.tags),
          summary: form.summary,
          markdown: form.markdown,
          pinned: form.pinned,
        })
        setPost(created)
        setForm(toForm(created))
        setSavedAt(new Date())
        navigate(`/posts/${created.id}`, { replace: true, state: { post: created } })
        return created
      }
      if (!post || !dirty) return post
      const updated = await updatePost(post.id, patch)
      setPost(updated)
      setForm(toForm(updated))
      setSavedAt(new Date())
      return updated
    } catch (e) {
      setSaveError(e)
      return null
    } finally {
      setBusy(null)
    }
  }, [form, isNew, post, dirty, patch, navigate])

  const togglePublish = async () => {
    if (!post) return
    // 有沒存的修改就先存，不然發布出去的是舊版本
    const current = dirty ? await save() : post
    if (!current) return
    setBusy('publish')
    setSaveError(null)
    try {
      const updated = await setPublished(current.id, current.status !== 'published')
      setPost(updated)
      setForm(toForm(updated))
      setSavedAt(new Date())
    } catch (e) {
      setSaveError(e)
    } finally {
      setBusy(null)
    }
  }

  const remove = async () => {
    if (!post) return
    if (!window.confirm(`刪除「${post.title}」？\n\n這個動作沒辦法復原。`)) return
    setBusy('delete')
    try {
      await deletePost(post.id)
      deletedRef.current = true
      navigate('/posts', { replace: true })
    } catch (e) {
      setSaveError(e)
      setBusy(null)
    }
  }

  // Cmd/Ctrl + S 存檔，瀏覽器預設的「另存網頁」在這裡沒有意義
  const saveRef = useRef(save)
  saveRef.current = save
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
        e.preventDefault()
        void saveRef.current()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  // 有沒存的修改時，離開前問一次：關分頁、重新整理用 beforeunload，站內跳頁用 blocker
  const unsaved = (isNew ? form.title !== '' || form.markdown !== '' : dirty) && busy === null
  useEffect(() => {
    if (!unsaved) return
    const onBeforeUnload = (e: BeforeUnloadEvent) => { e.preventDefault() }
    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [unsaved])
  const blocker = useBlocker(({ currentLocation, nextLocation }) =>
    unsaved && !deletedRef.current && currentLocation.pathname !== nextLocation.pathname)
  useEffect(() => {
    if (blocker.state !== 'blocked') return
    if (window.confirm('有還沒儲存的修改，確定要離開嗎？')) blocker.proceed()
    else blocker.reset()
  }, [blocker])

  if (loadError) {
    return (
      <>
        <BackLink />
        <ErrorNotice error={loadError} onRetry={() => void load()} />
      </>
    )
  }
  if (!isNew && !post) {
    return (
      <>
        <BackLink />
        <p className="muted">載入中…</p>
      </>
    )
  }

  const published = post?.status === 'published'
  const slugChanged = !isNew && published && form.slug.trim() !== base.slug

  return (
    <form className="editor" onSubmit={(e) => { e.preventDefault(); void save() }}>
      <BackLink />

      <TitleInput value={form.title} onChange={(v) => set('title', v)} autoFocus={isNew} />

      <div className="fields">
        <label className="field field--wide">
          <span className="field__label">網址</span>
          <span className="slug-input">
            <span className="slug-input__prefix">/posts/</span>
            <input className="input input--mono" value={form.slug} onChange={(e) => set('slug', e.target.value)}
              placeholder={isNew ? '留空會從標題產生' : ''} spellCheck={false} aria-label="網址" />
          </span>
          {slugChanged && <span className="field__hint">已發布的文章改網址，舊網址會 301 轉到新網址。</span>}
        </label>

        <div className="field">
          <span className="field__label" id="kind-label">類型</span>
          <div className="tabs" role="radiogroup" aria-labelledby="kind-label">
            {(['post', 'note'] as const).map((k) => (
              <button key={k} type="button" role="radio" className="tabs__item" aria-checked={form.kind === k}
                aria-pressed={form.kind === k} onClick={() => set('kind', k)}>
                {KIND_LABEL[k]}
              </button>
            ))}
          </div>
        </div>

        <label className="field field--wide">
          <span className="field__label">標籤</span>
          <input className="input input--mono" value={form.tags} onChange={(e) => set('tags', e.target.value)}
            placeholder="go, gcp" spellCheck={false} aria-label="標籤（以逗號分隔）" />
        </label>

        <label className="field field--check">
          <input type="checkbox" checked={form.pinned} onChange={(e) => set('pinned', e.target.checked)}
            aria-label="置頂" />
          <span>置頂</span>
        </label>
      </div>

      <label className="field">
        <span className="field__label">摘要</span>
        <textarea className="input input--area" rows={2} value={form.summary}
          onChange={(e) => set('summary', e.target.value)} placeholder="留空會從內文前 120 字產生"
          aria-label="摘要" />
      </label>

      <label className="field">
        <span className="field__label">內文 <span className="muted">Markdown</span></span>
        <textarea className="input input--area input--code editor__body" value={form.markdown}
          onChange={(e) => set('markdown', e.target.value)} spellCheck={false} aria-label="內文（Markdown）" />
      </label>

      {saveError != null && <ErrorNotice error={saveError} />}

      <div className="editor__bar">
        <p className="editor__state">
          {post && <span className={`status status--${post.status}`}>{STATUS_LABEL[post.status]}</span>}
          {post?.publishedAt && (
            <span>{published ? '發布於' : '上次發布'} {formatDateTime(post.publishedAt)}</span>
          )}
          <span className="muted">
            {busy === 'save' ? '儲存中…'
              : dirty ? '有還沒儲存的修改'
              : savedAt ? `已儲存 ${formatTime(savedAt)}`
              : isNew ? '新文章' : `最後修改 ${formatDateTime(post?.updatedAt ?? null)}`}
          </span>
        </p>
        <div className="editor__actions">
          {post && (
            <button type="button" className="link-btn link-btn--danger" onClick={() => void remove()}
              disabled={busy !== null}>
              {busy === 'delete' ? '刪除中…' : '刪除'}
            </button>
          )}
          {post && (
            <button type="button" className="btn" onClick={() => void togglePublish()} disabled={busy !== null}>
              {busy === 'publish' ? '處理中…' : published ? '退回草稿' : '發布'}
            </button>
          )}
          <button type="submit" className="btn btn--primary" disabled={!canSave}
            aria-keyshortcuts="Meta+S Control+S">
            {isNew ? '建立草稿' : '儲存'}
          </button>
        </div>
      </div>
    </form>
  )
}

// 標題用會自己長高的 textarea：中文長標題在單行 input 裡會被截掉，
// 看不到完整標題就沒辦法判斷它好不好。Enter 不換行，標題本來就只有一行。
function TitleInput({ value, onChange, autoFocus }: {
  value: string
  onChange: (v: string) => void
  autoFocus: boolean
}) {
  const ref = useRef<HTMLTextAreaElement>(null)

  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${el.scrollHeight}px`
  }, [value])

  return (
    <textarea ref={ref} className="editor__title" rows={1} value={value} placeholder="標題" aria-label="標題"
      autoFocus={autoFocus}
      onChange={(e) => onChange(e.target.value.replace(/\n/g, ' '))}
      onKeyDown={(e) => { if (e.key === 'Enter' && !e.nativeEvent.isComposing) e.preventDefault() }} />
  )
}

function BackLink() {
  return <Link className="back-link" to="/posts">← 文章列表</Link>
}
