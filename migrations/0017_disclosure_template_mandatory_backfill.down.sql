-- Remove rows inserted by 0017_disclosure_template_mandatory_backfill.up.sql (block_id = 'mb-' + 32-char MD5 hex).

SET NAMES utf8mb4;

DELETE FROM disclosure_template_blocks
WHERE block_id REGEXP '^mb-[a-f0-9]{32}$';
