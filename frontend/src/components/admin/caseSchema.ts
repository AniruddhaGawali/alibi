import { z } from 'zod'
import type {
  AdminCaseDetail,
  AdminCaseFields,
  AdminSceneFields,
  AdminSuspectFields,
  Difficulty,
} from '@/api/admin'
import { emptyImage, imageValueSchema } from '@/components/form'
import { CASE_TRACKS } from '@/utils/music'

/** Mirrors the backend's llm.ValidModelID (stories_llm_model_format). */
const MODEL_RE = /^[a-z0-9][a-z0-9._-]*\/[a-z0-9][a-z0-9._-]*(:(free|nitro|floor))?$/
/** Mirrors the backend's "keyword" rule: word starts joined by "+". */
const KEYWORD_RE = /^[a-z0-9]{2,40}(\+[a-z0-9]{2,40}){0,4}$/

export const DEFAULT_MODEL = 'google/gemma-4-31b-it'
export const MIN_SUSPECTS = 2
export const HINT_COUNT = 3
export const TIP_COUNT = 5

/** Form value for "no case track": Radix Select cannot hold '', the API's main-theme value. */
export const MAIN_THEME = 'main'

export const MUSIC_OPTIONS = [
  { value: MAIN_THEME, label: 'Main theme (default)' },
  ...CASE_TRACKS.map((t) => ({ value: t.key, label: t.label })),
]

export const DIFFICULTY_OPTIONS: { value: Difficulty; label: string }[] = [
  { value: 'easy', label: 'Easy' },
  { value: 'medium', label: 'Medium' },
  { value: 'hard', label: 'Hard' },
]

/** A whole number 0-1000 typed into a text field. */
const trophies = z
  .string()
  .trim()
  .regex(/^\d{1,4}$/, 'Enter a whole number')
  .refine((v) => Number(v) <= 1000, 'At most 1000')

/** Keywords are typed comma-separated; blanks are ignored. */
export function splitKeywords(raw: string): string[] {
  return raw
    .split(',')
    .map((k) => k.trim().toLowerCase())
    .filter(Boolean)
}

const suspectSchema = z
  .object({
    /** '' until the suspect has been saved. */
    id: z.string(),
    name: z.string().trim().min(1, 'Enter a name').max(100, 'At most 100 characters'),
    role: z.string().trim().min(1, 'Enter their role, e.g. the butler').max(200, 'At most 200 characters'),
    relationship_to_victim: z.string().trim().max(500, 'At most 500 characters'),
    avatar: imageValueSchema,
    public_knowledge: z.string().trim().max(4000, 'At most 4000 characters'),
    initial_testimony: z.string().trim().min(1, 'Enter what they first tell the detective').max(4000, 'At most 4000 characters'),
    is_culprit: z.boolean(),
    persona: z.string().trim().max(4000, 'At most 4000 characters'),
    public_alibi: z.string().trim().max(4000, 'At most 4000 characters'),
    hidden_truth: z.string().trim().max(6000, 'At most 6000 characters'),
    confession_triggers: z.string().trim().max(4000, 'At most 4000 characters'),
    confession_keywords: z.string().max(1500, 'Too many keywords'),
  })
  .superRefine((s, ctx) => {
    const keywords = splitKeywords(s.confession_keywords)
    const bad = keywords.find((k) => !KEYWORD_RE.test(k))
    if (bad) {
      ctx.addIssue({
        code: 'custom',
        path: ['confession_keywords'],
        message: `"${bad}" is not valid: use lower-case word starts joined by + (e.g. last+study)`,
      })
    } else if (keywords.length > 30) {
      ctx.addIssue({ code: 'custom', path: ['confession_keywords'], message: 'At most 30 keywords' })
    }
    if (s.is_culprit && !s.hidden_truth) {
      ctx.addIssue({ code: 'custom', path: ['hidden_truth'], message: 'The culprit needs a hidden truth: what really happened' })
    }
    if ((s.hidden_truth || s.confession_triggers) && keywords.length === 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['confession_keywords'],
        message: 'Add at least one keyword, or the secret is in the prompt from the first question',
      })
    }
  })

