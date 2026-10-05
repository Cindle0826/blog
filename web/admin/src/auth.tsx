import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { watchUser, type SessionUser } from './firebase'

type AuthState =
  | { status: 'loading' }
  | { status: 'signed-out' }
  | { status: 'signed-in'; user: SessionUser }

const AuthContext = createContext<AuthState>({ status: 'loading' })

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' })

  useEffect(() => watchUser((user) => {
    setState(user ? { status: 'signed-in', user } : { status: 'signed-out' })
  }), [])

  return <AuthContext.Provider value={state}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  return useContext(AuthContext)
}

/** 只在已登入的畫面裡使用 */
export function useUser(): SessionUser {
  const state = useAuth()
  if (state.status !== 'signed-in') throw new Error('useUser 只能在登入後的畫面使用')
  return state.user
}
