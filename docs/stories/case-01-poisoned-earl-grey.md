# Case #01: The Poisoned Earl Grey

- **Difficulty:** Easy (tutorial case)
- **Rewards:** +1 trophy (winning never pays tokens; players hold at most 10, refilled daily)
- **Status:** seeded as the only dev story (`backend/sql/seed/dev_seed.sql`): briefing, scenes, suspects (P3) and the interrogation personas (P4.1: relationship, persona, public alibi, hidden truth, confession triggers). The accusation (P5) is still to come.

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
- Added for the interrogation prompts so the trigger question has a trail to follow (edit in the seed if you want it different):
  - Thomas glimpsed Beatrice leaving the study at about 9:45 AM but protects her out of loyalty; he admits it when pressed on who went near the study after he served the tea.
  - Lady Eleanor admits the gambling debt when asked about money or why she is so defensive.
  - Arthur keeps rat poison (arsenic, not cyanide) in a locked shed and admits it when asked about poisons.

## Hints (P6)

Revealed one at a time with the Hint button; hint *n* costs *n* trophies off the reward if the case is solved (1, 3 or 6 in total, never below a 0 reward). They are in `backend/sql/seed/dev_seed.sql`.

1. Look at the clock: the tea was served at 9:30 and the body found at 10:00, so the poisoner acted inside that half hour.
2. The cyanide went into the cup, so the poisoner was in the study after the tea was served: ask who went near the study afterwards.
3. One suspect claims to have been upstairs all morning yet was seen leaving the study at about 9:45: ask who was last in the study.
