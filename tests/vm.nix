# VM test: the real binary, a stub Freshservice, and the assertions only a
# booted machine can make: the unit comes up hardened, the API key is
# delivered by systemd and readable by nothing else, it appears in neither
# argv nor the environment, and it still reaches Freshservice as Basic auth.
# The stub also pages tickets with a Link header, answers the ticket filter
# with a total, and rate limits once, so the client's paging, counting and
# 429 retry run against a real network stack.
#
# Not covered: real Freshservice payload shapes, plan rate limits, or what a
# least-privilege agent may see. Those need a live tenant.
{ pkgs, self }:

let
  stubPort = 9443;
  mcpPort = 8234;

  apiKey = "s3cr3t-api-key";
  bearerToken = "bearer-abc";

  stub = pkgs.writers.writePython3Bin "stub-freshservice" { } ''
    import base64
    import http.server
    import json
    import urllib.parse

    KEY = ${builtins.toJSON apiKey}
    TICKETS = [{"id": i, "subject": "t%d" % i, "status": 2,
                "custom_fields": {}} for i in range(1, 151)]
    throttled = []


    def record(path, value):
        with open(path, "a") as fh:
            fh.write(value + "\n")


    class Handler(http.server.BaseHTTPRequestHandler):
        def reply(self, status, payload, headers=()):
            body = json.dumps(payload).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.send_header("X-Ratelimit-Total", "200")
            self.send_header("X-Ratelimit-Remaining", "190")
            for k, v in headers:
                self.send_header(k, v)
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):
            auth = self.headers.get("Authorization", "")
            key = ""
            if auth.startswith("Basic "):
                key = base64.b64decode(auth[6:]).decode().partition(":")[0]
            record("/tmp/seen-keys", key)
            if key != KEY:
                return self.reply(401, {"description": "Authentication failure"})
            url = urllib.parse.urlparse(self.path)
            q = urllib.parse.parse_qs(url.query)
            if url.path == "/api/v2/workspaces":
                return self.reply(200, {"workspaces": [
                    {"id": 2, "name": "IT", "primary": True}]})
            if url.path == "/api/v2/assets":
                return self.reply(200, {"assets": []})
            if url.path == "/api/v2/tickets/filter":
                return self.reply(200, {"tickets": [], "total": 42})
            if url.path == "/api/v2/tickets":
                if not throttled:
                    throttled.append(1)
                    return self.reply(429, {"description": "slow down"},
                                      [("Retry-After", "1")])
                page = int(q.get("page", ["1"])[0])
                per = int(q.get("per_page", ["30"])[0])
                chunk = TICKETS[(page - 1) * per:page * per]
                hdrs = []
                if page * per < len(TICKETS):
                    nxt = "<http://x/api/v2/tickets?page=%d>; rel=\"next\""
                    hdrs.append(("Link", nxt % (page + 1)))
                return self.reply(200, {"tickets": chunk}, hdrs)
            self.reply(404, {"description": "not found"})

        def log_message(self, *args):
            pass


    addr = ("127.0.0.1", ${toString stubPort})
    http.server.HTTPServer(addr, Handler).serve_forever()
  '';

  # initialize, tools/list, then status, a two-page ticket list and a count.
  # Streamable HTTP may answer as JSON or as an SSE frame; accept both.
  probe = pkgs.writers.writePython3Bin "mcp-probe" { } ''
    import json
    import urllib.request

    URL = "http://127.0.0.1:${toString mcpPort}/mcp"
    BEARER = ${builtins.toJSON bearerToken}


    def post(payload, session=None):
        req = urllib.request.Request(URL, data=json.dumps(payload).encode(),
                                     method="POST")
        req.add_header("Content-Type", "application/json")
        req.add_header("Accept", "application/json, text/event-stream")
        req.add_header("Authorization", "Bearer " + BEARER)
        if session:
            req.add_header("Mcp-Session-Id", session)
        resp = urllib.request.urlopen(req, timeout=20)
        raw = resp.read().decode()
        for line in raw.splitlines():
            if line.startswith("data:"):
                raw = line[5:].strip()
                break
        parsed = json.loads(raw) if raw.strip() else {}
        return resp.headers.get("Mcp-Session-Id"), parsed


    def tool(session, n, name, args):
        _, out = post({"jsonrpc": "2.0", "id": n, "method": "tools/call",
                       "params": {"name": name, "arguments": args}}, session)
        res = out["result"]
        assert not res.get("isError"), res
        return res["structuredContent"]


    session, init = post({
        "jsonrpc": "2.0", "id": 1, "method": "initialize",
        "params": {"protocolVersion": "2025-06-18", "capabilities": {},
                   "clientInfo": {"name": "vm-test", "version": "0"}},
    })
    name = init.get("result", {}).get("serverInfo", {}).get("name")
    assert name == "freshservice-mcp", f"unexpected serverInfo: {init}"
    post({"jsonrpc": "2.0", "method": "notifications/initialized"}, session)

    _, listed = post({"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
                     session)
    tools = listed.get("result", {}).get("tools", [])
    assert len(tools) >= 20, f"expected the full tool surface, got {tools}"
    # No capability is enabled: every tool must be read-only.
    writable = [t["name"] for t in tools
                if not t.get("annotations", {}).get("readOnlyHint")]
    assert not writable, f"writable tools with no capability: {writable}"

    status = tool(session, 3, "freshservice_status", {})
    assert status["connected"] and status["asset_api"] == "classic", status

    listing = tool(session, 4, "freshservice_ticket",
                   {"action": "list", "limit": 150})
    ids = [t["id"] for t in listing["results"]]
    assert ids == list(range(1, 151)), f"paging broke: {ids[:3]}..{ids[-3:]}"

    count = tool(session, 5, "freshservice_ticket",
                 {"action": "count", "query": "status:2"})
    assert count["count"] == 42, count
    print(f"ok: {len(tools)} tools, {len(ids)} tickets over two pages")
  '';
in
pkgs.testers.runNixOSTest {
  name = "freshservice-mcp-vm";

  nodes.machine =
    { ... }:
    {
      imports = [ self.nixosModules.freshservice-mcp ];

      environment.systemPackages = [
        pkgs.curl
        probe
      ];

      systemd.services.stub-freshservice = {
        description = "Stub Freshservice";
        wantedBy = [ "multi-user.target" ];
        before = [ "freshservice-mcp.service" ];
        serviceConfig = {
          ExecStart = pkgs.lib.getExe stub;
          Restart = "on-failure";
        };
      };

      # Root-only 0400 files, the shape sops-nix and agenix produce.
      systemd.tmpfiles.settings."10-freshservice-mcp" = {
        "/run/freshservice-api-key".f = {
          user = "root";
          group = "root";
          mode = "0400";
          argument = apiKey;
        };
        "/run/freshservice-mcp-bearer".f = {
          user = "root";
          group = "root";
          mode = "0400";
          argument = bearerToken;
        };
      };

      services.freshservice-mcp = {
        enable = true;
        domain = "http://127.0.0.1:${toString stubPort}";
        apiKeyFile = "/run/freshservice-api-key";
        bearerTokenFile = "/run/freshservice-mcp-bearer";
        port = mcpPort;
      };
    };

  testScript = ''
    import re

    machine.wait_for_unit("stub-freshservice.service")
    machine.wait_for_unit("freshservice-mcp.service")
    machine.wait_for_open_port(${toString mcpPort})

    with subtest("health is reachable without the bearer token"):
        out = machine.succeed("curl -fsS http://127.0.0.1:${toString mcpPort}/healthz")
        assert '"status":"ok"' in out, out

    with subtest("the MCP endpoint refuses unauthenticated callers"):
        for hdr in ("", "-H 'Authorization: Bearer wrong'"):
            machine.succeed(
                f"test \"$(curl -s -o /dev/null -w %{{http_code}} -X POST {hdr} "
                "-H 'Content-Type: application/json' -d '{}' "
                "http://127.0.0.1:${toString mcpPort}/mcp)\" = 401"
            )

    with subtest("a full MCP session works: read-only, paged, counted, retried"):
        print(machine.succeed("mcp-probe"))

    with subtest("the API key reached Freshservice as Basic auth"):
        # LoadCredential -> $CREDENTIALS_DIRECTORY -> Authorization header.
        machine.succeed("grep -qxF ${apiKey} /tmp/seen-keys")
        machine.fail("grep -vxF ${apiKey} /tmp/seen-keys")

    with subtest("the credentials are not readable by anything else"):
        creds = "/run/credentials/freshservice-mcp.service"
        for path in (creds, f"{creds}/api-key", f"{creds}/http-auth-token"):
            mode = machine.succeed(f"stat -L -c %a {path}").strip()
            assert mode[-1] == "0", f"{path} is mode {mode}: readable by other"
        machine.fail(f"runuser -u nobody -- cat {creds}/api-key")
        machine.fail(f"runuser -u nobody -- cat {creds}/http-auth-token")
        machine.succeed("test \"$(stat -c %a /run/freshservice-api-key)\" = 400")

    with subtest("the secrets are in neither argv nor the environment"):
        pid = machine.succeed("systemctl show -p MainPID --value freshservice-mcp.service").strip()
        for f in ("cmdline", "environ"):
            machine.fail(f"tr '\\0' '\\n' < /proc/{pid}/{f} | grep -qF ${apiKey}")
            machine.fail(f"tr '\\0' '\\n' < /proc/{pid}/{f} | grep -qF ${bearerToken}")
        machine.fail(f"tr '\\0' '\\n' < /proc/{pid}/environ | grep -q FRESHSERVICE_API_KEY")

    with subtest("the unit is actually hardened"):
        out = machine.succeed("systemd-analyze security freshservice-mcp.service --no-pager | tail -1")
        print(out)
        found = re.search(r"level for \S+: ([0-9.]+)", out)
        assert found is not None, f"could not read an exposure score from: {out}"
        assert float(found.group(1)) < 3.0, f"unit exposure score regressed: {out}"
        machine.fail(f"nsenter --mount --target {pid} -- test -w /etc")

    with subtest("it survives a restart"):
        machine.succeed("systemctl restart freshservice-mcp.service")
        machine.wait_for_open_port(${toString mcpPort})
        machine.succeed("curl -fsS http://127.0.0.1:${toString mcpPort}/healthz")
  '';
}
