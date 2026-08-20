import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from './useAuth'

export function RequireAuth({ children }: { children: ReactNode }) {
  const { session } = useAuth()
  if (session === 'loading') return null
  if (session === null) return <Navigate to="/login" replace />
  return <>{children}</>
}
