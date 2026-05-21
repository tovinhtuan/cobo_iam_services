SET NAMES utf8mb4;

-- titles already exists (0001_init_core). Add sort_order for display ordering (C3: nút ↑↓).
-- membership_titles already supports multi-title per member — no changes needed there.
ALTER TABLE titles
  ADD COLUMN sort_order INT NOT NULL DEFAULT 0 AFTER title_name;
