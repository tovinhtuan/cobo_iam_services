#!/bin/bash
set -eu
echo '=== API LOG ERRORS ==='
docker logs --since 15m cobo-iam-api 2>&1 | grep -iE 'panic|foreign key|FATAL' | head -20 || true
echo '=== WORKER LOG ERRORS / DELETED IDS ==='
docker logs --since 15m cobo-iam-worker 2>&1 | grep -iE 'panic|foreign key|FATAL' | head -20 || true
docker logs --since 15m cobo-iam-worker 2>&1 | grep -E 'type_id=(bang-tinh-luong-nhan-vien[^-]|bao-cao-|qa-)' | head -20 || true
echo '=== ROOTS STILL ==='
docker exec cobo-iam-mysql mysql -uroot -proot --default-character-set=utf8mb4 cobo_iam -Nse 'SELECT COUNT(*) FROM disclosure_types'
