-- Development-only test data: crime stories for the dashboard cards, fake
-- detectives for the leaderboard.
-- Safe to run repeatedly. Run it with `make seed-dev` (it sets :asset_base); it is NOT a migration and
-- must never run against a real database. The seeded users cannot log in (their
-- password hash is not a valid bcrypt hash).

-- The only dev story for now. Full story data (suspects, culprit) is in
-- docs/stories/case-01-poisoned-earl-grey.md. Any other story is removed.
DELETE FROM active_case_locks
WHERE story_id IN (SELECT id FROM stories WHERE title <> 'The Poisoned Earl Grey');
DELETE FROM stories WHERE title <> 'The Poisoned Earl Grey';

INSERT INTO stories (title, synopsis, thumbnail_url, difficulty_level, reward_trophies)
SELECT v.title, v.synopsis, v.thumbnail_url, v.difficulty_level, v.reward_trophies
FROM (VALUES
    ('The Poisoned Earl Grey',
     'Lord Harrington was found dead in his study at 10:00 AM, a spilled cup of Earl Grey tea staining his oak desk. The coroner confirmed cyanide poisoning. Only four staff members and family had access to the study this morning. It is up to you to interrogate them, cross-reference their alibis, and point the finger at the true killer before your tokens run out.',
     :'asset_base' || '/cases/case-01/thumbnail.webp', 'easy', 1)
) AS v(title, synopsis, thumbnail_url, difficulty_level, reward_trophies)
WHERE NOT EXISTS (SELECT 1 FROM stories s WHERE s.title = v.title);

-- Case #01 briefing: the plot, three scenes and four suspects. Upserted, so
-- re-running updates the text in place and keeps the row ids stable. The
-- testimonies are drafts: edit them here and re-run `make seed-dev`.
UPDATE stories SET
    thumbnail_url = :'asset_base' || '/cases/case-01/thumbnail.webp',
    main_plot = 'Lord Harrington, master of Harrington Manor, was found dead in his study at 10:00 AM. A cup of Earl Grey lay spilled across his oak desk, the stain still spreading when the body was discovered. The coroner has confirmed cyanide poisoning, and the tea is the only thing his lordship consumed that morning.' || E'\n\n' ||
        'His morning tea was served at 9:30 AM, and the study was left undisturbed until the alarm was raised half an hour later. Only four people had access to that part of the house this morning: his wife, his butler, his groundskeeper and his maid. All four were home, and each of them gives an account of where they were.' || E'\n\n' ||
        'Interrogate each of them, compare their stories and find where the timeline does not hold. You have a limited number of questions, so spend them wisely, then name the person who poisoned the tea.'
WHERE title = 'The Poisoned Earl Grey';

INSERT INTO case_scenes (case_id, scene_name, image_url, display_order)
SELECT s.id, v.scene_name, :'asset_base' || v.path, v.display_order
FROM stories s,
     (VALUES
        ('The Study (Crime Scene)', '/cases/case-01/scenes/01-study.webp',   1),
        ('The Kitchen',             '/cases/case-01/scenes/02-kitchen.webp', 2),
        ('The Rose Garden',         '/cases/case-01/scenes/03-garden.webp',  3)
     ) AS v(scene_name, path, display_order)
WHERE s.title = 'The Poisoned Earl Grey'
ON CONFLICT (case_id, display_order) DO UPDATE
    SET scene_name = EXCLUDED.scene_name, image_url = EXCLUDED.image_url;

INSERT INTO suspects (case_id, name, role, avatar_url, initial_testimony, is_culprit, display_order)
SELECT s.id, v.name, v.role, :'asset_base' || v.avatar, v.testimony, v.is_culprit, v.display_order
FROM stories s,
     (VALUES
        ('Lady Eleanor', 'The Wife',          '/cases/case-01/suspects/lady-eleanor.webp',
         'I was in the rose garden all morning, tending the beds. I did not hear a thing until Thomas came running out, white as a sheet. Please, I would rather not talk about my husband''s affairs.',
         false, 1),
        ('Thomas',       'The Butler',        '/cases/case-01/suspects/thomas.webp',
         'I served his lordship his morning tea at 9:30 AM precisely, just as I have done for twenty years. Then I went to polish the silver in the pantry. I never saw anyone near that cup but me, and I would never harm him.',
         false, 2),
        ('Arthur',       'The Groundskeeper', '/cases/case-01/suspects/arthur.webp',
         'I was mending the greenhouse door all morning, ask anyone. Aye, I had no love for the man, he paid me a pittance. But I stay out of the house and I stay out of his study.',
         false, 3),
        ('Beatrice',     'The Maid',          '/cases/case-01/suspects/beatrice.webp',
         'I was sweeping the upstairs corridor, same as every morning. I heard Thomas shouting and came down with the rest. I keep to my work and I do not go poking about in his lordship''s study.',
         true, 4)
     ) AS v(name, role, avatar, testimony, is_culprit, display_order)
WHERE s.title = 'The Poisoned Earl Grey'
ON CONFLICT (case_id, name) DO UPDATE
    SET role = EXCLUDED.role, avatar_url = EXCLUDED.avatar_url, initial_testimony = EXCLUDED.initial_testimony,
        is_culprit = EXCLUDED.is_culprit, display_order = EXCLUDED.display_order;

