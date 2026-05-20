SET NAMES utf8mb4;

CREATE TABLE disclosure_records (
  record_id     VARCHAR(36) PRIMARY KEY,
  company_id    VARCHAR(36) NOT NULL,
  department_id VARCHAR(64)  NOT NULL,
  title         VARCHAR(512) NOT NULL,
  content       MEDIUMTEXT   NOT NULL,
  status        VARCHAR(32)  NOT NULL,
  created_by    VARCHAR(36)  NOT NULL,
  updated_by    VARCHAR(36)  NOT NULL,
  created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_disclosure_company_status (company_id, status),
  CONSTRAINT fk_disclosure_records_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workflow_instances (
  workflow_instance_id VARCHAR(36) PRIMARY KEY,
  company_id             VARCHAR(36) NOT NULL,
  record_id              VARCHAR(36) NOT NULL,
  status                 VARCHAR(32) NOT NULL,
  current_step_code      VARCHAR(64) NOT NULL,
  created_by             VARCHAR(36) NOT NULL,
  created_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_wf_instances_company (company_id),
  CONSTRAINT fk_wf_instances_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workflow_tasks (
  task_id                  VARCHAR(36) PRIMARY KEY,
  company_id               VARCHAR(36) NOT NULL,
  workflow_instance_id     VARCHAR(36) NOT NULL,
  step_code                VARCHAR(64) NOT NULL,
  assignee_membership_id   VARCHAR(36) NOT NULL,
  status                   VARCHAR(32) NOT NULL,
  created_at               TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at               TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_wf_tasks_instance (workflow_instance_id),
  CONSTRAINT fk_wf_tasks_instance FOREIGN KEY (workflow_instance_id) REFERENCES workflow_instances(workflow_instance_id) ON DELETE CASCADE,
  CONSTRAINT fk_wf_tasks_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notification_jobs (
  notification_job_id VARCHAR(36) PRIMARY KEY,
  company_id            VARCHAR(36) NOT NULL,
  event_type            VARCHAR(128) NOT NULL,
  resource_type         VARCHAR(64)  NOT NULL,
  resource_id           VARCHAR(64)  NOT NULL,
  payload_json          JSON         NOT NULL,
  status                VARCHAR(32)  NOT NULL,
  created_at            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_notification_jobs_company_status (company_id, status),
  CONSTRAINT fk_notification_jobs_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notification_deliveries (
  notification_delivery_id VARCHAR(36) PRIMARY KEY,
  notification_job_id      VARCHAR(36) NOT NULL,
  recipient                VARCHAR(191) NOT NULL,
  status                   VARCHAR(32)  NOT NULL,
  created_at               TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_notification_deliveries_job FOREIGN KEY (notification_job_id) REFERENCES notification_jobs(notification_job_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
