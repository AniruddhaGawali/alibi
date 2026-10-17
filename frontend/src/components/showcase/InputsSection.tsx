import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import Section from './Section'

export default function InputsSection() {
  return (
    <>
      <Section title="Inputs">
        <div className="w-64 space-y-2">
          <Label htmlFor="demo-input">Text input</Label>
          <Input id="demo-input" placeholder="Type here" />
        </div>
        <div className="w-64 space-y-2">
          <Label htmlFor="demo-invalid">Invalid input</Label>
          <Input id="demo-invalid" aria-invalid defaultValue="Wrong value" />
        </div>
        <div className="w-64 space-y-2">
          <Label htmlFor="demo-textarea">Textarea</Label>
          <Textarea id="demo-textarea" placeholder="Your notes" />
        </div>
        <div className="w-64 space-y-2">
          <Label>Select</Label>
          <Select>
            <SelectTrigger>
              <SelectValue placeholder="Choose suspect" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="a">The Butler</SelectItem>
              <SelectItem value="b">The Heiress</SelectItem>
              <SelectItem value="c">The Gardener</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </Section>
      <Section title="Toggles">
        <div className="flex items-center gap-2">
          <Checkbox id="demo-check" defaultChecked />
          <Label htmlFor="demo-check">Checkbox</Label>
        </div>
        <div className="flex items-center gap-2">
          <Switch id="demo-switch" defaultChecked />
          <Label htmlFor="demo-switch">Switch</Label>
        </div>
        <RadioGroup defaultValue="one" className="flex gap-4">
          <div className="flex items-center gap-2">
            <RadioGroupItem value="one" id="r1" />
            <Label htmlFor="r1">One</Label>
          </div>
          <div className="flex items-center gap-2">
            <RadioGroupItem value="two" id="r2" />
            <Label htmlFor="r2">Two</Label>
          </div>
        </RadioGroup>
      </Section>
    </>
  )
}
