package tools

var ticketSummary = summaries(View{Path: "/api/v2/tickets/filter", Filter: "query", Total: true, AllQuery: "created_at:>'2000-01-01'",
	FieldsPath: "/api/v2/ticket_form_fields", WS: WSAll})

var ticketBrief = []string{"id", "subject", "type", "status", "priority", "source", "requester_id", "responder_id", "group_id",
	"department_id", "category", "workspace_id", "created_at", "updated_at", "due_by"}

var requesterBrief = []string{"id", "first_name", "last_name", "primary_email", "job_title", "department_ids", "location_id", "reporting_manager_id", "active"}

var agentBrief = []string{"id", "first_name", "last_name", "email", "job_title", "department_ids", "location_id", "active", "occasional", "member_of", "workspace_ids"}

// Core tools: tickets and the people and lookups every ticket refers to.
var coreTools = []Tool{
	{Name: "freshservice_workspace", Group: "core", Title: "Workspaces",
		Description: "Freshservice workspaces (IT, HR, Facilities, ...). Every scoped tool takes workspace as an id, a name or \"all\"; without it the server uses its default workspace (see freshservice_status).",
		Views: []View{
			{Action: "list", Help: "workspaces the API key's agent can see, with primary and restricted flags.", Path: "/api/v2/workspaces", List: true},
			{Action: "get", Help: "workspace id.", Path: "/api/v2/workspaces/{id}"},
			{Action: "fields", Help: "workspace (client) form fields; MSP accounts only.", Path: "/api/v2/workspace_form_fields"},
		}},
	{Name: "freshservice_ticket", Group: "core", Title: "Tickets and service requests",
		Description: "Freshservice tickets: incidents and service requests, with their conversations, tasks, time entries, approvals and CSAT. " +
			"status: 2 Open, 3 Pending, 4 Resolved, 5 Closed. priority: 1 Low, 2 Medium, 3 High, 4 Urgent. source: 1 Email, 2 Portal, 3 Phone, 4 Chat, 7 Walk-up, 9 Workflow. " +
			"Sub-record actions take the ticket as id and the sub-record id in params.",
		Views: []View{
			{Action: "list", Help: "tickets, newest first; params updated_since (RFC 3339; default 2000-01-01 so older tickets are included), order_type asc|desc, include (stats, requester).",
				Path: "/api/v2/tickets", List: true, Brief: ticketBrief, WS: WSAll, AllNote: true, Defaults: map[string]string{"updated_since": "2000-01-01T00:00:00Z"}},
			{Action: "filter", Help: "tickets matching query (fields: status, priority, group_id, agent_id, requester_id, type, source, tag, created_at, updated_at, due_by, fr_due_by, custom fields); returns total. Always newest first.",
				Path: "/api/v2/tickets/filter", List: true, Filter: "query", Brief: ticketBrief, WS: WSAll, AllNote: true},
			{Action: "get", Help: "ticket id in full; params include (conversations, requester, requested_for, stats, problem, assets, change, related_tickets).",
				Path: "/api/v2/tickets/{id}", Link: "/a/tickets/{id}"},
			{Action: "fields", Help: "ticket form fields, with the choices for status, priority, type and custom fields.", Path: "/api/v2/ticket_form_fields", WS: WSOne},
			{Action: "activities", Help: "activity log of ticket id.", Path: "/api/v2/tickets/{id}/activities"},
			{Action: "conversations", Help: "replies and notes of ticket id.", Path: "/api/v2/tickets/{id}/conversations", List: true},
			{Action: "tasks", Help: "tasks of ticket id.", Path: "/api/v2/tickets/{id}/tasks", List: true},
			{Action: "task", Help: "task params.task_id of ticket id.", Path: "/api/v2/tickets/{id}/tasks/{task_id}"},
			{Action: "time_entries", Help: "time entries of ticket id.", Path: "/api/v2/tickets/{id}/time_entries", List: true},
			{Action: "time_entry", Help: "time entry params.time_entry_id of ticket id.", Path: "/api/v2/tickets/{id}/time_entries/{time_entry_id}"},
			{Action: "approvals", Help: "approvals of ticket id.", Path: "/api/v2/tickets/{id}/approvals", List: true},
			{Action: "approval", Help: "approval params.approval_id of ticket id.", Path: "/api/v2/tickets/{id}/approvals/{approval_id}"},
			{Action: "approval_groups", Help: "approval groups of ticket id.", Path: "/api/v2/tickets/{id}/approval-groups"},
			{Action: "requested_items", Help: "catalog items requested by service request id.", Path: "/api/v2/tickets/{id}/requested_items", List: true},
			{Action: "csat", Help: "CSAT survey response of ticket id.", Path: "/api/v2/tickets/{id}/csat_response"},
			ticketSummary[0], ticketSummary[1],
			{Action: "emails", Help: "email collaboration threads of ticket id.", Path: "/api/v2/tickets/{id}/communications", List: true},
			{Action: "email", Help: "email params.email_id of ticket id.", Path: "/api/v2/tickets/{id}/communications/{email_id}"},
			{Action: "backlog", Help: "open tickets (every status but Resolved and Closed) by status x by (group_id, the default, or priority), plus how many are overdue. One API call per cell.",
				Summary: "backlog", Path: "/api/v2/tickets/filter", Filter: "query", Total: true, FieldsPath: "/api/v2/ticket_form_fields", WS: WSAll},
			{Action: "trend", Help: "tickets created and resolved per day or week from from to to (YYYY-MM-DD), optionally within query. Two API calls per period.",
				Summary: "trend", Path: "/api/v2/tickets/filter", Filter: "query", Total: true, WS: WSAll},
			{Action: "all_approvals", Help: "approvals across tickets, changes and releases; params parent (ticket|change|release), status (requested|approved|rejected|cancelled).",
				Path: "/api/v2/approvals", List: true},
		}},
	{Name: "freshservice_requester", Group: "core", Title: "Requesters and requester groups",
		Description: "People who raise tickets, and requester groups. Account-level: requesters are shared by every workspace.",
		Views: append([]View{
			{Action: "list", Help: "requesters; params email, mobile_phone_number, include_agents.", Path: "/api/v2/requesters", List: true, Brief: requesterBrief},
			{Action: "filter", Help: "requesters matching query (fields: first_name, last_name, job_title, primary_email, department_id, location_id, created_at, updated_at, custom fields).",
				Path: "/api/v2/requesters", List: true, Filter: "query", Brief: requesterBrief},
			{Action: "get", Help: "requester id in full.", Path: "/api/v2/requesters/{id}"},
			{Action: "fields", Help: "requester form fields.", Path: "/api/v2/requester_fields"},
			{Action: "assignment_history", Help: "assets assigned to user id over time.", Path: "/api/v2/users/{id}/assignment-history"},
			{Action: "groups", Help: "requester groups.", Path: "/api/v2/requester_groups", List: true},
			{Action: "group", Help: "requester group id.", Path: "/api/v2/requester_groups/{id}"},
			{Action: "group_members", Help: "members of requester group id.", Path: "/api/v2/requester_groups/{id}/members", List: true, Brief: requesterBrief},
		}, summaries(View{Path: "/api/v2/requesters", Filter: "query", ScanPath: "/api/v2/requesters"})...)},
	{Name: "freshservice_agent", Group: "core", Title: "Agents, agent groups and roles",
		Description: "Staff who work tickets, the groups tickets are assigned to, and agent roles. Agents are account-level and carry workspace_ids.",
		Views: append([]View{
			{Action: "list", Help: "agents; params email, active, state (fulltime|occasional).", Path: "/api/v2/agents", List: true, Brief: agentBrief},
			{Action: "filter", Help: "agents matching query (fields: first_name, last_name, job_title, email, department_id, location_id, created_at, updated_at, custom fields).",
				Path: "/api/v2/agents", List: true, Filter: "query", Brief: agentBrief},
			{Action: "get", Help: "agent id in full.", Path: "/api/v2/agents/{id}"},
			{Action: "fields", Help: "agent form fields.", Path: "/api/v2/agent_fields"},
			{Action: "assignment_history", Help: "assets assigned to user id over time.", Path: "/api/v2/users/{id}/assignment-history"},
			{Action: "groups", Help: "agent groups of one workspace (Freshservice cannot list them across workspaces).", Path: "/api/v2/groups", List: true, WS: WSOne},
			{Action: "group", Help: "agent group id.", Path: "/api/v2/groups/{id}"},
			{Action: "roles", Help: "agent roles.", Path: "/api/v2/roles", List: true},
			{Action: "role", Help: "agent role id.", Path: "/api/v2/roles/{id}"},
			{Action: "delegation", Help: "approval delegation set by user id.", Path: "/api/v2/users/{id}/delegation"},
		}, summaries(View{Path: "/api/v2/agents", Filter: "query", ScanPath: "/api/v2/agents"})...)},
	{Name: "freshservice_lookup", Group: "core", Title: "Departments, locations, SLAs and canned responses",
		Description: "Reference data tickets point at: departments, locations, business hours, SLA policies and canned responses.",
		Views: []View{
			{Action: "departments", Help: "departments.", Path: "/api/v2/departments", List: true},
			{Action: "departments_filter", Help: "departments matching query (fields: name, created_at, updated_at, custom fields).", Path: "/api/v2/departments", List: true, Filter: "query"},
			{Action: "department", Help: "department id.", Path: "/api/v2/departments/{id}"},
			{Action: "department_fields", Help: "department form fields.", Path: "/api/v2/department_fields"},
			{Action: "locations", Help: "locations.", Path: "/api/v2/locations", List: true},
			{Action: "locations_filter", Help: "locations matching query (fields: name, parent_location_id, primary_contact_id, created_at, updated_at).", Path: "/api/v2/locations", List: true, Filter: "query"},
			{Action: "location", Help: "location id.", Path: "/api/v2/locations/{id}"},
			{Action: "business_hours", Help: "business hours configurations (workspace \"all\" means the global ones).", Path: "/api/v2/business_hours", List: true, WS: WSGlobal},
			{Action: "business_hour", Help: "business hours configuration id.", Path: "/api/v2/business_hours/{id}"},
			{Action: "sla_policies", Help: "SLA policies of a workspace plus the global ones (workspace \"all\" means global only).", Path: "/api/v2/sla_policies", List: true, WS: WSGlobal},
			{Action: "canned_responses", Help: "canned responses (workspace \"all\" means the global ones).", Path: "/api/v2/canned_responses", List: true, WS: WSGlobal},
			{Action: "canned_response", Help: "canned response id.", Path: "/api/v2/canned_responses/{id}"},
			{Action: "canned_response_folders", Help: "canned response folders.", Path: "/api/v2/canned_response_folders", List: true, WS: WSGlobal},
			{Action: "canned_response_folder", Help: "canned response folder id.", Path: "/api/v2/canned_response_folders/{id}"},
			{Action: "folder_responses", Help: "canned responses in folder id.", Path: "/api/v2/canned_response_folders/{id}/canned_responses", List: true},
		}},
}
