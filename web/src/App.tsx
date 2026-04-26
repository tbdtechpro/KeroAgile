import { useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { getToken, clearToken } from './api/client'
import LoginPage from './pages/LoginPage'
import BoardPage from './pages/BoardPage'
import SyncSettingsPage from './pages/SyncSettingsPage'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
    },
  },
})

function AppShell() {
  const [authed, setAuthed] = useState(() => !!getToken())
  const [page, setPage] = useState<'board' | 'sync'>('board')

  function handleLogin() {
    setAuthed(true)
  }

  function handleLogout() {
    clearToken()
    queryClient.clear()
    setAuthed(false)
  }

  if (!authed) {
    return <LoginPage onLogin={handleLogin} />
  }

  return (
    <div className="flex flex-col h-screen">
      <header
        className="flex items-center justify-between px-4 shrink-0"
        style={{ background: 'var(--ka-accent)', height: '48px' }}
      >
        <span className="font-bold text-white">⬡ KeroAgile</span>
        <nav className="flex items-center gap-4">
          <button
            onClick={() => setPage('board')}
            className={`text-xs transition-opacity ${page === 'board' ? 'text-white' : 'text-white opacity-60 hover:opacity-100'}`}
          >
            Board
          </button>
          <button
            onClick={() => setPage('sync')}
            className={`text-xs transition-opacity ${page === 'sync' ? 'text-white' : 'text-white opacity-60 hover:opacity-100'}`}
          >
            Sync
          </button>
          <button
            onClick={handleLogout}
            className="text-xs text-white opacity-70 hover:opacity-100 transition-opacity"
          >
            Sign out
          </button>
        </nav>
      </header>

      {page === 'board' ? <BoardPage /> : <SyncSettingsPage />}
    </div>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AppShell />
    </QueryClientProvider>
  )
}
