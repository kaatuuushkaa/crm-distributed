CREATE TABLE projects (
                          uuid            uuid         NOT NULL DEFAULT gen_random_uuid(),
                          name            varchar(100) NOT NULL DEFAULT '',
                          description     text         NOT NULL DEFAULT '',
                          federation_uuid uuid         NOT NULL,
                          company_uuid    uuid         NOT NULL,
                          created_by      varchar(100) NOT NULL DEFAULT '',
                          responsible_by  varchar(200) NOT NULL DEFAULT '',
                          task_id         integer      NOT NULL DEFAULT 0,
                          status_graph    jsonb        NOT NULL DEFAULT '{}',
                          options         jsonb        NOT NULL DEFAULT '{}',
                          meta            jsonb        NOT NULL DEFAULT '{}',
                          created_at      timestamptz  NOT NULL DEFAULT now(),
                          updated_at      timestamptz  NOT NULL DEFAULT now(),
                          deleted_at      timestamptz,
                          CONSTRAINT fk_projects_federation
                              FOREIGN KEY (federation_uuid) REFERENCES federations (uuid) ON DELETE CASCADE,
                          CONSTRAINT fk_projects_company
                              FOREIGN KEY (company_uuid) REFERENCES companies (uuid) ON DELETE CASCADE,
                          PRIMARY KEY (uuid)
);

CREATE INDEX projects_company_uuid ON projects (company_uuid)
    WHERE deleted_at IS NULL;
CREATE INDEX projects_federation_uuid ON projects (federation_uuid)
    WHERE deleted_at IS NULL;
CREATE INDEX projects_created_at ON projects (created_at DESC);

CREATE TABLE project_users (
                               uuid            uuid        NOT NULL DEFAULT gen_random_uuid(),
                               project_uuid    uuid        NOT NULL,
                               federation_uuid uuid        NOT NULL,
                               company_uuid    uuid        NOT NULL,
                               user_uuid       uuid        NOT NULL,
                               added_at        timestamptz NOT NULL DEFAULT now(),
                               CONSTRAINT fk_project_users_project
                                   FOREIGN KEY (project_uuid) REFERENCES projects (uuid) ON DELETE CASCADE,
                               CONSTRAINT fk_project_users_user
                                   FOREIGN KEY (user_uuid) REFERENCES users (uuid) ON DELETE CASCADE,
                               PRIMARY KEY (uuid)
);

CREATE UNIQUE INDEX idx_project_users_unique
    ON project_users (project_uuid, user_uuid);

CREATE INDEX idx_project_users_user ON project_users (user_uuid);

CREATE TABLE company_fields (
                                uuid                 uuid        NOT NULL DEFAULT gen_random_uuid(),
                                company_uuid         uuid        NOT NULL,
                                hash                 varchar(15) NOT NULL,
                                name                 varchar(100) NOT NULL DEFAULT '',
                                description          text        NOT NULL DEFAULT '',
                                icon                 varchar(50) NOT NULL DEFAULT '',
                                data_type            integer     NOT NULL DEFAULT 0,
                                required_on_statuses jsonb       NOT NULL DEFAULT '[]',
                                style                varchar(20) NOT NULL DEFAULT '',
                                created_by           varchar(100) NOT NULL DEFAULT '',
                                created_at           timestamptz NOT NULL DEFAULT now(),
                                updated_at           timestamptz NOT NULL DEFAULT now(),
                                deleted_at           timestamptz,
                                CONSTRAINT fk_company_fields_company
                                    FOREIGN KEY (company_uuid) REFERENCES companies (uuid) ON DELETE CASCADE,
                                PRIMARY KEY (uuid)
);

CREATE UNIQUE INDEX idx_company_fields_unique
    ON company_fields (company_uuid, hash)
    WHERE deleted_at IS NULL;