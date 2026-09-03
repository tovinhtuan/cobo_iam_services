#!/bin/bash
set -euo pipefail
docker exec -i cobo-iam-mysql mysql -uroot -proot --default-character-set=utf8mb4 cobo_iam < /tmp/exec_tx_proc.sql > /tmp/exec_tx_out.txt 2>/tmp/exec_tx_err.txt
echo EXIT=$?
echo '=== STDOUT ==='
cat /tmp/exec_tx_out.txt
echo '=== STDERR ==='
cat /tmp/exec_tx_err.txt
echo '=== FRESH SESSION ==='
docker exec cobo-iam-mysql mysql -uroot -proot --default-character-set=utf8mb4 cobo_iam -Nse 'SELECT COUNT(*) FROM disclosure_types'
docker exec cobo-iam-mysql mysql -uroot -proot --default-character-set=utf8mb4 cobo_iam -e 'SELECT type_id, status, active_version_no FROM disclosure_types'
