#!/usr/bin/env python3
"""DEV-only: print disclosure record scope fields for approved fixture."""
import subprocess
import sys

RECORD = "52698f3f-53c0-53e7-9e40-d99f90774f41"
sql = f"""
SELECT record_id, company_id, COALESCE(department_id,''), COALESCE(org_unit_id,''), type_id, LEFT(COALESCE(title,''),80)
FROM disclosure_records WHERE record_id='{RECORD}' LIMIT 1;
"""
# find mysql container
ps = subprocess.check_output(["docker", "ps", "--format", "{{.ID}} {{.Names}}"], text=True)
cid = None
for line in ps.splitlines():
    if "mysql" in line.lower():
        cid = line.split()[0]
        break
if not cid:
    print("NO_MYSQL_CONTAINER")
    sys.exit(1)
print("CID", cid)
# try common passwords / env
for cmd in [
    ["docker", "exec", cid, "mysql", "-N", "-uroot", "-proot", "cobo", "-e", sql],
    ["docker", "exec", cid, "mysql", "-N", "-uroot", "cobo", "-e", sql],
    ["docker", "exec", "-e", "MYSQL_PWD=root", cid, "mysql", "-N", "-uroot", "cobo", "-e", sql],
]:
    try:
        out = subprocess.check_output(cmd, stderr=subprocess.STDOUT, text=True)
        print("OUT", out)
        break
    except subprocess.CalledProcessError as e:
        print("FAIL", e.output[:200] if e.output else e)
