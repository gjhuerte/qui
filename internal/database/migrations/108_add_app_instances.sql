-- Copyright (c) 2026, s0up and the autobrr contributors.
-- SPDX-License-Identifier: GPL-2.0-or-later

-- Media apps (Sonarr, Radarr, Prowlarr, Bazarr, Tautulli) shown on the
-- Services page. Separate from arr_instances, which cross-seed uses for ID
-- lookups and which accepts only Sonarr and Radarr.
CREATE TABLE app_instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (type IN ('sonarr', 'radarr', 'prowlarr', 'bazarr', 'tautulli')),
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    api_key_encrypted TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
