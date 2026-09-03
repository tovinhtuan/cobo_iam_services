SET NAMES utf8mb4;

-- 1) Full inventory (active version name)
SELECT dt.type_id, dt.company_id, dt.status, dt.active_version_no,
       dtv.name,
       (SELECT COUNT(*) FROM disclosure_type_versions v WHERE v.type_id = dt.type_id) AS version_count
FROM disclosure_types dt
LEFT JOIN disclosure_type_versions dtv
  ON dtv.type_id = dt.type_id AND dtv.version_no = dt.active_version_no
ORDER BY dtv.name, dt.type_id;

-- 2) Exact name candidates
SELECT dt.type_id, dt.status, dt.active_version_no, dtv.name
FROM disclosure_types dt
JOIN disclosure_type_versions dtv
  ON dtv.type_id = dt.type_id AND dtv.version_no = dt.active_version_no
WHERE TRIM(dtv.name) = 'Bảng tính lương nhân viên tháng';

-- 3) Similar names (for audit table)
SELECT dt.type_id, dt.status, dt.active_version_no, dtv.name
FROM disclosure_types dt
JOIN disclosure_type_versions dtv
  ON dtv.type_id = dt.type_id AND dtv.version_no = dt.active_version_no
WHERE dtv.name LIKE '%Bảng tính lương%'
ORDER BY dtv.name;