-- Case #01 interrogation personas (Phase 4.1): server-only prompt data. The
-- confession trigger follows docs/stories/case-01-poisoned-earl-grey.md.
UPDATE suspects AS x SET
    relationship_to_victim = v.relationship,
    persona                = v.persona,
    public_alibi           = v.alibi,
    hidden_truth           = v.truth,
    confession_triggers    = v.triggers,
    confession_keywords    = v.keywords
FROM stories s,
     (VALUES
        ('Lady Eleanor',
         'his wife',
         'Proud, cold and aristocratic. Sharp-tongued and highly defensive; treats the questioning as an insult to the family name.',
         'In the rose garden all morning, tending the beds, until Thomas came running out.',
         'She did not poison the tea. She has run up a massive gambling debt she has hidden from her husband and the police, and she is terrified it will come out and make her look guilty.',
         'If the detective asks about money, debts, cards or why she is so defensive, she grows flustered and finally admits the gambling debt, while insisting it has nothing to do with the murder.',
         ARRAY['money','debt','card','gambl','bankrupt','afford']),
        ('Thomas',
         'his butler for twenty years',
         'Fiercely loyal to the family, formal and precise, easily flustered when his routine is questioned.',
         'Served his lordship his morning tea at 9:30 AM precisely, then polished the silver in the pantry.',
         'He served the tea at 9:30 AM and did not poison it. Around 9:45 AM, on his way back from the pantry, he glimpsed Beatrice leaving the study, which she had no reason to enter; out of loyalty to the household he does not want to accuse a fellow servant.',
         'If the detective presses him on whether anyone else went near the study after he served the tea, he admits, reluctantly, that he saw Beatrice coming out of the study at about 9:45 AM.',
         ARRAY['anyone+study','else+study','near+study','see+study','saw+study','visit+study','enter+study']),
        ('Arthur',
         'his groundskeeper',
         'Gruff, blunt and resentful; makes no secret that he disliked his lordship for underpaying him.',
         'Mending the greenhouse door all morning.',
         'He did not poison the tea and was at the greenhouse all morning. He openly hated Lord Harrington for paying him a pittance, and he keeps rat poison in the shed, which makes him nervous about being blamed.',
         'If the detective asks about poisons, the shed or what he keeps for pests, he admits there is rat poison in the shed but swears it is arsenic, not cyanide, and that the shed stays locked.',
         ARRAY['poison','shed','pest','rat']),
        ('Beatrice',
         'his maid',
         'Quiet, polite and eager to please; keeps her eyes down and her answers short.',
         'Sweeping the upstairs corridor all morning; came down when Thomas shouted.',
         'She poisoned the tea with cyanide at 9:45 AM. Lord Harrington had caught her stealing from the household and was going to have her arrested. She slipped into the study after Thomas served the tea and before the body was found at 10:00 AM.',
         'She fiercely denies everything unless the detective asks who was last in the study, or confronts her with being seen near the study at 9:45 AM. Then she breaks down and confesses that she poisoned the tea because he was going to have her arrested for stealing.',
         ARRAY['last+study','seen+study','near+study','leaving+study','out+study','45+study'])
     ) AS v(name, relationship, persona, alibi, truth, triggers, keywords)
WHERE s.title = 'The Poisoned Earl Grey' AND x.case_id = s.id AND x.name = v.name;

INSERT INTO users (username, email, password_hash, trophies, cases_solved, token_balance)
VALUES
    ('sherlock_h',   'sherlock@seed.alibi.test',  '!', 87, 14, 10),
    ('dr_watson',    'watson@seed.alibi.test',    '!', 64, 11, 10),
    ('hercule_p',    'hercule@seed.alibi.test',   '!', 59, 10, 10),
    ('miss_marple',  'marple@seed.alibi.test',    '!', 48, 8, 10),
    ('father_brown', 'brown@seed.alibi.test',     '!', 41, 7, 10),
    ('nancy_d',      'nancy@seed.alibi.test',     '!', 36, 6, 10),
    ('philip_m',     'marlowe@seed.alibi.test',   '!', 29, 5, 10),
    ('lord_peter',   'wimsey@seed.alibi.test',    '!', 24, 4, 10),
    ('kinsey_m',     'kinsey@seed.alibi.test',    '!', 17, 3, 10),
    ('lisbeth_s',    'lisbeth@seed.alibi.test',   '!', 12, 2, 10),
    ('jessica_f',    'jessica@seed.alibi.test',   '!', 8,  1, 10),
    ('rookie_cop',   'rookie@seed.alibi.test',    '!', 3,  0, 10)
ON CONFLICT (lower(email)) DO UPDATE
    SET trophies = EXCLUDED.trophies, cases_solved = EXCLUDED.cases_solved;

-- Put the real account(s) on the board too, so you can see your own row highlighted,
-- give them a few solved cases so those stats show something. No case is
-- started for them: press Start on the dashboard to take the lock.
UPDATE users SET trophies = 18, cases_solved = 3 WHERE email NOT LIKE '%@seed.alibi.test' AND cases_solved = 0;
