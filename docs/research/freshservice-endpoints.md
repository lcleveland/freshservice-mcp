# Freshservice API v2 endpoint inventory

Research for #2 (part of #1, blocks #5 and #7). Researched 2026-10-01.

## Sources

- **[S1] Freshservice API v2 reference**, https://api.freshservice.com/ (one single-page HTML document, fetched 2026-10-01, latest changelog entry "September 2026"). This is the only source for every module, method, path, OAuth scope, product tag and field list below. Each operation links to its anchor in [S1].
- Paths and counts were pulled out of [S1]'s HTML mechanically: each `<h2>` section, its `api-request-url` block(s), its `api-oauth-scope` block and its `api-mode-tag` product badges. Where the path shown in the heading block disagrees with the section's own curl sample, the curl sample was used and the row is listed under "Doc errata".

Claims marked **(inference)** are my own reasoning and are not documented. Check them against a sandbox before relying on them.

## Key findings

1. **The surface is large: 536 documented operations (494 distinct method+path pairs) in 49 modules.** 242 are reads, 215 writes, 79 deletes (classification below). The distinct count is lower because [S1] documents variants of the same call as separate operations (for example 3 "Create a Contract" variants, 4 "Create Solution Article" variants, Email and Zoom both on `POST /tickets/[id]/communications`).
2. **Everything is under `https://<domain>.freshservice.com/api/v2/`, JSON only, HTTPS only, and it does not work through custom CNAMEs** [S1 "What's New"]. Auth is HTTP Basic with the API key as the username and any password (`apikey:X`). Username/password auth was deprecated on 2023-05-31 [S1 "Authentication"]. OAuth 2.0 (Freshworks org) has existed since April 2024, and most operations list a scope [S1 "Changelog"]. **73 operations list no OAuth scope**: CABs, legacy projects, the service catalog item writes and shared fields, attachment download, `GET /approvals`, assignment history, and all ITAM endpoints.
3. **Pagination** uses `page` (starting at 1) and `per_page` (default 30, max 100; a value over 100 is an error). The next page is given in the `link` header with `rel="next"`, and the header is missing on the last page [S1 "Pagination"]. Several newer modules break this pattern: Alert logs use `start_token`, and on-call uses its own `page`/`per_page` query strings (see their rows). `GET /approvals` is capped at 30 per page [S1].
4. **Rate limits depend on the plan and are counted per minute, per account** (for accounts created on or after 2020-09-01). The overall limit is 100/200/400/500 per minute on Starter/Growth/Pro/Enterprise. There are sub-limits for list tickets, view/create/update ticket, list assets, update asset, list agents and list requesters (for example list tickets is 40/70/120/140). FSBT Pro and MSP Core get the Growth numbers. Paid add-ons (Pro and Enterprise only) raise the overall limit to 1000 or 2000. **Invalid requests count against the limit.** `include=` embeds cost +1 credit on a show and +2 or +3 on a list ([S1] says 3 in "Rate limit" and 2 in "Embedding"). Response headers: `X-Ratelimit-Total`, `X-Ratelimit-Remaining`, `X-Ratelimit-Used-CurrentRequest`, and `Retry-After` on a 429 [S1 "Rate limit"].
5. **There are three product flavors (FS = Freshservice, FSBT = Freshservice for Business Teams, MSP = Freshservice for MSPs) and a fourth tag for ITAM.** Each operation is badged with the products it works on (the Products column). ITIL modules (problems, changes, releases, assets, contracts, software, POs, products, vendors, alerts, on-call, status page, onboarding/offboarding, delegation) are **FS only**. Projects, journeys, custom objects and canned responses are **FS and FSBT, not MSP**. Client create/filter/update/delete is **MSP only**. In MSP, requesters are called "contacts" and departments "companies", and they are per client (`workspace_id`) [S1 module callouts].
6. **ITAM is a separate product, under `/api/v2/itam/...`.** It has its own assets, devices, physical subtypes, cloud resources and lifecycle events, and is "available only for new signups after the March 31, 2026 release" [S1]. Existing accounts use the classic `/api/v2/assets` (CMDB) API instead. **An MCP server built for an existing tenant should target classic assets and treat ITAM as optional (inference).**
7. **Deletes are often soft.** Tickets, problems, releases, assets and solution items go to trash and have a `restore` endpoint. Assets and solutions also have a permanent `delete_forever`. Note that asset `delete_forever` is a **`PUT`**, while solutions use `DELETE`. Requester/agent `DELETE` only *deactivates* the user (there is a `reactivate`). The `.../forget` endpoint is the permanent GDPR erase. A delete gate in the MCP server therefore has to key on the operation, not just on the HTTP method. **Some writes are not POST/PUT**: `POST /audit_log/export` and `POST /journeys/requests/view` are reads, `PUT .../approvals/[id]` *cancels* an approval, and contract approve/reject is `PUT /contracts/[id]?operation=...`.
8. **Bulk and destructive-by-replacement calls exist.** `DELETE /relationships?ids=`, `DELETE /applications?ids=` (bulk software delete), the bulk software users and installations calls, and `PUT /solutions/articles/bulk_restore`. Also, `PUT /tickets/[id]` with `assets` **replaces** the whole linked-asset set: "Existing assets, if they are different from what are given in request, are destroyed" [S1 "Update a Ticket"].
9. **No webhook or subscription endpoints, and no global time-entry or conversation listing.** Conversations can only be listed per ticket, and there is no "view one conversation" endpoint. Time entries, tasks and notes are always nested under a ticket, problem, change or release.

## Plan-gated, beta and deprecated items

| Item | Status | Source |
|---|---|---|
| Username/password Basic auth | **Deprecated** 2023-05-31. Use the API key or OAuth. | [S1 Authentication] |
| `POST /tickets/[id]/approvals` (Request Approval) | **"Planned for deprecation."** Accounts with Parallel Approvals must use Approval Groups. | [S1 Request Approval for a Service Request] |
| `associate_ci` single-asset format on ticket create/update | **"Will be deprecated soon."** Use `assets: [{display_id}]`. | [S1 Create a Ticket] |
| Asset filter `query=[query]` | **Deprecated** (end of June 2022). Use `filter=`. | [S1 Filter Assets] |
| Projects (legacy) `/api/v2/projects` | **Legacy**. Customers are being migrated to new-gen `/api/v2/pm/projects`, and the two are not compatible. | [S1 Projects (Legacy)] |
| Agent attributes `ticket_scope`, `problem_scope`, `change_scope`, `release_scope`, `scopes`, `group_ids`, `role_ids` | **No longer supported** (Aug 2023). Use `roles[].assignment_scope`, `member_of`/`observer_of`, `roles[].role_id`. | [S1 Changelog] |
| Child tickets | Not available on the **Sprout** plan. | [S1 Create a Child Ticket] |
| ITAM `/api/v2/itam/*` | Only for **new signups after 2026-03-31**. | [S1 ITAM sections] |
| Rate-limit add-ons | Pro and Enterprise only. | [S1 Rate limit] |
| `require_feature` error | Returned when a module's feature is not enabled on the portal. This is how plan-gated modules (problems, changes, releases, CMDB, on-call, alerts, status page, PIR, journeys, custom objects) fail **(inference: [S1] does not give a per-module plan matrix)**. | [S1 Errors] |
| Audit log export | "Applicable only to accounts with workspaces" for the `workspace_id` filter. | [S1 Audit Log Export] |
| Beta APIs | [S1 "Policies"] says beta APIs can change without notice but **does not mark any operation as beta**. Breaking changes to production APIs get 60 days' notice. | [S1 Policies] |

## Doc errata (path in heading block vs curl sample)

| Operation | Heading shows | Curl sample / used here |
|---|---|---|
| User Assignment History (in both Requesters and Agents) | `GET api/v2/users`, `GET api/v2/agents` | `GET /api/v2/users/[id]/assignment-history` |
| Asset Assignment History | `GET /api/v2/assets` | `GET /api/v2/assets/[display_id]/assignment-history` |
| CSAT Response | `/tickets/id]/csat_response` | `/api/v2/tickets/[id]/csat_response` |
| Filter Clients | `api/v2/workspaces/query=[query]` | `/api/v2/workspaces?query="name:'Acme'"` |
| Create Client | `POST api/v2/workspace` (singular) | Same in the curl sample. Every other workspace path is plural. **Verify in a sandbox.** |
| Legacy Update/View Project Task | `/projects/[pid]/task/[id]`, `.../tasks/[id]]` | `/api/v2/projects/[pid]/tasks/[id]` |
| Onboarding/Offboarding view | `/onboarding_requests/id` | `/onboarding_requests/[id]` |
| Delete multiple Software | `DELETE /api/v2/applications/` | `DELETE /api/v2/applications?ids=9216,9218` |
| Service catalog | Reads use `/service_catalog/...` (underscore). Item create/update/delete and shared fields use `/service-catalog/...` (hyphen). | Both forms are shown as documented. Do not normalize them. |
| Several paths | Missing the leading `/` | A leading `/` was added. |

## Classification rule

`read` = GET, plus the POST-but-read calls that are flagged. `write` = create, update, state transitions (approve, cancel, acknowledge, promote, move workspace, convert, merge) and restore. `delete` = DELETE, plus `PUT .../delete_forever`. Subtypes: *soft, reversible* (deactivate), *permanent*, *GDPR*. Removing members or links (`DELETE` of a group membership or software users) is listed as **write (unlink)**, because it does not destroy a record. Classification is **(inference)** from the HTTP method and the operation text in [S1].

## Endpoint count per module

| Module | Endpoints | Read | Write | Delete |
|---|---:|---:|---:|---:|
| Tickets | 18 | 7 | 8 | 3 |
| Service Requests | 5 | 1 | 4 | 0 |
| Ticket approvals | 10 | 3 | 7 | 0 |
| Major incidents | 2 | 0 | 2 | 0 |
| Ticket tasks | 5 | 2 | 2 | 1 |
| CSAT | 1 | 1 | 0 | 0 |
| Conversations | 6 | 1 | 3 | 2 |
| Problems | 23 | 9 | 10 | 4 |
| Changes | 31 | 12 | 15 | 4 |
| Releases | 24 | 10 | 10 | 4 |
| Change Advisory Boards (CABs) | 5 | 2 | 2 | 1 |
| Workspaces / Clients | 7 | 4 | 2 | 1 |
| Approvals (cross-module) | 1 | 1 | 0 | 0 |
| Requesters / Contacts | 12 | 5 | 5 | 2 |
| Agents | 11 | 5 | 4 | 2 |
| Agent roles | 2 | 2 | 0 | 0 |
| Agent groups | 5 | 2 | 2 | 1 |
| Requester groups / Contact groups | 8 | 3 | 4 | 1 |
| Locations | 6 | 3 | 2 | 1 |
| Products | 5 | 2 | 2 | 1 |
| Vendors | 5 | 2 | 2 | 1 |
| Alerts (Alert Management) | 13 | 5 | 6 | 2 |
| Assets / CMDB (classic) | 22 | 12 | 7 | 3 |
| ITAM assets (Freshservice IT Asset Management) | 7 | 2 | 4 | 1 |
| Purchase orders | 6 | 2 | 3 | 1 |
| Asset types | 6 | 3 | 2 | 1 |
| Software | 17 | 7 | 8 | 2 |
| Contracts | 14 | 6 | 8 | 0 |
| Departments / Companies | 7 | 4 | 2 | 1 |
| Business hours | 2 | 2 | 0 | 0 |
| Projects (legacy) | 12 | 4 | 6 | 2 |
| Projects (new-gen, /pm) | 36 | 17 | 11 | 8 |
| Solutions (knowledge base) | 30 | 8 | 16 | 6 |
| Service catalog | 16 | 6 | 8 | 2 |
| Announcements | 5 | 2 | 2 | 1 |
| Employee onboarding | 5 | 4 | 1 | 0 |
| Employee offboarding | 5 | 4 | 1 | 0 |
| Journeys | 10 | 6 | 3 | 1 |
| On-call management | 34 | 23 | 8 | 3 |
| Custom objects | 6 | 3 | 2 | 1 |
| Post-incident report templates | 7 | 3 | 3 | 1 |
| SLA policies | 1 | 1 | 0 | 0 |
| Canned responses | 5 | 5 | 0 | 0 |
| Audit logs | 1 | 1 | 0 | 0 |
| Attachments | 2 | 2 | 0 | 0 |
| Collaboration (Email, Zoom) | 7 | 4 | 3 | 0 |
| Status page | 39 | 18 | 14 | 7 |
| Delegation | 4 | 1 | 2 | 1 |
| ITAM physical subtypes, devices, cloud | 25 | 10 | 9 | 6 |
| **Total** | **536** | **242** | **215** | **79** |
## Key resource fields

These come from the attribute table at the top of each module in [S1]. `*` = required on create. Every main record also has `id`, `created_at` and `updated_at`, and has `workspace_id` where workspaces apply.

