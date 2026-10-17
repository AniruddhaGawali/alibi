import ActionsSection from '@/components/showcase/ActionsSection'
import DisplaySection from '@/components/showcase/DisplaySection'
import FormSection from '@/components/showcase/FormSection'
import InputsSection from '@/components/showcase/InputsSection'
import OverlaysSection from '@/components/showcase/OverlaysSection'

export default function HomePage() {
  return (
    <div className="space-y-10">
      <div>
        <h1 className="text-4xl text-primary">Alibi</h1>
        <p className="text-muted-foreground">Interrogate the suspects. Find the culprit. — UI showcase</p>
      </div>
      <ActionsSection />
      <InputsSection />
      <DisplaySection />
      <OverlaysSection />
      <FormSection />
    </div>
  )
}
