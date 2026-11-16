import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Artifact } from '../api/types'

const TEXT_KINDS = new Set(['script', 'log'])
const TEXT_LIMIT = 64 * 1024

function previewable(a: Artifact): boolean {
  const t = a.media_type ?? ''
  return t.startsWith('image/') || t.startsWith('audio/') || t.startsWith('video/') || TEXT_KINDS.has(a.kind) || t.startsWith('text/')
}

// FilePreview shows an image, plays audio or video, or shows the start of a
// text file, inline in the ticket view. It loads through a short-lived
// signed link, so the file is fetched (from the remote store if need be)
// only when asked for.
export default function FilePreview({ file }: { file: Artifact }) {
  const [open, setOpen] = useState(false)
  if (file.where === 'unavailable' || !previewable(file)) return null
  return (
    <div className="mt-1">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className="text-xs opacity-70 hover:opacity-100"
        style={{ color: 'var(--ka-accent-lt)' }}
        aria-expanded={open}
        aria-label={`${open ? 'Hide' : 'View'} ${file.name}`}
      >
        {open ? 'Hide' : 'View'}
      </button>
      {open && <Viewer file={file} />}
    </div>
  )
}

function Viewer({ file }: { file: Artifact }) {
  const [url, setUrl] = useState<string | null>(null)
  const [text, setText] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const type = file.media_type ?? ''
  const isText = !type.startsWith('image/') && !type.startsWith('audio/') && !type.startsWith('video/')

  useEffect(() => {
    let cancelled = false
    api.artifactURL(file.id)
      .then(async ({ url }) => {
        if (cancelled) return
        if (!isText) {
          setUrl(url)
          return
        }
        const res = await fetch(url, { headers: { Range: `bytes=0-${TEXT_LIMIT - 1}` } })
        if (!res.ok) {
          const body = await res.json().catch(() => ({ error: res.statusText }))
          throw new Error(body.error ?? res.statusText)
        }
        const body = await res.text()
        if (!cancelled) setText(file.size > TEXT_LIMIT ? body + '\n… (first 64 KiB; download for the rest)' : body)
      })
      .catch(e => {
        if (!cancelled) setError(e instanceof Error ? e.message : 'could not load the file')
      })
    return () => {
      cancelled = true
    }
  }, [file.id, file.size, isText])

  const failed = (what: string) => () =>
    setError(file.where === 'remote' ? `${what} — the file is stored on the remote, which may not be reachable now` : what)

  if (error) return <p className="text-xs mt-1" style={{ color: 'var(--ka-red)' }}>{error}</p>
  if (isText) {
    if (text === null) return <p className="text-xs mt-1" style={{ color: 'var(--ka-muted)' }}>Loading…</p>
    return (
      <pre
        className="text-xs mt-1 p-2 rounded overflow-auto max-h-64 whitespace-pre-wrap"
        style={{ background: '#0f172a', color: 'var(--ka-text)' }}
      >
        {text}
      </pre>
    )
  }
  if (!url) return <p className="text-xs mt-1" style={{ color: 'var(--ka-muted)' }}>Loading…</p>
  if (type.startsWith('image/'))
    return <img src={url} alt={file.note || file.name} className="mt-1 max-w-full rounded" onError={failed('The image could not be loaded')} />
  if (type.startsWith('audio/'))
    return <audio controls src={url} className="mt-1 w-full" onError={failed('The audio could not be played')} />
  return <video controls src={url} className="mt-1 max-w-full rounded" onError={failed('The video could not be played')} />
}
