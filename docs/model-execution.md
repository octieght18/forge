# Model execution — F24

The worker can finish a research workflow after retrieval. The selected hosted backend is OpenRouter with `google/gemma-4-26b-a4b-it:free`, no paid fallback, and `provider.data_collection` set to `deny`. [D39](decision-log.md#d39--f24-model-execution-and-evidence-based-results) records that choice. Tests use a deterministic fixture and do not call the network.

## Secrets

Set `FORGE_MODEL_API_KEY` in the worker environment. The key is sent only as an `Authorization` header. It is not written into the request body, the workflow result, or an error string. The repository does not contain a key. `FORGE_MODEL_FIXTURE=true` selects the fixture when no key is set. With neither setting, generation stays `model_unavailable` and the workflow finishes `fail_closed`.

## Result

A configured model receives the frozen question and the passages `corpus.search` already authorized. The worker checks the model output against the report schema. A claim may cite only evidence IDs from those passages. A citation outside that set is `model_invalid` and is not stored.

The stored report records the outcome, the requested and actual model IDs, the provider, the evidence IDs, token usage, and limitations. `insufficient_evidence` is the uncertainty status when retrieval returned no passage. The durable copy is the workflow result in Temporal history. This process also keeps the report in memory for the worker lifetime. F24 does not add a product report table or a public run route.

The fixture cites the first passage and records prompt and completion sizes from the question and the claim. The OpenRouter client records the usage object from the HTTP response.
