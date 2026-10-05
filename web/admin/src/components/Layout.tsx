import { useState, type ReactNode } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { useUser } from '../auth'
import { MOCK, signOutUser } from '../firebase'

export function Layout() {
  const user = useUser()

  return (
    <div className="shell">
      <header className="topbar">
        <div className="topbar__inner">
          <a className="brand" href="/">
            <span className="brand__mark">~/</span>cindle
            <span className="brand__sub">admin</span>
          </a>

          <nav className="topnav" aria-label="後台導覽">
            <NavLink to="/posts" className="topnav__link">文章</NavLink>
            <NavLink to="/tags" className="topnav__link">標籤</NavLink>
          </nav>

          <div className="topbar__end">
            {MOCK && <span className="mock-flag" title="npm run dev:mock：沒有連 Firebase，也沒有打 API">假資料</span>}
            <span className="topbar__user">{user.email}</span>
            <ThemeToggle />
            <button type="button" className="link-btn" onClick={() => void signOutUser()}>登出</button>
          </div>
        </div>
      </header>

      <main className="page">
        <Outlet />
      </main>
    </div>
  )
}

function ThemeToggle() {
  const [theme, setTheme] = useState(() => document.documentElement.dataset.theme || 'light')
  const next = theme === 'dark' ? 'light' : 'dark'

  const toggle = () => {
    document.documentElement.dataset.theme = next
    try { localStorage.setItem('theme', next) } catch { /* 無痕模式寫不進去就算了 */ }
    setTheme(next)
  }

  return (
    <button type="button" className="link-btn" onClick={toggle}>
      {next === 'dark' ? '深色' : '淺色'}
    </button>
  )
}

export function PageHead({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="page-head">
      <h1 className="page-head__title">{title}</h1>
      {children && <div className="page-head__actions">{children}</div>}
    </div>
  )
}
