package tools

// Tool is one first-class MCP tool: a set of actions over Freshservice
// endpoints. Adding an endpoint is adding a View; there is no per-endpoint
// handler code.
type Tool struct {
	Name        string // freshservice_<resource>
	Group       string
	Title       string
	Description string // what it is; per-action help is appended
	Views       []View
}

// View is one action of a tool.
type View struct {
	Action string
	Help   string // one line: what this action does and its useful params
	Method string // default GET
	// Path may hold {placeholders}: {id} comes from the id input, any other
	// from params (removed from params once used).
	Path string

	// Reads.
	List     bool              // pages through a list; otherwise one object
	Filter   string            // query param carrying the Freshservice query ("query" or "filter"); makes query required
	Brief    []string          // fields a list keeps (plus custom_fields) unless fields is given; nil keeps all
	Defaults map[string]string // params sent unless the caller sets them
	WS       WSMode
	AllNote  bool   // workspace "all" returns only global fields (Freshservice drops workspace custom fields)
	Link     string // agent-portal path for a get, e.g. "/a/tickets/{id}"
}

// WSMode is how a view takes a workspace.
type WSMode int

const (
	WSNone   WSMode = iota // account-level: no workspace
	WSAll                  // workspace_id; "all" sends 0
	WSGlobal               // workspace_id; "all" sends 1 (global)
	WSOne                  // workspace_id; "all" is not possible
)

func (v View) method() string {
	if v.Method == "" {
		return "GET"
	}
	return v.Method
}

// Tools is the whole first-class tool table, in registration order.
func Tools() []Tool {
	var all []Tool
	for _, t := range [][]Tool{coreTools} {
		all = append(all, t...)
	}
	return all
}
