# Alibi UI TODO (game feel)

Tick items off here when done. Phase work stays in `development_phases_and_todo_list.md`.

## Done
- [x] Magnifying-glass cursor, fingerprint mark on button hover
- [x] Gold `frame` on auth card, leaderboard, stat tiles, in-progress case
- [x] Film grain + vignette
- [x] Case-file props: tape, redacted text, CLASSIFIED stamp, typewriter reveal, red string
- [x] Stat tiles (Tokens, Trophies, Solved), Start / Continue buttons, Locked cards
- [x] Sound effects (synthesised, no files): click, typewriter key, paper rustle, stamp; mute toggle in the navbar

## Still to do
### Look and feel
- [ ] Real case thumbnails / scene art on story cards (needs images, see below)
- [ ] Polaroid / paper-clipped photo style for thumbnails
- [x] CSS textures ready in `index.css`: `.texture-paper`, `.texture-cork`, `.texture-wood`
- [ ] Apply the textures: paper on testimonies / case files, cork behind the story grid and suspect board, wood on the navbar
- [x] Locked-card shake animation when clicked
- [x] Loading screen ("Searching the archives...") with a spinning magnifier, replacing skeletons on first load
- [x] Page transitions (fade / iris wipe)
- [ ] Hover tooltip and sound variety for Start / Continue
- [x] Rank titles (Rookie, Detective, Inspector, Chief) on the leaderboard and avatar
- [ ] Profile card with a `frame` (avatar, username, trophies, solved, rank)
- [ ] Navbar restyle as a detective's desk bar (logo mark, badge-style wallet)
- [x] Themed 404 page ("Case not found")
- [ ] Themed empty states (no cases, no leaderboard entries)
- [ ] Mobile pass: cursor fallback on touch, tap targets, stats row, red string hidden

### Screens that depend on later phases
- [ ] Suspect corkboard with photos and red string between suspects (P3)
- [ ] Case briefing page with scene images (P3)
- [ ] Testimonies and chat in the typewriter font (P3/P4)
- [ ] Chat bubbles styled as typed notes, token counter with a red flash on deduction (P4)
- [ ] SOLVED / GUILTY / WRONG outcome stamp and win/loss animation (P5)

### Sound polish
- [ ] Ambient background loop (low rain / city hum), off by default
- [ ] Success and failure jingles for the outcome screen
- [ ] Per-category volume (effects vs ambient)
- [ ] Replace synthesised sounds with recorded ones if desired (`src/utils/sound.ts`)

## What I need from you
| Need | For | Format / notes |
|---|---|---|
| Story content: title, synopsis, difficulty, rewards, 3-6 scene images per case | Story cards, briefing page | Currently only seed data exists (`backend/sql/seed/`) |
| Thumbnail per story | Story cards | 16:9, about 1280x720, JPG/WebP, under 300 KB |
| Suspect portraits (per case, 4-8) | Corkboard, avatars | Square, 512x512 PNG/WebP, consistent art style |
| Default user avatars | Onboarding (4 placeholders now) | 4 square images, 256x256 |
| Paper / cork / wood textures | Case files, corkboard | Not needed: done in CSS (image files optional, tileable WebP under 200 KB) |
| Logo / favicon | Navbar, browser tab | SVG preferred |
| Ambient loop + jingles (optional) | Sound polish | OGG/MP3, short, royalty-free (tell me the licence) |
| Decision: keep the synthesised sounds or use recorded ones | Sound | Current ones need nothing from you |
| Decision: keep the golden-sunset palette or move to sepia / manila | Theme | Currently golden sunset |
| Decision: red string curve fixed vs linking cards in order | Dashboard | Currently one fixed curve |
| Google Login and bot protection (Turnstile / hCaptcha) | Phase 1 | Skipped on request; will need keys later |
| Brevo credentials + verified sender, DLQ alert | Production email | Needed before deploying |
