# F02 documentation validation

Date: 5 October 2026

F02 is an architecture/documentation task. Its evidence is the architecture comparison, four ADRs, owner decisions D09–D14 and this repository change. No API, workflow, auth or deployment implementation is claimed.

Checks for this change:

- Resolve all local Markdown links and section anchors across README/docs.
- Check fenced-block balance, normalize document line endings and run Git whitespace checks (allowing intentional Markdown hard breaks).
- Map the three Wekan acceptance criteria to the architecture tables, model/deployment ADR and build-boundary ADR.
- Record the explicit owner amendment from a managed cloud target to Compose first/local kind later.
- Verify the selected free model and endpoint against OpenRouter's public catalog with zero prompt/completion prices.

These checks passed before commit. Runtime tests were not run: no application/runtime code exists yet. The architecture test matrix describes future obligations, not test results.

Material limitations: actual free-route eligibility under the requested data policy requires account-level verification during implementation; free endpoint availability/rate limits can change; schema/citation integrity is not proof of claim support; a crash before persisting a model response can repeat inference. Keycloak adds first-release setup work. Numerical thresholds remain deferred to F08 and backlog dates remain unrevised.

[Architecture](architecture.md), [ADR index](adr/README.md), [owner decisions](decision-log.md).
