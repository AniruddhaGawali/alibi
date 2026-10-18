# Case #01: The Poisoned Earl Grey

- **Difficulty:** Easy (tutorial case)
- **Rewards:** +1 trophy, +5 bonus chat tokens
- **Status:** the title, synopsis, difficulty and rewards are seeded as the only dev story (`backend/sql/seed/dev_seed.sql`). Suspects and culprit logic below are for later phases (P3 briefing, P4 interrogation prompts, P5 accusation); they have no tables yet.

## Synopsis

Lord Harrington was found dead in his study at 10:00 AM, a spilled cup of Earl Grey tea staining his oak desk. The coroner confirmed cyanide poisoning. Only four staff members and family had access to the study this morning. It is up to you to interrogate them, cross-reference their alibis, and point the finger at the true killer before your tokens run out.

## The suspects (story data)

| Suspect | Role | Claimed alibi | Truth / behaviour |
|---|---|---|---|
| Lady Eleanor | The wife | In the rose garden all morning. | Innocent, but hiding a massive gambling debt from the police, so she acts highly defensive. |
| Thomas | The butler | Polishing silver in the pantry. | Served the tea at 9:30 AM but did not poison it. Fiercely loyal, easily flustered. |
| Arthur | The groundskeeper | Fixing the greenhouse door. | Innocent, but openly hated Lord Harrington for underpaying him. |
| Beatrice | The maid | **THE CULPRIT.** Sweeping the upstairs corridor. | Poisoned the tea at 9:45 AM because Harrington caught her stealing and was going to have her arrested. Fiercely denies everything unless asked who was last in the study. |

## Notes for the LLM prompts (P4)

- Beatrice uses dual-state prompting: deny by default; the trigger question is "who was last in the study?".
- Timeline: 9:30 tea served (Thomas), 9:45 poisoned (Beatrice), 10:00 body found.
