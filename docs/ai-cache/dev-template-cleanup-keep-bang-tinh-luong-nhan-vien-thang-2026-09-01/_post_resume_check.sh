#!/bin/bash
set -eu
echo RUNNING=$(docker ps -q --filter name=cobo-iam-worker --filter status=running | wc -l)
echo '=== WORKER LOGS ==='
docker logs --since 3m cobo-iam-worker 2>&1 | tail -100
echo '=== POST RESUME DB ==='
sleep 20
docker exec cobo-iam-mysql mysql -uroot -proot --default-character-set=utf8mb4 cobo_iam -Nse 'SELECT COUNT(*) FROM disclosure_types'
docker exec cobo-iam-mysql mysql -uroot -proot --default-character-set=utf8mb4 cobo_iam -Nse 'SELECT COUNT(*) FROM periodic_cycles pc WHERE pc.type_id COLLATE utf8mb4_unicode_ci <> "bang-tinh-luong-nhan-vien-ban-sao-2" COLLATE utf8mb4_unicode_ci'
docker exec cobo-iam-mysql mysql -uroot -proot --default-character-set=utf8mb4 cobo_iam -Nse 'SELECT COUNT(*) FROM disclosure_records r WHERE r.type_id COLLATE utf8mb4_unicode_ci <> "bang-tinh-luong-nhan-vien-ban-sao-2" COLLATE utf8mb4_unicode_ci'
docker exec cobo-iam-mysql mysql -uroot -proot --default-character-set=utf8mb4 cobo_iam -Nse 'SELECT COUNT(*) FROM periodic_cycles WHERE type_id COLLATE utf8mb4_unicode_ci = "bang-tinh-luong-nhan-vien-ban-sao-2" COLLATE utf8mb4_unicode_ci'
docker inspect cobo-iam-worker --format '{{range .Config.Env}}{{println .}}{{end}}' | grep -E '^PERIODIC_SEEDING_ENABLED=|^WORKFLOW_SNAPSHOT_ENABLED='
echo '=== API HEALTH ==='
curl -s -o /dev/null -w 'healthz=%{http_code}\n' http://127.0.0.1:8080/healthz || true
curl -s -o /dev/null -w 'readyz=%{http_code}\n' http://127.0.0.1:8080/readyz || true
