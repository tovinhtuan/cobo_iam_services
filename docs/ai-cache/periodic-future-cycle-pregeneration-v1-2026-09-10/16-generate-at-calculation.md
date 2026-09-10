# GenerateAt

GenerateAtDate(T, lead) = T.HCM_date.AddDate(0,0,-leadDays)
Eligible when TodayHCM >= GenerateAtDate
Anchor = T only (not PublishedAt/DueAt/OpenAt/AF)
