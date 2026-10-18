-- name: GetInterrogationCase :one
-- What the interrogation prompt needs from the case, including its model.
SELECT id, title, main_plot, llm_model
FROM stories
WHERE id = @id AND is_published;

-- name: GetInterrogationSuspect :one
-- Server only: the one suspect being questioned, with the hidden truth and the
-- culprit flag that go into that suspect's own system prompt. Never return this
-- row to a client, and never load these columns for any other suspect.
SELECT id, name, role, relationship_to_victim, persona, public_alibi,
       hidden_truth, confession_triggers, confession_keywords, is_culprit
FROM suspects
WHERE case_id = @case_id AND id = @id;

-- name: ListInterrogationAcquaintances :many
-- What a suspect may know about the others: public facts only. The secret
-- columns are deliberately not selected, so they cannot reach another prompt.
SELECT name, role, relationship_to_victim, persona
FROM suspects
WHERE case_id = @case_id AND id <> @suspect_id
ORDER BY display_order, name;
