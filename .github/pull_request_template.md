<!-- Checklist structure inspired by the local Space.h pull request template. -->

## Summary

- 

## Change Type

- [ ] Foundation/runtime
- [ ] Discord surface
- [ ] Plugin/runtime capability
- [ ] Database/storage
- [ ] Documentation
- [ ] CI / deployment
- [ ] Security/privacy

## Quality Review

- [ ] Scope is focused and unrelated churn is avoided
- [ ] Code follows existing Go and Docker Compose patterns
- [ ] User-facing or operator-facing behavior is described clearly
- [ ] Edge cases and failure paths were considered
- [ ] External resources, references, or borrowed material are credited
- [ ] No secrets, credentials, private Discord content, or local-only artifacts are committed
- [ ] Agent artifacts remain ignored and unstaged

## Tests and Verification

- [ ] `gofmt` passes or formatting is not affected
- [ ] `go vet ./...` passes or Go code is not affected
- [ ] `go test ./...` passes or tests are not affected
- [ ] `go build ./cmd/gigi` passes or build is not affected
- [ ] Docker Compose smoke passes or deploy/runtime path is not affected
- [ ] New/changed behavior has tests, or test gap is explained

## Deployment Risk

- [ ] Safe to merge to `main`
- [ ] Coolify impact is described if deployment behavior changes
- [ ] Database/schema impact is described if storage changes
- [ ] Rollback or mitigation notes included if risk is non-trivial

## Notes

- 
