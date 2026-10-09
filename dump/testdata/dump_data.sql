-- ids are deterministic: the tests refer to them.
INSERT INTO status (code) VALUES ('new'), ('open'), ('won'), ('lost');          -- 1..4
INSERT INTO hub (name) SELECT 'hub ' || g FROM generate_series(1, 3) g;        -- 1..3
INSERT INTO hub_log (hub_id) SELECT 1 + g % 3 FROM generate_series(1, 30) g;
INSERT INTO person (name) VALUES ('p1'), ('p2');                               -- 1, 2
INSERT INTO person_emp (name, hub_id) VALUES ('e1', 1), ('e2', 2);             -- 3, 4
INSERT INTO person_acq (name) VALUES ('q1');                                   -- 5
INSERT INTO a (hub_id) SELECT 1 + g % 3 FROM generate_series(1, 6) g;         -- a.hub_id = 1 + id % 3
INSERT INTO b (a_id) SELECT g FROM generate_series(1, 6) g;                   -- b.id = b.a_id
UPDATE a SET b_id = id;
UPDATE a SET emp_id = 3 WHERE id = 2;
INSERT INTO node (parent_id, a_id, label) VALUES
    (NULL, 1, E'tab\there'),
    (1, NULL, E'new\nline'),
    (2, NULL, E'back\\slash'),
    (3, 6, NULL);
INSERT INTO a_status (a_id, status_code, note) VALUES
    (1, 'new', '{"k": 1}'),
    (2, 'new', NULL), (2, 'open', NULL),
    (3, 'new', NULL), (3, 'open', NULL), (3, 'won', NULL),
    (4, 'new', NULL),
    (5, 'new', NULL),
    (6, 'lost', NULL);
INSERT INTO "Select" (a_id) VALUES (1), (1), (2);                              -- 1, 2, 3
