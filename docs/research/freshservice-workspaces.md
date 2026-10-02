# Freshservice workspace semantics (API v2)

Research for issue #4. Question: how do workspaces behave in the Freshservice v2 API on an account with several workspaces — which endpoints take `workspace_id`, what an omitted `workspace_id` returns, which resources are global vs workspace-scoped, and how the API key's agent role limits visibility.

Sources (read 2026-10-01):

- **[API]** Freshservice API reference, https://api.freshservice.com/ (single page; section names given in each citation).
- **[KB-primary]** "What is a primary workspace?", https://support.freshservice.com/support/solutions/articles/50000005552-what-is-a-primary-workspace-
- **[KB-manage]** "Managing multiple workspaces", https://support.freshservice.com/support/solutions/articles/50000005585-managing-multiple-workspaces
- **[KB-admin]** "How does administration differ between a single workspace account and a multiple workspaces account?", https://support.freshservice.com/support/solutions/articles/50000006117
- **[KB-roles]** "How do Roles and Permissions work in Single and Multiple workspace setups?", https://support.freshservice.com/support/solutions/articles/50000008929
- **[KB-restricted]** "Managing Workspace Managers for Restricted Workspaces", https://support.freshservice.com/support/solutions/articles/50000011435

This covers Freshservice / Freshservice for Business Teams (ESM). Freshservice for MSPs reuses `workspace_id` to mean "client" and behaves differently in places; those notes are left out unless they help.

## TL;DR for the MCP server

1. **Leaving out `workspace_id` does not mean "everything".** On almost every list endpoint it means **primary workspace only**. If the tool's default is "omit the param", results silently miss every other workspace.
2. **`workspace_id=0` means "all workspaces this agent can access"** on tickets, problems, changes, releases, assets, software, contracts, purchase orders, solution categories, and announcements. For ITSM records the docs say it returns **global fields only**, so workspace-specific custom fields are dropped.
3. **`0` does not work everywhere.** Agent groups cannot be listed across workspaces at all. Business hours, canned responses, and SLA policies use `1` for "global". Locations, departments, and requesters are account-level, so `workspace_id` does not apply to them in ESM.
4. **IDs:** `1` = global settings, `2` = primary workspace (by default), `>=2` = real workspaces. List them with `GET /api/v2/workspaces`, where each entry has `primary: true/false`.
5. **Visibility is the API-key agent's.** Restricted workspaces are hidden even from account admins unless the agent is a member.

Recommended default: the tool's `workspace_id` param, when omitted, uses a server-configured default. That default is `0` for record lists that support it and the primary workspace (from `GET /workspaces`) everywhere else. Endpoints where `0` is not documented should get a real ID or nothing at all.

## 1. Workspace IDs and the workspace list

