export class ApiError extends Error {
  code: string
  status: number

  constructor(code: string, message: string, status: number) {
    super(message)
    this.code = code
    this.status = status
  }
}

interface ErrorEnvelope {
  error?: { code: string; message: string }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })

  if (!response.ok) {
    let envelope: ErrorEnvelope = {}
    try {
      envelope = (await response.json()) as ErrorEnvelope
    } catch {
      // Response body wasn't JSON — fall through to the generic error below.
    }
    throw new ApiError(
      envelope.error?.code ?? 'unknown',
      envelope.error?.message ?? `Request failed with status ${response.status}`,
      response.status,
    )
  }

  return (await response.json()) as T
}

export function requestOtp(email: string): Promise<{ status: string }> {
  return request('/api/auth/otp/request', {
    method: 'POST',
    body: JSON.stringify({ email }),
  })
}

export function verifyOtp(email: string, code: string): Promise<{ status: string }> {
  return request('/api/auth/otp/verify', {
    method: 'POST',
    body: JSON.stringify({ email, code }),
  })
}

export function logout(): Promise<{ status: string }> {
  return request('/api/auth/logout', { method: 'POST' })
}

export function me(): Promise<{ email: string }> {
  return request('/api/auth/me')
}
