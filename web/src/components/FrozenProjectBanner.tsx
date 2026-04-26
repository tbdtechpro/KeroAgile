interface Props {
  projectId: string
  primaryUrl: string
  onDetach: () => void
  onDelete: () => void
}

export function FrozenProjectBanner({ primaryUrl, onDetach, onDelete }: Props) {
  return (
    <div className="bg-amber-50 border border-amber-300 rounded p-3 text-sm space-y-2">
      <p className="font-medium">Sharing of this project has been revoked by the primary.</p>
      <p className="text-gray-600">Your local copy is preserved but no longer syncs with {primaryUrl}.</p>
      <div className="flex gap-3">
        <button className="text-blue-600 underline text-xs" onClick={onDetach}>
          Detach to local (keep as personal project)
        </button>
        <button className="text-red-600 underline text-xs" onClick={onDelete}>
          Delete local copy
        </button>
      </div>
    </div>
  )
}
