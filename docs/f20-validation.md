# F20 verification — 11 October 2026

F20 publishes the [self-service contract](self-service-contract.md). [D34](decision-log.md#d34--f20-self-service-contract) records the boundary. The document explains the path from a provision request to the environment controller, the lifecycle of both records, the namespace isolation choice, cleanup, and the recovery samples from [F18](f18-validation.md) and [F19](f19-validation.md).

No API route, database grant, or controller permission changes. The installed controller still cannot write operation rows. Stronger isolation than one namespace per workload remains a later decision.

Checked with the documentation and PowerShell parsers on 11 October 2026. Windows Go tests for the template, CLI, and environment packages were already green on `69de030`; this ticket adds no Go change, so those packages were not re-run as a behavior check.

Documentation checks reported 51 Markdown documents and 376 relative links. The PowerShell checker parsed 7 scripts and 29 documented PowerShell blocks.