const sceneSchema = z
  .object({
    id: z.string(),
    scene_name: z.string().trim().min(1, 'Name the scene').max(200, 'At most 200 characters'),
    image: imageValueSchema,
    display_order: z.number(),
  })
  .refine((s) => s.image.url || s.image.file, { path: ['image'], message: 'Pick an image for the scene' })

export const caseSchema = z
  .object({
    title: z.string().trim().min(1, 'Enter a title').max(200, 'At most 200 characters'),
    synopsis: z.string().trim().min(1, 'Enter a synopsis for the dashboard card').max(2000, 'At most 2000 characters'),
    difficulty_level: z.enum(['easy', 'medium', 'hard'], 'Pick a difficulty'),
    reward_trophies: trophies,
    penalty_trophies: trophies,
    llm_model: z.string().trim().regex(MODEL_RE, 'Use an OpenRouter model id like google/gemma-4-31b-it'),
    music_track: z.enum([MAIN_THEME, ...CASE_TRACKS.map((t) => t.key)], 'Pick the music'),
    thumbnail: imageValueSchema,
    main_plot: z.string().trim().max(10000, 'At most 10000 characters'),
    resolution: z.string().trim().max(10000, 'At most 10000 characters'),
    hints: z.array(z.string().trim().max(1000, 'At most 1000 characters')).length(HINT_COUNT),
    chat_tips: z.array(z.string().trim().max(300, 'At most 300 characters')).length(TIP_COUNT),
    is_published: z.boolean(),
    suspects: z.array(suspectSchema).max(12, 'At most 12 suspects'),
    scenes: z.array(sceneSchema).max(12, 'At most 12 scenes'),
  })
  .superRefine((c, ctx) => {
    // Hints are revealed in order, so there can be no gap before a filled one.
    c.hints.forEach((h, i) => {
      if (h && i > 0 && !c.hints[i - 1]) {
        ctx.addIssue({ code: 'custom', path: ['hints', i - 1], message: 'Fill the hints in order: this one comes first' })
      }
    })
    const culprits = c.suspects.filter((s) => s.is_culprit).length
    if (culprits > 1) {
      ctx.addIssue({ code: 'custom', path: ['suspects'], message: 'Only one suspect can be the culprit' })
    }
    if (c.is_published && (c.suspects.length < MIN_SUSPECTS || culprits !== 1)) {
      ctx.addIssue({
        code: 'custom',
        path: ['is_published'],
        message: `A published case needs at least ${MIN_SUSPECTS} suspects and exactly one culprit`,
      })
    }
  })

export type CaseFormValues = z.infer<typeof caseSchema>
export type SuspectFormValues = CaseFormValues['suspects'][number]
export type SceneFormValues = CaseFormValues['scenes'][number]

export const emptySuspect = (): SuspectFormValues => ({
  id: '',
  name: '',
  role: '',
  relationship_to_victim: '',
  avatar: emptyImage(),
  public_knowledge: '',
  initial_testimony: '',
  is_culprit: false,
  persona: '',
  public_alibi: '',
  hidden_truth: '',
  confession_triggers: '',
  confession_keywords: '',
})

export const emptyScene = (order: number): SceneFormValues => ({ id: '', scene_name: '', image: emptyImage(), display_order: order })

