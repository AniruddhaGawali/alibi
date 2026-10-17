import type { ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Avatar, AvatarFallback, AvatarGroup } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { FieldGroup } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Popover, PopoverContent, PopoverDescription, PopoverHeader, PopoverTitle, PopoverTrigger } from '@/components/ui/popover'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { FormCheckbox, FormInput, FormRadioGroup, FormSelect, FormSwitch, FormTextarea } from '@/components/form'

const schema = z.object({
  name: z.string().min(2, 'Name must be at least 2 characters'),
  difficulty: z.string().min(1, 'Pick a difficulty'),
  role: z.string().min(1, 'Choose a role'),
  notes: z.string().max(80, 'Keep notes under 80 characters'),
  hints: z.boolean(),
  agree: z.boolean().refine((v) => v, 'You must accept the rules'),
})

type FormValues = z.infer<typeof schema>

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-4">
      <h2 className="text-xl text-primary">{title}</h2>
      <Separator />
      <div className="flex flex-wrap items-start gap-4">{children}</div>
    </section>
  )
}

export default function HomePage() {
  const { control, handleSubmit } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: '', difficulty: '', role: '', notes: '', hints: true, agree: false },
  })

  return (
    <div className="space-y-10">
      <div>
        <h1 className="text-4xl text-primary">Alibi</h1>
        <p className="text-muted-foreground">Interrogate the suspects. Find the culprit. — UI showcase</p>
      </div>

      <Section title="Buttons">
        <Button>Primary</Button>
        <Button variant="secondary">Secondary</Button>
        <Button variant="outline">Outline</Button>
        <Button variant="ghost">Ghost</Button>
        <Button variant="destructive">Destructive</Button>
        <Button variant="link">Link</Button>
        <Button size="sm">Small</Button>
        <Button size="lg">Large</Button>
        <Button disabled>Disabled</Button>
      </Section>

      <Section title="Badges">
        <Badge>Default</Badge>
        <Badge variant="secondary">Secondary</Badge>
        <Badge variant="outline">Outline</Badge>
        <Badge variant="destructive">Destructive</Badge>
      </Section>

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

      <Section title="Cards">
        <Card className="w-80">
          <CardHeader>
            <CardTitle>The Manor Murder</CardTitle>
            <CardDescription>A lord is found dead in his study.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex gap-2">
              <Badge variant="outline">Hard</Badge>
              <Badge>+3 trophies</Badge>
            </div>
          </CardContent>
          <CardFooter>
            <Button>Start case</Button>
          </CardFooter>
        </Card>
        <Card className="w-80">
          <CardHeader>
            <Skeleton className="h-5 w-40" />
            <Skeleton className="h-4 w-56" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-16 w-full" />
          </CardContent>
        </Card>
      </Section>

      <Section title="Alerts">
        <Alert className="max-w-md">
          <AlertTitle>New clue</AlertTitle>
          <AlertDescription>A torn letter was found near the fireplace.</AlertDescription>
        </Alert>
        <Alert variant="destructive" className="max-w-md">
          <AlertTitle>Out of tokens</AlertTitle>
          <AlertDescription>Your questions refresh tomorrow.</AlertDescription>
        </Alert>
      </Section>

      <Section title="Avatars">
        <AvatarGroup>
          <Avatar>
            <AvatarFallback>AB</AvatarFallback>
          </Avatar>
          <Avatar>
            <AvatarFallback>CD</AvatarFallback>
          </Avatar>
          <Avatar>
            <AvatarFallback>EF</AvatarFallback>
          </Avatar>
        </AvatarGroup>
      </Section>

      <Section title="Tabs">
        <Tabs defaultValue="testimony" className="w-full max-w-lg">
          <TabsList>
            <TabsTrigger value="testimony">Testimony</TabsTrigger>
            <TabsTrigger value="chat">Chat</TabsTrigger>
            <TabsTrigger value="notes">Notes</TabsTrigger>
          </TabsList>
          <TabsContent value="testimony">"I was in the garden all evening."</TabsContent>
          <TabsContent value="chat">No messages yet.</TabsContent>
          <TabsContent value="notes">Nothing noted.</TabsContent>
        </Tabs>
      </Section>

      <Section title="Overlays">
        <Dialog>
          <DialogTrigger asChild>
            <Button variant="outline">Dialog</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Case file</DialogTitle>
              <DialogDescription>Details of the investigation.</DialogDescription>
            </DialogHeader>
          </DialogContent>
        </Dialog>
        <Sheet>
          <SheetTrigger asChild>
            <Button variant="outline">Sheet</Button>
          </SheetTrigger>
          <SheetContent>
            <SheetHeader>
              <SheetTitle>Evidence board</SheetTitle>
              <SheetDescription>Everything collected so far.</SheetDescription>
            </SheetHeader>
          </SheetContent>
        </Sheet>
        <Popover>
          <PopoverTrigger asChild>
            <Button variant="outline">Popover</Button>
          </PopoverTrigger>
          <PopoverContent>
            <PopoverHeader>
              <PopoverTitle>Hint</PopoverTitle>
              <PopoverDescription>Check the alibi timeline.</PopoverDescription>
            </PopoverHeader>
          </PopoverContent>
        </Popover>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline">Dropdown</Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuLabel>Detective</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem>Profile</DropdownMenuItem>
            <DropdownMenuItem>Log out</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="outline">Tooltip</Button>
          </TooltipTrigger>
          <TooltipContent>Hover text</TooltipContent>
        </Tooltip>
        <Button variant="outline" onClick={() => toast('Clue discovered', { description: 'A bloody glove.' })}>
          Toast
        </Button>
      </Section>

      <Section title="Scroll area">
        <ScrollArea className="h-32 w-72 border p-3">
          {Array.from({ length: 12 }, (_, i) => (
            <p key={i} className="py-1 text-sm">
              Clue #{i + 1}
            </p>
          ))}
        </ScrollArea>
      </Section>

      <Section title="Form (zod validation — submit empty to see errors)">
        <form className="w-full max-w-md" onSubmit={handleSubmit((v) => toast.success(`Welcome, ${v.name}`))}>
          <FieldGroup>
            <FormInput control={control} name="name" label="Detective name" placeholder="Name" />
            <FormSelect
              control={control}
              name="difficulty"
              label="Difficulty"
              placeholder="Select"
              options={[
                { value: 'easy', label: 'Easy' },
                { value: 'hard', label: 'Hard' },
              ]}
            />
            <FormRadioGroup
              control={control}
              name="role"
              label="Role"
              options={[
                { value: 'inspector', label: 'Inspector' },
                { value: 'journalist', label: 'Journalist' },
              ]}
            />
            <FormTextarea control={control} name="notes" label="Notes" description="Optional, max 80 characters" />
            <FormSwitch control={control} name="hints" label="Enable hints" />
            <FormCheckbox control={control} name="agree" label="I accept the rules" />
            <Button type="submit">Start</Button>
          </FieldGroup>
        </form>
      </Section>
    </div>
  )
}
