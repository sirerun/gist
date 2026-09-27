-- Order catalog versions by SemVer 2.0.0 precedence instead of plain text, so
-- 1.10.0 sorts after 1.9.0 and a pre-release sorts below its release. The key
-- mirrors storage.compareSemver byte for byte:
--   valid:   '0', then each core number as a 3-digit length plus its digits
--            without leading zeros, then chr(2) for a release, or, for a
--            pre-release, each identifier as chr(1) followed by '0' + 3-digit
--            length + digits (numeric) or '1' + the identifier (alphanumeric);
--   invalid: '1' followed by the raw version, so invalid strings sort after
--            every valid one and in byte order among themselves.
-- Build metadata is ignored, so versions differing only in build metadata tie;
-- ListVersions breaks ties on the version text in the C collation.
CREATE OR REPLACE FUNCTION registry_semver_key(v text)
RETURNS bytea LANGUAGE plpgsql IMMUTABLE STRICT PARALLEL SAFE AS $$
DECLARE
    core text;
    pre text;
    parts text[];
    ident text;
    digits text;
    dash integer;
    key text := '0';
BEGIN
    core := split_part(v, '+', 1);
    dash := position('-' IN core);
    IF dash > 0 THEN
        pre := substr(core, dash + 1);
        core := substr(core, 1, dash - 1);
        IF pre = '' THEN
            RETURN convert_to('1' || v, 'UTF8');
        END IF;
    END IF;
    parts := string_to_array(core, '.');
    IF array_length(parts, 1) IS DISTINCT FROM 3 THEN
        RETURN convert_to('1' || v, 'UTF8');
    END IF;
    FOREACH ident IN ARRAY parts LOOP
        IF ident !~ '^[0-9]+$' THEN
            RETURN convert_to('1' || v, 'UTF8');
        END IF;
        digits := ltrim(ident, '0');
        key := key || lpad(length(digits)::text, 3, '0') || digits;
    END LOOP;
    IF pre IS NULL THEN
        RETURN convert_to(key || chr(2), 'UTF8');
    END IF;
    FOREACH ident IN ARRAY string_to_array(pre, '.') LOOP
        IF ident ~ '^[0-9]+$' THEN
            digits := ltrim(ident, '0');
            key := key || chr(1) || '0' || lpad(length(digits)::text, 3, '0') || digits;
        ELSE
            key := key || chr(1) || '1' || ident;
        END IF;
    END LOOP;
    RETURN convert_to(key, 'UTF8');
END $$;

ALTER TABLE catalog_versions
    ADD COLUMN IF NOT EXISTS version_key bytea GENERATED ALWAYS AS (registry_semver_key(version)) STORED;
CREATE INDEX IF NOT EXISTS catalog_versions_semver
    ON catalog_versions (workspace_id, kind, artifact_id, version_key, version COLLATE "C");
