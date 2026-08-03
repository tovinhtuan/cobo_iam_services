# Rollout plan

1. Close Phase 0 approvals  
2. Merge Backend Reader + additive API  
3. If Case C: migrate DEV → seed  
4. Deploy Backend DEV  
5. Verify old FE (ignores `plan`)  
6. API smoke matrix  
7. Merge FE consumer (badge + remove personal) — optional FE flag if desired  
8. FE DEV deploy + authenticated smoke (Premium company + non-Premium + switch)  
9. Only then remove personal Premium in production FE  

Feature flag: **not required** for additive Backend field; **optional** FE flag for badge cutover. Explain: old FE safe; new FE fail-closed.
