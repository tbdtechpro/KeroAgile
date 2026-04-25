import { useState } from 'react'
import { api, setToken } from '../api/client'

export default function LoginPage({ onLogin }: { onLogin: () => void }) {
  const [userId, setUserId] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      const { token } = await api.login(userId, password)
      setToken(token)
      onLogin()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex items-center justify-center min-h-screen" style={{ background: 'var(--ka-bg)' }}>
      <div className="w-full max-w-sm p-8 rounded-lg border" style={{ borderColor: 'var(--ka-accent)', background: '#0f1629' }}>
        <h1 className="text-2xl font-bold mb-2" style={{ color: 'var(--ka-accent-lt)' }}>⬡ KeroAgile</h1>
        <p className="text-sm mb-6" style={{ color: 'var(--ka-muted)' }}>Sign in to your board</p>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <input
            type="text"
            placeholder="User ID"
            value={userId}
            onChange={e => setUserId(e.target.value)}
            required
            className="w-full px-3 py-2 rounded border text-sm"
            style={{ background: '#1e293b', borderColor: 'var(--ka-muted)', color: 'var(--ka-text)' }}
          />
          <input
            type="password"
            placeholder="Password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            required
            className="w-full px-3 py-2 rounded border text-sm"
            style={{ background: '#1e293b', borderColor: 'var(--ka-muted)', color: 'var(--ka-text)' }}
          />
          {error && <p className="text-sm" style={{ color: 'var(--ka-red)' }}>{error}</p>}
          <button
            type="submit"
            disabled={loading}
            className="w-full py-2 rounded font-semibold text-sm transition-opacity"
            style={{ background: 'var(--ka-accent)', color: 'white', opacity: loading ? 0.6 : 1 }}
          >
            {loading ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  )
}
