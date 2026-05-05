CREATE TABLE notifications (
                               uuid            uuid         NOT NULL DEFAULT gen_random_uuid(),
                               recipient_uuid  uuid         NOT NULL,
                               type            varchar(50)  NOT NULL DEFAULT '',
                               title           varchar(200) NOT NULL DEFAULT '',
                               body            text         NOT NULL DEFAULT '',
                               entity_uuid     uuid,
                               entity_type     varchar(50)  NOT NULL DEFAULT '',
                               meta            jsonb        NOT NULL DEFAULT '{}',
                               status          integer      NOT NULL DEFAULT 0,
                               delivered_at    timestamptz,
                               read_at         timestamptz,
                               created_at      timestamptz  NOT NULL DEFAULT now(),
                               PRIMARY KEY (uuid)
);

CREATE INDEX notifications_recipient_status
    ON notifications (recipient_uuid, status, created_at DESC);

CREATE INDEX notifications_entity
    ON notifications (entity_uuid, entity_type)
    WHERE entity_uuid IS NOT NULL;