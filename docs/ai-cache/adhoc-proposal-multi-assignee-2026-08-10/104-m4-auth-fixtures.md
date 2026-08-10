# Auth fixtures (no secrets)

- Account A (creator/admin): admin.dn@example.com membership m_102 company c_001
- Assignee A: truong.phong@example.com / m_103 (also used as d_legal head after DEV fixture patch)
- Assignee B: truong.nhom@example.com / m_104
- Staff (limited confirm RBAC): nhanvien@example.com / m_105
- Password/JWT not stored in evidence

DEV fixture note: PATCH /api/v1/admin/departments/d_legal set head_membership_id=m_103 because prior only CNTT head was invalid (head not in department_memberships). Kept for DEV head-default QA.
