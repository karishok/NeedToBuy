import { createContext, useEffect, useState, type ReactNode } from 'react'
import { me as fetchMe } from '../api/client'

export interface Session {
  email: string
}

export type SessionState = 'loading' | Session | null

interface AuthContextValue {
  session: SessionState
  setSession: (session: Session | null) => void
}

export const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionState>('loading')

  useEffect(() => {
    fetchMe()
      .then((result) => setSession({ email: result.email }))
      .catch(() => setSession(null))
  }, [])

  return <AuthContext.Provider value={{ session, setSession }}>{children}</AuthContext.Provider>
}
