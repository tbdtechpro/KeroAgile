import { useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { getToken, clearToken } from './api/client'
import LoginPage from './pages/LoginPage'
import BoardPage from './pages/BoardPage'

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
        <button
          onClick={handleLogout}
          className="text-xs text-white opacity-70 hover:opacity-100 transition-opacity"
        >
          Sign out
        </button>
      </header>

      <BoardPage />
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
