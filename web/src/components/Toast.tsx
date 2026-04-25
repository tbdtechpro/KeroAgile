import type { Toast } from '../hooks/useToast'

const BG: Record<Toast['type'], string> = {
  success: '#166534',
  error: '#7f1d1d',
  info: '#1e1b4b',
}

const BORDER: Record<Toast['type'], string> = {
  success: 'var(--ka-green)',
  error: 'var(--ka-red)',
  info: 'var(--ka-accent)',
}

export default function ToastContainer({ toasts, dismiss }: { toasts: Toast[]; dismiss: (id: string) => void }) {
  if (toasts.length === 0) return null
  return (
    <div className="fixed bottom-4 right-4 flex flex-col gap-2 z-50">
      {toasts.map(t => (
        <div
          key={t.id}
          onClick={() => dismiss(t.id)}
          className="px-4 py-3 rounded-lg border text-sm cursor-pointer flex items-center gap-3 shadow-lg"
          style={{ background: BG[t.type], borderColor: BORDER[t.type], color: 'var(--ka-text)' }}
        >
          <span className="flex-1">{t.message}</span>
          <span style={{ color: 'var(--ka-muted)' }}>×</span>
        </div>
      ))}
    </div>
  )
}
