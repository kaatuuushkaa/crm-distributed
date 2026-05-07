CREATE TABLE IF NOT EXISTS files (
                                     uuid          uuid         PRIMARY KEY,
                                     owner_uuid    uuid         NOT NULL,
                                     owner_type    varchar(50)  NOT NULL,
    original_name varchar(500) NOT NULL,
    storage_key   varchar(500) NOT NULL UNIQUE,
    content_type  varchar(200) NOT NULL,
    size_bytes    bigint       NOT NULL CHECK (size_bytes > 0),
    created_by    uuid         NOT NULL,
    created_at    timestamptz  NOT NULL DEFAULT now(),
    deleted_at    timestamptz
    );

CREATE INDEX files_owner ON files (owner_uuid, owner_type)
    WHERE deleted_at IS NULL;

CREATE INDEX files_created_by ON files (created_by)
    WHERE deleted_at IS NULL;