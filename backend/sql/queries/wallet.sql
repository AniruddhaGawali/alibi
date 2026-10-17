-- name: GetTokenBalance :one
SELECT token_balance FROM users WHERE id = @id;

-- name: AdjustTokenBalance :one
-- Adds delta (negative to spend). The WHERE clause turns an overdraw into "no
-- rows", so concurrent spends can never push the balance below zero.
UPDATE users
SET token_balance = token_balance + @delta::int
WHERE id = @id AND token_balance + @delta::int >= 0
RETURNING token_balance;
