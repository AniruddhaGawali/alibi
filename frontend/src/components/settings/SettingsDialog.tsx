import { toast } from 'sonner'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import AvatarForm from '@/components/auth/AvatarForm'
import PlayerModeForm from './PlayerModeForm'
import UsernameForm from './UsernameForm'

export type SettingsPanel = 'username' | 'avatar' | 'mode'

const PANELS: Record<SettingsPanel, { title: string; description: string; saved: string }> = {
  username: { title: 'Change username', description: 'Pick a name nobody else is using.', saved: 'Username updated' },
  avatar: { title: 'Change avatar', description: 'Upload your own image or pick one of the defaults.', saved: 'Avatar updated' },
  mode: { title: 'Difficulty mode', description: 'Switch at any time; it applies to every case at once.', saved: 'Difficulty mode updated' },
}

/** The settings menu's dialogs: one panel at a time, null = closed. */
export default function SettingsDialog({ panel, onClose }: { panel: SettingsPanel | null; onClose: () => void }) {
  const info = panel ? PANELS[panel] : null
  const saved = () => {
    if (info) toast.success(info.saved)
    onClose()
  }
  return (
    <Dialog open={panel !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        {info && (
          <DialogHeader>
            <DialogTitle className="font-heading">{info.title}</DialogTitle>
            <DialogDescription>{info.description}</DialogDescription>
          </DialogHeader>
        )}
        {panel === 'username' && <UsernameForm onSaved={saved} />}
        {panel === 'avatar' && <AvatarForm onSaved={saved} />}
        {panel === 'mode' && <PlayerModeForm onSaved={saved} />}
      </DialogContent>
    </Dialog>
  )
}
