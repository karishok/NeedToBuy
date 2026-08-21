import { Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { useAuth } from './auth/useAuth'
import { LoginPage } from './auth/LoginPage'
import { AppNav } from './components/AppNav'
import { CatalogPage } from './catalog/CatalogPage'

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
          <>
            <AppNav />
            <CatalogPage />
          </>
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
