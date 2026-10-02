# Freshservice API mechanics

Research for issue #3. It covers how the Freshservice v2 REST API behaves at runtime, so we can design the Go client's rate limiter, paging, filter usage and error mapping.

Sources (read 2026-10-01):

- **[DOCS]** Freshservice API v2 reference, https://api.freshservice.com/. This is one long page. The section anchors are `#rate_limit`, `#authentication`, `#embedding`, `#error`, `#pagination`, `#best_practices`, `#filter_tickets`, `#view_all_ticket`, `#filter_assets`, `#view_all_assets`, `#filter_requesters`, `#filter_agents` and `#filter_departments`. Quotes below are copied from that page.
- **[KB-RL]** "What is the rate limit for APIs across all plans?", https://support.freshservice.com/support/solutions/articles/50000000293
- **[KB-V2]** "Introducing upgraded APIs", https://support.freshservice.com/support/solutions/articles/50000004220
- **[KB-EUC]** "EUC data centre migration: Here's what you should do", https://support.freshservice.com/support/solutions/articles/234149

## 1. Base URL, regions, custom domains

- Every resource URI has the form `https://<your_helpdesk_domain_name>/api/v2/<resource>`, for example `https://acmeinc.freshservice.com/api/v2/tickets`. [DOCS #authentication]
- The API accepts HTTPS only. Plain HTTP fails with error code `ssl_required`. [DOCS Introduction, Errors]
- v2 "Works only via Freshservice domains and not via custom CNAMEs" [DOCS "What's New?"]. **A custom portal domain such as `help.acme.com` cannot be used as the API base.** Always use `<sub>.freshservice.com`.
- Regions: the docs list no regional API hostnames (no `freshservice.eu` or similar). Every example uses `domain.freshservice.com`. EU-hosted (Frankfurt, EUC) accounts are still reached at `yourdomainname.freshservice.com`. The KB only changes the target of a custom-URL CNAME, to `euc-lb1.freshservice.com`. [KB-EUC] **Decision: configure the full host (`FRESHSERVICE_DOMAIN=acme.freshservice.com`) or the subdomain. Do not model a region setting.** (Inference: data residency is decided by the account and resolved by DNS behind `*.freshservice.com`. We found no first-party page that names a per-region API host.)
- Products: one API surface serves Freshservice, Freshservice for Business Teams (FSBT), Freshservice for MSPs and Freshservice IT Asset Management. Each endpoint is tagged with the products it applies to. In MSP accounts, "Workspace" is called "Client", but the attribute is still `workspace_id`. [DOCS Introduction]
- Asset API split: `GET /api/v2/assets` "is valid for Freshservice signups before March 31, 2026". Newer signups use the ITAM endpoint `GET /api/v2/itam/assets`, which pages with `page`/`per_page` and takes `include_cols`. [DOCS #view_all_assets, ITAM "View list of assets"]

## 2. Authentication

- Basic auth with the API key as the username and any dummy password: `curl -u apikey:X ...`. "Freshservice ... supports Basic Access Authorization only with API key." [DOCS #authentication]
- Header form: `Authorization: Basic base64("<apikey>:X")`. The docs give forgetting the Base64 step as a common cause of `invalid_credentials`. [DOCS #authentication, Errors]
- Username/password basic auth was removed on 31 May 2023. It now fails with `unsupported_authentication_type` and status 401. [DOCS Changelog Jun 2023, Errors]
- OAuth is also supported (since April 2024), and each endpoint lists its scope, for example `freshservice.tickets.view`. Expired or invalid tokens return `access_token_expired` or `access_token_invalid`. [DOCS Changelog Apr 2024, Errors] We can ignore OAuth for an API-key server.
- What a key can do depends on the agent's role. For example, a view-only agent role is refused write calls. [DOCS #authentication] **For error mapping: a 403 from a valid key is a permission problem, not an authentication problem.**

## 3. Rate limits

### Model

- Accounts created on or after 1 Sep 2020 have **per-minute** limits. The limit is **account-wide**, "irrespective of factors such as the number of agents or IP addresses". [DOCS #rate_limit, KB-RL]
- Accounts created before 1 Sep 2020 "will eventually be migrated to minute-level rate limiting. The previous limits would continue to apply until the migration is completed." [KB-RL] The old v1 limit was 1000 calls per hour. [KB-RL] The docs also describe `X-RateLimit-Total` as "Total number of API calls allowed per hour". Their own sample shows `X-Ratelimit-Total: 3000` and `Retry-After: 1521`, which looks like a leftover from the hourly era. [DOCS #rate_limit] **Decision: do not hardcode the window. Trust `Retry-After` and the `X-Ratelimit-*` headers.**

### Per-plan limits (requests per minute) [DOCS #rate_limit]

| Action | Starter | Growth | Pro | Enterprise | Add-on 1 | Add-on 2 |
|---|---|---|---|---|---|---|
| **Overall** | 100 | 200 | 400 | 500 | 1000 | 2000 |
| List All Tickets | 40 | 70 | 120 | 140 | 180 | 480 |
| View Ticket | 50 | 80 | 140 | 160 | 300 | 600 |
| Create Ticket | 50 | 80 | 140 | 160 | 300 | 600 |
| Update Ticket | 50 | 80 | 140 | 160 | 300 | 600 |
| List All Assets | 40 | 70 | 120 | 140 | 180 | 480 |
| Update Asset | 50 | 80 | 140 | 160 | 300 | 600 |
| List All Agents | 40 | 70 | 120 | 140 | 180 | 480 |
| List All Requesters | 40 | 70 | 120 | 140 | 180 | 480 |

- Sub-limits apply inside the overall limit. Endpoints without a sub-limit share only the overall limit. [DOCS #rate_limit]
- FSBT (Pro) and MSP (Core) accounts have the Growth limits for the overall, ticket, agent and requester rows. [DOCS #rate_limit]
- Add-on packs are sold only for Pro and Enterprise. [DOCS #rate_limit]
- **Calls from other clients count against the same budget:** "Even invalid requests will count", and "Some custom apps (freshplugs) consume API calls which also count towards the rate limit." [DOCS #rate_limit] Our process is not the only consumer.

### Credit cost of `include`

- Each embedded resource costs **+1 credit on a show (single object) call and +2 credits on a list call**. For example, `GET /tickets?include=stats` costs 3 credits in total. [DOCS #embedding, #view_all_ticket, #view_all_assets] The Rate-limit section words it as "1 credit if performed from a SHOW action, and 3 credits if performed from a LIST action", which means the same thing.
- `X-Ratelimit-Used-CurrentRequest` reports what the current call cost. [DOCS #rate_limit]

### Headers and 429 [DOCS #rate_limit, Errors]

Every response carries:

```
X-Ratelimit-Total: <limit>
X-Ratelimit-Remaining: <left in current window>
X-Ratelimit-Used-CurrentRequest: <credits this call consumed>
X-Freshservice-Api-Version: latest=v2; requested=v2
```

When the limit is exceeded, the API returns `HTTP 429` (text "Rate Limit Exceeded") with `Retry-After: <seconds>`. Freshservice sends `Retry-After` "only when the rate limit has been reached".

Best practice, from Freshservice: queue calls on the client side, retry after the `Retry-After` time, cache data that rarely changes (such as the agent name to ID mapping), and reuse HTTP connections. [DOCS #best_practices]

**Implications for the client:**

1. On a 429, honour `Retry-After` (integer seconds), then retry. This is the ninjaone-mcp pattern. It is safe for GETs. Retrying writes needs care: a 429 means the request was rejected, so a retry should be safe, but this is our inference, not a documented guarantee.
2. A local `x/time/rate` limiter is optional. The real limit varies by plan and is shared with other consumers. A fixed local rate either wastes budget or still collides. If we add one, make it configurable with a conservative default (for example 100/min for Starter, or about 40/min when the call is a ticket, asset, agent or requester list), and let the 429 path handle the rest. `X-Ratelimit-Remaining` could drive adaptive throttling, but YAGNI for now.
3. Avoid `include` on list calls unless the tool needs it, because it triples the cost.

## 4. Pagination

- List endpoints page with `page` (starting at 1) and `per_page`. **`per_page` defaults to 30 and the maximum is 100. "Invalid values and values greater than 100 will result in an error."** [DOCS #pagination]
- **Link header:** when another page exists, the response carries `link: <https://domain.freshservice.com/api/v2/tickets?filter=all_tickets&page=2>;rel="next"`. "If you have reached the last page of objects, then the link header will not be set." [DOCS #pagination] **The client can loop until `rel="next"` is missing.**
- Deep paging: "avoid making calls referencing page numbers over 500 ... extremely long response times." [DOCS #best_practices]
- Some endpoints paginate differently:
  - Ticket activities use `next_page_url` / `start_token`. [DOCS Get Ticket Activity]
  - Custom object records use `page_size` and `next_page_link`. [DOCS List all records of a Custom Object]
- `GET /api/v2/tickets` returns only tickets **created in the last 30 days** by default. To see older tickets, pass `updated_since=<ISO8601>`. [DOCS #view_all_ticket] This is easy to miss and will look like missing data.
- Workspaces: when `workspace_id` is omitted, the list contains only the **primary workspace**. `workspace_id=0` returns all workspaces, but only global fields (tickets and assets). [DOCS #view_all_ticket, #view_all_assets]

## 5. Filter query language

### Endpoints

| Resource | Endpoint | Page size | Page cap | Total count |
|---|---|---|---|---|
| Tickets | `GET /api/v2/tickets/filter?query="..."` | 30 (fixed) | not documented | `"total"` in body |
| Assets | `GET /api/v2/assets?filter="..."` (note: `filter=`, not `query=`) | 30 | **40 pages** | "in the headers" |
| Asset search | `GET /api/v2/assets?search="name:'dell'"` | 30 | — | in headers |
| Requesters | `GET /api/v2/requesters?query="..."` | 30 | **page ≤ 40** | — |
| Agents | `GET /api/v2/agents?query="..."` | 30 | **page ≤ 40** | — |
| Changes | `GET /api/v2/changes?query="..."` | default 30 | — | in body |
| Departments | `GET /api/v2/departments?query="name:'Sales'"` | — | — | — |
| Workspaces (MSP clients) | `GET /api/v2/workspaces?query="..."` | — | — | — |

Sources: DOCS #filter_tickets, #filter_assets, Search Assets, #filter_requesters, #filter_agents, Changes "Query", #filter_departments, Filter Clients.

**Effective cap:** asset, requester and agent filters stop at 40 pages × 30 = **1,200 results**. For bigger sets, use the plain list endpoint (per_page 100, Link header) instead of a filter.

### Syntax [DOCS #filter_tickets, #filter_requesters, #filter_assets]

- The whole query is wrapped in **double quotes**, at most **512 characters**, and **URL-encoded**. Example: `query="priority:3"` is sent as `query=%22priority%3A3%22`.
- Field names are snake_case, are **case-sensitive**, and come from the resource's fields endpoint or its documented field list. Custom fields can be used too.
- Operators:
  - `:` means equals.
  - `:>` means **greater than or equal** and `:<` means **less than or equal**. They work only on number and date fields.
  - `AND` and `OR` combine conditions, and parentheses group them.
  - Changes also support `!:` for not-equal.
  - Requesters and agents support a starts-with search: `~[first_name|last_name]:'abc'`.
- Values:
  - Strings go in single quotes, such as `'IN STOCK'`. Escape an apostrophe as `\'`.
  - Numbers go bare.
  - Dates are written `'yyyy-mm-dd'` (UTC) and allow only `:>` and `:<`.
  - Use the `null` keyword to match "unassigned" (agent, group, or any field on requesters and assets).
- Filtered results **cannot be sorted**. They are always sorted by `created_at` descending. [DOCS #filter_requesters, #filter_assets, #filter_agents]
- Search indexes lag: changes "may take a few minutes to get indexed". [DOCS #filter_assets, #filter_requesters, #filter_agents] So a filter can miss a record we just wrote.
- Ticket filter and asset filter both default to the primary workspace. Use `workspace_id` in the query or parameters to change that. [DOCS #filter_tickets, #filter_assets]
- Custom objects use a **different** grammar: conditions are joined only with `AND`, `<` and `>` are allowed, `:` can take `[1,2,4]` lists, and the query is URL-encoded **without** surrounding quotes. [DOCS List all records of a Custom Object]

**Implication:** one shared query builder can serve tickets, assets, requesters, agents and changes (quote the whole query, escape `'` in strings, URL-encode). Custom objects need their own builder. Validate the 512-character limit before sending.

## 6. `include` (embedding)

- Use `?include=<name>`. Values are endpoint-specific:
  - Tickets: `requester`, `stats`, `tags`, `requested_for`, `department`, `created_by`, `onboarding_context`, `offboarding_context`, `skill`, `skillset`, `journey_requests`, `journey_data`.
  - Assets: `type_fields`.
  - Purchase orders: `purchase_items`, `custom_fields`.
  [DOCS #embedding, #view_all_ticket, #view_all_assets, Changelog]
- Cost: +1 credit on a show call and +2 on a list call (see §3).
- v2 dropped many v1 default fields, such as the requester name on a ticket. Use `include` to get them back. [DOCS #embedding]

## 7. Errors

### Status codes [DOCS #error]

| Status | Meaning | Suggested MCP mapping |
|---|---|---|
| 400 | Client or validation error. Also returned for deprecated attributes such as `ticket_scope`. | tool error: show `errors[]` |
| 401 | Authorization header missing or wrong | config error: bad API key |
| 403 | Agent lacks permission, a feature is disabled, login attempts are locked out, or the agent limit is reached | permission error |
| 404 | Invalid ID, invalid domain, or invalid URL | not found. A 404 on every call suggests a wrong domain. |
| 405 | Wrong HTTP verb | bug |
| 406 | Unsupported Accept header. Only `application/json` and `*/*` are accepted. | bug |
| 409 | Inconsistent or conflicting state | tool error |
| 415 | Unsupported Content-Type. Only `application/json`, or multipart for uploads. | bug |
| 429 | Rate limit exceeded, with `Retry-After` | retry |
| 500 | Server error | retry with backoff (our choice). Not documented as transient. |

### Body format

"Most errors" return JSON with this shape [DOCS #error, sample bodies on the Requesters and Agents pages]:

```json
{ "description": "Validation failed",
  "errors": [ { "field": "department_id",
                "message": "[\"It should be a/an Positive Integer\"]",
                "code": "invalid_value" } ] }
```

- `field` appears only on 400 errors. `message` is free text. `code` is machine-readable.
- Codes: `missing_field`, `invalid_value`, `duplicate_value`, `datatype_mismatch`, `invalid_field`, `invalid_json`, `invalid_credentials`, `access_denied`, `require_feature`, `account_suspended`, `ssl_required`, `readonly_field`, `password_lockout`, `password_expired`, `no_content_required`, `inaccessible_field`, `incompatible_field`, `unsupported_authentication_type`, `access_token_expired`, `access_token_invalid`. [DOCS #error]
- **Implication:** decode `{description, errors[]{field,message,code}}` in a lenient way (both parts are optional). Fall back to the raw body when it is not JSON.

## 8. Other schema facts worth knowing [DOCS Schema]

- Blank fields are returned as `null`, not omitted.
- Timestamps are UTC, in the form `YYYY-MM-DDTHH:MM:SSZ`. Date inputs can be any ISO 8601 form, and a value without a zone is treated as UTC.
- `POST` returns `201` with a `Location` header pointing to the new resource.
- The API is JSON only. The exception is attachments, which need `multipart/form-data`.

## Open items (not documented, verify against a live tenant)

- Whether the ticket filter has a page cap. Freshdesk caps at 10 pages, but the Freshservice docs are silent.
- Whether `per_page` is honoured on filter endpoints. The docs say "30" with no override.
- The exact `X-Ratelimit-Total` value and window on our account (per-minute or legacy hourly).
- Whether `Retry-After` is also sent on sub-limit 429s. Presumably yes.
