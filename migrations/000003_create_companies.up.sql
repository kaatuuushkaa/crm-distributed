CREATE TABLE companies (
                           uuid            uuid         NOT NULL DEFAULT gen_random_uuid(),
                           name            varchar(100) NOT NULL DEFAULT '',
                           federation_uuid uuid         NOT NULL,
                           created_by      varchar(100) NOT NULL DEFAULT '',
                           created_by_uuid uuid         NOT NULL,
                           meta            jsonb        NOT NULL DEFAULT '{}',
                           created_at      timestamptz  NOT NULL DEFAULT now(),
                           updated_at      timestamptz  NOT NULL DEFAULT now(),
                           deleted_at      timestamptz,
                           CONSTRAINT fk_companies_federation
                               FOREIGN KEY (federation_uuid) REFERENCES federations (uuid) ON DELETE CASCADE,
                           PRIMARY KEY (uuid)
);

CREATE INDEX companies_federation_uuid ON companies (federation_uuid);
CREATE INDEX companies_created_at ON companies (created_at DESC);

CREATE TABLE company_users (
                               uuid            uuid        NOT NULL DEFAULT gen_random_uuid(),
                               federation_uuid uuid        NOT NULL,
                               company_uuid    uuid        NOT NULL,
                               user_uuid       uuid        NOT NULL,
                               added_at        timestamptz NOT NULL DEFAULT now(),
                               CONSTRAINT fk_company_users_federation
                                   FOREIGN KEY (federation_uuid) REFERENCES federations (uuid) ON DELETE CASCADE,
                               CONSTRAINT fk_company_users_company
                                   FOREIGN KEY (company_uuid) REFERENCES companies (uuid) ON DELETE CASCADE,
                               CONSTRAINT fk_company_users_user
                                   FOREIGN KEY (user_uuid) REFERENCES users (uuid) ON DELETE CASCADE,
                               PRIMARY KEY (uuid)
);

CREATE UNIQUE INDEX idx_company_users_unique
    ON company_users (company_uuid, user_uuid);

CREATE INDEX idx_company_users_user ON company_users (user_uuid);