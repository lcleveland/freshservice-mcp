# Freshservice MCP

An MCP server that gives Claude direct, gated access to our Freshservice instance through the v2 REST API.

## Language

**Workspace**:
A Freshservice partition (IT, HR, Facilities, and so on) with its own tickets, catalog and agents; some records are global and shared across all workspaces.
_Avoid_: Tenant, instance, department

**Agent**:
A Freshservice staff account that works tickets; the API key belongs to one and inherits its role's visibility.
_Avoid_: Technician, user, tech

**Requester**:
A person who raises tickets or service requests and is not acting as an agent.
_Avoid_: User, customer, end user

**Capability**:
An operator-enabled class of writes (for example, ticket updates) that unlocks a set of write actions; reads never need one.
_Avoid_: Permission, scope, verb flag
