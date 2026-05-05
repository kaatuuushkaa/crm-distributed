CREATE EXTENSION IF NOT EXISTS ltree;

CREATE TABLE tasks (
                       uuid            uuid         NOT NULL DEFAULT gen_random_uuid(),
                       id              integer      NOT NULL DEFAULT 0,
                       name            varchar(100) NOT NULL DEFAULT '',
                       description     text         NOT NULL DEFAULT '',
                       icon            varchar(20)  NOT NULL DEFAULT '',
                       federation_uuid uuid         NOT NULL,
                       company_uuid    uuid         NOT NULL,
                       project_uuid    uuid         NOT NULL,
                       created_by      varchar(100) NOT NULL DEFAULT '',
                       managed_by      varchar(100) NOT NULL DEFAULT '',
                       responsible_by  varchar(100) NOT NULL DEFAULT '',
                       implement_by    varchar(100) NOT NULL DEFAULT '',
                       finished_by     varchar(100) NOT NULL DEFAULT '',
                       co_workers_by   text[]       NOT NULL DEFAULT '{}',
                       watch_by        text[]       NOT NULL DEFAULT '{}',
                       all_people      text[]       NOT NULL DEFAULT '{}',
                       tags            text[]       NOT NULL DEFAULT '{}',
                       status          integer      NOT NULL DEFAULT 0,
                       priority        integer      NOT NULL DEFAULT 10,
                       is_epic         boolean      NOT NULL DEFAULT false,
                       path            ltree,
                       fields          jsonb        NOT NULL DEFAULT '{}',
                       meta            jsonb        NOT NULL DEFAULT '{}',
                       finish_to       timestamptz,
                       finished_at     timestamptz,
                       activity_at     timestamptz  NOT NULL DEFAULT now(),
                       created_at      timestamptz  NOT NULL DEFAULT now(),
                       updated_at      timestamptz  NOT NULL DEFAULT now(),
                       deleted_at      timestamptz,

                       CONSTRAINT fk_tasks_project
                           FOREIGN KEY (project_uuid) REFERENCES projects (uuid) ON DELETE CASCADE,
                       PRIMARY KEY (uuid)
);

CREATE INDEX tasks_project_status ON tasks (project_uuid, status, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX tasks_all_people ON tasks USING GIN (all_people)
    WHERE deleted_at IS NULL;

CREATE INDEX tasks_tags ON tasks USING GIN (tags)
    WHERE deleted_at IS NULL;

CREATE INDEX tasks_path ON tasks USING GIST (path)
    WHERE deleted_at IS NULL;

CREATE INDEX tasks_name ON tasks USING GIN (to_tsvector('russian', name))
    WHERE deleted_at IS NULL;