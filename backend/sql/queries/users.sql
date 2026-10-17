-- name: CreateUser :one
INSERT INTO users (username, email, password_hash, token_balance)
VALUES (@username, @email, @password_hash, @token_balance)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower(@email);

-- name: GetUserByID :one
SELECT * FROM users WHERE id = @id;

-- name: CheckAvailability :one
-- Used by signup to reject a taken email or username before sending an OTP.
SELECT
    EXISTS (SELECT 1 FROM users WHERE lower(email) = lower(@email))       AS email_taken,
    EXISTS (SELECT 1 FROM users WHERE lower(username) = lower(@username)) AS username_taken;
