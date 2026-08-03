# FE follow-up plan (after Backend DEV verified)

1. Extend `CompanyProfile` + raw mapper for `plan`  
2. Extend `AuthorizedCompany` + `/me/companies` normalizer  
3. Render badge in company overview next to company name (`CompanyProfilePage` overview — not legal/contact)  
4. Remove `user.subscriptionTier` badge from `PersonalOpsScreen`  
5. Query keys include `companyId`  
6. Tests + `make deploy-fe` + smoke  
7. Evidence pack separate from this Backend plan  

**Forbidden:** map `user.subscriptionTier` → company badge.
