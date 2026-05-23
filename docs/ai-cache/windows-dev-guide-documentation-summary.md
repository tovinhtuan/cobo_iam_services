# Windows dev guide documentation summary

- Task type: documentation
- Objective/question: tao mot tai lieu huong dan chay `cobo_iam_services` tren Windows ma khong can `make`, dong thoi mapping cac target `make` sang PowerShell/Docker/Go/NPM tuong duong
- Created by: documentation-and-adrs skill
- Created at: 2026-05-22
- Last updated: 2026-05-22
- Validity: den khi `Makefile`, `docker-compose.dev.yml`, `docker-compose.artifacts.yml`, `deploy-dev.sh`, `deploy-artifacts/push-migration.sh`, hoac script FE thay doi dang ke

## Summary

Da them tai lieu `docs/windows-dev-guide.md` de ho tro may Windows khong co `make`.

Tai lieu moi uu tien 3 luong:

1. Chay local full stack bang `docker compose -f .\docker-compose.dev.yml up --build`
2. Chay truc tiep API, Worker, Frontend bang `go run` va `npm run dev`
3. Deploy len dev server bang `ssh`, `scp`, `go build`, `npm run build` thay cho `make deploy-*`

## Key decisions

- Uu tien Docker Compose cho Windows vi day la luong on dinh nhat va gan nhat voi cac target `dc-*` trong `Makefile`.
- Giu PowerShell la shell chinh; khong yeu cau Git Bash hay WSL.
- Tich hop bang mapping `make -> PowerShell` de user khong phai tu doc `Makefile`.
- Ghi ro gotcha Windows:
  - `curl.exe` thay cho alias `curl`
  - `npm run clean` khong portable vi dang dung `rm -rf dist`
  - `docker compose` co the ghi status sang stderr trong PowerShell nhung van thanh cong

## Source references

- `cobo_iam_services/Makefile`
- `cobo_iam_services/docker-compose.dev.yml`
- `cobo_iam_services/deploy-dev.sh`
- `cobo_iam_services/deploy-artifacts/push-migration.sh`
- `cobo_iam_services/docs/build-run.md`
- `cobo_iam_services/README.md`
- `cobo_iam_services/docs/deploy-dev-guide.md`
- `cobo_web_design/package.json`

## Remaining gaps / follow-up

- Neu team muon thao tac hang ngay gon hon tren Windows, buoc tiep theo hop ly la them `scripts/dev-windows.ps1` va `scripts/deploy-dev-windows.ps1`.
- `npm run clean` trong `cobo_web_design/package.json` van khong portable tren Windows; neu can, nen doi thanh script cross-platform.
