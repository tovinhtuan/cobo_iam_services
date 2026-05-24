-- 0060-deadline-v2: upgrade legacy deadline_rule_catalog (0053) then seed CMS rows.
-- If you still see "Unknown column rule_id" at line ~23, the server is running the OLD 0060 file.

SET NAMES utf8mb4;

SET @legacy = (
  SELECT COUNT(1) FROM information_schema.tables t
  WHERE t.table_schema = DATABASE() AND t.table_name = 'deadline_rule_catalog'
) > 0
AND (
  SELECT COUNT(1) FROM information_schema.columns c
  WHERE c.table_schema = DATABASE()
    AND c.table_name = 'deadline_rule_catalog'
    AND c.column_name = 'rule_id'
) = 0;

SET @fresh = (
  SELECT COUNT(1) FROM information_schema.tables t
  WHERE t.table_schema = DATABASE() AND t.table_name = 'deadline_rule_catalog'
) = 0;

SET @sql_fresh = IF(
  @fresh = 1,
  'CREATE TABLE deadline_rule_catalog (
    rule_id       VARCHAR(36)   NOT NULL,
    code          VARCHAR(64)   NOT NULL,
    label_vi      VARCHAR(255)  NOT NULL,
    pattern       VARCHAR(255)  NOT NULL,
    input_type    VARCHAR(32)   NOT NULL DEFAULT ''text'',
    is_active     TINYINT(1)    NOT NULL DEFAULT 1,
    display_order INT           NOT NULL DEFAULT 0,
    created_by    VARCHAR(36)   NOT NULL DEFAULT '''',
    updated_by    VARCHAR(36)   NOT NULL DEFAULT '''',
    created_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (rule_id),
    UNIQUE KEY uq_deadline_rule_code (code)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci',
  'SELECT 1'
);
PREPARE stmt FROM @sql_fresh; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql_legacy_add = IF(
  @legacy = 1,
  'ALTER TABLE deadline_rule_catalog
     ADD COLUMN rule_id VARCHAR(36) NULL FIRST,
     ADD COLUMN display_order INT NOT NULL DEFAULT 0,
     ADD COLUMN created_by VARCHAR(36) NOT NULL DEFAULT '''',
     ADD COLUMN updated_by VARCHAR(36) NOT NULL DEFAULT '''',
     ADD COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     ADD COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP',
  'SELECT 1'
);
PREPARE stmt FROM @sql_legacy_add; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql_legacy_fill = IF(
  @legacy = 1,
  'UPDATE deadline_rule_catalog SET rule_id = UUID() WHERE rule_id IS NULL OR rule_id = ''''',
  'SELECT 1'
);
PREPARE stmt FROM @sql_legacy_fill; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql_legacy_pk = IF(
  @legacy = 1,
  'ALTER TABLE deadline_rule_catalog
     MODIFY rule_id VARCHAR(36) NOT NULL,
     MODIFY code VARCHAR(64) NOT NULL,
     MODIFY pattern VARCHAR(255) NOT NULL,
     MODIFY input_type VARCHAR(32) NOT NULL DEFAULT ''text'',
     DROP PRIMARY KEY,
     ADD PRIMARY KEY (rule_id),
     ADD UNIQUE KEY uq_deadline_rule_code (code)',
  'SELECT 1'
);
PREPARE stmt FROM @sql_legacy_pk; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ready = (
  SELECT COUNT(1) FROM information_schema.columns c
  WHERE c.table_schema = DATABASE()
    AND c.table_name = 'deadline_rule_catalog'
    AND c.column_name = 'rule_id'
);

SET @sql_seed = IF(
  @ready > 0,
  'INSERT INTO deadline_rule_catalog (rule_id, code, label_vi, pattern, input_type, display_order, created_by, updated_by)
   VALUES
     (UUID(), ''T+N'',   ''Trong vòng N ngày kể từ ngày sự kiện'', ''^T\\\\+\\\\d+$'',       ''number'',  1, ''system'', ''system''),
     (UUID(), ''dd/mm'', ''Ngày dd/mm hàng năm'',                  ''^\\\\d{2}/\\\\d{2}$'',  ''date_dm'', 2, ''system'', ''system'')
   ON DUPLICATE KEY UPDATE
     label_vi = VALUES(label_vi),
     pattern = VALUES(pattern),
     input_type = VALUES(input_type),
     display_order = VALUES(display_order)',
  'SELECT ''ERROR: deadline_rule_catalog missing rule_id — deploy 0060-deadline-v2 file'' AS migration_error'
);
PREPARE stmt FROM @sql_seed; EXECUTE stmt; DEALLOCATE PREPARE stmt;
