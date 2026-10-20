import type { Control } from 'react-hook-form'
import { FieldGroup } from '@/components/ui/field'
import { FormImagePicker, FormInput, FormSelect, FormTextarea } from '@/components/form'
import AdminSection from './AdminSection'
import ModelPicker from './ModelPicker'
import { DIFFICULTY_OPTIONS, HINT_COUNT, MUSIC_OPTIONS, TIP_COUNT, type CaseFormValues } from './caseSchema'

interface Props {
  control: Control<CaseFormValues>
  disabled?: boolean
}

/** Title, card synopsis, difficulty, trophies, thumbnail and the AI model. */
export function CaseBasicsSection({ control, disabled }: Props) {
  return (
    <AdminSection title="Basic info" description="What players see on the dashboard card.">
      <FieldGroup>
        <div className="grid gap-5 md:grid-cols-[1fr_auto]">
          <div className="space-y-5">
            <FormInput control={control} name="title" label="Title" placeholder="The Poisoned Earl Grey" />
            <FormTextarea control={control} name="synopsis" label="Synopsis" description="Shown on the dashboard card." rows={3} />
          </div>
          <FormImagePicker control={control} name="thumbnail" label="Thumbnail" disabled={disabled} />
        </div>
        <div className="grid gap-5 sm:grid-cols-3">
          <FormSelect control={control} name="difficulty_level" label="Difficulty" options={DIFFICULTY_OPTIONS} />
          <FormInput
            control={control}
            name="reward_trophies"
            label="Reward trophies"
            description="For a correct accusation (hints cost 1, 3, 6 of it)."
            inputMode="numeric"
          />
          <FormInput
            control={control}
            name="penalty_trophies"
            label="Penalty trophies"
            description="Taken for a wrong accusation."
            inputMode="numeric"
          />
        </div>
        <div className="grid gap-5 md:grid-cols-[2fr_1fr]">
          <ModelPicker control={control} disabled={disabled} />
          <FormSelect
            control={control}
            name="music_track"
            label="Case music"
            description="Loops on every page while a player is in this case; the main theme plays otherwise."
            options={MUSIC_OPTIONS}
          />
        </div>
      </FieldGroup>
    </AdminSection>
  )
}

/** The full story, the closing reveal, the progressive hints and the chat tips. */
export function CaseStorySection({ control }: Props) {
  return (
    <AdminSection title="Story & progression">
      <FieldGroup>
        <FormTextarea
          control={control}
          name="main_plot"
          label="Main case file"
          description="The complete story shown in the briefing; every suspect also knows it."
          rows={12}
        />
        <FormTextarea
          control={control}
          name="resolution"
          label="Resolution"
          description="Revealed to the player once they make their accusation."
          rows={5}
        />
        <div className="space-y-5">
          <div>
            <h3 className="font-heading text-base text-gold">Progressive hints</h3>
            <p className="text-sm text-muted-foreground">Easiest first. Each one revealed costs trophies on a win; leave the last ones empty for fewer.</p>
          </div>
          {Array.from({ length: HINT_COUNT }, (_, i) => (
            <FormInput key={i} control={control} name={`hints.${i}`} label={`Hint ${i + 1}`} />
          ))}
        </div>
        <div className="space-y-5">
          <div>
            <h3 className="font-heading text-base text-gold">Detective tips (Investigator mode only)</h3>
            <p className="text-sm text-muted-foreground">
              Free nudges shown under the chat box, one at random per conversation. Every player receives them, so never give away the
              answer. Leave all empty to use the generic tips.
            </p>
          </div>
          {Array.from({ length: TIP_COUNT }, (_, i) => (
            <FormInput
              key={i}
              control={control}
              name={`chat_tips.${i}`}
              label={`Tip ${i + 1}`}
              placeholder={i === 0 ? 'Ask each suspect when they last heard the clock chime.' : undefined}
            />
          ))}
        </div>
      </FieldGroup>
    </AdminSection>
  )
}
