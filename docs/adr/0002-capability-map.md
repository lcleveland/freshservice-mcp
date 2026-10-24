# Writes gated by ten capabilities, classified per operation

Following ninjaone-mcp's ADR 0001, Freshservice writes are gated by **capability**, not by HTTP verb. Freshservice's verbs don't track risk: asset `delete_forever` is a PUT, user DELETE only deactivates, `forget` is a GDPR erase, and `PUT /tickets/[id]` with `assets` silently replaces the linked set. So every operation in the [endpoint inventory](../research/freshservice-endpoints.md) is classified by hand.

There are ten opt-in flags, all off by default: `tickets`, `ticket-replies` (anything that emails someone), `itil`, `assets`, `knowledge`, `projects`, `people`, `approvals` (votes cast as the API key's agent), `ops` (status page, on-call, alerts, journeys) and `custom-objects`. Approve/reject, status page publishing, ticket updates that carry `assets`, and merge/move-workspace/convert also need `confirm`.

Never exposed: every delete (including `delete_forever`, `forget`, deactivation and bulk deletes), agent/role/agent-group writes, workspace writes, catalog item admin, and custom object definitions. The `freshservice_api` raw-API tool obeys the same flags through a method+path route table and rejects unknown write routes. Read tool groups are all on by default, and the asset path (classic `/assets` or `/itam`) is detected at startup.

See [Capability and tool-group map](https://github.com/lcleveland/freshservice-mcp/issues/5).

**Amended during the build.** The v2 API documents no approve/reject endpoints for tickets, changes or releases (only for requesting, cancelling and reminding approvals, which sit under `tickets`/`itil`). So `approvals` covers contract approve/reject and approval delegation. Two more writes are never exposed: the on-call roster override, because the same call also deletes overrides; and status page publishing from a maintenance window, because a window has no readable record to `confirm` against (publish from the change instead).

**Amended 2026-10: reversible deletes and unlinks.** Two kinds of `DELETE` are exposed after all; every other delete stays refused.
- **Unlinks** (since the build): `remove_group_member` (requester group), and software `remove_installations` and `remove_users`. They detach records and keep both; they sit under `people` and `assets`.
- **`soft-delete`**, a new opt-in capability that is off by default: ticket `trash` (`DELETE /tickets/[id]`, undone by `restore`), requester `deactivate` and agent `deactivate` (`DELETE /requesters/[id]`, `/agents/[id]`, undone by `reactivate`). Each needs `confirm` (the subject or email) and is marked destructive. Trashing a parent service request also trashes its children. `forget`, `delete_forever` and every other delete stay refused, and `freshservice_api` still refuses `DELETE`.

**Amended 2026-10: agent and agent-group admin.** A new opt-in `admin` capability, off by default, unlocks agent create, update and reactivate, agent-to-requester and requester-to-agent conversion, and agent group create and update. Conversion needs `confirm` (email), as does an agent update that sends `roles`, because that changes what the agent may do. Role writes, workspace writes, catalog item admin and custom field definitions stay refused.
