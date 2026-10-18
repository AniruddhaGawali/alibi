import type { ReactNode } from 'react'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'

interface AuthCardProps {
  title: string
  description?: string
  footer?: ReactNode
  children: ReactNode
}

/** Centered card shared by every auth screen. */
export default function AuthCard({ title, description, footer, children }: AuthCardProps) {
  return (
    <Card className="frame-vintage mx-auto mt-8 w-full max-w-md">
      <CardHeader>
        <CardTitle className="font-heading text-2xl">{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>{children}</CardContent>
      {footer && <CardFooter className="justify-center text-sm text-muted-foreground">{footer}</CardFooter>}
    </Card>
  )
}
