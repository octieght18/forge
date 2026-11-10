# F51 verification — 10 November 2026

F51 prepares publishable repository hygiene. [D66](decision-log.md#d66--f51-public-repository-hygiene) records the boundary. **G5 stays open.** The GitHub repository was **not** made public.

## Delivered

- [LICENSE](../LICENSE) (MIT)
- [CONTRIBUTING.md](../CONTRIBUTING.md)
- [SECURITY.md](../SECURITY.md)
- [.env.example](../.env.example)
- [public-repository.md](public-repository.md)
- [release-checklist.md](release-checklist.md)
- `.gitignore` entries for `.cursor/` and common private token filenames

## Checks

```text
python3 scripts/check-docs.py
go test -count=1 -timeout 60s ./internal/policy
```

Secrets scan: no committed `.env`, private keys, or Terraform state. Test-only secret strings remain for redaction assertions. Operator docs that already redact were not deleted.
