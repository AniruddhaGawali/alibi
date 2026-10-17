import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Avatar, AvatarFallback, AvatarGroup } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import Section from './Section'

export default function DisplaySection() {
  return (
    <>
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
      <Section title="Scroll area">
        <ScrollArea className="h-32 w-72 border p-3">
          {Array.from({ length: 12 }, (_, i) => (
            <p key={i} className="py-1 text-sm">
              Clue #{i + 1}
            </p>
          ))}
        </ScrollArea>
      </Section>
    </>
  )
}
