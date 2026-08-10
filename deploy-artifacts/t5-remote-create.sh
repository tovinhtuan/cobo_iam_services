#!/bin/bash
TOKEN='atk_019fec20-f911-7c49-87ed-afec979e7c59'
echo token_len=${#TOKEN}
CODE=curl -s -o /tmp/out.json -w '%{http_code}' -X POST http://127.0.0.1:8080/api/v1/company/ad-hoc-proposals -H "Authorization: Bearer " -H 'Content-Type: application/json' -d '{"type_id":"dt-co-1783321973009265050","use_template_workflow":true,"change_note":"qa-t5","reviewer_membership_ids":["m_102"]}'
echo HTTP=
cat /tmp/out.json; echo
CODE2=curl -s -o /tmp/out2.json -w '%{http_code}' "http://127.0.0.1:8080/api/v1/company/ad-hoc-proposals?page=1&page_size=1" -H "Authorization: Bearer "
echo HTTP_LIST=
head -c 200 /tmp/out2.json; echo
