-- Migration: Add UNIQUE constraints on id column for composite PK tables.
-- BIGSERIAL IDs are globally unique per project, so these constraints always
-- succeed on existing data. They provide efficient id-only lookups via unique index.
--
-- Tables with composite PK (project, id) where id is a BIGINT:
--   plan, issue, task, task_run, plan_check_run

ALTER TABLE plan ADD CONSTRAINT plan_id_unique UNIQUE (id);
ALTER TABLE issue ADD CONSTRAINT issue_id_unique UNIQUE (id);
ALTER TABLE task ADD CONSTRAINT task_id_unique UNIQUE (id);
ALTER TABLE task_run ADD CONSTRAINT task_run_id_unique UNIQUE (id);
ALTER TABLE plan_check_run ADD CONSTRAINT plan_check_run_id_unique UNIQUE (id);
