-- +migrate Up

CREATE TYPE enum_alert_severity AS ENUM (
    'info',
    'warning',
    'high',
    'critical'
);

ALTER TABLE alerts ADD COLUMN severity enum_alert_severity NOT NULL DEFAULT 'info';

-- +migrate Down

ALTER TABLE alerts DROP COLUMN severity;
DROP TYPE enum_alert_severity;