- **Ticket**: `subject`, `description`/`description_text`, `requester_id` or `email`/`phone`, `responder_id`, `group_id`, `department_id`, `status` (2 Open, 3 Pending, 4 Resolved, 5 Closed), `priority` (1-4 Low..Urgent), `urgency`, `impact`, `source` (1 Email, 2 Portal, 3 Phone, 4 Chat, 5 Feedback widget, 6 Yammer, 7 AWS Cloudwatch, 8 Pagerduty, 9 Walkup, 10 Slack), `type`, `category`/`sub_category`/`item_category`, `due_by`, `fr_due_by`, `is_escalated`, `fr_escalated`, `cc_emails`, `fwd_emails`, `reply_cc_emails`, `to_emails`, `email_config_id`, `tags`, `custom_fields`, `attachments`, `spam`, `deleted`, `resolution_notes(_html)`. List filters: `filter=` predefined views (for example `watching`), `updated_since`, `include=` embeds. Query search uses `/tickets/filter?query="..."` (URL-encoded, field names in snake case).
- **Conversation**: `body`/`body_text`, `incoming`, `private`, `source`, `user_id`, `ticket_id`, `to_emails`, `support_email`, `attachments`.
- **Time entry**: `agent_id`, `time_spent` ("hh:mm"), `billable`, `executed_at`, `start_time`, `end_time`, `timer_running`, `task_id`, `note`, `work_type`.
- **Task** (ticket/problem/change/release): `title`, `description`, `agent_id`, `group_id`, `status`, `due_date`, `notify_before`, `closed_at`.
- **Note** (problem/change/release): `body`/`body_text`, `user_id`, `notify_emails`.
- **Approval**: `approver_id`, `approval_type`, `approval_group`, `level`, `approval_status`, `delegatee`, `latest_remark`, `email_content`. **Approval group**: `name`, `approval_type`, `rule`, `level`, `approvals[]`.
- **CSAT response**: `overall_rating`, `overall_rating_text`, `primary_question`, `questionnaire_responses[]`.
- **Problem**: `subject*`, `description*`, `requester_id*`, `priority*`, `status*`, `impact*`, `due_by*`, `agent_id`, `group_id`, `department_id`, `known_error`, `category`..., `associated_change`, `custom_fields`, `analysis_fields`, `assets`.
- **Change**: `subject`, `description`, `requester_id`, `agent_id`, `group_id`, `priority*`, `impact*`, `status*`, `risk*`, `change_type*`, `approval_status`, `planned_start_date`, `planned_end_date`, `department_id`, `category`..., `custom_fields`, `maintenance_window`, `blackout_window`, `assets`, `impacted_services`. The list supports `query`, predefined `view`, and sorting (Nov 2024).
- **Release**: `subject`, `description`, `release_type`, `priority`, `status`, `planned_start/end_date`, `work_start/end_date`, `agent_id`, `group_id`, `department_id`, `associated_assets`, `associated_changes`, `custom_fields`, `planning_fields`, `assets`.
- **CAB**: `name`, `description`, `members` (max 100 IDs per call), `member_details`.
- **Workspace/Client**: `name`, `description`, `type`, `primary`, `restricted`, `state`, `template_name`. The MSP `metadata` has `primary_contact.{first_name,last_name,email,phone}`, `email_domains` and `custom_fields`.
- **Requester/Contact**: `first_name`, `last_name`, `primary_email*` / `work_phone_number*` / `mobile_phone_number*` (one of the three is required), `secondary_emails`, `job_title`, `department_ids`, `reporting_manager_id`, `location_id`, `time_zone`, `language`, `address`, `active`, `has_logged_in`, `is_agent`, `external_id` (IdP/SCIM), `custom_fields`, MSP `belongs_to_workspace_ids`. The filter uses `?query="..."`.
- **Agent**: same person fields, plus `roles[]` (`role_id`, `assignment_scope`, `groups`), `member_of`, `observer_of`, `occasional`, and MSP `belongs_to_workspace_ids` (see deprecated attributes). The fields come from `/agent_fields`.
- **Role**: `name`, `description`, `default`, `role_type`. Roles are read-only through the API.
- **Agent group**: `name`, `description`, `agent_ids`/`members`, `observers`, `leaders` (+`*_pending_approval`), `escalate_to`, `unassigned_for`, `business_hours_id`, `restricted`, `approval_required`, `auto_ticket_assign`.
- **Requester group**: `name`, `description`, `type` (manual/rule-based), `workspace_id`.
- **Location**: `name`, `parent_location_id`, `primary_contact_id`, `line1`, `line2`, `city`, `state`, `country`, `zipcode`, `email`, `phone`, `contact_name`.
- **Department/Company**: `name`, `description`, `head_user_id`, `prime_user_id`, `domains`, `custom_fields`.
- **Product**: `name`, `asset_type_id`, `manufacturer`, `status`, `mode_of_procurement`, `depreciation_type_id`, `description`. **Vendor**: `name`, `description`, `primary_contact_id`, address fields.
- **Asset (classic CMDB)**: `display_id` (the path key, not `id`), `name`, `description`, `asset_type_id`, `asset_tag`, `impact`, `usage_type`, `author_type`, `user_id`, `agent_id`, `group_id`, `location_id`, `department_id`, `assigned_on`, plus `type_fields` (per-asset-type fields, listed by `GET /asset_types/[id]/fields`). Search uses `?search="name:'x'"` and filter uses `?filter="..."`. Asset relationships: `relationship_type_id`, `primary_id`/`primary_type`, `secondary_id`/`secondary_type`.
- **Asset type**: `name`, `description`, `parent_asset_type_id`, `visible`. **Component**: per-type fields under an asset.
- **Software (applications)**: `name`, `description`, `application_type`, `status`, `publisher_id`, `managed_by_id`, `category`, `source`, `notes`, `user_count`, `installation_count`. **Software user**: `user_id`, `license_id`, `allocated_date`, `first_used`, `last_used`. **Installation**: `installation_machine_id`, `installation_path`, `version`, `user_id`, `department_id`, `installation_date`.
- **Contract**: `name`, `contract_number`, `contract_type_id`, `vendor_id`, `approver_id`, `cost`, `status`, `start_date`/`end_date`, `auto_renew`, `notify_expiry`, `notify_before`, `notify_to`, `visible_to_id`, `future_contract_id`, `custom_fields`, `has_associated_assets`, `has_attachments`.
- **Purchase order**: `name`, `po_number`, `vendor_id`, `vendor_details`, `status`, `expected_delivery_date`, `shipping_address`, `billing_address`, `billing_same_as_shipping`, `currency_code`, `conversion_rate`, `department_id`, `discount_percentage`, `tax_percentage`, `shipping_cost`, `purchase_items[]`, `custom_fields` (embed with `include=`).
- **Alert**: `subject`, `resource`, `severity` (Critical/Error/Warning/Ok), `state`, `tags`, `integration_id`/`integration_name`, `metric_name`/`metric_value`, `node`, `acknowledged_by_id`/`acknowledged_at`, `incident_id`, `suppressed`, `archived`, `occurrence_time`, `additional_info`.
- **Business hours**: `name`, `is_default`, `time_zone`, `service_desk_hours`, `list_of_holidays`. **SLA policy**: `name`, `position`, `is_default`, `active`, `sla_targets[]` (`priority`, `respond_within`, `resolve_within`, `business_hours`, `escalation_enabled`), `applicable_to.*`, `escalation.*`.
- **Project (new-gen)**: `name`, `key`, `description`, `status_id`, `priority_id`, `project_type`, `manager_id`, `start_date`, `end_date`, `visibility`, `sprint_duration`, `archived`, `custom_fields`. **Project task**: `title`, `description`, `type_id`, `reporter_id`, `assignee_id`, `status_id`, `priority_id`, `parent_id`, `planned_start_date`/`planned_end_date`, `planned_effort`, `planned_duration`, `story_points`, `sprint_id`, `version_id`, `custom_fields`. Legacy project: `title`, `description`, `start_date`, `end_date`, `owner_id`, `status`, `priority`.
- **Solution category**: `name`, `description`, `position`, `default_category`, `visible_in_portals`. **Folder**: `name`, `category_id`, `visibility`, `approval_settings`, `department_ids`, `group_ids`, `requester_group_ids`, `manage_by_group_ids`, MSP `applicable_to_workspace_ids`. **Article**: `title`, `description`, `folder_id`, `article_type`, `status` (draft/published), `approval_status`, `tags`, `keywords`, `review_date`, `url` (external article), `attachments`, `thumbs_up`/`thumbs_down`, `views`. Lists support `filter=trash` (Mar 2026). Search uses `?search_term=` and optionally `user_email`.
- **Service item**: `display_id` (the path key), `name`, `category_id`, `short_description`, `description`, `cost`, `delivery_time`, `visibility`, `group_visibility`, `item_type`, `product_id`, `ci_type_id`, `quantity`, `allow_quantity`, `allow_attachments`, `is_bundle`, `create_child`, `child_items`, `custom_fields`, MSP `applicable_to_workspace_ids`. **Service category**: `name`, `description`, `position`, `parent_id` (Sep 2025).
- **Announcement**: `title`, `body`/`body_html`, `state`, `visible_from`, `visible_till`, `visibility`, `departments`, `groups`, `visible_to_workspace_ids`, `send_email`, `additional_emails`, `is_read`.
- **Onboarding/Offboarding request**: the form-defined `fields` plus `initiator_id` (May 2024). The tickets endpoint returns the child tickets created. **Journey request**: config-defined data fields. Activities can be filtered by `activity_type`.
- **On-call**: schedules → shifts → rosters/overrides, escalation policies per schedule. Shift events take `start_time`, `end_time`, `user_id`, `schedule_id`, `shift_id`, `export_type` (ical). All schedule paths are scoped by `/oncall/ws/[workspace_id]/`.
- **Custom object record**: object-defined fields. `GET /objects/[id]` returns the schema.
- **PIR template**: `title*`, `content_html*`, `state`, `primary`, `workspace_id`, `related_incident_enabled`, `meta`.
- **Canned response**: `title`, `folder_id`, `content`/`content_html`. Folder: `name`, `type`, `responses_count`.
- **Audit log export** (request body): `since*`, `before*`, `type[]` (account, agent, group, change_field, change_lifecycle, sandbox_job, sla_policy, workflow, plans & billing, requester), `actor`, `agent_id`, `group_id`, `workflow_id`, `sla_policy_id`, `change_lifecycle_id`, `requester_id`, `workspace_id`.
- **Communication (Email/Zoom)**: `type*`, `subject`, `message`, `meta_info`, `notifiers`, `attachments`, `sender_id`, `object_id`, `object_type`.
- **Status page incident**: `title`, `description`, `status`, `impacted_services[]` (`id`, `status`), `ticket_id`, `ended_at`. **Maintenance**: also `started_at`, `maintenance_window_id`, `notification_options`. **Update**: `message`, `status_id`, `posted_on`. **Subscriber**: `email`, `service_ids`, `subscribe_all_services`, `type`, `timezone`, `verified`.
- **Delegation**: `delegatee_id`, `start_date`, `end_date`, `notes`.

## Endpoint inventory

Products: FS = Freshservice, FSBT = Freshservice for Business Teams, MSP = Freshservice for MSPs, ITAM = Freshservice IT Asset Management. Path placeholders are copied exactly from [S1] (`[id]`, `{id}` and `[display_id]` are all used).

