-- name: GetTokenBalance :one
SELECT token_balance FROM users WHERE id = @id;

-- name: AdjustTokenBalance :one
-- Adds delta (negative to spend). The WHERE clause turns an overdraw into "no
-- rows", so concurrent spends can never push the balance below zero, and LEAST
-- stops a grant or refund from taking it above @cap.
UPDATE users
SET token_balance = LEAST(token_balance + @delta::bigint, @cap::bigint)
WHERE id = @id AND token_balance + @delta::bigint >= 0
RETURNING token_balance;

-- name: ResetAllTokenBalances :execrows
-- The daily reset: every balance becomes exactly @tokens. Unused tokens do not
-- roll over, and bonus tokens above the daily amount are not kept either.
UPDATE users SET token_balance = @tokens::bigint WHERE token_balance <> @tokens::bigint;
