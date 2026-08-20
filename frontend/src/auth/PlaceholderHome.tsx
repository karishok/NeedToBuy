import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { logout } from '../api/client'
import { useAuth } from './useAuth'

export function PlaceholderHome() {
  const { session, setSession } = useAuth()
  const email = session && session !== 'loading' ? session.email : ''

  async function handleLogout() {
    await logout()
    setSession(null)
  }

  return (
    <Card kicker="Нужняшки">
      <h3>Вы вошли как {email}</h3>
      <p>Вишлист и профиль ребёнка появятся здесь в следующих срезах.</p>
      <Button variant="secondary" onClick={handleLogout}>
        Выйти
      </Button>
    </Card>
  )
}
