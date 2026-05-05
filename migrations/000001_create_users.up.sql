CREATE TABLE users (
                       uuid               uuid         NOT NULL DEFAULT gen_random_uuid(),
                       name               varchar(30)  NOT NULL DEFAULT '',
                       lname              varchar(30)  NOT NULL DEFAULT '',
                       pname              varchar(30)  NOT NULL DEFAULT '',
                       email              varchar(100) NOT NULL DEFAULT '',
                       phone              bigint       NOT NULL DEFAULT 0,
                       password           varchar(255) NOT NULL DEFAULT '',
                       is_valid           boolean      NOT NULL DEFAULT false,
                       provider           integer      NOT NULL DEFAULT 0,
                       color              varchar(7)   NOT NULL DEFAULT '#3B82F6',
                       has_photo          boolean      NOT NULL DEFAULT false,
                       valid_at           timestamptz,
                       validation_send_at timestamptz,
                       meta               jsonb        NOT NULL DEFAULT '{}',
                       created_at         timestamptz  NOT NULL DEFAULT now(),
                       updated_at         timestamptz  NOT NULL DEFAULT now(),
                       deleted_at         timestamptz,
                       PRIMARY KEY (uuid)
);

CREATE UNIQUE INDEX users_email_unique ON users (email)
    WHERE deleted_at IS NULL;

CREATE INDEX users_created_at ON users (created_at DESC);
CREATE INDEX users_email ON users (email) WHERE deleted_at IS NULL;