-- KATO, the official classifier of administrative-territorial objects (stat.gov.kz).
-- Every download is kept as a version; exactly one version is active at a time.

CREATE TABLE kato_version (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    origin      text        NOT NULL CHECK (origin IN ('embedded', 'stat.gov.kz')),
    source_url  text        NOT NULL,
    source_date date,
    sha256      text        NOT NULL UNIQUE,
    rows        integer     NOT NULL CHECK (rows > 0),
    loaded_at   timestamptz NOT NULL DEFAULT now(),
    active      boolean     NOT NULL DEFAULT false
);

-- at most one active version
CREATE UNIQUE INDEX kato_version_active ON kato_version (active) WHERE active;

CREATE TABLE kato_object (
    version_id    bigint   NOT NULL REFERENCES kato_version (id) ON DELETE CASCADE,
    code          char(9)  NOT NULL,
    parent        char(9),
    kind          smallint NOT NULL,
    name_ru       text     NOT NULL,
    name_kk       text     NOT NULL,
    name_ru_clean text     NOT NULL,
    name_kk_clean text     NOT NULL,
    PRIMARY KEY (version_id, code)
);

CREATE INDEX kato_object_name_ru ON kato_object (version_id, lower(name_ru_clean));

-- every weekly check, successful or not, for the operator and the defense
CREATE TABLE kato_sync_log (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    checked_at timestamptz NOT NULL DEFAULT now(),
    status     text        NOT NULL CHECK (status IN ('unchanged', 'updated', 'failed', 'seeded')),
    source_url text,
    sha256     text,
    message    text
);
