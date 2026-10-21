# Generating a case with AI

1. Copy the prompt below into your AI chat and paste the contents of `case_template.json` where it says so.
2. Check the answer against the rules (or ask the AI to check itself), then enter it in the admin
   case builder (Admin → New case). Fields map one to one to the builder's fields.
3. Images: the `*_image_prompt` fields are not saved anywhere. Generate the pictures with them, then
   upload them in the builder (thumbnail 16:9, scenes landscape, suspect avatars square). The builder
   converts them to WebP.
4. Save as draft, read through it, then publish (needs at least 2 suspects and exactly one culprit).

## Prompt

```
You are writing a murder-mystery case for "Alibi", a detective game. The player reads a case
file, interrogates AI-played suspects (each question costs a token from a limited daily allowance), then accuses
one suspect. Fill in the JSON template below and answer with valid JSON only, no comments.

Rules:
- Exactly ONE suspect has "is_culprit": true. Use 4 to 7 suspects (at least 2).
- The case must be fair: the main_plot, public knowledge and testimonies together contain the
  clues needed to find the culprit through questioning. No twist that the player could not reach.
- main_plot, synopsis, public_knowledge, initial_testimony, persona, public_alibi, hints and
  chat_tips are seen by players: they must never name or clearly expose the culprit.
- public_knowledge is common knowledge about the suspect (job, reputation, debts everyone knows of).
  The suspect never denies it and every other suspect knows it. It is never a secret.
- Every suspect has a hidden_truth (innocent ones hide something unrelated or partly related, so
  everyone looks suspicious). The culprit's hidden_truth explains the crime fully.
- Every suspect with a hidden_truth or confession_triggers needs confession_keywords: the words a
  detective's question must contain for the suspect to start revealing the truth. Each keyword is
  lower-case letters/digits only, 2-40 characters, word starts are fine (e.g. "poison" matches
  "poisoned"); join up to 5 with "+" when all must appear in one question ("last+study").
  Give 3-8 keyword groups per suspect, tied to the concrete clues in the case.
- Keep the clues consistent: times, places, objects and names must match across all fields.
- hints: 3 hints, easiest first, the last one nearly gives the answer away.
- chat_tips: 5 short, general interrogation nudges for this case that never give away the answer.
- difficulty_level easy | medium | hard; reward_trophies 5-30 (higher for harder), penalty_trophies
  about half the reward.
- Respect every max length written in the template.
- Fill the *_image_prompt fields with detailed prompts for an image generator in a dark,
  1920s-1940s noir painting style with warm golden lighting.

Template:
<paste case_template.json here>
```