/** Form values for an existing case, or a blank draft. */
export function toFormValues(c?: AdminCaseDetail): CaseFormValues {
  if (!c) {
    return {
      title: '',
      synopsis: '',
      difficulty_level: 'medium',
      reward_trophies: '10',
      penalty_trophies: '5',
      llm_model: DEFAULT_MODEL,
      music_track: MAIN_THEME,
      thumbnail: emptyImage(),
      main_plot: '',
      resolution: '',
      hints: ['', '', ''],
      chat_tips: Array.from({ length: TIP_COUNT }, () => ''),
      is_published: false,
      suspects: [],
      scenes: [],
    }
  }
  return {
    title: c.title,
    synopsis: c.synopsis,
    difficulty_level: c.difficulty_level,
    reward_trophies: String(c.reward_trophies),
    penalty_trophies: String(c.penalty_trophies),
    llm_model: c.llm_model,
    // A track removed from the list since falls back to the main theme.
    music_track: CASE_TRACKS.some((t) => t.key === c.music_track) ? (c.music_track as CaseFormValues['music_track']) : MAIN_THEME,
    thumbnail: emptyImage(c.thumbnail_url),
    main_plot: c.main_plot,
    resolution: c.resolution,
    hints: Array.from({ length: HINT_COUNT }, (_, i) => c.hints[i] ?? ''),
    chat_tips: Array.from({ length: TIP_COUNT }, (_, i) => c.chat_tips?.[i] ?? ''),
    is_published: c.is_published,
    suspects: c.suspects.map((s) => ({
      id: s.id,
      name: s.name,
      role: s.role,
      relationship_to_victim: s.relationship_to_victim,
      avatar: emptyImage(s.avatar_url),
      public_knowledge: s.public_knowledge,
      initial_testimony: s.initial_testimony,
      is_culprit: s.is_culprit,
      persona: s.persona,
      public_alibi: s.public_alibi,
      hidden_truth: s.hidden_truth,
      confession_triggers: s.confession_triggers,
      confession_keywords: s.confession_keywords.join(', '),
    })),
    scenes: c.scenes.map((s) => ({ id: s.id, scene_name: s.scene_name, image: emptyImage(s.image_url), display_order: s.display_order })),
  }
}

/** The case fields the API takes, without the thumbnail (uploaded first) and publishing (sent last). */
export function caseFields(v: CaseFormValues): Omit<AdminCaseFields, 'thumbnail_url' | 'is_published'> {
  return {
    title: v.title,
    synopsis: v.synopsis,
    main_plot: v.main_plot,
    difficulty_level: v.difficulty_level,
    reward_trophies: Number(v.reward_trophies),
    penalty_trophies: Number(v.penalty_trophies),
    resolution: v.resolution,
    hints: v.hints.filter(Boolean),
    chat_tips: v.chat_tips.filter(Boolean),
    llm_model: v.llm_model,
    music_track: v.music_track === MAIN_THEME ? '' : v.music_track,
  }
}

export function suspectFields(s: SuspectFormValues, avatarUrl: string, order: number): AdminSuspectFields {
  return {
    name: s.name,
    role: s.role,
    avatar_url: avatarUrl,
    initial_testimony: s.initial_testimony,
    public_knowledge: s.public_knowledge,
    is_culprit: s.is_culprit,
    display_order: order,
    relationship_to_victim: s.relationship_to_victim,
    persona: s.persona,
    public_alibi: s.public_alibi,
    hidden_truth: s.hidden_truth,
    confession_triggers: s.confession_triggers,
    confession_keywords: splitKeywords(s.confession_keywords),
  }
}

export function sceneFields(s: SceneFormValues, imageUrl: string): AdminSceneFields {
  return { scene_name: s.scene_name, image_url: imageUrl, display_order: s.display_order }
}

/** The keys of next whose values differ from prev (deep, via JSON). */
export function changedFields<T extends object>(next: T, prev: Partial<T> | undefined): Partial<T> {
  if (!prev) return next
  const out: Partial<T> = {}
  for (const key of Object.keys(next) as (keyof T)[]) {
    // A field the loaded case does not have (cached from before it existed) is not a change while it is still empty.
    const value = next[key] as unknown
    if (prev[key] === undefined && (value === '' || (Array.isArray(value) && value.length === 0))) continue
    if (JSON.stringify(next[key]) !== JSON.stringify(prev[key])) out[key] = next[key]
  }
  return out
}
