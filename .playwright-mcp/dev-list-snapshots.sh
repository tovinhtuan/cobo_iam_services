#!/bin/sh
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse "SELECT disclosure_record_id, COUNT(*), MIN(step_code) FROM workflow_step_document_requirement_snapshots GROUP BY disclosure_record_id ORDER BY COUNT(*) DESC LIMIT 20;"
