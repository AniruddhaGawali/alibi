-- name: UserIsAdmin :one
SELECT is_admin FROM users WHERE id = @id;

-- name: AdminCreateCase :one
INSERT INTO stories (title, synopsis, main_plot, thumbnail_url, difficulty_level,
                     reward_trophies, is_published, llm_model)
VALUES (@title, @synopsis, @main_plot, @thumbnail_url, @difficulty_level,
        @reward_trophies, @is_published, @llm_model)
RETURNING id, title, synopsis, main_plot, thumbnail_url, difficulty_level,
          reward_trophies, is_published, llm_model, created_at;

-- name: AdminUpdateCase :one
-- Partial update: a NULL parameter keeps the current value.
UPDATE stories SET
    title            = COALESCE(sqlc.narg('title')::text, title),
    synopsis         = COALESCE(sqlc.narg('synopsis')::text, synopsis),
    main_plot        = COALESCE(sqlc.narg('main_plot')::text, main_plot),
    thumbnail_url    = COALESCE(sqlc.narg('thumbnail_url')::text, thumbnail_url),
    difficulty_level = COALESCE(sqlc.narg('difficulty_level')::text, difficulty_level),
    reward_trophies  = COALESCE(sqlc.narg('reward_trophies')::int, reward_trophies),
    is_published     = COALESCE(sqlc.narg('is_published')::boolean, is_published),
    llm_model        = COALESCE(sqlc.narg('llm_model')::text, llm_model)
WHERE id = @id
RETURNING id, title, synopsis, main_plot, thumbnail_url, difficulty_level,
          reward_trophies, is_published, llm_model, created_at;
