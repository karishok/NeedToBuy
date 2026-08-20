import { Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { useAuth } from './auth/useAuth'
import { LoginPage } from './auth/LoginPage'
import { PlaceholderHome } from './auth/PlaceholderHome'
import { RequireAuth } from './auth/RequireAuth'

function LoginRoute() {
  const { session } = useAuth()
  if (session === 'loading') return null
  if (session !== null) return <Navigate to="/" replace />
  return <LoginPage />
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginRoute />} />
      <Route
        path="/"
        element={
          <RequireAuth>
            <PlaceholderHome />
          </RequireAuth>
        }
      />
    </Routes>
  )
}

export default function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  )
}
