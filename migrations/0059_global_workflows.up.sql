-- 0059: Global workflow templates for platform CMS
-- Platform admins configure these; portal disclosure creation checks them to gate has_workflow.

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS global_workflows (
    workflow_id   VARCHAR(36)   NOT NULL,
    type_id       VARCHAR(64)   NOT NULL,
    status        VARCHAR(32)   NOT NULL DEFAULT 'active',  -- 'active' | 'archived'
    change_note   TEXT,
    created_by    VARCHAR(36)   NOT NULL DEFAULT '',
    updated_by    VARCHAR(36)   NOT NULL DEFAULT '',
    created_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (workflow_id),
    UNIQUE KEY uq_global_workflow_type (type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS global_workflow_steps (
    step_id           VARCHAR(36)   NOT NULL,
    workflow_id       VARCHAR(36)   NOT NULL,
    stage             VARCHAR(255)  NOT NULL,
    department_id     VARCHAR(36)   NOT NULL DEFAULT '',
    assignee_role_ids JSON          NOT NULL,
    due_rule          VARCHAR(64)   NOT NULL DEFAULT '',
    processing_days   INT           NOT NULL DEFAULT 0,
    display_order     INT           NOT NULL DEFAULT 0,
    documents_json    JSON,
    created_at        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (step_id),
    KEY idx_gwf_steps_workflow (workflow_id),
    CONSTRAINT fk_gwf_steps_workflow FOREIGN KEY (workflow_id) REFERENCES global_workflows (workflow_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
