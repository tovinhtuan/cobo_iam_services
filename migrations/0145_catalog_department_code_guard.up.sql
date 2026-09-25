-- 0145: block catalog department_code changes and register codes already present in snapshots.
-- Display-name updates stay allowed. Delete stays blocked by 0144.

INSERT INTO workflow_template_department_code_registry (department_code)
SELECT DISTINCT jt.dept
FROM workflow_instances wi
JOIN JSON_TABLE(
  wi.snapshot_json,
  '$[*]' COLUMNS (dept VARCHAR(64) PATH '$.department')
) jt
JOIN workflow_template_departments cat
  ON cat.department_code = (jt.dept COLLATE utf8mb4_unicode_ci)
WHERE jt.dept IS NOT NULL AND jt.dept <> ''
ON DUPLICATE KEY UPDATE department_code = workflow_template_department_code_registry.department_code;

DROP TRIGGER IF EXISTS trg_wtd_no_code_update;

DELIMITER $$
CREATE TRIGGER trg_wtd_no_code_update
BEFORE UPDATE ON workflow_template_departments
FOR EACH ROW
BEGIN
  IF NOT (OLD.department_code <=> NEW.department_code) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'catalog department code cannot change';
  END IF;
END$$
DELIMITER ;

DROP TRIGGER IF EXISTS trg_wtd_no_reuse_retired;

DELIMITER $$
CREATE TRIGGER trg_wtd_no_reuse_retired
BEFORE INSERT ON workflow_template_departments
FOR EACH ROW
BEGIN
  IF EXISTS (
    SELECT 1 FROM workflow_template_department_code_registry r
    WHERE r.department_code = NEW.department_code
      AND r.retired_at IS NOT NULL
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'catalog department code cannot be reused';
  END IF;
END$$
DELIMITER ;
