// Firebase 只用來做 Google 登入、拿 ID token。資料一律經過 /api，
// 前端不直接碰 Firestore——權限檢查只有一個地方，就是後端的 RequireAdmin。

import { initializeApp } from '@firebase/app'
import {
  GoogleAuthProvider,
  getAuth,
  onAuthStateChanged,
  signInWithPopup,
  signOut,
  type Auth,
  type User,
} from '@firebase/auth'

export const MOCK = import.meta.env.DEV && import.meta.env.VITE_MOCK_API === '1'

const env = import.meta.env
export const missingConfig = !MOCK && !(env.VITE_FIREBASE_BROWSER_KEY && env.VITE_FIREBASE_AUTH_DOMAIN
  && env.VITE_FIREBASE_PROJECT_ID && env.VITE_FIREBASE_APP_ID)

let auth: Auth | null = null

function getAuthInstance(): Auth {
  if (!auth) {
    const app = initializeApp({
      apiKey: env.VITE_FIREBASE_BROWSER_KEY, // SDK 的欄位名就叫 apiKey，這個改不了
      authDomain: env.VITE_FIREBASE_AUTH_DOMAIN,
      projectId: env.VITE_FIREBASE_PROJECT_ID,
      appId: env.VITE_FIREBASE_APP_ID,
    })
    auth = getAuth(app)
    auth.languageCode = 'zh-TW'
  }
  return auth
}

export interface SessionUser {
  uid: string
  email: string
}

const MOCK_USER: SessionUser = { uid: 'mock-uid', email: 'mock@localhost' }

export function watchUser(cb: (user: SessionUser | null) => void): () => void {
  if (MOCK) {
    cb(MOCK_USER)
    return () => {}
  }
  return onAuthStateChanged(getAuthInstance(), (u: User | null) => {
    cb(u ? { uid: u.uid, email: u.email ?? '' } : null)
  })
}

export async function signInWithGoogle(): Promise<void> {
  const provider = new GoogleAuthProvider()
  // 每次都讓使用者選帳號。只有一個 Google 帳號登入著的話，不加這個會直接
  // 用那個帳號登入，想換帳號（例如測 403）就得先去 Google 登出。
  provider.setCustomParameters({ prompt: 'select_account' })
  await signInWithPopup(getAuthInstance(), provider)
}

export async function signOutUser(): Promise<void> {
  if (MOCK) return
  await signOut(getAuthInstance())
}

export async function getIdToken(forceRefresh = false): Promise<string> {
  const user = getAuthInstance().currentUser
  if (!user) throw new Error('尚未登入')
  return user.getIdToken(forceRefresh)
}
