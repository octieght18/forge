# F31 verification — 12 October 2026

F31 writes the tenant threat model. [D46](decision-log.md#d46--f31-tenant-threat-model) records the boundary. The model is [threat-model.md](threat-model.md).

G3 is still an open gate. This ticket does not mark it done.

The model names spoofing, confused deputy, cross-tenant data access, SSRF, and tool abuse. It states the controls already in the code: Keycloak verification, owner isolation, operator read without mutation of another owner's records, and the read-only MCP command. It does not claim production isolation. The environment controller stays privileged infrastructure. Tenant and project identifiers are defined and are not stored yet.

Checked:

```text
python3 scripts/check-docs.py
pwsh -File scripts/check-powershell.ps1
```

No API or schema change was made.
