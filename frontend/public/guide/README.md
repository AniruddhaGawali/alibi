# Handbook screenshots

Shown on `/how-to-play` (`components/rules/Walkthrough.tsx`, `ModesSection.tsx`) from the asset
bucket at `guide/<file>`. Until a file is uploaded the page shows a placeholder with its description.

To add or replace one: save it here as WebP (`cwebp -q 82 in.png -o 01-dashboard.webp`), run
`cd backend && make assets-upload`, then delete the local copy. 16:9, about 1600x900, desktop
width, dark theme, in investigator mode.

| File | What it shows |
|---|---|
| `01-dashboard.webp` | Dashboard: stats on top, case cards with difficulty, reward and Start |
| `02-case-file.webp` | Case page: title, scene photos and the case story |
| `03-suspects.webp` | Suspect cards with first statement and the Public knowledge block |
| `04-chat-open.webp` | Interrogation panel open: suspect picker, token counter, first statement, question box |
| `05-chat-answer.webp` | A question with the suspect's answer, detective tip under the box |
| `06-log-notes.webp` | Interrogation log with Add note on an entry, and the notes |
| `07-hint.webp` | Hint dialog asking to confirm the next hint and its trophy cost |
| `08-accuse.webp` | Accusation dialog: one suspect selected in red, Submit Deduction button |
| `09-verdict.webp` | Closed case: SOLVED stamp, trophy change and resolution |
| `10-settings-mode.webp` | Profile menu → Settings submenu and the difficulty mode dialog |