- `GET /api/v2/workspaces` "Retrieves information about all the workspaces created in the account". `GET /api/v2/workspaces/[id]` views one. OAuth scope `freshservice.workspaces.view`. [API, "List All Workspaces / Clients", "View a Workspace / Client"]
- The workspace object has `id`, `name`, `description`, `type` ("Workspace" in ESM), `primary` (bool), `restricted` (bool), `state` (`active`/`inactive`), `template_name` (e.g. `it`, `hr`), `created_at`, `updated_at`. In the sample response, IT is `id: 2, primary: true` and HR is `id: 3, primary: false`. [API, "Workspace/Client" (#workspaces), "List All Workspaces / Clients"]
- "If available by default, id = 2 refers to the primary workspace." [API, "Workspace/Client"]
- `workspace_id = 1` is "Global" (system-generated). [API, "Create Client"; also "View List of SLAs", "View List of Canned Responses"]
- "The first workspace created in the account is tagged as the primary workspace." "On creating more than one workspace, the primary workspace cannot be changed." [KB-primary]
- Create, update, and delete on `/workspaces` are documented as MSP-only ("Create Client", "Update a Client", "Delete a Client"). In ESM the API is read-only for workspaces. [API]

## 2. What omitting `workspace_id` does, per endpoint

### List endpoints: omitted = primary only, `0` = all accessible

| Endpoint | Omitted | `0` | Source ([API] section) |
|---|---|---|---|
| `GET /tickets` | primary workspace only | all workspaces, "just the global fields" | View List of Tickets (#view_all_ticket), note 3 |
| `GET /tickets/filter` | primary workspace (example 8 shows `workspace_id=0` for "all accessible workspaces") | all, global fields only | Filter Tickets |
| `GET /problems` | primary only | all, global fields only | View List of Problems |
| `GET /changes` | primary only | all, global fields only | View List of Changes |
| `GET /releases` | primary only | all, global fields only | View list of Releases |
| `GET /assets` (and `/assets?filter=`) | primary only | all workspaces ("only global level fields") | View List of Assets, note 5; Filter Assets, note 12 |
| `GET /applications` (software) | primary only | all | List all Software |
| `GET /contracts` | primary only | all | List all contracts |
| `GET /purchase_orders` | (not stated) | all | List all Purchase Orders |
| `GET /solutions/categories` | primary only | all | View List of Solution Categories |
| `GET /announcements` | primary only | all | List all Announcements |
| `GET /ticket_form_fields` (and problem/change/release fields) | "only global fields and fields from the primary workspace" | not documented | List all Ticket Fields, etc. |
| `GET /service_catalog/items` | primary only ("defaults to primary workspace if not specified") | not documented | View List of Service Items |
| `GET /service_catalog/categories` | primary only | not documented | View List of Service Categories |
| `GET /objects` (custom objects) | primary only | not documented | View List of Custom Objects |
| `GET /canned_responses`, `/canned_response_folders` | primary only; `1` = global canned responses | not documented | View List of Canned Responses / Folders |
| `GET /business_hours` | primary only; `1` = all global business hours | not documented | View List of Business Hours Configurations |
| `GET /sla_policies` | global + primary; `1` = global only; `>1` = that workspace + global | not documented | View List of SLAs |
| `GET /groups` (agent groups) | primary only | **impossible**: "It is not possible to retrieve the list of agent groups across all workspaces." | List all Groups; Changelog |
| `GET /status/pages` | **all workspaces** (the exception) | n/a | List all status pages |
| Audit log export | omitted or `0` = workspaces the user can access; `1` = global-settings entries only | n/a | Audit Log Export |

All the "omitted = primary" notes carry the qualifier "Applicable only to accounts with workspaces."

On-call management puts the workspace in the **path**, so it is required: `/api/v2/oncall/ws/[workspace_id]/schedules...`. [API, "Create a schedule", "View all schedules"]

### Creates: omitted = primary

- Tickets, child tickets, problems, changes, releases, assets, software, contracts, purchase orders, and announcements all say: "If not provided, the ID of the primary workspace will be defaulted." [API, property tables for each module]
- Tasks and time entries inherit from the parent ticket/problem/change/release. Time entry `workspace_id` is READ-ONLY. [API, "Create a Task", "Time Entries"]
- Agent groups default to `2` (primary). [API, "Create a Group"]
- Solution categories default to the primary workspace. [API, "Create Solution Category"]
- Service catalog `place_request`: `workspace_id` "Minimum 2". Service items cannot be created in workspace `1`. [API, "Create a Service Request", "Create Service Catalog item"]
- Support doc, same rule: tickets from "marketplace apps, integrations, or API calls" with no workspace go to the primary workspace. Exceptions: service-catalog tickets go to the item's home workspace, and email tickets go to the mailbox's workspace. [KB-primary]

### Get/update by ID and moving

- `GET /tickets/[id]` and the other single-record reads take no `workspace_id` in the docs. A record is fetched by its ID and its `workspace_id` comes back in the response. (Inference: no workspace param is documented on view endpoints.) [API, "View a Ticket"]
- "The workspace_id attribute cannot be updated through the Update operation. It can only be updated through the Move operation." The move endpoints are `PUT /tickets/[id]/move_workspace`, `/problems/[id]/move_workspace`, `/changes/[id]/move_workspace`, `/releases/[id]/move_workspace`, `/assets/[display_id]/move_workspace`, `/applications/[id]/move_workspace`, `/contracts/[id]/move_workspace`, and `/purchase_orders/[id]/move_workspace`. [API, "Update a Ticket" note 3, "Move a Ticket", etc.]
- A group's `workspace_id` "cannot be modified". [API, "Update a Group"]

## 3. Global vs workspace-scoped resources (ESM)

**Account-level (global). `workspace_id` does not apply:**

- Requesters: "Requesters are created at the account level." [API, "List All Requesters/Contacts"]
- Agents: "Agent is defined at account level." They carry `workspace_ids` (the workspaces the agent is added to) and `workspace_info`. [API, "Agents"]
- Locations: "defined at the account level". Departments: "maintained at the overall account level". [API, "List all Locations"; "Tickets API behavior" callout]
  - The notes "returns all locations/departments with workspace_id=2" and "value should be greater than 1" are MSP client semantics. [API, "List all Locations", "List all Departments"]
- Vendors, products, and asset types: no workspace attribute anywhere in their sections (inference from absence). [API]
- The support docs list Agents, Roles, Departments, Requesters, "Assets, vendors and discovery configurations" as global admin settings. [KB-manage]
  - This means asset *configuration* (asset types) is global, while asset *records* have a `workspace_id` and can be moved. [API, "Assets"]

**Workspace-scoped (the record has a single `workspace_id`):**

- Tickets/service requests, problems, changes, releases, tasks, time entries, assets, software, contracts, purchase orders, agent groups, solution categories/folders/articles, service items and categories, announcements, canned responses, custom objects, on-call schedules. [API, property tables]
- The support docs list service catalog, canned responses, scenario automations, and tags as workspace-only. [KB-manage]

**Both global and per-workspace (global = `workspace_id 1`):**

- Form fields, SLA policies, business hours, workflow automator, email notifications, and more. [KB-manage]
- In the API this shows up as: field lists return "global fields and fields from the primary workspace", SLA lists include global policies, and business hours and canned responses accept `1`. [API]

## 4. How the API key's agent role limits visibility

- The API key is the agent's personal key: "Your ability to access data depends on the permissions available for your ... profile". The API enforces the same role restrictions as the UI. [API, "Authentication — Will everyone have the same access rights?"]
- `workspace_id=0` returns tickets "from workspaces to which the user has access", not literally every workspace. The same wording appears for problems, releases, assets, software, contracts, purchase orders, and announcements. [API, examples in each "View List" section]
- Standard agent permissions "can only be granted within a workspace". Only admin roles can be account-wide. An account-wide admin is "auto-added to all non-restricted workspaces". [KB-roles, KB-admin]
- Restricted workspaces: "account-wide admins are automatically removed from it, preventing them from ... accessing data without the workspace teams' permission." [KB-restricted] The `restricted` flag is in the `/workspaces` response. [API]
  - Not documented: whether `GET /workspaces` lists restricted workspaces to an agent who isn't a member. Verify once against the live account.
- An agent's role assignments are per workspace. The `roles[].assignment_scope` (`entire_helpdesk`, `member_groups`, `specified_groups`, `assigned_items`) further narrows which tickets inside a workspace are visible. [API, "Agents" roles attribute]

## 5. Implications and open points

- **Default:** omitting the tool param must not pass through as "omitted" to list endpoints, because that silently means primary-only. The server should resolve a default explicitly:
  - `0` where documented (section 2 table)
  - otherwise the configured or primary workspace ID
  - never `0` to `/groups`, fields, catalog, or canned-response endpoints, where it is undocumented
- **The cost of `0`:** with `0`, ITSM lists return global fields only, so workspace custom fields are missing. If the caller needs custom fields, it should scope to one workspace.
- **Account-level tools** (requesters, agents, locations, departments, vendors, products, asset types): drop or ignore `workspace_id`.
- **Writes:** creates default to primary. An MCP create tool should probably *require* `workspace_id`, or state the primary default clearly. Changing the workspace of an existing record needs the `move_workspace` endpoint, not update.
- **Unverified** (docs are silent; check once against the live API, not via the budgeted MCP):
  - what an invalid or inaccessible `workspace_id` returns (403 vs empty)
  - whether `/workspaces` hides restricted workspaces the agent can't access
  - whether `GET /tickets/[id]` for a ticket in an inaccessible workspace returns 403 or 404
