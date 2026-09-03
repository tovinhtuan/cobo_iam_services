# 08 — Backup

## Method

```text
docker exec cobo-iam-mysql mysqldump -uroot -p*** --single-transaction --routines --triggers --hex-blob cobo_iam | gzip
```

Credentials not stored in evidence.

## Artifact

```text
BACKUP_PATH=/root/cobo_project/backups/cobo_iam-before-template-cleanup-20260901T020624Z.sql.gz
BACKUP_CREATED=true
BACKUP_VALIDATED=true
```

## Validation

| Check | Result |
|-------|--------|
| exit code | 0 |
| file size | 1,003,186 bytes (>0) |
| gzip -t | OK |
| header | MySQL dump 8.0.46, Database: cobo_iam |
| CREATE TABLE count | 84 |

## Restore concept

```text
ROLLBACK_METHOD=MYSQLDUMP_RESTORE
BACKUP_RETENTION=KEEP_UNTIL_USER_CLEANUP
```

1. Pause worker
2. `gunzip -c <backup> | docker exec -i cobo-iam-mysql mysql -uroot -p*** cobo_iam`
3. Resume worker / verify
