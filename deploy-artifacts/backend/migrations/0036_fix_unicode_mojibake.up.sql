-- 0036: Repair Mojibake in disclosure_display_groups
--
-- Root cause: migration 0035 was executed without SET NAMES utf8mb4 and the
-- migration runner (run_dev_migrations.sh) did not pass --default-character-set=utf8mb4
-- to the mysql CLI. MySQL therefore used the server default charset (often latin1)
-- for the connection, so each UTF-8 byte of Vietnamese text was stored as a
-- separate Latin-1 code point (Mojibake).
--
-- Detection: correct Vietnamese text with tone marks always contains at least one
-- character in U+1E00-U+1EFF (UTF-8: E1 B8 xx … E1 BB xx). Garbled rows have only
-- Latin-1 code points (U+0000-U+00FF) and their HEX never contains "E1B".
--
-- Repair: CONVERT(CONVERT(col USING latin1) USING utf8mb4) re-interprets the
-- stored Latin-1 code-point bytes as UTF-8, recovering the original Vietnamese.
-- The WHERE guard ensures already-correct rows are never touched.

SET NAMES utf8mb4;

-- Fix table collation while we are here (idempotent ALTER).
ALTER TABLE disclosure_display_groups
    CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Repair garbled rows. Only rows whose name_vi contains no Vietnamese Extended
-- characters (U+1E00-U+1EFF) are updated; those rows are Mojibake.
UPDATE disclosure_display_groups
SET
    name_vi     = CONVERT(CONVERT(name_vi     USING latin1) USING utf8mb4),
    name_en     = CONVERT(CONVERT(name_en     USING latin1) USING utf8mb4),
    description = CONVERT(CONVERT(description USING latin1) USING utf8mb4)
WHERE HEX(name_vi) NOT LIKE '%E1B%';
