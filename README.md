# freshservice-mcp

An MCP server for the [Freshservice](https://www.freshworks.com/freshservice/) API v2, written in Go. It serves over stdio or streamable HTTP and is packaged as a Nix flake with a NixOS module.

It calls the REST API directly with an agent API key, so unlike the claude.ai Freshservice connector it has no monthly action budget; it does share the account's per-minute rate limit with every other app ([ADR 0001](docs/adr/0001-own-mcp-not-claude-connector.md)).

The server is read-only by default. Writes are turned on per **capability** (tickets, ITIL, assets, ...), every write needs a reason that goes to the audit log, and nothing can be deleted.

## Tools

| Group | Tools |
|---|---|
| core (always on) | `freshservice_status` (connection, workspaces, asset API and rate-limit probe), `freshservice_api` (any documented `/api/v2/` route, capability-gated), `freshservice_workspace`, `freshservice_ticket` (incidents and service requests, conversations, tasks, time entries, approvals, CSAT, emails), `freshservice_requester` (+ requester groups), `freshservice_agent` (+ agent groups, roles, delegation), `freshservice_lookup` (departments, locations, business hours, SLA policies, canned responses) |
| itil | `freshservice_problem`, `freshservice_change` (+ CABs), `freshservice_release` (+ post-incident report templates) |
| assets | `freshservice_asset` (classic or ITAM, detected; relationships, asset types, ITAM devices and cloud), `freshservice_software`, `freshservice_contract`, `freshservice_procurement` (vendors, products, purchase orders) |
| knowledge | `freshservice_solution`, `freshservice_announcement` |
| catalog | `freshservice_catalog`, `freshservice_onboarding` (+ offboarding) |
| projects | `freshservice_project` (new-gen `/pm` and legacy) |
| ops | `freshservice_statuspage`, `freshservice_oncall`, `freshservice_alert`, `freshservice_journey` |
| custom | `freshservice_custom_object` |

Every tool takes an `action`: `list`, `get` and `filter` (a Freshservice query in `query`, e.g. `status:2 AND priority:>3`) where the API has them, plus named actions for sub-records. Lists:
- return brief fields plus `custom_fields` unless you pass `fields`;
- page for you up to `limit` (default 500, capped by `--max-records`) and 60 KiB, adding a `_truncation` note when they have to cut;
- return `next_cursor` when there is more. Pass it back as `cursor`.

`get` results carry a `url` to the record in the agent portal. On `freshservice_asset`, `id` is the asset's Display ID.

**Summaries.** `count` and `group_by` on tickets, problems, changes, releases, assets, requesters and agents; `backlog` (open tickets by status × group or priority, plus overdue) and `trend` (created and resolved per day or week) on tickets. The ticket filter returns a total, so a ticket count is one API call and a `group_by` on a fixed-value field is one call per value. Other resources tally by scanning up to `--max-records`. A summary needing more than `--max-buckets` calls is refused with the count, and one stops early, marked `partial`, when under 20% of the account's rate budget is left. `trend`'s resolved series approximates (resolved or closed tickets last updated in the period).

## Workspaces

Every workspace-scoped tool takes `workspace`: an id, a name (`"HR"`) or `"all"`. Without it the server sends its **default workspace** (`--default-workspace`, else the primary), never nothing, because Freshservice reads a missing `workspace_id` as "primary only".
- `"all"` maps to `0` on most lists, which Freshservice answers with **global fields only**: workspace custom fields are dropped, and the result says so in `_note`.
- SLA policies, business hours and canned responses map `"all"` to the global ones; agent groups cannot be listed across workspaces at all.
- Account-level records (requesters, agents, departments, locations, vendors, products, asset types) take no workspace.
- Creating a workspace-scoped record needs an explicit `workspace`; the default is never applied silently to a write.

## Write capabilities

All are off by default. A disabled capability's actions are removed from the tool schemas and also refused by the handler.

| Flag | Unlocks |
|---|---|
| `--allow-tickets` | ticket and service request create and update, child tickets, notes, tasks, time entries, approval requests (request, cancel, remind), requested items, move workspace, restore, catalog `place_request` |
| `--allow-ticket-replies` | replies and email/Zoom collaboration: anything that emails a requester or a third party |
| `--allow-itil` | problems, changes and releases (create, update, move, notes, tasks, time entries, restore), change approval management, CABs, major incident promote/demote, post-incident report templates |
| `--allow-assets` | assets (classic and ITAM), components, CMDB relationships, asset types, software (+ installations and users), contracts, vendors, products, purchase orders |
| `--allow-knowledge` | solution categories, folders and articles (create, update, publish, approval, restore), announcements |
| `--allow-projects` | new-gen and legacy projects and tasks |
| `--allow-people` | requesters (create, update, merge, reactivate), requester groups and membership, departments, locations, onboarding and offboarding requests |
| `--allow-approvals` | contract approve/reject as the API key's agent, approval delegation |
| `--allow-ops` | status page incidents and maintenances (published from a ticket or change) and subscribers, on-call schedules, shifts and escalation policies, alerts, journeys |
| `--allow-custom-objects` | custom object record create and update |

**Never exposed**, whatever the flags: every delete (including the `PUT` `delete_forever`, user deactivation and the GDPR `forget`), agent, role and agent-group writes, workspace writes, convert to agent, ticket form and catalog item administration, custom field definitions, the on-call roster override (it also deletes), and status page publishing from a maintenance window. The v2 API has no approve/reject for tickets, changes or releases.

Why capabilities rather than create/update/delete flags: Freshservice's HTTP verbs do not track risk. `delete_forever` is a `PUT`, a requester `DELETE` only deactivates, and a ticket update carrying `assets` silently replaces every linked asset. See [ADR 0002](docs/adr/0002-capability-map.md).

**Write safety:**
- Every write requires `reason`, which goes to the audit log (slog, stderr) with the tool, action, path and status.
- `confirm` must equal the record's subject or name for: move workspace, requester merge, a ticket update carrying `assets`, contract approve/reject, and every status page publish. A mismatch fails before anything is sent.
- Writes act as the API key's agent and are never retried automatically: Freshservice writes have no idempotency key, so a retried reply is a second email.
- `freshservice_api` routes every write through the same table as the curated tools, refuses routes it does not know and refuses `DELETE`. A test checks every documented write is either mapped to exactly one capability or refused with a reason.

## Configuration

| Flag | Env | Default |
|---|---|---|
| `--domain` (`acme`, `acme.freshservice.com` or a URL) | `FRESHSERVICE_DOMAIN` | required; https only, except to loopback |
| `--api-key-file` | `FRESHSERVICE_API_KEY_FILE` | see below |
| `--default-workspace` (id or name) | `FRESHSERVICE_DEFAULT_WORKSPACE` | the primary workspace |
| `--max-records` | `FRESHSERVICE_MAX_RECORDS` | 2000 |
| `--max-buckets` | `FRESHSERVICE_MAX_BUCKETS` | 60 |
| `--tool-groups` | `FRESHSERVICE_TOOL_GROUPS` | all |
| `--allow-<capability>` | | all off |
| `--stdio` / `--http` | | stdio |
| `--addr`, `--path` | | `127.0.0.1:8234`, `/mcp` |
| `--http-auth-token-file` | `FRESHSERVICE_MCP_HTTP_AUTH_TOKEN_FILE` | none (a non-loopback listener requires one) |
| `--request-timeout`, `--log-level` | `FRESHSERVICE_MCP_LOG_LEVEL` | `30s`, `info` |
| `--version` | | |

**API key.** The server looks for it in this order:
1. `--api-key-file`
2. `FRESHSERVICE_API_KEY_FILE`
3. `FRESHSERVICE_API_KEY` (logs a warning)
4. the systemd credential `api-key`

There is no flag that takes the key directly. Over HTTP, the bearer token is likewise read from `--http-auth-token-file` or the systemd credential `http-auth-token`.

### Getting an API key

Freshservice API keys belong to an agent, and the server can see and do exactly what that agent can.

1. Create a dedicated agent (for example `mcp@yourcompany.com`) rather than using a person's key.
2. Give it a role scoped to what you will enable: read-only roles for a read-only server; edit rights only for the modules whose capabilities you turn on. Add it to the workspaces it should see; restricted workspaces hide even from admins who are not members.
3. Signed in as that agent, open **Profile settings** and copy the API key from the right-hand pane.

`freshservice_status` shows the workspaces the key can see, the default workspace and which asset API answered.

## Nix

```nix
{
  inputs.freshservice-mcp.url = "github:lcleveland/freshservice-mcp";

  outputs = { nixpkgs, freshservice-mcp, ... }: {
    nixosConfigurations.host = nixpkgs.lib.nixosSystem {
      modules = [
        freshservice-mcp.nixosModules.default
        {
          services.freshservice-mcp = {
            enable = true;
            domain = "acme.freshservice.com";
            # sops-nix / agenix path, or a root-only file:
            #   printf %s '<key>' | sudo install -m 0400 /dev/stdin /persist/secrets/freshservice-api-key
            apiKeyFile = "/persist/secrets/freshservice-api-key";
            defaultWorkspace = "IT";
            allowTickets = true;
          };
        }
      ];
    };
  };
}
```

The module:
- passes secrets through systemd `LoadCredential`, so they never reach the Nix store, argv or the environment;
- runs the service as a hardened DynamicUser (the VM test holds the `systemd-analyze security` exposure score under 3);
- refuses a non-loopback listener without `bearerTokenFile`;
- warns when `allowTicketReplies`, `allowApprovals` or `allowOps` is on.

| Option | Default | Notes |
|---|---|---|
| `enable`, `package` | off, this flake's build | |
| `domain`, `apiKeyFile` | | the key file is a runtime path string, never a Nix path |
| `defaultWorkspace` | `null` (primary) | id or name |
| `allowTickets`, `allowTicketReplies`, `allowItil`, `allowAssets`, `allowKnowledge`, `allowProjects`, `allowPeople`, `allowApprovals`, `allowOps`, `allowCustomObjects` | `false` | one per capability |
| `maxRecords`, `maxBuckets`, `toolGroups` | 2000, 60, all | |
| `listenAddress`, `port`, `path` | `127.0.0.1`, 8234, `/mcp` | |
| `bearerTokenFile`, `openFirewall` | none, `false` | |
| `requestTimeout`, `logLevel` | `30s`, `info` | |
| `user`, `group` | DynamicUser | |
| `installCli` | `enable` | binary on PATH |
| `extraArgs`, `environment` | | never put a secret here |

To install only the CLI for stdio use on a workstation, set `services.freshservice-mcp.installCli = true;` and leave `enable` off. `overlays.default` provides `pkgs.freshservice-mcp`.

### Claude Code

Over HTTP, against the NixOS service:

```sh
claude mcp add --scope user --transport http freshservice http://127.0.0.1:8234/mcp
```

Add `--header "Authorization: Bearer <token>"` when the service has a `bearerTokenFile`.

Over stdio:

```sh
claude mcp add freshservice -e FRESHSERVICE_DOMAIN=acme.freshservice.com \
  -e FRESHSERVICE_API_KEY_FILE=$HOME/.config/freshservice-mcp/api-key -- freshservice-mcp
```

## Checked against a live tenant

These behaviours were confirmed on a real account ([#21](https://github.com/lcleveland/freshservice-mcp/issues/21)):
- **Ticket filter paging:** honours `per_page` up to 100 (the server sends 100). It returns nothing past its first **10,000 matches**: deeper pages come back empty with `total` 0, not an error. The server stops there with a `_truncation` note (or `partial` on a summary).
- **Other filters:** the change and requester queries take `per_page`. The asset filter rejects it and pages by 30. Only the ticket filter returns `total`, so the other summaries scan.
- **Rate limit:** per minute, not hourly, and `X-Ratelimit-Total` can change between minutes, so the server reads the budget from each response rather than assuming one.
- **Workspaces:** an unknown or inaccessible `workspace_id` on a list is 403, the same as a restricted workspace. `GET /workspaces/{id}` for one is 404.
- **`resolved_at`:** the ticket filter rejects it (400), so `trend`'s resolved series stays an approximation.
- **Portal links:** `get` links to `/helpdesk/tickets/{id}` and `/itil/{problems,changes,releases}/{id}`. The `/a/...` paths 404.

Not yet checked: whether `/workspaces` lists restricted workspaces the agent is not a member of.

## Development

```sh
nix develop            # go, gopls, nixfmt, jq
go test ./...          # if /tmp is noexec: GOTMPDIR=$PWD/.gotmp go test ./...
nix flake check        # package build + Go tests + module eval checks + VM test
nix build .#checks.x86_64-linux.vm   # the VM test alone (about a minute with KVM)
```

`internal/tools/testdata/{reads,writes}.txt` are every documented operation, extracted from the [endpoint inventory](docs/research/freshservice-endpoints.md); the coverage tests check every one is a tool view or deliberately excluded.

The design decisions are recorded on [Freshservice MCP in Go (wayfinder map)](https://github.com/lcleveland/freshservice-mcp/issues/1). Research notes are in [`docs/research/`](docs/research/).
