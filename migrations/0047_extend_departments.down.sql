ALTER TABLE departments
  DROP FOREIGN KEY fk_departments_head,
  DROP KEY idx_departments_head,
  DROP COLUMN sort_order,
  DROP COLUMN head_membership_id;
