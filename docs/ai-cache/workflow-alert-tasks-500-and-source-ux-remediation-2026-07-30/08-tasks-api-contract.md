# Tasks API contract
200 + tasks[] (including system assignee)
Empty → 200 []
Invalid → 400; missing → via authz/not found paths
No 500 for missing department / system membership
