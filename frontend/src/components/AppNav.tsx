import { Link } from 'react-router-dom'
import { Button } from './Button'
import { logout } from '../api/client'
import { useAuth } from '../auth/useAuth'

export function AppNav() {
  const { session, setSession } = useAuth()

  async function handleLogout() {
    await logout()
    setSession(null)
  }

  return (
    <nav className="nav">
      <span className="nav-brand">Нужняшки</span>
      <span>
        {session === 'loading' ? null : session === null ? (
          <Link to="/login">Войти</Link>
        ) : (
          <Button variant="ghost" onClick={handleLogout}>
            Выйти
          </Button>
        )}
      </span>
    </nav>
  )
}
