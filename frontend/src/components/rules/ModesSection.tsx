import { RiSettings3Line } from '@remixicon/react'
import { PLAYER_MODE_OPTIONS } from '@/components/settings/playerModes'
import GuideScreenshot from './GuideScreenshot'

/** The two difficulty modes, for the handbook page. */
export default function ModesSection() {
  return (
    <>
      <p>
        When you sign up you pick a badge. It decides how much help you get on <strong>every</strong> case; the cases, tokens and
        trophies are the same in both.
      </p>
      <div className="grid gap-4 sm:grid-cols-2">
        {PLAYER_MODE_OPTIONS.map((m) => (
          <div key={m.value} className="space-y-2 border bg-background/40 p-4">
            <p className="flex items-center gap-2 font-heading text-lg text-gold">
              {m.icon && <m.icon className="size-5" aria-hidden />}
              {m.label}
            </p>
            <p className="text-sm">{m.description}</p>
          </div>
        ))}
      </div>
      <ul className="list-disc space-y-2 pl-5">
        <li>
          <strong>Investigator</strong> shows each suspect’s public knowledge on their card and a detective tip under the chat box.
        </li>
        <li>
          <strong>Master Sleuth</strong> shows only the first statements. Everything else you find out by asking.
        </li>
      </ul>
      <p className="flex items-start gap-2">
        <RiSettings3Line className="mt-0.5 size-5 shrink-0 text-gold" aria-hidden />
        <span>
          Change it any time: open your profile menu (top right), then <strong>Settings → Change difficulty mode</strong>.
        </span>
      </p>
      <GuideScreenshot
        file="10-settings-mode.webp"
        alt="The profile menu’s Settings submenu and the difficulty mode dialog with the two badges"
        caption="Profile menu → Settings → Change difficulty mode."
      />
    </>
  )
}
