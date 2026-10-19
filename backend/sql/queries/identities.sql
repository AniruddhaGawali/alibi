-- name: GetUserByIdentity :one
SELECT u.* FROM users u
JOIN user_identities i ON i.user_id = u.id
WHERE i.provider = @provider AND i.subject = @subject;

-- name: LinkIdentity :execrows
-- Attaches a provider account to an existing user. Does nothing if the user already
-- has one for this provider or the subject belongs to someone else.
INSERT INTO user_identities (provider, subject, user_id, email)
VALUES (@provider, @subject, @user_id, @email)
ON CONFLICT DO NOTHING;

-- name: CreateUserWithIdentity :one
-- A provider sign-up: the user and the link are created in one statement, so there
-- is never a user without its identity.
WITH new_user AS (
    INSERT INTO users (username, email, password_hash, token_balance)
    VALUES (@username, @email, @password_hash, @token_balance)
    RETURNING *
), link AS (
    INSERT INTO user_identities (provider, subject, user_id, email)
    SELECT @provider::text, @subject::text, id, email FROM new_user
)
SELECT * FROM new_user;
