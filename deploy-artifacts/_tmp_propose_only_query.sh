#!/bin/bash
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse "SELECT u.login_id, IF(u.email_verified_at IS NULL,0,1) AS verified, m.membership_id, m.status FROM users u JOIN memberships m ON m.user_id=u.user_id JOIN membership_roles mr ON mr.membership_id=m.membership_id WHERE mr.role_id='r_qa_propose_only_t5' OR u.login_id LIKE 'qa.propose%' LIMIT 20;"