### Tickets

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Ticket](https://api.freshservice.com/#create_ticket) | `POST` | `/api/v2/tickets` | write | `freshservice.tickets.create` | FS, FSBT, MSP |
| [View a Ticket](https://api.freshservice.com/#view_a_ticket) | `GET` | `/api/v2/tickets/[id]` | read | `freshservice.tickets.view` | FS, FSBT, MSP |
| [Filter Tickets](https://api.freshservice.com/#filter_tickets) | `GET` | `/api/v2/tickets/filter?query=[query]` | read | `freshservice.tickets.view` | FS, FSBT, MSP |
| [View List of Tickets](https://api.freshservice.com/#view_all_ticket) | `GET` | `/api/v2/tickets` | read | `freshservice.tickets.view` | FS, FSBT, MSP |
| [Update a Ticket](https://api.freshservice.com/#update_a_ticket) | `PUT` | `/api/v2/tickets/[id]` | write | `freshservice.tickets.edit` | FS, FSBT, MSP |
| [Move a Ticket](https://api.freshservice.com/#move_a_ticket) | `PUT` | `/api/v2/tickets/[id]/move_workspace` | write | `freshservice.tickets.edit` | FS, FSBT, MSP |
| [Delete a Ticket](https://api.freshservice.com/#delete_a_ticket) | `DELETE` | `/api/v2/tickets/[id]` | delete | `freshservice.tickets.delete` | FS, FSBT, MSP |
| [Delete a Ticket Attachment](https://api.freshservice.com/#delete_a_ticket_attachment) | `DELETE` | `/api/v2/tickets/[ticket_id]/attachments/[id]` | delete | `freshservice.tickets.edit` | FS, FSBT, MSP |
| [Restore a Ticket](https://api.freshservice.com/#restore_a_ticket) | `PUT` | `/api/v2/tickets/[id]/restore` | write (restore) | `freshservice.tickets.delete` | FS, FSBT, MSP |
| [Create a Child Ticket](https://api.freshservice.com/#create_child_ticket) | `POST` | `/api/v2/tickets/[parent_id]/create_child_ticket` | write | `freshservice.tickets.create` | FS, FSBT, MSP |
| [List all Ticket Fields](https://api.freshservice.com/#view_all_ticket_fields) | `GET` | `/api/v2/ticket_form_fields` | read | `freshservice.tickets.fields.manage` | FS, FSBT, MSP |
| [Get Ticket Activity](https://api.freshservice.com/#get_ticket_activities) | `GET` | `/api/v2/tickets/[id]/activities` | read | `freshservice.tickets.view` | FS, FSBT, MSP |
| ***Time Entries*** | | | | | |
| [Create a Time Entry](https://api.freshservice.com/#create_ticket_time_entry) | `POST` | `/api/v2/tickets/[ticket_id]/time_entries` | write | `freshservice.tickets.time_entries.create` | FS, FSBT, MSP |
| [View a Time Entry](https://api.freshservice.com/#view_ticket_time_entry) | `GET` | `/api/v2/tickets/[ticket_id]/time_entries/[id]` | read | `freshservice.tickets.time_entries.view` | FS, FSBT, MSP |
| [List all Time Entries of a Ticket](https://api.freshservice.com/#list_all_ticket_time_entries) | `GET` | `/api/v2/tickets/[ticket_id]/time_entries` | read | `freshservice.tickets.time_entries.view` | FS, FSBT, MSP |
| [Update a Time Entry](https://api.freshservice.com/#update_ticket_time_entry) | `PUT` | `/api/v2/tickets/[ticket_id]/time_entries/[id]` | write | `freshservice.tickets.time_entries.edit` | FS, FSBT, MSP |
| [Delete a Time Entry](https://api.freshservice.com/#delete_ticket_time_entry) | `DELETE` | `/api/v2/tickets/[ticket_id]/time_entries/[id]` | delete | `freshservice.tickets.time_entries.delete` | FS, FSBT, MSP |
| [Create a Source](https://api.freshservice.com/#create_custom_ticket_source) | `POST` | `/api/v2/ticket_fields/sources` | write | `freshservice.tickets.fields.manage` | FS, FSBT, MSP |

### Service Requests

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Service Request](https://api.freshservice.com/#create_service_request) | `POST` | `/api/v2/service_catalog/items/{display_id}/place_request` | write | `freshservice.tickets.create` | FS, FSBT, MSP |
| [Create a Child Service Request](https://api.freshservice.com/#create_child_service_request) | `POST` | `/api/v2/service_catalog/items/{display_id}/place_request` | write | `freshservice.tickets.create` | FS, FSBT, MSP |
| [View Requested Items of a Service Request](https://api.freshservice.com/#view_req_items_of_sr) | `GET` | `/api/v2/tickets/[id]/requested_items` | read | `freshservice.tickets.view` | FS, FSBT, MSP |
| [Update Requested Items of a Service Request](https://api.freshservice.com/#update_req_items_of_sr) | `PUT` | `/api/v2/tickets/[id]/requested_items/[id]` | write | `freshservice.tickets.edit` | FS, FSBT, MSP |
| [Add Catalog Item to Existing Service Request](https://api.freshservice.com/#add_catalog_item_to_existing_sr) | `POST` | `/api/v2/tickets/[ticket_id]/requested_items` | write | `freshservice.tickets.edit` | FS, FSBT |

### Ticket approvals

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Request Approval for a Service Request](https://api.freshservice.com/#create_ticket_approval) | `POST` | `/api/v2/tickets/[ticket_id]/approvals` | write | `freshservice.tickets.edit` | FS, FSBT, MSP |
| [List all Ticket Approvals](https://api.freshservice.com/#list_all_ticket_approvals) | `GET` | `/api/v2/tickets/[ticket_id]/approvals` | read | `freshservice.tickets.view` | FS, FSBT, MSP |
| [View an Approval](https://api.freshservice.com/#view_a_ticket_approval) | `GET` | `/api/v2/tickets/[ticket_id]/approvals/[id]` | read | `freshservice.tickets.view` | FS, FSBT, MSP |
| [Send Reminder for an Approval](https://api.freshservice.com/#resend_reminder_ticket_approval) | `PUT` | `/api/v2/tickets/[ticket_id]/approvals/[id]/remind` | write | `freshservice.tickets.edit` | FS, FSBT, MSP |
| [Cancel an approval](https://api.freshservice.com/#cancel_a_ticket_approval) | `PUT` | `/api/v2/tickets/[ticket_id]/approvals/[id]` | write | `freshservice.tickets.edit` | FS, FSBT, MSP |
| ***Approval Groups*** | | | | | |
| [Create Approval Groups for a Service Request](https://api.freshservice.com/#create_approval_groups) | `POST` | `/api/v2/tickets/[ticket_id]/approval-groups` | write | `freshservice.tickets.edit` | FS, FSBT, MSP |
| [Update Approval Group In A Service Request](https://api.freshservice.com/#update_approval_groups) | `PUT` | `/api/v2/tickets/[ticket_id]/approval-groups/[id]` | write | `freshservice.tickets.edit` | FS, FSBT, MSP |
| [List all Ticket Approval Groups](https://api.freshservice.com/#list_all_ticket_approval_groups) | `GET` | `/api/v2/tickets/[ticket_id]/approval-groups` | read | `freshservice.tickets.edit` | FS, FSBT, MSP |
| [Cancel Approval Group In A Service Request](https://api.freshservice.com/#cancel_approval_groups) | `PUT` | `/api/v2/tickets/[ticket_id]/approval-groups/[id]` | write | `freshservice.tickets.edit` | FS, FSBT, MSP |
| [Update approval chain rule for a service request](https://api.freshservice.com/#update_service_request_approval_chain_rule) | `PUT` | `/api/v2/tickets/[ticket_id]/approval-chain-rule` | write | `freshservice.tickets.edit` | FS, FSBT, MSP |

### Major incidents

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Promote an incident to a major incident](https://api.freshservice.com/#promote_incident_to_major) | `PUT` | `/api/v2/tickets/[ticket_id]/promote` | write | `freshservice.tickets.edit` | FS |
| [Demote an incident from a major incident](https://api.freshservice.com/#demote_incident_from_major) | `PUT` | `/api/v2/tickets/[ticket_id]/demote` | write | `freshservice.tickets.edit` | FS |

### Ticket tasks

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Task](https://api.freshservice.com/#create_ticket_task) | `POST` | `/api/v2/tickets/[id]/tasks` | write | `freshservice.tickets.tasks.create` | FS, FSBT, MSP |
| [View a Task](https://api.freshservice.com/#view_a_ticket_task) | `GET` | `/api/v2/tickets/[id]/tasks/[id]` | read | `freshservice.tickets.tasks.view` | FS, FSBT, MSP |
| [View all Tasks](https://api.freshservice.com/#view_all_ticket_tasks) | `GET` | `/api/v2/tickets/[id]/tasks` | read | `freshservice.tickets.tasks.view` | FS, FSBT, MSP |
| [Update a Task](https://api.freshservice.com/#update_a_ticket_task) | `PUT` | `/api/v2/tickets/[id]/tasks/[id]` | write | `freshservice.tickets.tasks.edit` | FS, FSBT, MSP |
| [Delete a Task](https://api.freshservice.com/#delete_a_ticket_task) | `DELETE` | `/api/v2/tickets/[id]/tasks/[id]` | delete | `freshservice.tickets.tasks.delete` | FS, FSBT, MSP |

### CSAT

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [CSAT Response](https://api.freshservice.com/#view_csat_response) | `GET` | `/api/v2/tickets/[id]/csat_response` | read | `freshservice.tickets.view` | FS, FSBT, MSP |

### Conversations

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Reply](https://api.freshservice.com/#create_a_reply) | `POST` | `/api/v2/tickets/[id]/reply` | write | `freshservice.tickets.conversations.create` | FS, FSBT, MSP |
| [Create a Note](https://api.freshservice.com/#create_a_note) | `POST` | `/api/v2/tickets/[ticket_id]/notes` | write | `freshservice.tickets.conversations.create` | FS, FSBT, MSP |
| [Update a Conversation](https://api.freshservice.com/#update_a_conversations) | `PUT` | `/api/v2/conversations/[id]` | write | `freshservice.tickets.conversations.edit` | FS, FSBT, MSP |
| [Delete a Conversation](https://api.freshservice.com/#delete_a_conversations) | `DELETE` | `/api/v2/conversations/[id]` | delete | `freshservice.tickets.conversations.delete` | FS, FSBT, MSP |
| [Delete a Conversation Attachment](https://api.freshservice.com/#delete_a_conversation_attachment) | `DELETE` | `/api/v2/conversations/[conversation_id]/attachments/[id]` | delete | `freshservice.tickets.conversations.delete` | FS, FSBT, MSP |
| [List all Conversations of a Ticket](https://api.freshservice.com/#list_all_conversations) | `GET` | `/api/v2/tickets/[id]/conversations` | read | `freshservice.tickets.conversations.view` | FS, FSBT, MSP |

### Problems

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Problem](https://api.freshservice.com/#create_problem) | `POST` | `/api/v2/problems` | write | `freshservice.problems.create` | FS |
| [View a Problem](https://api.freshservice.com/#view_a_problem) | `GET` | `/api/v2/problems/[id]` | read | `freshservice.problems.view` | FS |
| [View List of Problems](https://api.freshservice.com/#view_all_problem) | `GET` | `/api/v2/problems` | read | `freshservice.problems.view` | FS |
| [Update a Problem](https://api.freshservice.com/#update_problem_priority) | `PUT` | `/api/v2/problems/[id]` | write | `freshservice.problems.edit` | FS |
| [Move a Problem](https://api.freshservice.com/#move_a_problem) | `PUT` | `/api/v2/problems/[id]/move_workspace` | write | `freshservice.problems.edit` | FS |
| [Delete a Problem](https://api.freshservice.com/#delete_a_problem) | `DELETE` | `/api/v2/problems/[id]` | delete | `freshservice.problems.delete` | FS |
| [Restore a Problem](https://api.freshservice.com/#restore_a_problem) | `PUT` | `/api/v2/problems/[id]/restore` | write (restore) | `freshservice.problems.delete` | FS |
| [List all Problem Fields](https://api.freshservice.com/#view_all_problem_fields) | `GET` | `/api/v2/problem_form_fields` | read | `freshservice.problems.fields.view` | FS |
| ***Notes*** | | | | | |
| [Create a note](https://api.freshservice.com/#create_problem_note) | `POST` | `/api/v2/problems/[id]/notes` | write | `freshservice.problems.notes.create` | FS |
| [View a note](https://api.freshservice.com/#view_a_problem_note) | `GET` | `/api/v2/problems/[id]/notes/[id]` | read | `freshservice.problems.notes.view` | FS |
| [View all notes](https://api.freshservice.com/#view_all_problem_notes) | `GET` | `/api/v2/problems/[id]/notes` | read | `freshservice.problems.notes.view` | FS |
| [Update a note](https://api.freshservice.com/#update_a_problem_note) | `PUT` | `/api/v2/problems/[id]/notes/[id]` | write | `freshservice.problems.notes.edit` | FS |
| [Delete a note](https://api.freshservice.com/#delete_a_problem_note) | `DELETE` | `/api/v2/problems/[id]/notes/[id]` | delete | `freshservice.problems.notes.delete` | FS |
| ***Time Entries*** | | | | | |
| [Create a Time Entry](https://api.freshservice.com/#create_problem_time_entries) | `POST` | `/api/v2/problems/[id]/time_entries` | write | `freshservice.problems.time_entries.create` | FS |
| [View a Time Entry](https://api.freshservice.com/#view_a_problem_time_entry) | `GET` | `/api/v2/problems/[id]/time_entries/[id]` | read | `freshservice.problems.time_entries.view` | FS |
| [List all Time Entries](https://api.freshservice.com/#view_all_problem_time_entries) | `GET` | `/api/v2/problems/[id]/time_entries` | read | `freshservice.problems.time_entries.view` | FS |
| [Update a Time Entry](https://api.freshservice.com/#update_problem_time_entry) | `PUT` | `/api/v2/problems/[id]/time_entries/[id]` | write | `freshservice.problems.time_entries.edit` | FS |
| [Delete a Time Entry](https://api.freshservice.com/#delete_a_problem_time_entry) | `DELETE` | `/api/v2/problems/[id]/time_entries/[id]` | delete | `freshservice.problems.time_entries.delete` | FS |
| ***Tasks*** | | | | | |
| [Create a Task](https://api.freshservice.com/#create_problem_task) | `POST` | `/api/v2/problems/[id]/tasks` | write | `freshservice.problems.tasks.create` | FS |
| [View a Task](https://api.freshservice.com/#view_a_problem_task) | `GET` | `/api/v2/problems/[id]/tasks/[id]` | read | `freshservice.problems.tasks.view` | FS |
| [View all Tasks](https://api.freshservice.com/#view_all_problem_tasks) | `GET` | `/api/v2/problems/[id]/tasks` | read | `freshservice.problems.tasks.view` | FS |
| [Update a Task](https://api.freshservice.com/#update_a_problem_task) | `PUT` | `/api/v2/problems/[id]/tasks/[id]` | write | `freshservice.problems.tasks.edit` | FS |
| [Delete a Task](https://api.freshservice.com/#delete_a_problem_task) | `DELETE` | `/api/v2/problems/[id]/tasks/[id]` | delete | `freshservice.problems.tasks.delete` | FS |

### Changes

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Change](https://api.freshservice.com/#create_change) | `POST` | `/api/v2/changes` | write | `freshservice.changes.create` | FS |
| [View a Change](https://api.freshservice.com/#view_a_change) | `GET` | `/api/v2/changes/[id]` | read | `freshservice.changes.view` | FS |
| [View List of Changes](https://api.freshservice.com/#view_all_changes) | `GET` | `/api/v2/changes` | read | `freshservice.changes.view` | FS |
| [Update a Change](https://api.freshservice.com/#update_change_priority) | `PUT` | `/api/v2/changes/[id]` | write | `freshservice.changes.edit` | FS |
| [List all Change Fields](https://api.freshservice.com/#view_all_change_fields) | `GET` | `/api/v2/change_form_fields` | read | `freshservice.changes.view` | FS |
| [Move a Change](https://api.freshservice.com/#move_a_change) | `PUT` | `/api/v2/changes/[id]/move_workspace` | write | `freshservice.changes.edit` | FS |
| [Delete a Change](https://api.freshservice.com/#delete_a_change) | `DELETE` | `/api/v2/changes/[id]` | delete | `freshservice.changes.delete` | FS |
| ***Approvals for Changes*** | | | | | |
| [List all Change Approvals](https://api.freshservice.com/#list_all_change_approvals) | `GET` | `/api/v2/changes/[change_id]/approvals` | read | `freshservice.changes.view` | FS |
| [View a Change Approval](https://api.freshservice.com/#view_a_change_approval) | `GET` | `/api/v2/changes/[change_id]/approvals/[approval_id]` | read | `freshservice.tickets.view` | FS |
| [Send Reminder for a Change Approval](https://api.freshservice.com/#resend_reminder_change_approval) | `PUT` | `/api/v2/changes/[change_id]/approvals/[approval_id]/remind` | write | `freshservice.changes.edit` | FS |
| [Cancel a Change Approval](https://api.freshservice.com/#cancel_change_approval) | `PUT` | `/api/v2/changes/[change_id]/approvals/[id]` | write | `freshservice.changes.edit` | FS |
| ***Approval Groups*** | | | | | |
| [Create an Approval Group for Changes](https://api.freshservice.com/#create_change_approval_groups) | `POST` | `/api/v2/changes/[change_id]/approval-groups` | write | `freshservice.changes.edit` | FS |
| [Update Approval Group In A Change Request](https://api.freshservice.com/#update_change_approval_groups) | `PUT` | `/api/v2/changes/[change_id]/approval-groups/[id]` | write | `freshservice.changes.edit` | FS |
| [List all Approval Groups within a Change](https://api.freshservice.com/#list_all_change_approval_groups) | `GET` | `/api/v2/changes/[change_id]/approval-groups` | read | `freshservice.changes.edit` | FS |
| [Cancel Approval Group In A Change Request](https://api.freshservice.com/#cancel_change_approval_groups) | `PUT` | `/api/v2/changes/[change_id]/approval-groups/[id]` | write | `freshservice.changes.edit` | FS |
| [Update approval chain rule for a change](https://api.freshservice.com/#update_approval_chain_rule_change) | `PUT` | `/api/v2/changes/[change_id]/approval-chain-rule` | write | `freshservice.changes.edit` | FS |
| ***Notes*** | | | | | |
| [Create a Note](https://api.freshservice.com/#create_change_note) | `POST` | `/api/v2/changes/[id]/notes` | write | `freshservice.changes.notes.create` | FS |
| [View a Note](https://api.freshservice.com/#view_a_change_note) | `GET` | `/api/v2/changes/[id]/notes/[id]` | read | `freshservice.changes.notes.view` | FS |
| [View all Notes](https://api.freshservice.com/#view_all_change_notes) | `GET` | `/api/v2/changes/[id]/notes` | read | `freshservice.changes.notes.view` | FS |
| [Update a Note](https://api.freshservice.com/#update_a_change_note) | `PUT` | `/api/v2/changes/[id]/notes/[id]` | write | `freshservice.changes.notes.edit` | FS |
| [Delete a Note](https://api.freshservice.com/#delete_a_change_note) | `DELETE` | `/api/v2/changes/[id]/notes/[id]` | delete | `freshservice.changes.notes.delete` | FS |
| ***Time Entries*** | | | | | |
| [Create a Time Entry](https://api.freshservice.com/#create_time_entries) | `POST` | `/api/v2/changes/[id]/time_entries` | write | `freshservice.changes.time_entries.create` | FS |
| [View a Time Entry](https://api.freshservice.com/#view_a_time_entry) | `GET` | `/api/v2/changes/[id]/time_entries/[id]` | read | `freshservice.changes.time_entries.view` | FS |
| [List all Time Entries](https://api.freshservice.com/#view_all_time_entries) | `GET` | `/api/v2/changes/[id]/time_entries` | read | `freshservice.changes.time_entries.view` | FS |
| [Update a Time Entry](https://api.freshservice.com/#update_time_entry) | `PUT` | `/api/v2/changes/[id]/time_entries/[id]` | write | `freshservice.changes.time_entries.edit` | FS |
| [Delete a Time Entry](https://api.freshservice.com/#delete_a_time_entry) | `DELETE` | `/api/v2/changes/[id]/time_entries/[id]` | delete | `freshservice.changes.time_entries.delete` | FS |
| ***Tasks*** | | | | | |
| [Create a Task](https://api.freshservice.com/#create_change_task) | `POST` | `/api/v2/changes/[id]/tasks` | write | `freshservice.changes.tasks.create` | FS |
| [View a Task](https://api.freshservice.com/#view_a_change_task) | `GET` | `/api/v2/changes/[id]/tasks/[id]` | read | `freshservice.changes.tasks.view` | FS |
| [View all Tasks](https://api.freshservice.com/#view_all_change_tasks) | `GET` | `/api/v2/changes/[id]/tasks` | read | `freshservice.changes.tasks.view` | FS |
| [Update a Task](https://api.freshservice.com/#update_a_change_task) | `PUT` | `/api/v2/changes/[id]/tasks/[id]` | write | `freshservice.changes.tasks.edit` | FS |
| [Delete a Task](https://api.freshservice.com/#delete_a_change_task) | `DELETE` | `/api/v2/changes/[id]/tasks/[id]` | delete | `freshservice.changes.tasks.delete` | FS |

### Releases

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Release](https://api.freshservice.com/#create_release) | `POST` | `/api/v2/releases` | write | `freshservice.releases.create` | FS |
| [View a Release](https://api.freshservice.com/#view_a_release) | `GET` | `/api/v2/releases/[id]` | read | `freshservice.releases.view` | FS |
| [Update a Release](https://api.freshservice.com/#update_release_priority) | `PUT` | `/api/v2/releases/[id]` | write | `freshservice.releases.edit` | FS |
| [Move a Release](https://api.freshservice.com/#move_a_release) | `PUT` | `/api/v2/releases/[id]/move_workspace` | write | `freshservice.releases.edit` | FS |
| [Delete a Release](https://api.freshservice.com/#delete_a_release) | `DELETE` | `/api/v2/releases/[id]` | delete | `freshservice.releases.delete` | FS |
| [Filter Releases](https://api.freshservice.com/#filter_releases) | `GET` | `/api/v2/releases?filter_name=[filter_name]` | read | `freshservice.releases.view` | FS |
| [View list of Releases](https://api.freshservice.com/#view_all_release) | `GET` | `/api/v2/releases` | read | `freshservice.releases.view` | FS |
| [Restore a Release](https://api.freshservice.com/#restore_a_release) | `PUT` | `/api/v2/releases/[id]/restore` | write (restore) | `freshservice.releases.delete` | FS |
| [View all Release Fields](https://api.freshservice.com/#view_all_release_fields) | `GET` | `/api/v2/release_form_fields` | read | `freshservice.releases.view` | FS |
| ***Notes*** | | | | | |
| [Create a note](https://api.freshservice.com/#create_release_note) | `POST` | `/api/v2/releases/[id]/notes` | write | `freshservice.releases.notes.create` | FS |
| [View a note](https://api.freshservice.com/#view_a_release_note) | `GET` | `/api/v2/releases/[id]/notes/[id]` | read | `freshservice.releases.notes.view` | FS |
| [View all notes](https://api.freshservice.com/#view_all_release_notes) | `GET` | `/api/v2/releases/[id]/notes` | read | `freshservice.releases.notes.view` | FS |
| [Update a note](https://api.freshservice.com/#update_a_release_note) | `PUT` | `/api/v2/releases/[id]/notes/[id]` | write | `freshservice.releases.notes.edit` | FS |
| [Delete a note](https://api.freshservice.com/#delete_a_release_note) | `DELETE` | `/api/v2/releases/[id]/notes/[id]` | delete | `freshservice.releases.notes.delete` | FS |
| ***Time Entries*** | | | | | |
| [Create a Time Entry](https://api.freshservice.com/#create_release_time_entries) | `POST` | `/api/v2/releases/[id]/time_entries` | write | `freshservice.releases.time_entries.create` | FS |
| [View a Time Entry](https://api.freshservice.com/#view_a_release_time_entry) | `GET` | `/api/v2/releases/[id]/time_entries/[id]` | read | `freshservice.releases.time_entries.view` | FS |
| [List all Time Entries](https://api.freshservice.com/#view_all_release_time_entries) | `GET` | `/api/v2/releases/[id]/time_entries` | read | `freshservice.releases.time_entries.view` | FS |
| [Update a Time Entry](https://api.freshservice.com/#update_release_time_entry) | `PUT` | `/api/v2/releases/[id]/time_entries/[id]` | write | `freshservice.releases.time_entries.edit` | FS |
| [Delete a Time Entry](https://api.freshservice.com/#delete_a_release_time_entry) | `DELETE` | `/api/v2/releases/[id]/time_entries/[id]` | delete | `freshservice.releases.time_entries.delete` | FS |
| ***Tasks*** | | | | | |
| [Create a Task](https://api.freshservice.com/#create_release_task) | `POST` | `/api/v2/releases/[id]/tasks` | write | `freshservice.releases.tasks.create` | FS |
| [View a Task](https://api.freshservice.com/#view_a_release_task) | `GET` | `/api/v2/releases/[id]/tasks/[id]` | read | `freshservice.releases.tasks.view` | FS |
| [View all Tasks](https://api.freshservice.com/#view_all_release_tasks) | `GET` | `/api/v2/releases/[id]/tasks` | read | `freshservice.releases.tasks.view` | FS |
| [Update a Task](https://api.freshservice.com/#update_a_release_task) | `PUT` | `/api/v2/releases/[id]/tasks/[id]` | write | `freshservice.releases.tasks.edit` | FS |
| [Delete a Task](https://api.freshservice.com/#delete_a_release_task) | `DELETE` | `/api/v2/releases/[id]/tasks/[id]` | delete | `freshservice.releases.tasks.delete` | FS |

### Change Advisory Boards (CABs)

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a CAB](https://api.freshservice.com/#create_a_cab) | `POST` | `/api/v2/cabs` | write | (none listed) | FS |
| [List all CABs](https://api.freshservice.com/#view_all_cabs) | `GET` | `/api/v2/cabs` | read | (none listed) | FS |
| [View a CAB](https://api.freshservice.com/#view_a_cab) | `GET` | `/api/v2/cabs/[id]` | read | (none listed) | FS |
| [Update a CAB](https://api.freshservice.com/#update_a_cab) | `PATCH` | `/api/v2/cabs/[id]` | write | (none listed) | FS |
| [Delete a CAB](https://api.freshservice.com/#delete_a_cab) | `DELETE` | `/api/v2/cabs/[id]` | delete | (none listed) | FS |

### Workspaces / Clients

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| ***Metadata attribute*** | | | | | |
| [Create Client](https://api.freshservice.com/#create_client) | `POST` | `/api/v2/workspace` | write | `freshservice.workspaces.create` | MSP |
| [View a Workspace / Client](https://api.freshservice.com/#view_a_workspace) | `GET` | `/api/v2/workspaces/[id]` | read | `freshservice.workspaces.view` | FS, FSBT, MSP |
| [List All Workspaces / Clients](https://api.freshservice.com/#view_all_workspaces) | `GET` | `/api/v2/workspaces` | read | `freshservice.workspaces.view` | FS, FSBT, MSP |
| [Filter Clients](https://api.freshservice.com/#filter_clients) | `GET` | `/api/v2/workspaces?query=[query]` | read | `freshservice.workspaces.view` | MSP |
| [Update a Client](https://api.freshservice.com/#update_a_client) | `PUT` | `/api/v2/workspaces/[id]` | write | `freshservice.workspaces.edit` | MSP |
| [Delete a Client](https://api.freshservice.com/#delete_a_client) | `DELETE` | `/api/v2/workspaces/[id]` | delete | `freshservice.workspaces.delete` | MSP |
| [List All Client Fields](https://api.freshservice.com/#list_all_client_fields) | `GET` | `/api/v2/workspace_form_fields` | read | `freshservice.workspaces.form.view` | MSP |

### Approvals (cross-module)

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [List all Approvals](https://api.freshservice.com/#list_all_approvals) | `GET` | `/api/v2/approvals?parent=[module]` | read | (none listed) | FS, FSBT, MSP |

### Requesters / Contacts

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Requester/Contact](https://api.freshservice.com/#create_a_requester) | `POST` | `/api/v2/requesters` | write | `freshservice.requesters.create` | FS, FSBT, MSP |
| [View a Requester/Contact](https://api.freshservice.com/#view_a_requester) | `GET` | `/api/v2/requesters/[id]` | read | `freshservice.requesters.view` | FS, FSBT, MSP |
| [List All Requesters/Contacts](https://api.freshservice.com/#list_all_requesters) | `GET` | `/api/v2/requesters` | read | `freshservice.requesters.view` | FS, FSBT, MSP |
| [Filter Requester/Contact](https://api.freshservice.com/#filter_requesters) | `GET` | `/api/v2/requesters?query=[query]` | read | `freshservice.requesters.view` | FS, FSBT, MSP |
| [List All Requester/Contact Fields](https://api.freshservice.com/#list_all_requester_fields) | `GET` | `/api/v2/requester_fields` | read | `freshservice.requesters.fields.view` | FS, FSBT, MSP |
| [Update a Requester/Contact](https://api.freshservice.com/#update_a_requester) | `PUT` | `/api/v2/requesters/[id]` | write | `freshservice.requesters.edit` | FS, FSBT, MSP |
| [Deactivate a Requester/Contact](https://api.freshservice.com/#deactivate_a_requester) | `DELETE` | `/api/v2/requesters/[id]` | delete (soft, reversible) | `freshservice.requesters.delete` | FS, FSBT, MSP |
| [Forget a Requester/Contact](https://api.freshservice.com/#forget_a_requester) | `DELETE` | `/api/v2/requesters/[id]/forget` | delete (permanent, GDPR) | `freshservice.requesters.delete` | FS, FSBT, MSP |
| [Convert To Agent](https://api.freshservice.com/#convert_to_agent) | `PUT` | `/api/v2/requesters/[id]/convert_to_agent` | write | `freshservice.requesters.manage` | FS, FSBT, MSP |
| [Merge Requesters/Contacts](https://api.freshservice.com/#merge_requesters) | `PUT` | `/api/v2/requesters/[id]/merge?secondary_requesters=111,222,333` | write | `freshservice.requesters.edit` | FS, FSBT, MSP |
| [Reactivate a Requester/Contact](https://api.freshservice.com/#reactivate_a_requester) | `PUT` | `/api/v2/requesters/[id]/reactivate` | write | `freshservice.requesters.delete` | FS, FSBT, MSP |
| [User Assignment History](https://api.freshservice.com/#user_assignment_history) | `GET` | `/api/v2/users/[id]/assignment-history` | read | (none listed) | FS, FSBT |

### Agents

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create an Agent](https://api.freshservice.com/#create_an_agent) | `POST` | `/api/v2/agents` | write | `freshservice.agents.manage` | FS, FSBT, MSP |
| [View an Agent](https://api.freshservice.com/#view_an_agent) | `GET` | `/api/v2/agents/[id]` | read | `freshservice.agents.manage` | FS, FSBT, MSP |
| [List all Agents](https://api.freshservice.com/#list_all_agents) | `GET` | `/api/v2/agents` | read | `freshservice.agents.manage` | FS, FSBT, MSP |
| [Filter Agents](https://api.freshservice.com/#filter_agents) | `GET` | `/api/v2/agents?query=[query]` | read | `freshservice.agents.manage` | FS, FSBT, MSP |
| [Update an Agent](https://api.freshservice.com/#update_an_agent) | `PUT` | `/api/v2/agents/[id]` | write | `freshservice.agents.manage` | FS, FSBT, MSP |
| [Deactivate an Agent](https://api.freshservice.com/#delete_an_agent) | `DELETE` | `/api/v2/agents/[id]` | delete (soft, reversible) | `freshservice.agents.manage` | FS, FSBT, MSP |
| [Forget an Agent](https://api.freshservice.com/#forget_an_agent) | `DELETE` | `/api/v2/agents/[id]/forget` | delete (permanent, GDPR) | `freshservice.agents.manage` | FS, FSBT, MSP |
| [Reactivate an Agent](https://api.freshservice.com/#reactivate_an_agent) | `PUT` | `/api/v2/agents/[id]/reactivate` | write | `freshservice.agents.manage` | FS, FSBT, MSP |
| [Convert To Requester](https://api.freshservice.com/#convert_an_agent_to_requester) | `PUT` | `/api/v2/agents/[id]/convert_to_requester` | write | `freshservice.agents.manage` | FS, FSBT, MSP |
| [List all Agent Fields](https://api.freshservice.com/#list_all_agent_fields) | `GET` | `/api/v2/agent_fields` | read | `freshservice.agents.fields.view` | FS, FSBT, MSP |
| [User Assignment History](https://api.freshservice.com/#user_assignment_history) | `GET` | `/api/v2/users/[id]/assignment-history` | read | (none listed) | FS, FSBT |

### Agent roles

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [View a Role](https://api.freshservice.com/#view_a_role) | `GET` | `/api/v2/roles/[id]` | read | `freshservice.agents.roles.view` | FS, FSBT, MSP |
| [List all Roles](https://api.freshservice.com/#view_all_role) | `GET` | `/api/v2/roles/` | read | `freshservice.agents.roles.view` | FS, FSBT, MSP |

### Agent groups

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Group](https://api.freshservice.com/#create_a_group) | `POST` | `/api/v2/groups` | write | `freshservice.agentgroups.manage.itsm` | FS, FSBT, MSP |
| [View a Group](https://api.freshservice.com/#view_a_group) | `GET` | `/api/v2/groups/[id]` | read | `freshservice.agentgroups.manage` | FS, FSBT, MSP |
| [List all Groups](https://api.freshservice.com/#view_all_group) | `GET` | `/api/v2/groups` | read | `freshservice.agentgroups.manage` | FS, FSBT, MSP |
| [Update a Group](https://api.freshservice.com/#update_a_group) | `PUT` | `/api/v2/groups/[id]` | write | `freshservice.agentgroups.manage` | FS, FSBT, MSP |
| [Delete a Group](https://api.freshservice.com/#delete_a_group) | `DELETE` | `/api/v2/groups/[id]` | delete | `freshservice.agentgroups.manage` | FS, FSBT, MSP |

### Requester groups / Contact groups

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Requester Group/Contact Group](https://api.freshservice.com/#create_a_requester_group) | `POST` | `/api/v2/requester_groups` | write | `freshservice.requesters.edit` | FS, FSBT, MSP |
| [View a Requester Group/Contact Group](https://api.freshservice.com/#view_a_requester_group) | `GET` | `/api/v2/requester_groups/[id]` | read | `freshservice.requesters.view` | FS, FSBT, MSP |
| [List All Requester Groups/Contact Groups](https://api.freshservice.com/#view_all_requester_group) | `GET` | `/api/v2/requester_groups` | read | `freshservice.requesters.view` | FS, FSBT, MSP |
| [Update a Requester Group/Contact Group](https://api.freshservice.com/#update_a_requester_group) | `PUT` | `/api/v2/requester_groups/[id]` | write | `freshservice.requesters.edit` | FS, FSBT, MSP |
| [Delete a Requester Group/Contact Group](https://api.freshservice.com/#delete_a_requester_group) | `DELETE` | `/api/v2/requester_groups/[id]` | delete | `freshservice.requesters.delete` | FS, FSBT, MSP |
| [Add Requester to Requester/Contact Group](https://api.freshservice.com/#add_member_to_requester_group) | `POST` | `/api/v2/requester_groups/[id]/members/[requester_id]` | write | `freshservice.requesters.edit` | FS, FSBT, MSP |
| [Delete Requester/Contact from Requester/Contact Group](https://api.freshservice.com/#delete_member_from_requester_group) | `DELETE` | `/api/v2/requester_groups/[id]/members/[requester_id]` | write (unlink) | `freshservice.requesters.edit` | FS, FSBT, MSP |
| [List Requester/Contact Group Members](https://api.freshservice.com/#list_members_of_requester_group) | `GET` | `/api/v2/requester_groups/[id]/members` | read | `freshservice.requesters.edit` | FS, FSBT, MSP |

### Locations

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Location](https://api.freshservice.com/#create_a_location) | `POST` | `/api/v2/locations` | write | `freshservice.locations.create` | FS, FSBT, MSP |
| [View a Location](https://api.freshservice.com/#view_a_location) | `GET` | `/api/v2/locations/[id]` | read | `freshservice.locations.view` | FS, FSBT, MSP |
| [List all Locations](https://api.freshservice.com/#list_all_locations) | `GET` | `/api/v2/locations` | read | `freshservice.locations.view` | FS, FSBT, MSP |
| [Filter Locations](https://api.freshservice.com/#filter_locations) | `GET` | `/api/v2/locations?query=[query]` | read | `freshservice.locations.view` | FS, FSBT, MSP |
| [Update a Location](https://api.freshservice.com/#update_a_location) | `PUT` | `/api/v2/locations/[id]` | write | `freshservice.locations.edit` | FS, FSBT, MSP |
| [Delete a Location](https://api.freshservice.com/#delete_a_location) | `DELETE` | `/api/v2/locations/[id]` | delete | `freshservice.locations.delete` | FS, FSBT, MSP |

### Products

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Product](https://api.freshservice.com/#create_a_product) | `POST` | `/api/v2/products` | write | `freshservice.products.create` | FS |
| [View a Product](https://api.freshservice.com/#view_a_product) | `GET` | `/api/v2/products/[id]` | read | `freshservice.products.view` | FS |
| [List all Products](https://api.freshservice.com/#view_all_products) | `GET` | `/api/v2/products` | read | `freshservice.products.view` | FS |
| [Update a Product](https://api.freshservice.com/#update_a_product) | `PUT` | `/api/v2/products/[id]` | write | `freshservice.products.update` | FS |
| [Delete a Product](https://api.freshservice.com/#delete_a_product) | `DELETE` | `/api/v2/products/[id]` | delete | `freshservice.products.delete` | FS |

### Vendors

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Vendor](https://api.freshservice.com/#create_a_vendor) | `POST` | `/api/v2/vendors` | write | `freshservice.vendors.create` | FS |
| [View a Vendor](https://api.freshservice.com/#view_a_vendor) | `GET` | `/api/v2/vendors/[id]` | read | `freshservice.vendors.view` | FS |
| [List all Vendors](https://api.freshservice.com/#view_all_vendors) | `GET` | `/api/v2/vendors` | read | `freshservice.vendors.view` | FS |
| [Update a Vendor](https://api.freshservice.com/#update_a_vendor) | `PUT` | `/api/v2/vendors/[id]` | write | `freshservice.vendors.edit` | FS |
| [Delete a Vendor](https://api.freshservice.com/#delete_a_vendor) | `DELETE` | `/api/v2/vendors/[id]` | delete | `freshservice.vendors.delete` | FS |

### Alerts (Alert Management)

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| ***Alert Properties*** | | | | | |
| [View an alert](https://api.freshservice.com/#view_alert) | `GET` | `/api/v2/ams/alerts/[id]` | read | `freshservice.alerts.view` | FS |
| [View Multiple Alerts using Filters](https://api.freshservice.com/#filter_alerts) | `GET` | `/api/v2/ams/alerts?query=[query]&order_by=[order_by_field]&order_type=[desc]&page=[page]&per_page=[per_page]` | read | `freshservice.alerts.view` | FS |
| [View all Alert Logs](https://api.freshservice.com/#view_alert_logs) | `GET` | `/api/v2/ams/alerts/[id]/logs?start_token=[start_token]` | read | `freshservice.alerts.view` | FS |
| [Acknowledge an alert](https://api.freshservice.com/#acknowledge_alert) | `PUT` | `/api/v2/ams/alerts/[id]/acknowledge` | write | `freshservice.alerts.edit` | FS |
| [Resolve an alert](https://api.freshservice.com/#resolve_alert) | `PUT` | `/api/v2/ams/alerts/[id]/resolve` | write | `freshservice.alerts.edit` | FS |
| [Suppress an alert](https://api.freshservice.com/#suppress_alert) | `PUT` | `/api/v2/ams/alerts/[id]/suppress` | write | `freshservice.alerts.edit` | FS |
| [Unsuppress an alert](https://api.freshservice.com/#unsuppress_alert) | `PUT` | `/api/v2/ams/alerts/[id]/unsuppress` | write | `freshservice.alerts.edit` | FS |
| [Delete an Alert](https://api.freshservice.com/#delete_alert) | `DELETE` | `/api/v2/ams/alerts/[id]` | delete | `freshservice.alerts.delete` | FS |
| ***Notes*** | | | | | |
| [Create an Alert Note](https://api.freshservice.com/#create_alert_note) | `POST` | `/api/v2/ams/alerts/[id]/notes` | write | `freshservice.alerts.edit` | FS |
| [View all alert notes](https://api.freshservice.com/#view_alert_notes) | `GET` | `/api/v2/ams/alerts/[id]/notes?page=[page]&per_page=[per_page]` | read | `freshservice.alerts.view` | FS |
| [View an alert note](https://api.freshservice.com/#view_alert_note) | `GET` | `/api/v2/ams/alerts/[id]/notes/[note_id]` | read | `freshservice.alerts.view` | FS |
| [Update an alert note](https://api.freshservice.com/#update_alert_note) | `PUT` | `/api/v2/ams/alerts/[id]/notes/[note_id]` | write | `freshservice.alerts.edit` | FS |
| [Delete an alert note](https://api.freshservice.com/#delete_alert_note) | `DELETE` | `/api/v2/ams/alerts/[id]/notes/[note_id]` | delete | `freshservice.alerts.delete` | FS |

### Assets / CMDB (classic)

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Asset Assignment History](https://api.freshservice.com/#asset_assignment_history) | `GET` | `/api/v2/assets/[display_id]/assignment-history` | read | (none listed) | FS |
| [Create an Asset](https://api.freshservice.com/#create_an_asset) | `POST` | `/api/v2/assets` | write | `freshservice.assets.manage` | FS |
| [View an Asset](https://api.freshservice.com/#view_an_asset) | `GET` | `/api/v2/assets/[display_id]` | read | `freshservice.assets.view` | FS |
| [View List of Assets](https://api.freshservice.com/#view_all_assets) | `GET` | `/api/v2/assets` | read | `freshservice.assets.view` | FS |
| [Search Assets](https://api.freshservice.com/#search_assets) | `GET` | `/api/v2/assets?search=[query]` | read | `freshservice.assets.view` | FS |
| [Filter Assets](https://api.freshservice.com/#filter_assets) | `GET` | `/api/v2/assets?filter=[query]` | read | `freshservice.assets.view` | FS |
| [Update an Asset](https://api.freshservice.com/#update_an_asset) | `PUT` | `/api/v2/assets/[display_id]` | write | `freshservice.assets.manage` | FS |
| [Delete an Asset](https://api.freshservice.com/#delete_an_asset) | `DELETE` | `/api/v2/assets/[display_id]` | delete | `freshservice.assets.delete` | FS |
| [Restore an Asset](https://api.freshservice.com/#restore_an_asset) | `PUT` | `/api/v2/assets/[display_id]/restore` | write (restore) | `freshservice.assets.delete` | FS |
| [Delete an Asset Permanently](https://api.freshservice.com/#delete_forever_an_asset) | `PUT` | `/api/v2/assets/[display_id]/delete_forever` | delete (permanent) | `freshservice.assets.delete` | FS |
| [Move an Asset](https://api.freshservice.com/#move_an_asset) | `PUT` | `/api/v2/assets/[display_id]/move_workspace` | write | `freshservice.assets.manage` | FS |
| [List all Asset Components](https://api.freshservice.com/#list_all_asset_components) | `GET` | `/api/v2/assets/[display_id]/components` | read | `freshservice.assets.view` | FS |
| [Create a Component](https://api.freshservice.com/#create_a_component) | `POST` | `/api/v2/assets/[display_id]/components` | write | `freshservice.assets.manage` | FS |
| [Update a Component](https://api.freshservice.com/#update_a_component) | `PUT` | `/api/v2/assets/[display_id]/components/[component_id]` | write | `freshservice.assets.manage` | FS |
| [List all Associated Requests](https://api.freshservice.com/#list_all_asset_requests) | `GET` | `/api/v2/assets/[display_id]/requests` | read | `freshservice.assets.view` | FS |
| [List all associated Contracts](https://api.freshservice.com/#list_contracts_of_an_asset) | `GET` | `/api/v2/assets/[display_id]/contracts` | read | `freshservice.assets.view` | FS |
| [Create Relationships in bulk](https://api.freshservice.com/#create_relationships) | `POST` | `/api/v2/relationships/bulk-create` | write | `freshservice.assets.manage` | FS |
| [View a Relationship](https://api.freshservice.com/#view_a_relationship) | `GET` | `/api/v2/relationships/[id]` | read | `freshservice.assets.view` | FS |
| [List all Relationships in the Account](https://api.freshservice.com/#list_relationships) | `GET` | `/api/v2/relationships` | read | `freshservice.assets.view` | FS |
| [List all Relationships for an Asset](https://api.freshservice.com/#list_relationships_of_an_asset) | `GET` | `/api/v2/assets/[display_id]/relationships` | read | `freshservice.assets.view` | FS |
| [Delete Relationships in bulk](https://api.freshservice.com/#delete_relationships) | `DELETE` | `/api/v2/relationships?ids={ids}` | delete | `freshservice.assets.manage` | FS |
| [List all Relationship Types](https://api.freshservice.com/#list_relationship_types) | `GET` | `/api/v2/relationship_types` | read | `freshservice.assets.view` | FS |

### ITAM assets (Freshservice IT Asset Management)

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create an asset](https://api.freshservice.com/#create_an_asset_for_freshservice_itam) | `POST` | `/api/v2/itam/assets/` | write | (none listed) | ITAM |
| [View list of assets](https://api.freshservice.com/#view_all_assets_for_freshservice_itam) | `GET` | `/api/v2/itam/assets` | read | (none listed) | ITAM |
| [View an asset](https://api.freshservice.com/#view_an_asset_for_freshservice_itam) | `GET` | `/api/v2/itam/assets/{id}` | read | (none listed) | ITAM |
| [Update an asset](https://api.freshservice.com/#update_an_asset_for_freshservice_itam) | `PUT` | `/api/v2/itam/assets/` | write | (none listed) | ITAM |
| [Update an asset by ID](https://api.freshservice.com/#update_an_asset_by_id_for_freshservice_itam) | `PUT` | `/api/v2/itam/assets/{id}/` | write | (none listed) | ITAM |
| [Delete an asset](https://api.freshservice.com/#delete_an_asset_for_freshservice_itam) | `DELETE` | `/api/v2/itam/assets/{id}/` | delete | (none listed) | ITAM |
| [Create or update custom field](https://api.freshservice.com/#create_or_update_custom_field_of_assets_for_freshservice_itam) | `PUT` | `/api/v2/itam/custom_fields/assets/` | write | (none listed) | ITAM |

### Purchase orders

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a new Purchase Order](https://api.freshservice.com/#create_a_new_purchase_order) | `POST` | `/api/v2/purchase_orders` | write | `freshservice.purchase_orders.create` | FS |
| [List all Purchase Orders](https://api.freshservice.com/#list_all_purchase_orders) | `GET` | `/api/v2/purchase_orders` | read | `freshservice.purchase_orders.view` | FS |
| [Move a Purchase Order](https://api.freshservice.com/#move_a_purchase_order) | `PUT` | `/api/v2/purchase_orders/[purchase_order_id]/move_workspace` | write | `freshservice.purchase_orders.edit` | FS |
| [View a Purchase Order](https://api.freshservice.com/#view_a_purchase_order) | `GET` | `/api/v2/purchase_orders/[purchase_order_id]` | read | `freshservice.purchase_orders.view` | FS |
| [Update a Purchase Order](https://api.freshservice.com/#_update_a_purchase_order) | `PUT` | `/api/v2/purchase_orders/[purchase_order_id]` | write | `freshservice.purchase_orders.edit` | FS |
| [Delete a Purchase Order](https://api.freshservice.com/#_delete_a_purchase_order) | `DELETE` | `/api/v2/purchase_orders/[purchase_order_id]` | delete | `freshservice.purchase_orders.delete` | FS |

### Asset types

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create an Asset Type](https://api.freshservice.com/#create_an_asset_type) | `POST` | `/api/v2/asset_types` | write | `freshservice.asset_types.create` | FS |
| [View an Asset Type](https://api.freshservice.com/#view_an_asset_type) | `GET` | `/api/v2/asset_types/[id]` | read | `freshservice.asset_types.view` | FS |
| [List all Asset Types](https://api.freshservice.com/#list_all_asset_types) | `GET` | `/api/v2/asset_types` | read | `freshservice.assets.view` | FS |
| [Update an Asset Type](https://api.freshservice.com/#update_an_asset_type) | `PUT` | `/api/v2/asset_types/[id]` | write | `freshservice.asset_types.edit` | FS |
| [Delete an Asset Type](https://api.freshservice.com/#delete_an_asset_type) | `DELETE` | `/api/v2/asset_types/[id]` | delete | `freshservice.asset_types.delete` | FS |
| [List all Fields of an Asset Type](https://api.freshservice.com/#list_asset_type_fields) | `GET` | `/api/v2/asset_types/[id]/fields` | read | `freshservice.assets.manage` | FS |

### Software

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Software](https://api.freshservice.com/#create_a_software) | `POST` | `/api/v2/applications` | write | `freshservice.assets.manage` | FS |
| [Update a Software](https://api.freshservice.com/#update_a_software) | `PUT` | `/api/v2/applications/[id]` | write | `freshservice.assets.manage` | FS |
| [View a Software](https://api.freshservice.com/#view_a_software) | `GET` | `/api/v2/applications/[id]` | read | `freshservice.assets.view` | FS |
| [List all Software](https://api.freshservice.com/#list_all_software) | `GET` | `/api/v2/applications/` | read | `freshservice.assets.view` | FS |
| [List all Licenses](https://api.freshservice.com/#list_all_licenses) | `GET` | `/api/v2/applications/[id]/licenses` | read | `freshservice.assets.manage` | FS |
| [Delete a Software](https://api.freshservice.com/#delete_a_software) | `DELETE` | `/api/v2/applications/[id]` | delete | `freshservice.assets.delete` | FS |
| [Delete multiple Software](https://api.freshservice.com/#delete_multiple_software) | `DELETE` | `/api/v2/applications?ids=[ids]` | delete | `freshservice.assets.delete` | FS |
| ***Software Users*** | | | | | |
| [Add Users to a Software in bulk](https://api.freshservice.com/#bulk_create_users) | `POST` | `/api/v2/applications/[id]/users` | write | `freshservice.assets.manage` | FS |
| [View a Software User](https://api.freshservice.com/#view_user) | `GET` | `/api/v2/applications/[id]/users/[application_user_id]` | read | `freshservice.assets.manage` | FS |
| [Move a Software](https://api.freshservice.com/#move_a_software) | `PUT` | `/api/v2/applications/[id]/move_workspace` | write | `freshservice.assets.manage` | FS |
| [List all Users of a Software](https://api.freshservice.com/#list_users) | `GET` | `/api/v2/applications/[id]/users` | read | `freshservice.assets.view` | FS |
| [Update Users of a Software in bulk](https://api.freshservice.com/#bulk_update_users) | `PUT` | `/api/v2/applications/[id]/users` | write | `freshservice.assets.manage` | FS |
| [Remove Users from a Software in bulk](https://api.freshservice.com/#bulk_delete_users) | `DELETE` | `/api/v2/applications/[id]/users` | write (unlink) | `freshservice.assets.manage` | FS |
| ***Software Installations*** | | | | | |
| [Add a device to a Software](https://api.freshservice.com/#create_an_installations) | `POST` | `/api/v2/applications/[id]/installations` | write | `freshservice.assets.manage` | FS |
| [List all installations of a Software](https://api.freshservice.com/#list_installations) | `GET` | `/api/v2/applications/[id]/installations` | read | `freshservice.assets.view` | FS |
| [Remove devices from a Software in bulk](https://api.freshservice.com/#bulk_delete_installations) | `DELETE` | `/api/v2/applications/[id]/installations` | write (unlink) | `freshservice.assets.manage` | FS |
| [List all Relationships for a Software](https://api.freshservice.com/#list_relationships_of_a_software) | `GET` | `/api/v2/applications/[id]/relationships` | read | `freshservice.assets.view` | FS |

### Contracts

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [List all contract types](https://api.freshservice.com/#list_all_contract_types) | `GET` | `/api/v2/contract_types` | read | `freshservice.contract_types.view` | FS |
| [List all fields of a contract type](https://api.freshservice.com/#list_all_contract_type_fields) | `GET` | `/api/v2/contract_types/[contract_type_id]/fields` | read | `freshservice.contract_types.view` | FS |
| [View a contract](https://api.freshservice.com/#view_a_contract) | `GET` | `/api/v2/contracts/[contract_id]` | read | `freshservice.contracts.view` | FS |
| [List all contracts](https://api.freshservice.com/#list_all_contracts) | `GET` | `/api/v2/contracts` | read | `freshservice.contracts.view` | FS |
| [Move a Contract](https://api.freshservice.com/#move_a_contract) | `PUT` | `/api/v2/contracts/[contract_id]/move_workspace` | write | `freshservice.contracts.edit` | FS |
| [Create a Contract](https://api.freshservice.com/#create_a_contract) | `POST` | `/api/v2/contracts` | write | `freshservice.contracts.create` | FS |
| [Create a Contract with associated assets](https://api.freshservice.com/#create_contract_with_associated_assets) | `POST` | `/api/v2/contracts` | write | `freshservice.contracts.create` | FS |
| [Create a Contract with attachment](https://api.freshservice.com/#create_contract_with_attachment) | `POST` | `/api/v2/contracts` | write | `freshservice.contracts.create` | FS |
| [Update a Contract](https://api.freshservice.com/#update_a_contract) | `PUT` | `/api/v2/contracts/[contract_id]` | write | `freshservice.contracts.edit` | FS |
| [Submit a contract for approval](https://api.freshservice.com/#submit_contract_for_approval) | `PUT` | `/api/v2/contracts/[contract_id]?operation=submit-for-approval` | write | `freshservice.contracts.edit` | FS |
| [Approve a Contract](https://api.freshservice.com/#approve_contract) | `PUT` | `/api/v2/contracts/[contract_id]?operation=approve` | write | `freshservice.contracts.edit` | FS |
| [Reject a Contract](https://api.freshservice.com/#reject_contract) | `PUT` | `/api/v2/contracts/[contract_id]?operation=reject` | write | `freshservice.contracts.edit` | FS |
| [List all associated assets of a contract](https://api.freshservice.com/#list_all_associated_assets) | `GET` | `/api/v2/contracts/[contract_id]/associated-assets` | read | `freshservice.contracts.view` | FS |
| [List all attachments of a contract](https://api.freshservice.com/#list_all_attachments) | `GET` | `/api/v2/contracts/[contract_id]/attachments` | read | `freshservice.contracts.view` | FS |

### Departments / Companies

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Department](https://api.freshservice.com/#create_a_department) | `POST` | `/api/v2/departments` | write | `freshservice.departments.manage` | FS, FSBT, MSP |
| [View a Department](https://api.freshservice.com/#view_a_department) | `GET` | `/api/v2/departments/[id]` | read | `freshservice.departments.view` | FS, FSBT, MSP |
| [List all Departments](https://api.freshservice.com/#list_all_departments) | `GET` | `/api/v2/departments` | read | `freshservice.departments.view` | FS, FSBT, MSP |
| [Filter Departments](https://api.freshservice.com/#filter_departments) | `GET` | `/api/v2/departments?query=[query]` | read | `freshservice.departments.view` | FS, FSBT, MSP |
| [Update a Department](https://api.freshservice.com/#update_a_department) | `PUT` | `/api/v2/departments/[id]` | write | `freshservice.departments.manage` | FS, FSBT, MSP |
| [Delete a Department](https://api.freshservice.com/#delete_a_department) | `DELETE` | `/api/v2/departments/[id]` | delete | `freshservice.departments.delete` | FS, FSBT, MSP |
| [List all Department Fields](https://api.freshservice.com/#list_all_department_fields) | `GET` | `/api/v2/department_fields` | read | `freshservice.departments.fields.view` | FS, FSBT, MSP |

### Business hours

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [View a Business Hours Configuration](https://api.freshservice.com/#view_a_business_hour) | `GET` | `/api/v2/business_hours/[id]` | read | `freshservice.business_hours.view` | FS, FSBT, MSP |
| [View List of Business Hours Configurations](https://api.freshservice.com/#list_all_business_hours) | `GET` | `/api/v2/business_hours` | read | `freshservice.business_hours.view` | FS, FSBT, MSP |

### Projects (legacy)

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Project](https://api.freshservice.com/#create_a_project) | `POST` | `/api/v2/projects` | write | (none listed) | FS, FSBT |
| [Update a Project](https://api.freshservice.com/#update_a_project) | `PUT` | `/api/v2/projects/[id]` | write | (none listed) | FS, FSBT |
| [View a Project](https://api.freshservice.com/#view_a_project) | `GET` | `/api/v2/projects/[id]` | read | (none listed) | FS, FSBT |
| [List all Projects](https://api.freshservice.com/#view_all_projects) | `GET` | `/api/v2/projects` | read | (none listed) | FS, FSBT |
| [Delete a Project](https://api.freshservice.com/#delete_a_project) | `DELETE` | `/api/v2/projects/[id]` | delete | (none listed) | FS, FSBT |
| [Archive a Project](https://api.freshservice.com/#archive_a_project) | `POST` | `/api/v2/projects/[id]/archive` | write | (none listed) | FS, FSBT |
| [Restore a Project](https://api.freshservice.com/#restore_a_project) | `POST` | `/api/v2/projects/[id]/restore` | write (restore) | (none listed) | FS, FSBT |
| ***Projects Tasks (Legacy)*** | | | | | |
| [Create a Project Task](https://api.freshservice.com/#create_a_project_task) | `POST` | `/api/v2/projects/[id]/tasks` | write | (none listed) | FS, FSBT |
| [Update a Project Task](https://api.freshservice.com/#update_a_project_task) | `PUT` | `/api/v2/projects/[project_id]/tasks/[id]` | write | (none listed) | FS, FSBT |
| [View a Project Task](https://api.freshservice.com/#view_a_project_task) | `GET` | `/api/v2/projects/[project_id]/tasks/[id]` | read | (none listed) | FS, FSBT |
| [List all Project Tasks](https://api.freshservice.com/#view_all_project_tasks) | `GET` | `/api/v2/projects/[id]/tasks` | read | (none listed) | FS, FSBT |
| [Delete a Project Task](https://api.freshservice.com/#delete_a_project_task) | `DELETE` | `/api/v2/projects/[project_id]/tasks/[id]` | delete | (none listed) | FS, FSBT |

### Projects (new-gen, /pm)

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a Project](https://api.freshservice.com/#create_a_project_newgen) | `POST` | `/api/v2/pm/projects` | write | `freshservice.projects.manage` | FS, FSBT |
| [Update a Project](https://api.freshservice.com/#update_a_project_newgen) | `PUT` | `/api/v2/pm/projects/[id]` | write | `freshservice.projects.manage` | FS, FSBT |
| [View a Project](https://api.freshservice.com/#view_a_project_newgen) | `GET` | `/api/v2/pm/projects/[id]` | read | `freshservice.projects.view` | FS, FSBT |
| [List all Projects](https://api.freshservice.com/#view_all_projects_newgen) | `GET` | `/api/v2/pm/projects` | read | `freshservice.projects.view` | FS, FSBT |
| [Delete a Project](https://api.freshservice.com/#delete_a_project_newgen) | `DELETE` | `/api/v2/pm/projects/[id]` | delete | `freshservice.projects.manage` | FS, FSBT |
| [Archive a Project](https://api.freshservice.com/#archive_a_project_newgen) | `POST` | `/api/v2/pm/projects/[id]/archive` | write | `freshservice.projects.manage` | FS, FSBT |
| [Restore a Project](https://api.freshservice.com/#restore_a_project_newgen) | `POST` | `/api/v2/pm/projects/[id]/restore` | write (restore) | `freshservice.projects.manage` | FS, FSBT |
| [View Project Fields](https://api.freshservice.com/#view_project_fields) | `GET` | `/api/v2/pm/project-fields` | read | `freshservice.projects.fields.view` | FS, FSBT |
| [View Project Templates](https://api.freshservice.com/#view_project_templates) | `GET` | `/api/v2/pm/project_templates` | read | `freshservice.projects.view` | FS, FSBT |
| [Add Members](https://api.freshservice.com/#add_project_members) | `POST` | `/api/v2/pm/projects/[id]/members` | write | `freshservice.projects.manage` | FS, FSBT |
| [Create Associations](https://api.freshservice.com/#create_project_associations) | `POST` | `/api/v2/pm/projects/[project_id]/[module_name]` | write | `freshservice.projects.manage` | FS, FSBT |
| [View Associations](https://api.freshservice.com/#view_project_associations) | `GET` | `/api/v2/pm/projects/[project_id]/[module_name]` | read | `freshservice.projects.view` | FS, FSBT |
| [Delete Association](https://api.freshservice.com/#delete_project_association) | `DELETE` | `/api/v2/pm/projects/[project_id]/[module_name]/[id]` | delete | `freshservice.projects.manage` | FS, FSBT |
| [Delete Attachment of a Project](https://api.freshservice.com/#delete_attachment_project_newgen) | `DELETE` | `/api/v2/pm/projects/[project_id]/attachments/[id]` | delete | (none listed) | FS, FSBT |
| ***Projects Tasks*** | | | | | |
| [Create a Project Task](https://api.freshservice.com/#create_a_project_task_newgen) | `POST` | `/api/v2/pm/projects/[id]/tasks` | write | `freshservice.projects.tasks.create` | FS, FSBT |
| [Update a Project Task](https://api.freshservice.com/#update_a_project_task_newgen) | `PUT` | `/api/v2/pm/projects/[project_id]/tasks/[id]` | write | `freshservice.projects.tasks.edit` | FS, FSBT |
| [View a Project Task](https://api.freshservice.com/#view_a_project_task_newgen) | `GET` | `/api/v2/pm/projects/[project_id]/tasks/[id]` | read | `freshservice.projects.view` | FS, FSBT |
| [List all Project Tasks](https://api.freshservice.com/#view_all_project_tasks_newgen) | `GET` | `/api/v2/pm/projects/[id]/tasks` | read | `freshservice.projects.view` | FS, FSBT |
| [Filter all Project Tasks](https://api.freshservice.com/#filter_all_project_tasks_newgen) | `GET` | `/api/v2/pm/projects/[id]/tasks/filter?query=[query]` | read | `freshservice.projects.view` | FS, FSBT |
| [Delete a Project Task](https://api.freshservice.com/#delete_a_project_task_newgen) | `DELETE` | `/api/v2/pm/projects/[project_id]/tasks/[id]` | delete | `freshservice.projects.tasks.delete` | FS, FSBT |
| [View Project Task Type Fields](https://api.freshservice.com/#view_project_task_type_fields) | `GET` | `/api/v2/pm/projects/[project_id]/task-types/[type_id]/fields` | read | `freshservice.projects.manage` | FS, FSBT |
| [View Project Task Types](https://api.freshservice.com/#view_project_task_types) | `GET` | `/api/v2/pm/projects/[project_id]/task-types` | read | `freshservice.projects.view` | FS, FSBT |
| [View Project Task Priorities](https://api.freshservice.com/#view_project_task_priorities) | `GET` | `/api/v2/pm/projects/[project_id]/task-priorities` | read | `freshservice.projects.view` | FS, FSBT |
| [View Project Task Statuses](https://api.freshservice.com/#view_project_task_statuses) | `GET` | `/api/v2/pm/projects/[project_id]/task-statuses` | read | `freshservice.projects.view` | FS, FSBT |
| [View Project Versions](https://api.freshservice.com/#view_project_versions) | `GET` | `/api/v2/pm/projects/[project_id]/versions` | read | `freshservice.projects.view` | FS, FSBT |
| [View Project Sprints](https://api.freshservice.com/#view_project_sprints) | `GET` | `/api/v2/pm/projects/[project_id]/sprints` | read | `freshservice.projects.view` | FS, FSBT |
| [View Project Memberships](https://api.freshservice.com/#view_project_memberships) | `GET` | `/api/v2/pm/projects/[project_id]/memberships` | read | `freshservice.projects.view` | FS, FSBT |
| [Create Associations](https://api.freshservice.com/#create_project_task_associations) | `POST` | `/api/v2/pm/projects/[project_id]/tasks/[task_id]/[module_name]` | write | `freshservice.projects.tasks.edit` | FS, FSBT |
| [View Associations](https://api.freshservice.com/#view_project_task_associations) | `GET` | `/api/v2/pm/projects/[project_id]/tasks/[task_id]/[module_name]` | read | `freshservice.projects.view` | FS, FSBT |
| [Delete Association](https://api.freshservice.com/#delete_project_task_association) | `DELETE` | `/api/v2/pm/projects/[project_id]/tasks/[task_id]/[module_name]/[id]` | delete | `freshservice.projects.tasks.edit` | FS, FSBT |
| [Create Note](https://api.freshservice.com/#create_note_task_newgen) | `POST` | `/api/v2/pm/projects/[project_id]/tasks/[id]/notes` | write | `freshservice.projects.view` | FS, FSBT |
| [View Notes](https://api.freshservice.com/#view_notes_task_newgen) | `GET` | `/api/v2/pm/projects/[project_id]/tasks/[id]/notes` | read | `freshservice.projects.view` | FS, FSBT |
| [Update Note](https://api.freshservice.com/#update_note_task_newgen) | `PUT` | `/api/v2/pm/projects/[project_id]/tasks/[task_id]/notes/[id]` | write | `freshservice.projects.view` | FS, FSBT |
| [Delete Note](https://api.freshservice.com/#delete_note_task_newgen) | `DELETE` | `/api/v2/pm/projects/[project_id]/tasks/[task_id]/notes/[id]` | delete | (none listed) | FS, FSBT |
| [Delete Attachment of a Note](https://api.freshservice.com/#delete_attachment_note_task_newgen) | `DELETE` | `/api/v2/pm/projects/[project_id]/tasks/[task_id]/notes/[note_id]/attachments/[id]` | delete | (none listed) | FS, FSBT |
| [Delete Attachment of a Task](https://api.freshservice.com/#delete_attachment_task_newgen) | `DELETE` | `/api/v2/pm/projects/[project_id]/tasks/[task_id]/attachments/[id]` | delete | (none listed) | FS, FSBT |

### Solutions (knowledge base)

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create Solution Category](https://api.freshservice.com/#create_solution_category) | `POST` | `/api/v2/solutions/categories` | write | `freshservice.solutions.manage` | FS, FSBT, MSP |
| [Update Solution Category](https://api.freshservice.com/#update_solution_category) | `PUT` | `/api/v2/solutions/categories/{id}` | write | `freshservice.solutions.manage` | FS, FSBT, MSP |
| [View Solution Category](https://api.freshservice.com/#view_solution_category) | `GET` | `/api/v2/solutions/categories/{id}` | read | `freshservice.solutions.view` | FS, FSBT, MSP |
| [View List of Solution Categories](https://api.freshservice.com/#view_all_solution_category) | `GET` | `/api/v2/solutions/categories` | read | `freshservice.solutions.view` | FS, FSBT, MSP |
| [Delete Solution Category](https://api.freshservice.com/#delete_solution_category) | `DELETE` | `/api/v2/solutions/categories/{id}` | delete | `freshservice.solutions.manage` | FS, FSBT, MSP |
| [Restore Solution Category](https://api.freshservice.com/#restore_solution_category) | `PUT` | `/api/v2/solutions/categories/{category_id}/restore` | write (restore) | `freshservice.solutions.manage` | FS, FSBT, MSP |
| [Permanently Delete Solution Category](https://api.freshservice.com/#delete_forever_solution_category) | `DELETE` | `/api/v2/solutions/categories/{category_id}/delete_forever` | delete (permanent) | `freshservice.solutions.manage` | FS, FSBT, MSP |
| ***Solution Folder*** | | | | | |
| [Create Solution Folder](https://api.freshservice.com/#create_solution_folder) | `POST` | `/api/v2/solutions/folders` | write | `freshservice.solutions.manage` | FS, FSBT, MSP |
| [Create Solution Folder with Approval](https://api.freshservice.com/#create_solution_folder_with_approval) | `POST` | `/api/v2/solutions/folders` | write | `freshservice.solutions.manage` | FS, FSBT |
| [Update Solution Folder](https://api.freshservice.com/#update_solution_folder) | `PUT` | `/api/v2/solutions/folders/{id}` | write | `freshservice.solutions.manage` | FS, FSBT, MSP |
| [View Solution Folder](https://api.freshservice.com/#view_solution_folder) | `GET` | `/api/v2/solutions/folders/{id}` | read | `freshservice.solutions.view` | FS, FSBT, MSP |
| [View Solution Sub Folders](https://api.freshservice.com/#view_solution_sub_folders) | `GET` | `/api/v2/solutions/folders/{id}/sub-folders` | read | `freshservice.solutions.view` | FS, FSBT, MSP |
| [View List of Solution Folder](https://api.freshservice.com/#view_all_solution_folder) | `GET` | `/api/v2/solutions/folders` | read | `freshservice.solutions.view` | FS, FSBT, MSP |
| [Delete Solution Folder](https://api.freshservice.com/#delete_solution_folder) | `DELETE` | `/api/v2/solutions/folders/{id}` | delete | `freshservice.solutions.manage` | FS, FSBT, MSP |
| [Restore Solution Folder](https://api.freshservice.com/#restore_solution_folder) | `PUT` | `/api/v2/solutions/folders/{folder_id}/restore` | write (restore) | `freshservice.solutions.manage` | FS, FSBT, MSP |
| [Permanently Delete Solution Folder](https://api.freshservice.com/#delete_forever_solution_folder) | `DELETE` | `/api/v2/solutions/folders/{folder_id}/delete_forever` | delete (permanent) | `freshservice.solutions.manage` | FS, FSBT, MSP |
| ***Solution Article*** | | | | | |
| [Create Solution Article](https://api.freshservice.com/#create_solution_article) | `POST` | `/api/v2/solutions/articles` | write | `freshservice.solutions.publish` | FS, FSBT, MSP |
| [Create Secondary Language Article](https://api.freshservice.com/#create_secondary_language_article) | `POST` | `/api/v2/solutions/articles` | write | `freshservice.solutions.publish` | FS, FSBT, MSP |
| [Create Article With Attachment](https://api.freshservice.com/#create_solution_article_with_attachment) | `POST` | `/api/v2/solutions/articles` | write | `freshservice.solutions.publish` | FS, MSP |
| [Create Article from External URL](https://api.freshservice.com/#create_external_article) | `POST` | `/api/v2/solutions/articles` | write | `freshservice.solutions.publish` | FS, FSBT, MSP |
| [Search Solution Articles](https://api.freshservice.com/#search_solution_article) | `GET` | `/api/v2/solutions/articles/search` | read | `freshservice.solutions.view` | FS, FSBT, MSP |
| [Send Article to Approval](https://api.freshservice.com/#send_article_to_approval) | `PUT` | `/api/v2/solutions/articles/{id}/send_for_approval` | write | `freshservice.solutions.publish` | FS, FSBT, MSP |
| [Publish Solution Article](https://api.freshservice.com/#publish_solution_article) | `PUT` | `/api/v2/solutions/articles/{id}` | write | `freshservice.solutions.publish` | FS, FSBT, MSP |
| [Update Solution Article](https://api.freshservice.com/#update_solution_article) | `PUT` | `/api/v2/solutions/articles/{id}` | write | `freshservice.solutions.publish` | FS, FSBT, MSP |
| [View Solution Article](https://api.freshservice.com/#view_solution_article) | `GET` | `/api/v2/solutions/articles/{id}` | read | `freshservice.solutions.view` | FS, FSBT, MSP |
| [View List of Solution Article](https://api.freshservice.com/#view_all_solution_article) | `GET` | `/api/v2/solutions/articles` | read | `freshservice.solutions.view` | FS, FSBT, MSP |
| [Delete Solution Article](https://api.freshservice.com/#delete_solution_article) | `DELETE` | `/api/v2/solutions/articles/{id}` | delete | `freshservice.solutions.delete` | FS, FSBT, MSP |
| [Restore Solution Article](https://api.freshservice.com/#restore_solution_article) | `PUT` | `/api/v2/solutions/articles/{article_id}/restore` | write (restore) | `freshservice.solutions.delete` | FS, FSBT, MSP |
| [Permanently Delete Solution Article](https://api.freshservice.com/#delete_forever_solution_article) | `DELETE` | `/api/v2/solutions/articles/{article_id}/delete_forever` | delete (permanent) | `freshservice.solutions.delete` | FS, FSBT, MSP |
| [Bulk Restore Solution Articles](https://api.freshservice.com/#bulk_restore_solution_article) | `PUT` | `/api/v2/solutions/articles/bulk_restore` | write (restore) | `freshservice.solutions.delete` | FS, FSBT, MSP |

### Service catalog

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| ***Service Item*** | | | | | |
| [View a Service Item](https://api.freshservice.com/#view_service_item) | `GET` | `/api/v2/service_catalog/items/[display_id]` | read | `freshservice.service_catalog.edit` | FS, FSBT, MSP |
| [View List of Service Items](https://api.freshservice.com/#list_all_service_items) | `GET` | `/api/v2/service_catalog/items` | read | `freshservice.service_catalog.edit` | FS, FSBT, MSP |
| [Search Service Items](https://api.freshservice.com/#search_service_item) | `GET` | `/api/v2/service_catalog/items/search` | read | `freshservice.service_catalog.view` | FS, FSBT, MSP |
| [Create Service Catalog item](https://api.freshservice.com/#create_service_catalog_item) | `POST` | `/api/v2/service-catalog/items` | write | (none listed) | FS, FSBT, MSP |
| [Create a Service Catalog Item with agent group visibilities](https://api.freshservice.com/#create_service_catalog_item_with_agent_group_visibilities) | `POST` | `/api/v2/service-catalog/items` | write | (none listed) | FS, MSP |
| [Create a Service Catalog Item With Shared Fields](https://api.freshservice.com/#create_service_catalog_item_with_shared_fields) | `POST` | `/api/v2/service-catalog/items` | write | (none listed) | FS, MSP |
| [Update a Service Catalog Item](https://api.freshservice.com/#update_service_item) | `PUT` | `/api/v2/service-catalog/items/{service_item_id}` | write | (none listed) | FS, FSBT, MSP |
| [Delete a Service Catalog Item](https://api.freshservice.com/#delete_a_service_item) | `DELETE` | `/api/v2/service-catalog/items/{service_item_id}` | delete | (none listed) | FS, FSBT, MSP |
| ***Service Category*** | | | | | |
| [View List of Service Categories](https://api.freshservice.com/#list_all_service_categories) | `GET` | `/api/v2/service_catalog/categories` | read | (none listed) | FS, FSBT, MSP |
| [List all shared fields based on search](https://api.freshservice.com/#shared_fields) | `GET` | `/api/v2/service-catalog/shared-fields` | read | (none listed) | FS, FSBT, MSP |
| [Create a shared field](https://api.freshservice.com/#create_shared_fields) | `POST` | `/api/v2/service-catalog/shared-fields` | write | (none listed) | FS, FSBT, MSP |
| [Update shared fields](https://api.freshservice.com/#update_shared_fields) | `PUT` | `/api/v2/service-catalog/shared-fields/{id}` | write | (none listed) | FS, FSBT, MSP |
| [Delete shared fields](https://api.freshservice.com/#delete_shared_fields) | `DELETE` | `/api/v2/service-catalog/shared-fields/{id}` | delete | (none listed) | FS, FSBT, MSP |
| [Retrieve shared fields data](https://api.freshservice.com/#retrieve_shared_fields_data) | `GET` | `/api/v2/service-catalog/shared-fields/{id}` | read | (none listed) | FS, FSBT, MSP |
| [Archive shared fields](https://api.freshservice.com/#archive_shared_fields) | `POST` | `/api/v2/service-catalog/shared-fields/{id}/archive` | write | (none listed) | FS, FSBT, MSP |
| [Unarchive shared fields](https://api.freshservice.com/#unarchive_shared_fields) | `POST` | `/api/v2/service-catalog/shared-fields/{id}/unarchive` | write | (none listed) | FS, FSBT, MSP |

### Announcements

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create an Announcement](https://api.freshservice.com/#create_an_announcement) | `POST` | `/api/v2/announcements` | write | `freshservice.announcements.create` | FS, FSBT, MSP |
| [View an Announcement](https://api.freshservice.com/#view_an_announcement) | `GET` | `/api/v2/announcements/[id]` | read | `freshservice.announcements.view` | FS, FSBT, MSP |
| [List all Announcements](https://api.freshservice.com/#list_all_announcements) | `GET` | `/api/v2/announcements` | read | `freshservice.announcements.view` | FS, FSBT, MSP |
| [Edit an Announcement](https://api.freshservice.com/#edit_an_announcement) | `PUT` | `/api/v2/announcements/[id]` | write | `freshservice.announcements.edit` | FS, FSBT, MSP |
| [Delete an Announcement](https://api.freshservice.com/#delete_an_announcement) | `DELETE` | `/api/v2/announcements/[id]` | delete | `freshservice.announcements.delete` | FS, FSBT, MSP |

### Employee onboarding

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [View Onboarding Form Fields](https://api.freshservice.com/#onboarding_form) | `GET` | `/api/v2/onboarding_requests/form` | read | `freshservice.onboarding_requests.fields.view` | FS |
| [Create an Onboarding Request](https://api.freshservice.com/#create_onboarding_request) | `POST` | `/api/v2/onboarding_requests` | write | `freshservice.onboarding_requests.create` | FS |
| [View an Onboarding Request](https://api.freshservice.com/#view_onboarding_request) | `GET` | `/api/v2/onboarding_requests/[id]` | read | `freshservice.onboarding_requests.view` | FS |
| [View all Onboarding Requests](https://api.freshservice.com/#list_all_onboarding_requests) | `GET` | `/api/v2/onboarding_requests` | read | `freshservice.onboarding_requests.view` | FS |
| [View Onboarding Tickets](https://api.freshservice.com/#view_onboarding_tickets) | `GET` | `/api/v2/onboarding_requests/[id]/tickets` | read | `freshservice.onboarding_requests.view` | FS |

### Employee offboarding

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [View Offboarding Form Fields](https://api.freshservice.com/#offboarding_form) | `GET` | `/api/v2/offboarding_requests/form` | read | `freshservice.offboarding_requests.fields.view` | FS |
| [Create an Offboarding Request](https://api.freshservice.com/#create_offboarding_request) | `POST` | `/api/v2/offboarding_requests` | write | `freshservice.offboarding_requests.create` | FS |
| [View an Offboarding Request](https://api.freshservice.com/#view_offboarding_request) | `GET` | `/api/v2/offboarding_requests/[id]` | read | `freshservice.offboarding_requests.view` | FS |
| [View all Offboarding Requests](https://api.freshservice.com/#list_all_offboarding_requests) | `GET` | `/api/v2/offboarding_requests` | read | `freshservice.offboarding_requests.view` | FS |
| [View Offboarding Tickets](https://api.freshservice.com/#view_offboarding_tickets) | `GET` | `/api/v2/offboarding_requests/[id]/tickets` | read | `freshservice.offboarding_requests.view` | FS |

### Journeys

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [View List of Published Journeys Configs](https://api.freshservice.com/#journey_configs) | `GET` | `/api/v2/journeys/configs` | read | `freshservice.journeys.view` | FS, FSBT |
| [View Journey Initiator Config Form Fields](https://api.freshservice.com/#journey_initiator_config_form) | `GET` | `/api/v2/journeys/configs/[id]/data-fields` | read | `freshservice.journeys.configs.fields.view` | FS, FSBT |
| [Create a Journey Request](https://api.freshservice.com/#create_journey_request) | `POST` | `/api/v2/journeys/requests` | write | `freshservice.journey_request.create` | FS, FSBT |
| [View a Journey Request](https://api.freshservice.com/#view_journey_request) | `GET` | `/api/v2/journeys/requests/[id]` | read | `freshservice.journey_requests.view` | FS, FSBT |
| [Filter Journey Requests](https://api.freshservice.com/#filter_journey_requests) | `POST` | `/api/v2/journeys/requests/view` | read (POST query) | `freshservice.journey_requests.view` | FS, FSBT |
| [View List of Journey Requests](https://api.freshservice.com/#view_all_journey_requests) | `GET` | `/api/v2/journeys/requests` | read | (none listed) | FS, FSBT |
| [Update a Journey Request](https://api.freshservice.com/#update_journey_request) | `PATCH` | `/api/v2/journeys/requests/[id]` | write | `freshservice.journey_request.edit` | FS, FSBT |
| [Cancel a Journey Request](https://api.freshservice.com/#cancel_journey_request) | `PUT` | `/api/v2/journeys/requests/[id]/cancel` | write | `freshservice.journey_request.cancel` | FS, FSBT |
| [Delete a Journey Request](https://api.freshservice.com/#delete_journey_request) | `DELETE` | `/api/v2/journeys/requests/[id]` | delete | `freshservice.journey_request.delete` | FS, FSBT |
| [View Journey Request Activities](https://api.freshservice.com/#view_journey_request_activities) | `GET` | `/api/v2/journeys/requests/[id]/activities` | read | `freshservice.journey_request.view` | FS, FSBT |

### On-call management

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a schedule](https://api.freshservice.com/#create_schedule) | `POST` | `/api/v2/oncall/ws/[workspace_id]/schedules` | write | `freshservice.oncall.manage` | FS |
| [Update a schedule](https://api.freshservice.com/#edit_schedule) | `PUT` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]` | write | `freshservice.oncall.manage` | FS |
| [View all schedules](https://api.freshservice.com/#view_all_schedules) | `GET` | `/api/v2/oncall/ws/[workspace_id]/schedules?page=[page]&per_page=[per_page]` | read | `freshservice.oncall.view` | FS |
| [Filter schedules](https://api.freshservice.com/#filter_schedules) | `GET` | `/api/v2/oncall/ws/[workspace_id]/schedules?page=[page]&per_page=[per_page]&query=[schedule_name]` | read | `freshservice.oncall.view` | FS |
| [View a schedule](https://api.freshservice.com/#view_a_schedule) | `GET` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]` | read | `freshservice.oncall.view` | FS |
| [Delete a schedule](https://api.freshservice.com/#delete_schedule) | `DELETE` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]` | delete | `freshservice.oncall.manage` | FS |
| [Create a shift](https://api.freshservice.com/#create_shift) | `POST` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/shifts` | write | `freshservice.oncall.manage` | FS |
| [Update a shift](https://api.freshservice.com/#edit_shift) | `PUT` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/shifts/[shift_id]` | write | `freshservice.oncall.manage` | FS |
| [View all shifts](https://api.freshservice.com/#view_all_shifts) | `GET` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/shifts` | read | `freshservice.oncall.view` | FS |
| [View a shift](https://api.freshservice.com/#view_a_shift) | `GET` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/shifts/[shift_id]` | read | `freshservice.oncall.view` | FS |
| [Delete a shift](https://api.freshservice.com/#delete_shift) | `DELETE` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/shifts/[shift_id]` | delete | `freshservice.oncall.manage` | FS |
| [Create/Update/Delete an override](https://api.freshservice.com/#override) | `PUT` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/shifts/[shift_id]/rosters/override` | write (+delete) | `freshservice.oncall.override.manage` | FS |
| [View oncall calendar events for a user](https://api.freshservice.com/#view_calendar_events_user) | `GET` | `/api/v2/oncall/shift-events?start_time=[date]&end_time=[date]&user_id=[user_id]` | read | `freshservice.oncall.view` | FS |
| [View oncall calendar events for a schedule](https://api.freshservice.com/#view_calendar_events_schedule) | `GET` | `/api/v2/oncall/shift-events?start_time=[date]&end_time=[date]&schedule_id=[schedule_id]` | read | `freshservice.oncall.view` | FS |
| [View oncall calendar events for a shift](https://api.freshservice.com/#view_calendar_events_shift) | `GET` | `/api/v2/oncall/shift-events?start_time=[date]&end_time=[date]&shift_id=[shift_id]&schedule_id=[schedule_id]` | read | `freshservice.oncall.view` | FS |
| [View oncall calendar events of a shift for a user](https://api.freshservice.com/#view_calendar_events_shift_user) | `GET` | `/api/v2/oncall/shift-events?start_time=[date]&end_time=[date]&shift_id=[shift_id]&schedule_id=[schedule_id]&user_id=[user_id]` | read | `freshservice.oncall.view` | FS |
| [View oncall calendar events of a schedule for a user](https://api.freshservice.com/#view_calendar_events_schedule_user) | `GET` | `/api/v2/oncall/shift-events?start_time=[date]&end_time=[date]&schedule_id=[schedule_id]&user_id=[user_id]` | read | `freshservice.oncall.view` | FS |
| [Export oncall calendar events for a user](https://api.freshservice.com/#export_calendar_events_user) | `GET` | `/api/v2/oncall/shift-events/export?user_id=[user_id]&export_type=[export_type]` | read | `freshservice.oncall.view` | FS |
| [Export oncall calendar events for a schedule](https://api.freshservice.com/#export_calendar_events_schedule) | `GET` | `/api/v2/oncall/shift-events/export?schedule_id=[schedule_id]&export_type=[export_type]` | read | `freshservice.oncall.view` | FS |
| [Export oncall calendar events for a shift](https://api.freshservice.com/#export_calendar_events_shift) | `GET` | `/api/v2/oncall/shift-events/export?shift_id=[shift_id]&schedule_id=[schedule_id]&export_type=[export_type]` | read | `freshservice.oncall.view` | FS |
| [Export oncall calendar events of a shift for a user](https://api.freshservice.com/#export_calendar_events_shift_user) | `GET` | `/api/v2/oncall/shift-events/export?shift_id=[shift_id]&schedule_id=[schedule_id]&user_id=[user_id]&export_type=[export_type]` | read | `freshservice.oncall.view` | FS |
| [Export oncall calendar events of a schedule for a user](https://api.freshservice.com/#export_calendar_events_schedule_user) | `GET` | `/api/v2/oncall/shift-events/export?schedule_id=[schedule_id]&user_id=[user_id]&export_type=[export_type]` | read | `freshservice.oncall.view` | FS |
| [Export oncall calendar events as an .ical file for a user](https://api.freshservice.com/#export_ical_calendar_events_user) | `GET` | `/api/v2/oncall/shift-events/export?user_id=[user_id]&start_time=[date]&end_time=[date]&export_type=[export_type]` | read | `freshservice.oncall.view` | FS |
| [Export oncall calendar events as an .ical file for a schedule](https://api.freshservice.com/#export_ical_calendar_events_schedule) | `GET` | `/api/v2/oncall/shift-events/export?schedule_id=[schedule_id]&start_time=[date]&end_time=[date]&export_type=[export_type]` | read | `freshservice.oncall.view` | FS |
| [Export oncall calendar events as an .ical file for a shift](https://api.freshservice.com/#export_ical_calendar_events_shift) | `GET` | `/api/v2/oncall/shift-events/export?shift_id=[shift_id]&schedule_id=[schedule_id]&start_time=[date]&end_time=[date]&export_type=[export_type]` | read | `freshservice.oncall.view` | FS |
| [Export oncall calendar events as an .ical file of a shift for a user](https://api.freshservice.com/#export_ical_calendar_events_shift_user) | `GET` | `/api/v2/oncall/shift-events/export?shift_id=[shift_id]&schedule_id=[schedule_id]&user_id=[user_id]&start_time=[date]&end_time=[date]&export_type=[export_type]` | read | `freshservice.oncall.view` | FS |
| [Export oncall calendar events as an .ical file of a schedule for a user](https://api.freshservice.com/#export_ical_calendar_events_schedule_user) | `GET` | `/api/v2/oncall/shift-events/export?schedule_id=[schedule_id]&user_id=[user_id]&start_time=[date]&end_time=[date]&export_type=[export_type]` | read | `freshservice.oncall.view` | FS |
| [View who is oncall](https://api.freshservice.com/#view_who_is_oncall) | `GET` | `/api/v2/oncall/shift-events/current?schedule_id=[schedule_id]` | read | `freshservice.oncall.view` | FS |
| [Create an escalation policy](https://api.freshservice.com/#create_ep) | `POST` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/escalation-policies` | write | `freshservice.oncall.manage` | FS |
| [Update an escalation policy](https://api.freshservice.com/#edit_ep) | `PUT` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/escalation-policies/[ep_id]` | write | `freshservice.oncall.manage` | FS |
| [View all escalation policies](https://api.freshservice.com/#view_all_ep) | `GET` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/escalation-policies` | read | `freshservice.oncall.view` | FS |
| [View an escalation policy](https://api.freshservice.com/#view_a_ep) | `GET` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/escalation-policies/[ep_id]` | read | `freshservice.oncall.view` | FS |
| [Delete an escalation policy](https://api.freshservice.com/#delete_ep) | `DELETE` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/escalation-policies/[ep_id]` | delete | `freshservice.oncall.manage` | FS |
| [Reorder an escalation policy](https://api.freshservice.com/#reorder_ep) | `PUT` | `/api/v2/oncall/ws/[workspace_id]/schedules/[schedule_id]/escalation-policies/reorder` | write | `freshservice.oncall.manage` | FS |

### Custom objects

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [View List of Custom Objects](https://api.freshservice.com/#list_all_custom_objects) | `GET` | `/api/v2/objects` | read | `freshservice.objects.manage` | FS, FSBT |
| [Show a Custom Object](https://api.freshservice.com/#show_custom_object) | `GET` | `/api/v2/objects/[id]` | read | `freshservice.objects.manage` | FS, FSBT |
| [Create new Custom Object Record](https://api.freshservice.com/#create_custom_object_record) | `POST` | `/api/v2/objects/[id]/records` | write | `freshservice.objects.manage` | FS, FSBT |
| [List all records of a Custom Object](https://api.freshservice.com/#list_all_custom_object_records) | `GET` | `/api/v2/objects/[id]/records` | read | `freshservice.objects.manage` | FS, FSBT |
| [Update Custom Object Record](https://api.freshservice.com/#put_custom_object_record) | `PUT` | `/api/v2/objects/[id]/records/[record_id]` | write | `freshservice.objects.manage` | FS, FSBT |
| [Delete Custom Object Record](https://api.freshservice.com/#delete_custom_object_record) | `DELETE` | `/api/v2/objects/[id]/records/[record_id]` | delete | `freshservice.objects.manage` | FS, FSBT |

### Post-incident report templates

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create new templates for post-incident reports](https://api.freshservice.com/#create_new_template) | `POST` | `/api/v2/post-incident-reports/templates` | write | `freshservice.pir_template.manage` | FS |
| [Get post-incident report templates by ID](https://api.freshservice.com/#get_pir_templates_by_id) | `GET` | `/api/v2/post-incident-reports/templates/[id]` | read | `freshservice.pir_template.manage` | FS |
| [Get all post-incident report templates](https://api.freshservice.com/#get_all_pir_templates) | `GET` | `/api/v2/post-incident-reports/templates` | read | `freshservice.pir_template.manage` | FS |
| [Enable template for post-incident reports](https://api.freshservice.com/#enable_pir_template) | `PUT` | `/api/v2/post-incident-reports/templates/[id]` | write | `freshservice.pir_template.manage` | FS |
| [Set template as primary for post-incident reports](https://api.freshservice.com/#mark_primary_pir_template) | `PUT` | `/api/v2/post-incident-reports/templates/[id]/mark-as-primary` | write | `freshservice.pir_template.manage` | FS |
| [Export the post-incident report template](https://api.freshservice.com/#export_pir_template) | `POST` | `/api/v2/post-incident-reports/templates/[id]/export` | read (export) | `freshservice.pir_template.manage` | FS |
| [Delete an existing post-incident report template](https://api.freshservice.com/#delete_pir_template) | `DELETE` | `/api/v2/post-incident-reports/templates/[id]` | delete | `freshservice.pir_template.manage` | FS |

### SLA policies

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [View List of SLAs](https://api.freshservice.com/#list_all_sla) | `GET` | `/api/v2/sla_policies` | read | `freshservice.sla_policies.view` | FS, FSBT, MSP |

### Canned responses

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [View List of Canned Response Folders](https://api.freshservice.com/#list_all_canned_response_folders) | `GET` | `/api/v2/canned_response_folders` | read | `freshservice.canned_responses.view` | FS, FSBT |
| [Show a Canned Response Folder](https://api.freshservice.com/#show_canned_response_folder) | `GET` | `/api/v2/canned_response_folders/[folder_id]` | read | `freshservice.canned_responses.view` | FS, FSBT |
| [List all Canned Responses In a Folder](https://api.freshservice.com/#list_all_canned_response_in_folder) | `GET` | `/api/v2/canned_response_folders/[folder_id]/canned_responses` | read | `freshservice.canned_responses.view` | FS, FSBT |
| ***Canned Responses*** | | | | | |
| [View List of Canned Responses](https://api.freshservice.com/#list_all_canned_responses) | `GET` | `/api/v2/canned_responses` | read | `freshservice.canned_responses.view` | FS, FSBT |
| [Show a Canned Response](https://api.freshservice.com/#show_canned_response) | `GET` | `/api/v2/canned_responses/[id]` | read | `freshservice.canned_responses.view` | FS, FSBT |

### Audit logs

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Audit Log Export](https://api.freshservice.com/#export_audit_logs) | `POST` | `/api/v2/audit_log/export` | read (async export) | `freshservice.audit_log.view` | FS |

### Attachments

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Download Attachment](https://api.freshservice.com/#download_attachment) | `GET` | `/api/v2/attachments/[id]` | read | (none listed) | FS, FSBT, MSP |
| [Download Purchase order and Contracts Attachment](https://api.freshservice.com/#download_purchase_and_contract_attachment) | `GET` | `/api/v2/itil_attachments/[id]` | read | (none listed) | FS |

### Collaboration (Email, Zoom)

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| ***Email*** | | | | | |
| [Trigger email for a major incident](https://api.freshservice.com/#trigger_email) | `POST` | `/api/v2/tickets/[ticket_id]/communications` | write | `freshservice.tickets.edit` | FS |
| [Trigger an Email with attachment for a Major Incident](https://api.freshservice.com/#trigger_email_with_attachment) | `POST` | `/api/v2/tickets/[ticket_id]/communications` | write | `freshservice.tickets.edit` | FS |
| [Retrieve specific emails using ID](https://api.freshservice.com/#retrieve_emails_with_id) | `GET` | `/api/v2/tickets/[ticket_id]/communications/[communication_id]` | read | `freshservice.tickets.edit` | FS |
| [Retrieve emails related to a ticket](https://api.freshservice.com/#retrieve_emails) | `GET` | `/api/v2/tickets/[ticket_id]/communications` | read | `freshservice.tickets.edit` | FS |
| ***Zoom*** | | | | | |
| [Trigger a new zoom meeting](https://api.freshservice.com/#trigger_new_meeting) | `POST` | `/api/v2/tickets/[ticket_id]/communications` | write | `freshservice.tickets.edit` | FS |
| [Retrieve zoom meetings related to a ticket with ID](https://api.freshservice.com/#retrieve_zoom_meetings_with_id) | `GET` | `/api/v2/tickets/[ticket_id]/communications/[communication_id]` | read | `freshservice.tickets.edit` | FS |
| [Retrieve zoom meetings related to a ticket](https://api.freshservice.com/#retrieve_zoom_meetings) | `GET` | `/api/v2/tickets/[ticket_id]/communications` | read | `freshservice.tickets.edit` | FS |

### Status page

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| ***Impacted Service Properties*** | | | | | |
| ***Incidents*** | | | | | |
| ***Incident properties*** | | | | | |
| [List all incidents](https://api.freshservice.com/#view_incidents) | `GET` | `/api/v2/status/pages/[status_page_id]/incidents` | read | `freshservice.statuspage.incidents.view` | FS |
| [View an incident](https://api.freshservice.com/#view_incident) | `GET` | `/api/v2/tickets/[ticket_id]/status/pages/[status_page_id]/incidents/[id]` | read | `freshservice.statuspage.incidents.view` | FS |
| [Create an incident](https://api.freshservice.com/#create_incident) | `POST` | `/api/v2/tickets/[ticket_id]/status/pages/[status_page_id]/incidents` | write | `freshservice.statuspage.incidents.publish` | FS |
| [Update an incident](https://api.freshservice.com/#update_incident) | `PUT` | `/api/v2/tickets/[ticket_id]/status/pages/[status_page_id]/incidents/[id]` | write | `freshservice.statuspage.incidents.publish` | FS |
| [Delete an incident](https://api.freshservice.com/#delete_incident) | `DELETE` | `/api/v2/tickets/[ticket_id]/status/pages/[status_page_id]/incidents/[id]` | delete | `freshservice.statuspage.incidents.delete` | FS |
| ***Incident Updates*** | | | | | |
| [List all updates for an incident](https://api.freshservice.com/#view_incident_updates) | `GET` | `/api/v2/tickets/[ticket_id]/status/pages/[status_page_id]/incidents/[incident_id]/updates` | read | `freshservice.statuspage.incidents.view` | FS |
| [Create incident update](https://api.freshservice.com/#create_incident_update) | `POST` | `/api/v2/tickets/[ticket_id]/status/pages/[status_page_id]/incidents/[incident_id]/updates` | write | `freshservice.statuspage.incidents.publish` | FS |
| [Edit an incident update](https://api.freshservice.com/#update_incident_update) | `PUT` | `/api/v2/tickets/[ticket_id]/status/pages/[status_page_id]/incidents/[incident_id]/updates/[id]` | write | `freshservice.statuspage.incidents.publish` | FS |
| [Delete an incident update](https://api.freshservice.com/#delete_incident_update) | `DELETE` | `/api/v2/tickets/[ticket_id]/status/pages/[status_page_id]/incidents/[incident_id]/updates/[id]` | delete | `freshservice.statuspage.incidents.delete` | FS |
| [List all incident statuses](https://api.freshservice.com/#list_all_incident_statuses) | `GET` | `/api/v2/status/pages/[status_page_id]/incidents/statuses` | read | `freshservice.statuspage.view` | FS |
| ***Maintenance*** | | | | | |
| ***Notification Options*** | | | | | |
| ***Maintenance properties*** | | | | | |
| [List all maintenances](https://api.freshservice.com/#view_maintenances) | `GET` | `/api/v2/status/pages/[status_page_id]/maintenances` | read | `freshservice.statuspage.maintenances.view` | FS |
| [Create a maintenance from a change](https://api.freshservice.com/#create_maintenance_from_change) | `POST` | `/api/v2/changes/[change_id]/status/pages/[status_page_id]/maintenances` | write | `freshservice.statuspage.maintenances.publish` | FS |
| [Update a maintenance from a change](https://api.freshservice.com/#update_maintenance_from_change) | `PUT` | `/api/v2/changes/[change_id]/status/pages/[status_page_id]/maintenances/[id]` | write | `freshservice.statuspage.maintenances.publish` | FS |
| [View a maintenance from a change](https://api.freshservice.com/#view_maintenance_from_change) | `GET` | `/api/v2/changes/[change_id]/status/pages/[status_page_id]/maintenances/[id]` | read | `freshservice.statuspage.maintenances.view` | FS |
| [Delete a maintenance from a change](https://api.freshservice.com/#delete_maintenance_from_change) | `DELETE` | `/api/v2/changes/[change_id]/status/pages/[status_page_id]/maintenances/[id]` | delete | `freshservice.statuspage.maintenances.delete` | FS |
| [Create a maintenance from a maintenance window](https://api.freshservice.com/#create_maintenance_from_maintenance_window) | `POST` | `/api/v2/maintenance-windows/[maintenance_window_id]/status/pages/[status_page_id]/maintenances` | write | `freshservice.statuspage.maintenances.publish` | FS |
| [Update a maintenance from a maintenance window](https://api.freshservice.com/#update_maintenance_from_maintenance_window) | `PUT` | `/api/v2/maintenance-windows/[maintenance_window_id]/status/pages/[status_page_id]/maintenances/[id]` | write | `freshservice.statuspage.maintenances.publish` | FS |
| [View a maintenance from a maintenance window](https://api.freshservice.com/#view_maintenance_from_maintenance_window) | `GET` | `/api/v2/maintenance-windows/[maintenance_window_id]/status/pages/[status_page_id]/maintenances/[id]` | read | `freshservice.statuspage.maintenances.view` | FS |
| [Delete a maintenance from a maintenance window](https://api.freshservice.com/#delete_maintenance_from_maintenance_window) | `DELETE` | `/api/v2/maintenance-windows/[maintenance_window_id]/status/pages/[status_page_id]/maintenances/[id]` | delete | `freshservice.statuspage.maintenances.delete` | FS |
| [List all maintenance statuses](https://api.freshservice.com/#list_all_maintenance_statuses) | `GET` | `/api/v2/status/pages/[status_page_id]/maintenances/statuses` | read | `freshservice.statuspage.view` | FS |
| ***Maintenance Updates*** | | | | | |
| [Create a maintenance update from a change](https://api.freshservice.com/#create_maintenance_update_from_change) | `POST` | `/api/v2/changes/[change_id]/status/pages/[status_page_id]/maintenances/[maintenance_id]/updates` | write | `freshservice.statuspage.maintenances.publish` | FS |
| [Update a maintenance update from a change](https://api.freshservice.com/#update_maintenance_update_from_change) | `PUT` | `/api/v2/changes/[change_id]/status/pages/[status_page_id]/maintenances/[maintenance_id]/updates/[id]` | write | `freshservice.statuspage.maintenances.publish` | FS |
| [List all maintenance updates from a change](https://api.freshservice.com/#view_maintenance_update_from_change) | `GET` | `/api/v2/changes/[change_id]/status/pages/[status_page_id]/maintenances/[id]/updates` | read | `freshservice.statuspage.maintenances.view` | FS |
| [Delete a maintenance update from a change](https://api.freshservice.com/#delete_maintenance_update_from_change) | `DELETE` | `/api/v2/changes/[change_id]/status/pages/[status_page_id]/maintenances/[id]/updates/[id]` | delete | `freshservice.statuspage.maintenances.delete` | FS |
| [Create a maintenance update from a maintenance window](https://api.freshservice.com/#create_maintenance_update_from_maintenance_window) | `POST` | `/api/v2/maintenance-windows/[maintenance_window_id]/status/pages/[status_page_id]/maintenances/[maintenance_id]/updates` | write | `freshservice.statuspage.maintenances.publish` | FS |
| [Update a maintenance update from a maintenance window](https://api.freshservice.com/#update_maintenance_update_from_maintenance_window) | `PUT` | `/api/v2/maintenance-windows/[maintenance_window_id]/status/pages/[status_page_id]/maintenances/[maintenance_id]/updates/[id]` | write | `freshservice.statuspage.maintenances.publish` | FS |
| [List all maintenance updates from a maintenance window](https://api.freshservice.com/#view_maintenance_update_from_maintenance_window) | `GET` | `/api/v2/maintenance-windows/[maintenance_window_id]/status/pages/[status_page_id]/maintenances/[id]/updates` | read | `freshservice.statuspage.maintenances.view` | FS |
| [Delete a maintenance update from a maintenance window](https://api.freshservice.com/#delete_maintenance_update_from_maintenance_window) | `DELETE` | `/api/v2/maintenance-windows/[maintenance_window_id]/status/pages/[status_page_id]/maintenances/[id]/updates/[id]` | delete | `freshservice.statuspage.maintenances.delete` | FS |
| [List all status pages](https://api.freshservice.com/#view_status_pages) | `GET` | `/api/v2/status/pages?workspace_id=[workspace_id]` | read | `freshservice.statuspage.view` | FS |
| [Identify publishable services from a ticket](https://api.freshservice.com/#identify_publishable_services_ticket) | `GET` | `/api/v2/tickets/[ticket_id]/status/pages/[status_page_id]/publishable-services` | read | `freshservice.statuspage.incidents.view` | FS |
| [Identify publishable services from a change](https://api.freshservice.com/#identify_publishable_services_change) | `GET` | `/api/v2/changes/[change_id]/status/pages/[status_page_id]/publishable-services` | read | `freshservice.statuspage.maintenances.view` | FS |
| [Identify publishable services from a maintenance window](https://api.freshservice.com/#identify_publishable_services_maintenance) | `GET` | `/api/v2/maintenance-windows/[maintenance_window_id]/status/pages/[status_page_id]/publishable-services` | read | `freshservice.statuspage.maintenances.view` | FS |
| [View a service component](https://api.freshservice.com/#view_service_component) | `GET` | `/api/v2/status/pages/[status_page_id]/service-components/[id]` | read | `freshservice.statuspage.view` | FS |
| [List all service components](https://api.freshservice.com/#view_service_components) | `GET` | `/api/v2/status/pages/[status_page_id]/service-components` | read | `freshservice.statuspage.view` | FS |
| ***Subscriber Management*** | | | | | |
| [List all subscribers](https://api.freshservice.com/#list_all_subscribers) | `GET` | `/api/v2/status/pages/[status_page_id]/subscribers` | read | `freshservice.statuspage.subscribers.view` | FS |
| [View a subscriber](https://api.freshservice.com/#view_a_subscriber) | `GET` | `/api/v2/status/pages/[status_page_id]/subscribers/[subscriber_id]` | read | `freshservice.statuspage.subscribers.view` | FS |
| [Create a subscriber](https://api.freshservice.com/#create_subscriber) | `POST` | `/api/v2/status/pages/[status_page_id]/subscribers` | write | `freshservice.statuspage.subscribers.manage` | FS |
| [Update a subscriber](https://api.freshservice.com/#update_subscriber) | `PUT` | `/api/v2/status/pages/[status_page_id]/subscribers/[subscriber_id]` | write | `freshservice.statuspage.subscribers.manage` | FS |
| [Delete a subscriber](https://api.freshservice.com/#delete_subscriber) | `DELETE` | `/api/v2/status/pages/[status_page_id]/subscribers/[subscriber_id]` | delete | `freshservice.statuspage.subscribers.delete` | FS |

### Delegation

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create a delegation](https://api.freshservice.com/#create_delegation) | `POST` | `/api/v2/users/[user_id]/delegation` | write | `freshservice.users.manage` | FS |
| [Update a delegation](https://api.freshservice.com/#update_delegation) | `PUT` | `/api/v2/users/[user_id]/delegation` | write | `freshservice.users.manage` | FS |
| [View a delegation](https://api.freshservice.com/#view_delegation) | `GET` | `/api/v2/users/[user_id]/delegation` | read | `freshservice.tickets.view (or freshservice.users.view)` | FS |
| [Delete a delegation](https://api.freshservice.com/#delete_delegation) | `DELETE` | `/api/v2/users/[user_id]/delegation` | delete | `freshservice.users.manage` | FS |

### ITAM physical subtypes, devices, cloud

| Operation | Method | Path | Class | OAuth scope | Products |
|---|---|---|---|---|---|
| [Create or update physical subtypes](https://api.freshservice.com/#create_physical_subtype) | `POST` | `/api/v2/itam/physical-subtypes` | write | (none listed) | ITAM |
| [View list of physical subtypes](https://api.freshservice.com/#view_all_physical_subtypes) | `GET` | `/api/v2/itam/physical-subtypes` | read | (none listed) | ITAM |
| [View a physical subtype](https://api.freshservice.com/#view_a_physical_subtype) | `GET` | `/api/v2/itam/physical-subtypes/{id}` | read | (none listed) | ITAM |
| [Delete a physical subtype](https://api.freshservice.com/#delete_physical_subtype) | `DELETE` | `/api/v2/itam/physical-subtypes/{id}/` | delete | (none listed) | ITAM |
| ***Devices*** | | | | | |
| [Create a device](https://api.freshservice.com/#create_device) | `POST` | `/api/v2/itam/devices` | write | (none listed) | ITAM |
| [View list of all devices](https://api.freshservice.com/#view_all_devices) | `GET` | `/api/v2/itam/devices` | read | (none listed) | ITAM |
| [View a device](https://api.freshservice.com/#view_a_device) | `GET` | `/api/v2/itam/devices/{id}` | read | (none listed) | ITAM |
| [Update a device](https://api.freshservice.com/#update_device) | `PUT` | `/api/v2/itam/devices/{id}/` | write | (none listed) | ITAM |
| [Create or update custom field](https://api.freshservice.com/#update_custom_field_of_devices) | `PUT` | `/api/v2/itam/custom_fields/devices/` | write | (none listed) | ITAM |
| [Delete a device](https://api.freshservice.com/#delete_device) | `DELETE` | `/api/v2/itam/devices/{id}/` | delete | (none listed) | ITAM |
| ***Resources*** | | | | | |
| [Update a cloud resource](https://api.freshservice.com/#update_cloud_resource) | `POST` | `/api/v2/itam/resources/` | write | (none listed) | ITAM |
| [View list of all cloud resources](https://api.freshservice.com/#view_all_cloud_resources) | `GET` | `/api/v2/itam/resources` | read | (none listed) | ITAM |
| [View a cloud resource](https://api.freshservice.com/#view_a_cloud_resource) | `GET` | `/api/v2/itam/resources/{id}` | read | (none listed) | ITAM |
| [Delete a cloud resource](https://api.freshservice.com/#delete_cloud_resource) | `DELETE` | `/api/v2/itam/resources/{id}/` | delete | (none listed) | ITAM |
| [Create or update custom field](https://api.freshservice.com/#update_custom_field_of_resources) | `PUT` | `/api/v2/itam/custom_fields/resources/` | write | (none listed) | ITAM |
| ***Resource relationships*** | | | | | |
| [View list of all cloud resource relationships](https://api.freshservice.com/#view_all_cloud_resource_relationships) | `GET` | `/api/v2/itam/resource_relationships` | read | (none listed) | ITAM |
| [Delete a cloud resource relationship](https://api.freshservice.com/#delete_cloud_resource_relationship) | `DELETE` | `/api/v2/itam/resource_relationships/{id}/` | delete | (none listed) | ITAM |
| ***Cloud infrastructure*** | | | | | |
| [Update cloud details](https://api.freshservice.com/#update_cloud_infrastructure) | `POST` | `/api/v2/itam/cloud_infrastructures/` | write | (none listed) | ITAM |
| [View cloud details](https://api.freshservice.com/#view_all_cloud_infrastructure) | `GET` | `/api/v2/itam/cloud_infrastructures` | read | (none listed) | ITAM |
| [Delete cloud details](https://api.freshservice.com/#delete_cloud_infrastructure) | `DELETE` | `/api/v2/itam/cloud_infrastructures/{id}/` | delete | (none listed) | ITAM |
| [Create or update custom field](https://api.freshservice.com/#update_custom_field_of_cloud_infrastructure) | `PUT` | `/api/v2/itam/custom_fields/cloudinfrastructures/` | write | (none listed) | ITAM |
| ***Lifecycle events*** | | | | | |
| [Create or update lifecycle events](https://api.freshservice.com/#update_lifecycle_events) | `PUT` | `/api/v2/itam/lifecycle_events/` | write | (none listed) | ITAM |
| [View list of all lifecycle events](https://api.freshservice.com/#view_all_lifecycle_events) | `GET` | `/api/v2/itam/lifecycle_events` | read | (none listed) | ITAM |
| [View a lifecycle event](https://api.freshservice.com/#view_a_lifecycle_event) | `GET` | `/api/v2/itam/lifecycle_events/{id}` | read | (none listed) | ITAM |
| [Delete a lifecycle event](https://api.freshservice.com/#delete_lifecycle_event) | `DELETE` | `/api/v2/itam/lifecycle_events/{id}/` | delete | (none listed) | ITAM |