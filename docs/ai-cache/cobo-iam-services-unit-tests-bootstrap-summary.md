# Unit tests bootstrap — summary

## Vi tri

- `internal/iam/app/service_test.go` — package `app_test`, dung in-memory IAM + companyaccess adapters.
- `internal/authorization/app/service_test.go` — package `app_test`, dung `authinmem` repository/resolver/checker mac dinh.

## Chay

- `go test ./...` tu root module `cobo_iam_services`.

## Chua lam (theo implementation-step-by-step mục 6)

- Integration HTTP + MySQL, migration smoke, P1/P2 projection/cache tests.
