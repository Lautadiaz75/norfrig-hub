const ROOTS = ['LAUTARO', 'PABLO_Recursos', 'RESPALDOS']

interface Props {
  root: string
  onRootChange: (root: string) => void
}

export function FilterBar({ root, onRootChange }: Props) {
  return (
    <div className="flex items-center gap-1.5">
      {['', ...ROOTS].map(r => {
        const active = root === r
        return (
          <button
            key={r || 'all'}
            onClick={() => onRootChange(r)}
            className="px-3 py-1 rounded-full text-xs font-medium transition-all"
            style={active
              ? { background: '#0a84ff', color: '#fff' }
              : { background: 'rgba(118,118,128,0.2)', color: 'rgba(235,235,245,0.6)' }
            }
          >
            {r || 'Todas'}
          </button>
        )
      })}
    </div>
  )
}

export function rootDotColor(root: string) {
  switch (root) {
    case 'LAUTARO':        return 'bg-blue-400'
    case 'PABLO_Recursos': return 'bg-purple-400'
    case 'RESPALDOS':      return 'bg-emerald-400'
    default:               return 'bg-zinc-400'
  }
}

export function rootBadgeClass(root: string) {
  switch (root) {
    case 'LAUTARO':        return 'bg-blue-900/50 text-blue-300'
    case 'PABLO_Recursos': return 'bg-purple-900/50 text-purple-300'
    case 'RESPALDOS':      return 'bg-emerald-900/50 text-emerald-300'
    default:               return 'bg-zinc-700/50 text-zinc-300'
  }
}
