---
name: alibi-ui
description: Conventions for building any UI in the Alibi frontend (frontend/). Use this skill whenever you implement, change, or add a page, component, form, dialog, button, input, layout, or style in the frontend, even for small tweaks. It requires shadcn/ui components (installing any that are missing), the golden-sunset sharp-corner game theme, zod-validated forms from components/form, and the API-layer rules.
---

# Alibi UI conventions

The frontend is Vite + React 19 (React Compiler on) + TypeScript + Tailwind v4 + shadcn/ui, in `frontend/`. These decisions are already made. Follow them rather than re-deciding, so every screen looks and behaves the same.

## 1. Use shadcn components, install what's missing

Build UI from `src/components/ui/` (shadcn). Do not hand-roll buttons, inputs, dialogs, tabs, tooltips and so on.

Before writing UI, check `src/components/ui/` for the component. If it is missing, install it, then use it:

```bash
cd frontend && npx shadcn@latest add <component> -y
```

Never edit files in `src/components/ui/`. They are generated and a re-run of the CLI would overwrite changes. Change appearance through the theme (section 2) or by passing `className` at the call site. Import with the `@/` alias, e.g. `@/components/ui/button`.

## 1b. Icons: Remix Icon only

Every icon in the frontend comes from `@remixicon/react` (`import { RiTrophyLine } from '@remixicon/react'`). Do not use `lucide-react`, inline SVGs or emoji as icons, and do not add another icon package.

- Component names are `Ri` + PascalCase name + `Line` (outline, the default here) or `Fill`, e.g. `RiCoinsLine`, `RiCloseLine`. Find a name on remixicon.com, and check it exists in the package before using it.
- Size with Tailwind (`className="size-4"`), colour with a token (`text-gold`), and add `aria-hidden` when the icon sits next to text.
- `components.json` has `"iconLibrary": "remixicon"`, so `npx shadcn@latest add <component>` generates shadcn components that already import Remix icons. If a generated file imports anything else, fix the import by re-adding it with `--overwrite` rather than hand-editing.

## 2. The theme lives in `src/index.css`

Golden-sunset, dark-only, game-like (think Hogwarts Legacy menus). Change it by editing CSS variables and the rules at the bottom of `src/index.css`, not component files.

- **Colours:** use tokens (`bg-background`, `text-primary`, `text-muted-foreground`, `bg-card`, `border-border`, plus `text-gold` / `text-ember`). Never hard-code hex colours.
- **Sharp corners everywhere:** `--radius` is 0 and a global `[data-slot]` rule squares everything, including radios, switches and avatars. Do not add `rounded-*` classes; they have no effect and signal drift.
- **Headings** use the Cinzel font via `font-heading` (already applied to h1–h4).
- **Glow:** gold glow on default-gold elements (primary button, default badge, checked checkbox/switch/radio, avatar, alerts) and on focused text fields; red glow on destructive buttons/badges/alerts and invalid fields. These use `--glow-gold`, `--glow-red` and their `-strong` variants. Reuse these tokens for new elements, and keep them subtle.
- **Button shine** on hover applies to primary, secondary, outline and destructive buttons only, not ghost, link or disabled.

If a new component needs the look, extend the `[data-slot='...']` selectors in `index.css`.

**Game props** (`src/components/game/`, styles at the bottom of `index.css`): `Tape`, `Stamp`, `Redacted`, `Typewriter` (with the `font-typewriter` Courier Prime utility) and `RedString`; add `className="frame"` to important panels for the gold corner frame (`frame-vintage` is the rococo version for hero panels, `GildedFrame` (components/game/) the carved wavy rococo frame for big pictures, drawn as SVG at the size of its box and overhanging it by ~30px so give it margin; `frame-portrait` the plain gilt moulding for the chat suspect image). `case-folder` + `case-sheet` + `PaperClip` make the aged paper case file. The cursor is a magnifier and buttons show a fingerprint on hover. Keep the golden-sunset palette; don't restyle these.

## 2b. Structure, providers and dependencies

- **Keep pages thin.** A page in `src/pages/` composes feature components; it does not hold hundreds of lines of JSX. Put feature components in `src/components/<feature>/` (e.g. `components/showcase/` holds the UI showcase sections that `HomePage` composes, and `components/layout/` the shared layout). Split a component once it mixes unrelated concerns or grows past roughly 150 lines.
- **Providers live in `App.tsx`:** `ThemeProvider` (next-themes, forced `dark`, which shadcn's `Toaster` reads for its theme), `TooltipProvider`, the router and `Toaster`. `QueryClientProvider` is in `main.tsx`. A new shadcn component that needs a provider (the CLI prints a note) gets it added in `App.tsx`; without it the component can misbehave at runtime even though it compiles.
- **Dependencies stay lean.** Before adding an npm package, check whether shadcn, Radix, Tailwind, zod, react-hook-form, React Router or an existing helper already does the job. Remove packages you stop using. `tailwindcss`, `tw-animate-css`, `shadcn` and the `@fontsource` packages look unused to scanners but are imported from `index.css`, so keep them.
- **Auth token:** the JWT is currently a JS-readable cookie handled only in `src/utils/cookieHelpers.ts` and read only in `apiClient.ts`. Keep it that way (no `localStorage`, no token handling in components) so it can move to an HttpOnly cookie set by the backend without touching UI code.

## 3. Forms: zod + react-hook-form + `components/form`

Never build form fields from raw inputs. Use the wrappers in `src/components/form/` (`FormInput`, `FormTextarea`, `FormSelect`, `FormCheckbox`, `FormSwitch`, `FormRadioGroup`). They take `control` and `name`, and render the label, description and shadcn `FieldError` automatically.

```tsx
const schema = z.object({ name: z.string().min(2, 'Name must be at least 2 characters') })
const { control, handleSubmit } = useForm({ resolver: zodResolver(schema), defaultValues: { name: '' } })
// <FormInput control={control} name="name" label="Name" />
```

Write user-facing error messages in the zod schema. If you need a control that has no wrapper yet, add a new `Form*` component in `src/components/form/` following the existing ones and export it from `index.ts`. Do not modify `ui/` files.

## 4. Data and state

- All HTTP calls live in `src/api/` (via `apiClient.ts`). Components never call `fetch` directly.
- Server state goes through the hooks generated by `createApi` in `src/api/` (e.g. `useGetDashboardQuery()`); components never import React Query directly. See the `alibi-api` skill. Client-only state goes in Zustand (`src/store/`).
- JWT cookie handling is in `src/utils/cookieHelpers.ts`.
- Pages go in `src/pages/`, routes in `src/routes/router.tsx` (React Router), shared layout in `src/components/layout/`.
- Don't add `useMemo`/`useCallback`/`React.memo` for performance; the React Compiler handles it.
- Use toasts (`sonner`) for transient feedback.

## 5. Before finishing

Run `cd frontend && npm run build && npm run lint`. The `ui/` fast-refresh warnings are expected; anything else (unused imports, new warnings) should be fixed. The build must pass. The only expected lint warnings are the stock shadcn "only-export-components" ones in `ui/`.
