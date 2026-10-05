# ADR 0004 — OpenRouter inference and local Kubernetes target

Date: 5 October 2026  
Status: Accepted backend/deployment choices; free model selected under owner delegation on 5 October 2026.

## Context

The first workflow uses a fixed corpus and needs cited reports. The current PC has approximately 16 GB installed RAM. Cloud deployment is outside the first release, but the original plan requires an explicit later target and cost/reversibility comparison.

## Decision and alternatives

The owner selected OpenRouter and local Kubernetes on 5 October 2026. This explicitly supersedes a managed cloud selection in F02. The owner accepted Docker Compose first and kind later, and delegated selection of a free model for testing. Select `google/gemma-4-26b-a4b-it:free`, verified in the live public catalog with zero input/output token prices. A paid model is not selected.

Use the direct HTTP adapter, public/synthetic fixtures and no paid/model fallback. The selected provider endpoint is `google-ai-studio`; keep provider fallback disabled and request the no-data-collection filter. Actual eligibility under that filter remains an implementation smoke-check obligation, not a result proven by catalog metadata. Failure must not silently switch to paid inference or weaken the filter. Free-tier rate limits, variable availability and output-schema validation are material limitations. [Model](https://openrouter.ai/google/gemma-4-26b-a4b-it:free), [catalog](https://openrouter.ai/api/v1/models), [routing controls](https://openrouter.ai/docs/guides/routing/provider-selection).

Hosted inference reduces local model-serving work but transfers supplied excerpts to the provider and adds usage charges. Ollama with a local model avoids that transfer but needs quality/latency/memory evaluation. vLLM is deferred because owning serving infrastructure adds no accepted release capability.

AWS EKS aligns with the original plan and the owner's Amazon experience; GKE is a portable Kubernetes alternative with different IAM/networking/billing. Neither is needed to run the local first release. Pricing is dated in the architecture document and excludes ancillary services.

## Reversibility and gates

A narrow model adapter limits code coupling, but changing model/provider requires repeating evidence/uncertainty tests and rechecking privacy and prices. Store the actual model/config identifier per run; do not depend on an untracked “latest” alias.

Cloud portability is at the application/container boundary, not an assertion that IAM/Terraform/network infrastructure is interchangeable. Region, spend ceiling, identities, persistent storage and exposure require later owner decisions. This ADR does not authorize provisioning, credentials creation or model calls.

See [cost and alternatives](../architecture.md#7-model-and-cloud-decision-record).
