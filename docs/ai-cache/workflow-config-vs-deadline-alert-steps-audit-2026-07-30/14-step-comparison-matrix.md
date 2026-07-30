# 14 — Step comparison matrix

| Index | CMS global config | Effective (global_template) | Snapshot | Alert steps API | Alert list UI | Match? |
|-------|-------------------|-----------------------------|----------|-----------------|---------------|--------|
| — | (empty 0) | 4 steps below | 4 steps | 4 steps | current only | dual-SoT |
| 1 | — | Tiếp nhận thông tin / role-reviewer | same | Tiếp nhận thông tin / current | Tiếp nhận thông tin | semantic OK |
| 2 | — | Chuẩn bị hồ sơ / role-finance-reviewer | same | Chuẩn bị hồ sơ | (hidden on list) | OK |
| 3 | — | Pháp chế rà soát / role-legal-reviewer | same | Pháp chế rà soát | (hidden) | OK |
| 4 | — | Phê duyệt / role-approver | same | Phê duyệt | (hidden) | OK |

Effective JSON codes: `["wf-step-1780040944730-4ytw97", "wf-step-1780040972266-8dij6p", "wf-step-1780040991507-2z1h2q", "wf-step-1780041023021-xmk7go"]`
Alert API codes: `["wf-step-1780040944730-4ytw97", "wf-step-1780040972266-8dij6p", "wf-step-1780040991507-2z1h2q", "wf-step-1780041023021-xmk7go"]`
semanticMatch(effective↔snapshot↔alert steps)=true
orderMatch=true
actorMatch(effective↔snapshot)=true
labelOnlyDifference=false for those layers
CMS global vs alert = different SoT (not semantic drift of same source)
