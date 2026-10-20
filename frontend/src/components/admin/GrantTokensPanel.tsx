import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { RiCoinsLine, RiRestartLine, RiSearchLine } from '@remixicon/react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { FormInput, FormSelect } from '@/components/form'
import {
  MAX_TOKEN_BALANCE,
  useGrantAdminTokensMutation,
  useResetAdminUserLimitsMutation,
  useSearchAdminUsersQuery,
  type AdminPlayer,
} from '@/api/admin'
import { cn } from '@/lib/utils'

const searchSchema = z.object({ q: z.string().trim().max(100, 'At most 100 characters') })
type SearchValues = z.infer<typeof searchSchema>

const grantSchema = z.object({ amount: z.string().regex(/^([1-9]|10)$/, 'Pick how many tokens') })
type GrantValues = z.infer<typeof grantSchema>

/** Find a player by username or email and give them question tokens now. */
export default function GrantTokensPanel() {
  const [q, setQ] = useState('')
  const [selectedId, setSelectedId] = useState<string>()
  const { data: players, isFetching, error } = useSearchAdminUsersQuery(q)
  const selected = players?.find((p) => p.id === selectedId)
  const resetLimits = useResetAdminUserLimitsMutation({
    onSuccess: (_r, id) => toast.success(`Interrogation limits reset for ${players?.find((p) => p.id === id)?.username ?? 'the player'}`),
    onError: (e) => toast.error(e.message),
  })

  const search = useForm<SearchValues>({ resolver: zodResolver(searchSchema), defaultValues: { q: '' } })

  return (
    <div className="space-y-6">
      <form
        onSubmit={search.handleSubmit((v) => {
          setQ(v.q)
          setSelectedId(undefined)
        })}
        noValidate
        className="flex max-w-xl items-end gap-3"
      >
        <div className="flex-1">
          <FormInput control={search.control} name="q" label="Find a player" placeholder="Username or email" autoComplete="off" />
        </div>
        <Button type="submit" variant="outline" disabled={isFetching}>
          <RiSearchLine className="size-4" aria-hidden />
          Search
        </Button>
      </form>

      {error ? (
        <p className="text-sm text-destructive">{error.message}</p>
      ) : !players ? (
        <Skeleton className="h-32 w-full" />
      ) : players.length === 0 ? (
        <p className="text-sm text-muted-foreground">No player matches “{q}”.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Player</TableHead>
              <TableHead className="text-right">Tokens</TableHead>
              <TableHead className="text-right">Trophies</TableHead>
              <TableHead className="sr-only">Select</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {players.map((p) => (
              <TableRow key={p.id} className={cn(p.id === selectedId && 'bg-muted/40')}>
                <TableCell>
                  <p className="font-medium">
                    {p.username} {p.role === 'admin' && <Badge variant="outline">admin</Badge>}
                  </p>
                  <p className="text-xs text-muted-foreground">{p.email}</p>
                </TableCell>
                <TableCell className="text-right text-gold">
                  {p.token_balance} / {MAX_TOKEN_BALANCE}
                </TableCell>
                <TableCell className="text-right">{p.trophies}</TableCell>
                <TableCell className="text-right">
                  <Button
                    variant={p.id === selectedId ? 'default' : 'ghost'}
                    size="sm"
                    onClick={() => setSelectedId(p.id)}
                    disabled={p.token_balance >= MAX_TOKEN_BALANCE}
                    title={p.token_balance >= MAX_TOKEN_BALANCE ? 'Already holds the maximum' : undefined}
                  >
                    <RiCoinsLine className="size-4" aria-hidden />
                    Give tokens
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => resetLimits.mutate(p.id)}
                    disabled={resetLimits.isPending && resetLimits.variables === p.id}
                    title="Clear today's interrogation limit so they can question suspects again"
                  >
                    <RiRestartLine className="size-4" aria-hidden />
                    Reset limits
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {selected && <GrantForm key={selected.id} player={selected} onDone={() => setSelectedId(undefined)} />}
    </div>
  )
}

function GrantForm({ player, onDone }: { player: AdminPlayer; onDone: () => void }) {
  const room = Math.max(MAX_TOKEN_BALANCE - player.token_balance, 1)
  const { control, handleSubmit } = useForm<GrantValues>({
    resolver: zodResolver(grantSchema),
    defaultValues: { amount: String(room) },
  })
  const grant = useGrantAdminTokensMutation({
    onSuccess: (r) => {
      toast.success(
        r.capped
          ? `Gave ${player.username} ${r.granted} token${r.granted === 1 ? '' : 's'}: they now hold the maximum of ${r.token_balance}`
          : `Gave ${player.username} ${r.granted} token${r.granted === 1 ? '' : 's'}: they now hold ${r.token_balance}`,
      )
      onDone()
    },
    onError: (e) => toast.error(e.message),
  })
  const options = Array.from({ length: room }, (_, i) => ({ value: String(i + 1), label: String(i + 1) }))

  return (
    <form
      onSubmit={handleSubmit((v) => grant.mutate({ id: player.id, amount: Number(v.amount) }))}
      noValidate
      className="max-w-xl space-y-4 border p-5"
    >
      <h3 className="font-heading text-base text-gold">Give tokens to {player.username}</h3>
      <FieldGroup>
        <FormSelect
          control={control}
          name="amount"
          label="Tokens"
          options={options}
          description={`They hold ${player.token_balance}; nobody can hold more than ${MAX_TOKEN_BALANCE}. Granted tokens last until the next daily reset, which sets everyone back to the daily allowance.`}
        />
        <div className="flex gap-3">
          <Button type="submit" disabled={grant.isPending}>
            {grant.isPending ? 'Giving…' : 'Give tokens'}
          </Button>
          <Button type="button" variant="ghost" onClick={onDone}>
            Cancel
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
