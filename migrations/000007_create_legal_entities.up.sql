CREATE TABLE IF NOT EXISTS legal_entities (
                                              uuid           uuid         PRIMARY KEY,
                                              company_uuid   uuid         NOT NULL,
                                              name           varchar(200) NOT NULL,
    inn            varchar(12)  NOT NULL,
    kpp            varchar(9)   NOT NULL DEFAULT '',
    legal_address  text         NOT NULL DEFAULT '',
    actual_address text         NOT NULL DEFAULT '',
    created_at     timestamptz  NOT NULL DEFAULT now(),
    updated_at     timestamptz  NOT NULL DEFAULT now(),
    deleted_at     timestamptz
    );

CREATE INDEX legal_entities_company_uuid ON legal_entities (company_uuid)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX legal_entities_inn_unique ON legal_entities (inn)
    WHERE deleted_at IS NULL;