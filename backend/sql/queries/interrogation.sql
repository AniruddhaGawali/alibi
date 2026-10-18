-- name: GetInterrogationCase :one
-- What the interrogation prompt needs from the case, including its model.
SELECT id, title, main_plot, llm_model
FROM stories
WHERE id = @id AND is_published;

-- name: ListInterrogationSuspects :many
-- Server only: includes the hidden truth and the culprit flag that go into the
-- suspect's own system prompt. Never return these rows to a client.
SELECT id, name, role, relationship_to_victim, persona, public_alibi,
       hidden_truth, confession_triggers, is_culprit
FROM suspects
WHERE case_id = @case_id
ORDER BY display_order, name;
