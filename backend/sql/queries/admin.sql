-- name: UserRole :one
SELECT role FROM users WHERE id = @id;

-- name: AdminCreateCase :one
INSERT INTO stories (title, synopsis, main_plot, thumbnail_url, difficulty_level,
                     reward_trophies, penalty_trophies, resolution, hints, is_published, llm_model, music_track)
VALUES (@title, @synopsis, @main_plot, @thumbnail_url, @difficulty_level,
        @reward_trophies, @penalty_trophies, @resolution, @hints::text[], @is_published, @llm_model, @music_track)
RETURNING id, title, synopsis, main_plot, thumbnail_url, difficulty_level,
          reward_trophies, penalty_trophies, resolution, hints, is_published, llm_model, music_track, created_at;

-- name: AdminUpdateCase :one
-- Partial update: a NULL parameter keeps the current value. old_thumbnail_url is the
-- value before the update, so a replaced image can be deleted from storage.
WITH old AS (SELECT o.id, o.thumbnail_url FROM stories o WHERE o.id = @id FOR UPDATE)
UPDATE stories s SET
    title            = COALESCE(sqlc.narg('title')::text, s.title),
    synopsis         = COALESCE(sqlc.narg('synopsis')::text, s.synopsis),
    main_plot        = COALESCE(sqlc.narg('main_plot')::text, s.main_plot),
    thumbnail_url    = COALESCE(sqlc.narg('thumbnail_url')::text, s.thumbnail_url),
    difficulty_level = COALESCE(sqlc.narg('difficulty_level')::text, s.difficulty_level),
    reward_trophies  = COALESCE(sqlc.narg('reward_trophies')::int, s.reward_trophies),
    penalty_trophies = COALESCE(sqlc.narg('penalty_trophies')::int, s.penalty_trophies),
    resolution       = COALESCE(sqlc.narg('resolution')::text, s.resolution),
    hints            = COALESCE(sqlc.narg('hints')::text[], s.hints),
    is_published     = COALESCE(sqlc.narg('is_published')::boolean, s.is_published),
    llm_model        = COALESCE(sqlc.narg('llm_model')::text, s.llm_model),
    music_track      = COALESCE(sqlc.narg('music_track')::text, s.music_track)
FROM old
WHERE s.id = old.id
RETURNING s.id, s.title, s.synopsis, s.main_plot, s.thumbnail_url, s.difficulty_level,
          s.reward_trophies, s.penalty_trophies, s.resolution, s.hints, s.is_published, s.llm_model, s.music_track, s.created_at,
          old.thumbnail_url AS old_thumbnail_url;

-- name: AdminListCases :many
-- Every case, drafts included, newest first, with what the list page needs to show
-- whether it is ready to publish.
SELECT s.id, s.title, s.synopsis, s.thumbnail_url, s.difficulty_level, s.reward_trophies,
       s.penalty_trophies, s.is_published, s.llm_model, s.music_track, s.created_at,
       (SELECT count(*) FROM suspects p WHERE p.case_id = s.id)::int                  AS suspect_count,
       (SELECT count(*) FROM suspects p WHERE p.case_id = s.id AND p.is_culprit)::int AS culprit_count,
       (SELECT count(*) FROM case_results r WHERE r.story_id = s.id)::int             AS plays
FROM stories s
ORDER BY s.created_at DESC, s.id;

-- name: AdminGetCase :one
SELECT id, title, synopsis, main_plot, thumbnail_url, difficulty_level,
       reward_trophies, penalty_trophies, resolution, hints, is_published, llm_model, music_track, created_at
FROM stories
WHERE id = @id;

-- name: AdminCaseReadiness :one
-- What publishing needs: at least two suspects and exactly one culprit.
SELECT (SELECT count(*) FROM suspects p WHERE p.case_id = @id::uuid)::int                AS suspect_count,
       (SELECT count(*) FROM suspects p WHERE p.case_id = @id::uuid AND p.is_culprit)::int AS culprit_count;

-- name: AdminCaseInUse :one
-- A case someone is playing or has played cannot be deleted (unpublish it instead):
-- deleting it would erase those players' results.
SELECT (EXISTS (SELECT 1 FROM active_case_locks l WHERE l.story_id = @id::uuid)
    OR EXISTS (SELECT 1 FROM case_results r WHERE r.story_id = @id::uuid))::boolean AS in_use;

-- name: AdminCaseImageURLs :many
-- Every image a case points at, so they can be removed from storage after a delete.
SELECT thumbnail_url AS url FROM stories WHERE stories.id = @id AND thumbnail_url <> ''
UNION ALL
SELECT avatar_url FROM suspects WHERE suspects.case_id = @id AND avatar_url <> ''
UNION ALL
SELECT image_url FROM case_scenes WHERE case_scenes.case_id = @id AND image_url <> '';

-- name: AdminDeleteCase :execrows
-- Suspects, scenes and notes go with it (ON DELETE CASCADE). An active lock still
-- points at the story without a cascade, so a player who starts the case in the
-- meantime makes this fail with a foreign key violation instead of losing their game.
DELETE FROM stories WHERE id = @id;

-- name: AdminCaseExists :one
SELECT EXISTS (SELECT 1 FROM stories WHERE id = @id) AS found;

-- name: AdminListSuspects :many
-- Admins see everything, secrets included. Never use this for a player response.
SELECT id, case_id, name, role, avatar_url, initial_testimony, is_culprit, display_order,
       relationship_to_victim, persona, public_alibi, hidden_truth, confession_triggers, confession_keywords
FROM suspects
WHERE case_id = @case_id
ORDER BY display_order, name;

