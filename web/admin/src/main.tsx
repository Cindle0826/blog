import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Link, Navigate, Outlet, RouterProvider, createBrowserRouter, useParams } from 'react-router-dom'
import { AuthProvider, useAuth } from './auth'
import { Layout } from './components/Layout'
import { Login } from './pages/Login'
import { PostEdit } from './pages/PostEdit'
import { PostList } from './pages/PostList'
import { Tags } from './pages/Tags'
import './styles.css'

/** 沒登入就只看得到登入頁；登入後的權限（白名單）由後端在每個 API 檢查。 */
function RequireSignIn() {
  const auth = useAuth()
  if (auth.status === 'loading') return null // Firebase 還原登入狀態通常 < 100ms，閃一個載入字反而更吵
  if (auth.status === 'signed-out') return <Login />
  return <Outlet />
}

// 從 /posts/new 存檔跳到 /posts/:id、或從一篇跳到另一篇時，React 會沿用同一個
// 元件實例，舊的表單狀態就會殘留。用 id 當 key 強制重建。
function EditRoute() {
  const { id } = useParams()
  return <PostEdit key={id ?? 'new'} />
}

function NotFound() {
  return (
    <>
      <p className="muted">找不到這個頁面。</p>
      <Link to="/posts">回文章列表</Link>
    </>
  )
}

// data router 才能用 useBlocker（編輯頁離開前的「還沒儲存」確認）
const router = createBrowserRouter([
  {
    element: <RequireSignIn />,
    children: [
      {
        element: <Layout />,
        children: [
          { index: true, element: <Navigate to="/posts" replace /> },
          { path: 'posts', element: <PostList /> },
          { path: 'posts/new', element: <EditRoute /> },
          { path: 'posts/:id', element: <EditRoute /> },
          { path: 'tags', element: <Tags /> },
          { path: '*', element: <NotFound /> },
        ],
      },
    ],
  },
], { basename: '/admin' })

const root = document.getElementById('root')
if (!root) throw new Error('#root 不存在')

createRoot(root).render(
  <StrictMode>
    <AuthProvider>
      <RouterProvider router={router} />
    </AuthProvider>
  </StrictMode>,
)
