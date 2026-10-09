-- Every shape bungen dump must handle, in miniature: a FK cycle (a <-> b), a
-- self-FK (node), a table without a primary key whose FK targets a UNIQUE
-- column (a_status -> status.code), inheritance children sharing a sequence
-- (person), a name that needs quoting ("Select"), and a trigger (audit).

CREATE TABLE status (
    id   serial PRIMARY KEY,
    code text NOT NULL UNIQUE
);

CREATE TABLE hub (
    id   serial PRIMARY KEY,
    name text NOT NULL
);

CREATE TABLE hub_log (
    id     serial PRIMARY KEY,
    hub_id int NOT NULL REFERENCES hub (id)
);

CREATE TABLE person (
    id   serial PRIMARY KEY,
    name text NOT NULL
);

-- The children inherit the default nextval('person_id_seq').
CREATE TABLE person_emp (hub_id int REFERENCES hub (id)) INHERITS (person);
ALTER TABLE person_emp ADD PRIMARY KEY (id);
CREATE TABLE person_acq () INHERITS (person);

CREATE TABLE a (
    id     serial PRIMARY KEY,
    b_id   int,
    hub_id int REFERENCES hub (id),
    emp_id int REFERENCES person_emp (id),
    ts     timestamptz NOT NULL DEFAULT '2026-01-02 03:04:05+02'
);

CREATE TABLE b (
    id   serial PRIMARY KEY,
    a_id int REFERENCES a (id)
);

ALTER TABLE a ADD FOREIGN KEY (b_id) REFERENCES b (id);

CREATE TABLE node (
    id        serial PRIMARY KEY,
    parent_id int REFERENCES node (id),
    a_id      int REFERENCES a (id),
    label     text
);

CREATE TABLE a_status (
    a_id        int  NOT NULL REFERENCES a (id),
    status_code text NOT NULL REFERENCES status (code),
    note        jsonb
);

CREATE TABLE "Select" (
    id   serial PRIMARY KEY,
    a_id int REFERENCES a (id)
);

CREATE TABLE audit (
    id  serial PRIMARY KEY,
    msg text NOT NULL
);

CREATE FUNCTION audit_a() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO audit (msg) VALUES ('a ' || NEW.id);
    RETURN NEW;
END
$$;

CREATE TRIGGER a_audit AFTER INSERT ON a FOR EACH ROW EXECUTE FUNCTION audit_a();
