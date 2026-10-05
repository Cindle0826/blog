import { useState } from 'react'
import { missingConfig, signInWithGoogle } from '../firebase'

// 使用者自己關掉彈窗不算錯誤，不要跳紅字。
const SILENT = new Set(['auth/popup-closed-by-user', 'auth/cancelled-popup-request'])

const MESSAGES: Record<string, string> = {
  'auth/popup-blocked': '瀏覽器擋掉了登入視窗，請允許這個網站開啟彈出式視窗。',
  'auth/unauthorized-domain': '這個網域還沒加進 Firebase 的授權網域（Authentication → 設定 → 授權網域）。',
  'auth/network-request-failed': '連不到 Google，請檢查網路。',
}

export function Login() {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const signIn = async () => {
    setBusy(true)
    setError(null)
    try {
      await signInWithGoogle()
    } catch (e) {
      const code = (e as { code?: string }).code ?? ''
      if (!SILENT.has(code)) setError(MESSAGES[code] ?? `登入失敗（${code || '未知錯誤'}）`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="login">
      <div className="login__box">
        <p className="brand brand--large">
          <span className="brand__mark">~/</span>cindle
          <span className="brand__sub">admin</span>
        </p>

        {missingConfig ? (
          <div className="notice notice--error">
            <p className="notice__message">找不到 Firebase 設定。</p>
            <p className="notice__body">
              <code>web/admin/.env.local</code> 不存在或缺少欄位。在專案根目錄執行
              <code>./infrastruct_as_code/scripts/04-local-env.sh</code> 產生它，然後重新啟動 <code>npm run dev</code>。
            </p>
          </div>
        ) : (
          <>
            <button type="button" className="btn btn--primary btn--block" onClick={() => void signIn()} disabled={busy}>
              {busy ? '等待 Google 回應…' : '用 Google 帳號登入'}
            </button>
            {error && <p className="login__error" role="alert">{error}</p>}
            <p className="login__hint">只有白名單內的帳號可以進入。</p>
          </>
        )}
      </div>
    </main>
  )
}
