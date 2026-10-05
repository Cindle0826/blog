// npm run dev:mock 用的假後端。行為照 docs/API.md 寫，讓後台不用等 Go 的
// handler 寫完就能開發和檢查畫面——跟 cmd/preview 對公開站的作用一樣。
//
// 只在開發模式被動態 import，正式 build 不會包含這個檔案。

import { ApiError, type PostDetail, type PostInput, type PostPage, type Tag } from './api'

const now = Date.now()
const iso = (daysAgo: number) => new Date(now - daysAgo * 86_400_000).toISOString()

let seq = 100
const posts: PostDetail[] = [
  post('aBc123XyZ9kLmN0pQrSt', 'cloud-run-cold-start', 'post', 'published',
    '把 Cloud Run 冷啟動從 1.8s 壓到 240ms', ['gcp', 'go'], true, 18, 2),
  post('dEf456UvW1xYz2AbC3dE', 'go-html-template-ssr', 'post', 'published',
    '用 html/template 做 SSR，不需要 Next.js', ['go', 'seo'], false, 12, 12),
  post('gHi789RsT4uVw5XyZ6aB', 'firestore-pagination', 'post', 'draft',
    'Firestore 分頁：Offset 與游標的取捨', ['gcp'], false, null, 0),
  post('jKl012OpQ7rSt8UvW9xY', 'bash-3-2-unbound-variable', 'note', 'published',
    'bash 3.2 會把中文字吃進變數名', ['bash'], false, 9, 9),
  post('mNo345LmN0oPq1RsT2uV', 'font-stack-cjk', 'note', 'published',
    '字型堆疊：通用字族一定要放最後', ['css'], false, 11, 11),
  post('pQr678IjK3lMn4OpQ5rS', 'post-tgv8a1Kz', 'note', 'draft',
    '還沒想好標題的筆記', [], false, null, 1),
]

function post(id: string, slug: string, kind: 'post' | 'note', status: 'draft' | 'published',
  title: string, tagSlugs: string[], pinned: boolean, publishedDaysAgo: number | null,
  updatedDaysAgo: number): PostDetail {
  return {
    id, slug, kind, status, title, tagSlugs, pinned,
    summary: '',
    markdown: `## ${title}\n\n這是假資料。`,
    cover: null,
    publishedAt: publishedDaysAgo === null ? null : iso(publishedDaysAgo),
    createdAt: iso(Math.max(publishedDaysAgo ?? 0, updatedDaysAgo) + 1),
    updatedAt: iso(updatedDaysAgo),
  }
}

const tags: Tag[] = [
  { slug: 'go', name: 'Go', count: 2 },
  { slug: 'gcp', name: 'GCP', count: 2 },
  { slug: 'seo', name: 'SEO', count: 1 },
  { slug: 'bash', name: 'bash', count: 1 },
  { slug: 'css', name: 'CSS', count: 1 },
]

const delay = () => new Promise((r) => setTimeout(r, 180 + Math.random() * 220))

const notFound = () => new ApiError(404, 'not_found', '找不到這篇文章')

function summary({ markdown: _m, cover: _c, createdAt: _cr, ...rest }: PostDetail) {
  return rest
}

export async function handle<T>(method: string, path: string, body?: unknown): Promise<T> {
  await delay()
  const url = new URL(path, 'http://mock')
  const parts = url.pathname.split('/').filter(Boolean) // ['api', 'posts', id?, action?]
  const [, resource, id, action] = parts

  if (resource === 'posts' && !id && method === 'GET') {
    const status = url.searchParams.get('status')
    const kind = url.searchParams.get('kind')
    const search = (url.searchParams.get('search') ?? '').trim().toLowerCase()
    const page = Number(url.searchParams.get('page') ?? '1')
    const limit = Number(url.searchParams.get('limit') ?? '50')
    if (!Number.isInteger(page) || page < 1) throw new ApiError(400, 'invalid_request', 'page 必須是正整數')

    const all = posts
      .filter((p) => !status || p.status === status)
      .filter((p) => !kind || p.kind === kind)
      .filter((p) => !search || p.title.toLowerCase().includes(search))
      .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
    const items = all.slice((page - 1) * limit, page * limit).map(summary)
    const res: PostPage = { items, page, totalPages: Math.ceil(all.length / limit), total: all.length }
    return res as T
  }

  if (resource === 'posts' && !id && method === 'POST') {
    const input = body as PostInput
    if (!input.title?.trim()) throw new ApiError(400, 'invalid_request', 'Title 必須填寫')
    const created: PostDetail = {
      ...post(`mock${++seq}AbCdEfGhIjKlMn`, input.slug || `post-mock${seq}`, input.kind ?? 'post',
        'draft', input.title, input.tagSlugs ?? [], input.pinned ?? false, null, 0),
      markdown: input.markdown ?? '',
      summary: input.summary ?? '',
    }
    posts.push(created)
    return created as T
  }

  const found = posts.find((p) => p.id === id)

  if (resource === 'posts' && id && !action) {
    if (!found) throw notFound()
    if (method === 'GET') return found as T
    if (method === 'PATCH') {
      const patch = body as Partial<PostDetail>
      if (patch.slug && posts.some((p) => p.slug === patch.slug && p.id !== id)) {
        throw new ApiError(409, 'slug_conflict', `slug「${patch.slug}」已經被使用`)
      }
      Object.assign(found, patch, { updatedAt: new Date().toISOString() })
      return found as T
    }
    if (method === 'DELETE') {
      posts.splice(posts.indexOf(found), 1)
      return undefined as T
    }
  }

  if (resource === 'posts' && id && action === 'publish' && method === 'POST') {
    if (!found) throw notFound()
    const { publish } = body as { publish: boolean }
    found.status = publish ? 'published' : 'draft'
    if (publish && !found.publishedAt) found.publishedAt = new Date().toISOString()
    found.updatedAt = new Date().toISOString()
    return found as T
  }

  if (resource === 'tags' && !id && method === 'GET') return { items: tags } as T

  if (resource === 'tags' && id && method === 'PUT') {
    const tag = tags.find((t) => t.slug === id)
    if (!tag) throw new ApiError(404, 'not_found', '找不到這個標籤')
    tag.name = (body as { name: string }).name
    return tag as T
  }

  throw new ApiError(405, 'not_implemented', `假後端沒有 ${method} ${url.pathname}`)
}
