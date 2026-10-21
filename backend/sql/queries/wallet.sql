-- name: GetTokenBalance :one
SELECT token_balance FROM users WHERE id = @id;

-- name: AdjustTokenBalance :one
-- Adds delta (negative to spend). The WHERE clause turns an overdraw into "no
-- rows", so concurrent spends can never push the balance below zero. A grant or
-- refund stops at @cap (the admin-set max_tokens); a spend is never capped, and a grant
-- never lowers a balance that is already above @cap (the admin lowered the cap since).
UPDATE users
SET token_balance = CASE
        WHEN @delta::bigint > 0 THEN LEAST(token_balance + @delta::bigint, GREATEST(@cap::bigint, token_balance))
        ELSE token_balance + @delta::bigint
    END
WHERE id = @id AND token_balance + @delta::bigint >= 0
RETURNING token_balance;

-- name: ResetAllTokenBalances :execrows
-- The daily reset: every balance becomes exactly @tokens. Unused tokens do not
-- roll over, and bonus tokens above the daily amount are not kept either.
UPDATE users SET token_balance = @tokens::bigint WHERE token_balance <> @tokens::bigint;