-- name: AdminClearCulprit :exec
-- Run before marking a suspect as the culprit, in the same transaction: a case has
-- one culprit (suspects_one_culprit_idx), so the flag moves instead of conflicting.
UPDATE suspects SET is_culprit = false
WHERE case_id = @case_id AND is_culprit AND id <> COALESCE(sqlc.narg('keep_id')::uuid, '00000000-0000-0000-0000-000000000000'::uuid);

-- name: AdminCreateSuspect :one
INSERT INTO suspects (case_id, name, role, avatar_url, initial_testimony, is_culprit, display_order,
                      relationship_to_victim, persona, public_alibi, hidden_truth, confession_triggers, confession_keywords)
VALUES (@case_id, @name, @role, @avatar_url, @initial_testimony, @is_culprit, @display_order,
        @relationship_to_victim, @persona, @public_alibi, @hidden_truth, @confession_triggers, @confession_keywords::text[])
RETURNING id, case_id, name, role, avatar_url, initial_testimony, is_culprit, display_order,
          relationship_to_victim, persona, public_alibi, hidden_truth, confession_triggers, confession_keywords;

-- name: AdminUpdateSuspect :one
-- Partial update like AdminUpdateCase; old_avatar_url is the value before it.
WITH old AS (SELECT o.id, o.avatar_url FROM suspects o WHERE o.id = @id AND o.case_id = @case_id FOR UPDATE)
UPDATE suspects s SET
    name                   = COALESCE(sqlc.narg('name')::text, s.name),
    role                   = COALESCE(sqlc.narg('role')::text, s.role),
    avatar_url             = COALESCE(sqlc.narg('avatar_url')::text, s.avatar_url),
    initial_testimony      = COALESCE(sqlc.narg('initial_testimony')::text, s.initial_testimony),
    is_culprit             = COALESCE(sqlc.narg('is_culprit')::boolean, s.is_culprit),
    display_order          = COALESCE(sqlc.narg('display_order')::int, s.display_order),
    relationship_to_victim = COALESCE(sqlc.narg('relationship_to_victim')::text, s.relationship_to_victim),
    persona                = COALESCE(sqlc.narg('persona')::text, s.persona),
    public_alibi           = COALESCE(sqlc.narg('public_alibi')::text, s.public_alibi),
    hidden_truth           = COALESCE(sqlc.narg('hidden_truth')::text, s.hidden_truth),
    confession_triggers    = COALESCE(sqlc.narg('confession_triggers')::text, s.confession_triggers),
    confession_keywords    = COALESCE(sqlc.narg('confession_keywords')::text[], s.confession_keywords)
FROM old
WHERE s.id = old.id
RETURNING s.id, s.case_id, s.name, s.role, s.avatar_url, s.initial_testimony, s.is_culprit, s.display_order,
          s.relationship_to_victim, s.persona, s.public_alibi, s.hidden_truth, s.confession_triggers, s.confession_keywords,
          old.avatar_url AS old_avatar_url;

-- name: AdminDeleteSuspect :one
DELETE FROM suspects WHERE id = @id AND case_id = @case_id
RETURNING avatar_url;

-- name: AdminListScenes :many
SELECT id, case_id, scene_name, image_url, display_order
FROM case_scenes
WHERE case_id = @case_id
ORDER BY display_order, id;

-- name: AdminCreateScene :one
INSERT INTO case_scenes (case_id, scene_name, image_url, display_order)
VALUES (@case_id, @scene_name, @image_url, @display_order)
RETURNING id, case_id, scene_name, image_url, display_order;

-- name: AdminUpdateScene :one
WITH old AS (SELECT o.id, o.image_url FROM case_scenes o WHERE o.id = @id AND o.case_id = @case_id FOR UPDATE)
UPDATE case_scenes c SET
    scene_name    = COALESCE(sqlc.narg('scene_name')::text, c.scene_name),
    image_url     = COALESCE(sqlc.narg('image_url')::text, c.image_url),
    display_order = COALESCE(sqlc.narg('display_order')::int, c.display_order)
FROM old
WHERE c.id = old.id
RETURNING c.id, c.case_id, c.scene_name, c.image_url, c.display_order, old.image_url AS old_image_url;

-- name: AdminDeleteScene :one
DELETE FROM case_scenes WHERE id = @id AND case_id = @case_id
RETURNING image_url;

-- name: GetAppSettings :one
SELECT daily_tokens, token_reset_time, token_reset_timezone, updated_at
FROM app_settings
WHERE id;

-- name: SeedAppSettings :exec
-- First startup only: an existing row (set from the admin panel) is never overwritten.
INSERT INTO app_settings (daily_tokens, token_reset_time, token_reset_timezone)
VALUES (@daily_tokens, @token_reset_time, @token_reset_timezone)
ON CONFLICT (id) DO NOTHING;

-- name: UpdateAppSettings :one
UPDATE app_settings SET
    daily_tokens         = @daily_tokens,
    token_reset_time     = @token_reset_time,
    token_reset_timezone = @token_reset_timezone,
    updated_at           = now(),
    updated_by           = @updated_by
WHERE id
RETURNING daily_tokens, token_reset_time, token_reset_timezone, updated_at;

-- name: AdminSearchUsers :many
-- Players for the admin panel, by username or email (@q already has LIKE wildcards
-- escaped by the caller). An empty @q lists the newest accounts.
SELECT id, username, email, role, token_balance, trophies, created_at
FROM users
WHERE @q::text = ''
   OR username ILIKE '%' || @q::text || '%'
   OR email ILIKE '%' || @q::text || '%'
ORDER BY (lower(username) = lower(@q::text)) DESC, created_at DESC
LIMIT @lim::int;

-- name: AdminGetUser :one
SELECT id, username, email, role, token_balance, trophies, created_at
FROM users
WHERE id = @id;
