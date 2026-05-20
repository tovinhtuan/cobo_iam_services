-- Holiday calendars uploaded from CMS (per year). Dates normalized to YYYY-MM-DD in Asia/Ho_Chi_Minh civil semantics.
CREATE TABLE IF NOT EXISTS holiday_calendars (
  calendar_id VARCHAR(36) NOT NULL,
  calendar_year SMALLINT UNSIGNED NOT NULL,
  source_file_name VARCHAR(512) NULL,
  content_sha256 CHAR(64) NULL,
  total_days INT UNSIGNED NOT NULL DEFAULT 0,
  description TEXT NULL,
  uploaded_by VARCHAR(64) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (calendar_id),
  UNIQUE KEY uk_holiday_calendar_year (calendar_year)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS holiday_dates (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  calendar_id VARCHAR(36) NOT NULL,
  calendar_year SMALLINT UNSIGNED NOT NULL,
  holiday_date DATE NOT NULL,
  day_type VARCHAR(32) NOT NULL DEFAULT 'PUBLIC_HOLIDAY',
  name VARCHAR(512) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_holiday_year_date (calendar_year, holiday_date),
  KEY idx_holiday_calendar_id (calendar_id),
  CONSTRAINT fk_holiday_dates_calendar
    FOREIGN KEY (calendar_id) REFERENCES holiday_calendars (calendar_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
