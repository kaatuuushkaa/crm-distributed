CREATE TABLE IF NOT EXISTS bank_accounts (
                                             uuid              uuid         PRIMARY KEY,
                                             legal_entity_uuid uuid         NOT NULL REFERENCES legal_entities(uuid) ON DELETE CASCADE,
    bank              varchar(200) NOT NULL,
    bik               varchar(9)   NOT NULL,
    corr_acc          varchar(20)  NOT NULL,
    pay_acc           varchar(20)  NOT NULL,
    address           text         NOT NULL DEFAULT '',
    currency          varchar(3)   NOT NULL DEFAULT 'RUB',
    comment           text         NOT NULL DEFAULT '',
    is_primary        boolean      NOT NULL DEFAULT false,
    created_at        timestamptz  NOT NULL DEFAULT now(),
    updated_at        timestamptz  NOT NULL DEFAULT now(),
    deleted_at        timestamptz
    );

CREATE INDEX bank_accounts_legal_entity ON bank_accounts (legal_entity_uuid)
    WHERE deleted_at IS NULL;