import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { Field } from '../components/Field'
import { requestOtp, verifyOtp, ApiError } from '../api/client'
import { useAuth } from './useAuth'

type Stage = 'email' | 'code'

const GENERIC_ERROR = 'Что-то пошло не так, попробуйте ещё раз'

export function LoginPage() {
  const [stage, setStage] = useState<Stage>('email')
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const { setSession } = useAuth()
  const navigate = useNavigate()

  async function handleRequestCode(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await requestOtp(email)
      setStage('code')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : GENERIC_ERROR)
    } finally {
      setSubmitting(false)
    }
  }

  async function handleVerifyCode(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await verifyOtp(email, code)
      setSession({ email })
      navigate('/')
    } catch (err) {
      setCode('')
      setError(err instanceof ApiError ? err.message : GENERIC_ERROR)
    } finally {
      setSubmitting(false)
    }
  }

  if (stage === 'code') {
    return (
      <Card kicker="Вход">
        <h3>Введите код</h3>
        <p>
          Отправили 6-значный код на <strong>{email}</strong>.
        </p>
        <form onSubmit={handleVerifyCode}>
          <Field
            id="code"
            label="Код из письма"
            value={code}
            onChange={(event) => setCode(event.target.value)}
            placeholder="000000"
          />
          {error ? <p className="error-text">{error}</p> : null}
          <Button type="submit" block disabled={submitting}>
            Подтвердить
          </Button>
          <Button type="button" variant="ghost" block onClick={() => setStage('email')}>
            Изменить почту
          </Button>
        </form>
      </Card>
    )
  }

  return (
    <Card kicker="Вход">
      <h3>Введите почту</h3>
      <p>Пришлём одноразовый код подтверждения — пароль не нужен.</p>
      <form onSubmit={handleRequestCode}>
        <Field
          id="email"
          label="Email"
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          placeholder="you@example.com"
        />
        {error ? <p className="error-text">{error}</p> : null}
        <Button type="submit" block disabled={submitting}>
          Получить код
        </Button>
      </form>
    </Card>
  )
}
