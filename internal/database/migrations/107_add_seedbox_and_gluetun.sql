-- Copyright (c) 2026, s0up and the autobrr contributors.
-- SPDX-License-Identifier: GPL-2.0-or-later

-- MyAnonamouse dynamic seedbox: one settings row (the encrypted mam_id and the
-- schedule) and one row per request to MAM.
CREATE TABLE seedbox_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    mam_id_encrypted TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    interval_seconds INTEGER NOT NULL DEFAULT 600,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE seedbox_contacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    reached INTEGER NOT NULL,
    success INTEGER,
    http_status INTEGER,
    message TEXT,
    ip TEXT,
    asn INTEGER,
    as_org TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_seedbox_contacts_created_at ON seedbox_contacts(created_at);

-- How qui reaches gluetun: its control server and, for logs and container
-- restarts, the Docker API. secret_encrypted holds the API key or password.
CREATE TABLE gluetun_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    control_url TEXT NOT NULL DEFAULT 'http://127.0.0.1:8000',
    auth_type TEXT NOT NULL DEFAULT 'none',
    username TEXT NOT NULL DEFAULT '',
    secret_encrypted TEXT,
    docker_host TEXT NOT NULL DEFAULT 'unix:///var/run/docker.sock',
    container_name TEXT NOT NULL DEFAULT 'gluetun',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
