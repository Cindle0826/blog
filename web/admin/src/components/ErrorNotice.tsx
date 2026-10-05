import { ApiError } from '../api'
import { signOutUser } from '../firebase'

/** 契約錯誤的 message 本來就是寫給人看的，直接顯示；code 留給除錯。 */
export function ErrorNotice({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const api = error instanceof ApiError ? error : null

  // 403：token 有效但 UID 不在白名單。重試沒有用，唯一能做的是換帳號。
  if (api?.code === 'forbidden') {
    return (
      <div className="notice notice--error" role="alert">
        <p className="notice__message">這個帳號沒有後台權限。</p>
        <p className="notice__meta">
          <code>403 forbidden</code>
          <button type="button" className="link-btn" onClick={() => void signOutUser()}>換一個帳號登入</button>
        </p>
      </div>
    )
  }

  const message = api?.message ?? (error instanceof Error ? error.message : '發生未預期的錯誤')

  return (
    <div className="notice notice--error" role="alert">
      <p className="notice__message">{message}</p>
      <p className="notice__meta">
        {api && <code>{api.status} {api.code}</code>}
        {onRetry && <button type="button" className="link-btn" onClick={onRetry}>重試</button>}
      </p>
    </div>
  )
}
