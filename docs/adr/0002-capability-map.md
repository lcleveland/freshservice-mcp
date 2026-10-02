# Writes gated by ten capabilities, classified per operation

Following ninjaone-mcp's ADR 0001, Freshservice writes are gated by **capability**, not by HTTP verb. Freshservice's verbs don't track risk: asset `delete_forever` is a PUT, user DELETE only deactivates, `forget` is a GDPR erase, and `PUT /tickets/[id]` with `assets` silently replaces the linked set. So every operation in the [endpoint inventory](../research/freshservice-endpoints.md) is classified by hand.

There are ten opt-in flags, all off by default: `tickets`, `ticket-replies` (anything that emails someone), `itil`, `assets`, `knowledge`, `projects`, `people`, `approvals` (votes cast as the API key's agent), `ops` (status page, on-call, alerts, journeys) and `custom-objects`. Approve/reject, status page publishing, ticket updates that carry `assets`, and merge/move-workspace/convert also need `confirm`.

Never exposed: every delete (including `delete_forever`, `forget`, deactivation and bulk deletes), agent/role/agent-group writes, workspace writes, catalog item admin, and custom object definitions. The `freshservice_api` raw-API tool obeys the same flags through a method+path route table and rejects unknown write routes. Read tool groups are all on by default, and the asset path (classic `/assets` or `/itam`) is detected at startup.

See [Capability and tool-group map](https://github.com/lcleveland/freshservice-mcp/issues/5).
