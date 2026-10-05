// 契約② 的前端實作。型別與 docs/API.md 一一對應，改契約時兩邊一起改。

import { getIdToken, signOutUser } from './firebase'

export type PostKind = 'post' | 'note'
export type PostStatus = 'draft' | 'published'

export interface Cover {
  url: string
  alt: string
  width: number
  height: number
}

export interface PostSummary {
  id: string
  slug: string
  kind: PostKind
  status: PostStatus
  title: string
  summary: string
  tagSlugs: string[]
  pinned: boolean
  publishedAt: string | null
  updatedAt: string
}

export interface PostDetail extends PostSummary {
  markdown: string
  cover: Cover | null
  createdAt: string
}

export interface PostPage {
  items: PostSummary[]
  page: number
  totalPages: number
  total: number
}

export interface Tag {
  slug: string
  name: string
  count: number
}

export interface PostInput {
  title: string
  kind?: PostKind
  markdown?: string
  slug?: string
  tagSlugs?: string[]
  summary?: string
  cover?: Cover | null
  pinned?: boolean
}

export type PostPatch = Partial<PostInput>

export interface ListParams {
  status?: PostStatus | 'all'
  kind?: PostKind | 'all'
  search?: string
  page?: number
  limit?: number
}

/** 契約的錯誤格式 { error: { code, message } } 解析後的樣子 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

type Method = 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'

async function request<T>(method: Method, path: string, body?: unknown): Promise<T> {
  // 條件要直接寫 import.meta.env，不能用從別的模組 import 進來的 MOCK 常數：
  // Vite 只會把這裡的 import.meta.env.DEV 原地換成 false，bundler 才看得出
  // 這段是死碼，連同 mock.ts 一起丟掉。透過變數的話它看不穿，假資料會被打包出去。
  if (import.meta.env.DEV && import.meta.env.VITE_MOCK_API === '1') {
    const { handle } = await import('./mock')
    return handle<T>(method, path, body)
  }

  const send = async (forceRefresh: boolean) => {
    const token = await getIdToken(forceRefresh)
    return fetch(path, {
      method,
      headers: {
        Authorization: `Bearer ${token}`,
        ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  }

  // ID token 一小時過期。SDK 通常會自己換新，但電腦睡眠醒來時可能拿到剛過期的，
  // 所以 401 時強制換一次再試，還是 401 才真的當作登入失效。
  let res = await send(false)
  if (res.status === 401) res = await send(true)

  if (res.status === 204) return undefined as T

  const data: unknown = await res.json().catch(() => null)
  if (!res.ok) {
    const err = toApiError(res.status, data)
    if (err.code === 'unauthorized') await signOutUser()
    throw err
  }
  return data as T
}

function toApiError(status: number, data: unknown): ApiError {
  const e = (data as { error?: { code?: unknown; message?: unknown } } | null)?.error
  if (e && typeof e.code === 'string' && typeof e.message === 'string') {
    return new ApiError(status, e.code, e.message)
  }
  // 不是契約格式。開發中最常見的原因是那支 handler 還沒寫：Go 的 ServeMux
  // 對沒註冊的路徑回純文字的 404，對方法不符回 405。
  // 502–504：請求沒有到達 Go server。本機最常見的原因是 server 沒開，
  // Vite 的 proxy 連不到 :8080 就回 502、沒有 body。
  if (status >= 502 && status <= 504) {
    return new ApiError(status, 'unavailable',
      import.meta.env.DEV
        ? `連不到後端（${status}）。確認 Go server 有在 :8080 跑。`
        : `連不到後端（${status}），請稍後再試。`)
  }
  if (status === 404 || status === 405) {
    return new ApiError(status, 'not_implemented',
      `這個 API 還沒實作（伺服器回 ${status}，而且不是契約的錯誤格式）`)
  }
  return new ApiError(status, 'internal', `伺服器回 ${status}，而且不是契約的錯誤格式`)
}

// Go 的 nil slice 會被編成 null，契約要求的是 []。這裡多擋一層，
// 畫面才不會因為一篇沒有標籤的文章就整頁壞掉。
function normalize<T extends { tagSlugs: string[] | null }>(p: T): T {
  return { ...p, tagSlugs: p.tagSlugs ?? [] }
}

export async function listPosts(params: ListParams): Promise<PostPage> {
  const qs = new URLSearchParams()
  if (params.status && params.status !== 'all') qs.set('status', params.status)
  if (params.kind && params.kind !== 'all') qs.set('kind', params.kind)
  if (params.search) qs.set('search', params.search)
  if (params.page && params.page > 1) qs.set('page', String(params.page))
  if (params.limit) qs.set('limit', String(params.limit))
  const query = qs.toString()
  const page = await request<PostPage>('GET', `/api/posts${query ? `?${query}` : ''}`)
  return { ...page, items: (page.items ?? []).map(normalize) }
}

export async function getPost(id: string): Promise<PostDetail> {
  return normalize(await request<PostDetail>('GET', `/api/posts/${encodeURIComponent(id)}`))
}

export async function createPost(input: PostInput): Promise<PostDetail> {
  return normalize(await request<PostDetail>('POST', '/api/posts', input))
}

export async function updatePost(id: string, patch: PostPatch): Promise<PostDetail> {
  return normalize(await request<PostDetail>('PATCH', `/api/posts/${encodeURIComponent(id)}`, patch))
}

export async function setPublished(id: string, publish: boolean): Promise<PostDetail> {
  return normalize(await request<PostDetail>('POST', `/api/posts/${encodeURIComponent(id)}/publish`, { publish }))
}

export async function deletePost(id: string): Promise<void> {
  await request<void>('DELETE', `/api/posts/${encodeURIComponent(id)}`)
}

export async function listTags(): Promise<Tag[]> {
  const res = await request<{ items: Tag[] | null }>('GET', '/api/tags')
  return res.items ?? []
}

export async function renameTag(slug: string, name: string): Promise<Tag> {
  return request<Tag>('PUT', `/api/tags/${encodeURIComponent(slug)}`, { name })
}
