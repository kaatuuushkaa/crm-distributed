CREATE TABLE federations (
                             uuid            uuid         NOT NULL DEFAULT gen_random_uuid(),
                             name            varchar(100) NOT NULL DEFAULT '',
                             created_by      varchar(100) NOT NULL DEFAULT '',
                             created_by_uuid uuid         NOT NULL,
                             meta            jsonb        NOT NULL DEFAULT '{}',
                             created_at      timestamptz  NOT NULL DEFAULT now(),
                             updated_at      timestamptz  NOT NULL DEFAULT now(),
                             deleted_at      timestamptz,
                             PRIMARY KEY (uuid)
);

CREATE INDEX federations_created_at ON federations (created_at DESC);

CREATE TABLE federation_users (
                                  uuid            uuid        NOT NULL DEFAULT gen_random_uuid(),
                                  federation_uuid uuid        NOT NULL,
                                  user_uuid       uuid        NOT NULL,
                                  added_at        timestamptz NOT NULL DEFAULT now(),
                                  CONSTRAINT fk_federation_users_federation
                                      FOREIGN KEY (federation_uuid) REFERENCES federations (uuid) ON DELETE CASCADE,
                                  CONSTRAINT fk_federation_users_user
                                      FOREIGN KEY (user_uuid) REFERENCES users (uuid) ON DELETE CASCADE,
                                  PRIMARY KEY (uuid)
);

CREATE UNIQUE INDEX idx_federation_users_unique
    ON federation_users (federation_uuid, user_uuid);

CREATE INDEX idx_federation_users_user ON federation_users (user_uuid);