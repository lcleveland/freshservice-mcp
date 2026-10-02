{
  config,
  lib,
  pkgs,
  ...
}:
let
  inherit (lib)
    mkIf
    mkOption
    mkEnableOption
    types
    optional
    literalExpression
    ;
  cfg = config.services.freshservice-mcp;
  groups = [
    "core"
    "itil"
    "assets"
    "knowledge"
    "catalog"
    "projects"
    "ops"
    "custom"
  ];
  # option name -> --allow-<capability>; see docs/adr/0002.
  capabilities = {
    allowTickets = "tickets";
    allowTicketReplies = "ticket-replies";
    allowItil = "itil";
    allowAssets = "assets";
    allowKnowledge = "knowledge";
    allowProjects = "projects";
    allowPeople = "people";
    allowApprovals = "approvals";
    allowOps = "ops";
    allowCustomObjects = "custom-objects";
  };
  capabilityHelp = {
    allowTickets = "create and update tickets and service requests, notes, tasks, time entries, approval requests, catalog orders and restores";
    allowTicketReplies = "replies and email or Zoom collaboration: anything that emails a requester or a third party";
    allowItil = "problems, changes, releases, CABs, major incidents and post-incident report templates";
    allowAssets = "assets (classic and ITAM), relationships, asset types, software, contracts, vendors, products and purchase orders";
    allowKnowledge = "solution categories, folders and articles, and announcements";
    allowProjects = "projects and project tasks, new-gen and legacy";
    allowPeople = "requesters, requester groups, departments, locations, and onboarding and offboarding requests";
    allowApprovals = "approve or reject contracts as the API key's agent, and approval delegation";
    allowOps = "status page publishing, on-call schedules, alerts and journeys";
    allowCustomObjects = "custom object record create and update";
  };
  isLoopback = a: a == "::1" || a == "localhost" || lib.hasPrefix "127." a;
  inStore = p: p != null && lib.hasPrefix builtins.storeDir p;
  staticUser = cfg.user != null;

  args = [
    "--http"
    "--addr"
    (
      if lib.hasInfix ":" cfg.listenAddress then
        "[${cfg.listenAddress}]:${toString cfg.port}"
      else
        "${cfg.listenAddress}:${toString cfg.port}"
    )
    "--path"
    cfg.path
    "--domain"
    cfg.domain
    "--tool-groups"
    (lib.concatStringsSep "," cfg.toolGroups)
    "--max-records"
    (toString cfg.maxRecords)
    "--max-buckets"
    (toString cfg.maxBuckets)
    "--request-timeout"
    cfg.requestTimeout
    "--log-level"
    cfg.logLevel
  ]
  ++ lib.optionals (cfg.defaultWorkspace != null) [
    "--default-workspace"
    cfg.defaultWorkspace
  ]
  ++ lib.concatMap (o: optional cfg.${o} "--allow-${capabilities.${o}}") (lib.attrNames capabilities)
  ++ cfg.extraArgs;
in
{
  options.services.freshservice-mcp = {
    enable = mkEnableOption "the Freshservice MCP server (streamable HTTP)";

    package = mkOption {
      type = types.package;
      default = pkgs.callPackage ../pkgs/freshservice-mcp.nix { };
      defaultText = literalExpression "pkgs.freshservice-mcp";
      description = "The freshservice-mcp package.";
    };

    domain = mkOption {
      type = types.str;
      example = "acme.freshservice.com";
      description = "Freshservice domain: `acme`, `acme.freshservice.com` or a full https URL. Custom CNAMEs do not work with the API.";
    };
    apiKeyFile = mkOption {
      type = types.nullOr types.str;
      default = null;
      example = "/run/secrets/freshservice-api-key";
      description = ''
        Runtime path to a file holding the API key of a dedicated, least-privilege
        Freshservice agent. Passed via systemd `LoadCredential`, so it never enters
        the Nix store, argv or the environment. Use sops-nix, agenix or a
        root-owned 0400 file.
      '';
    };
    defaultWorkspace = mkOption {
      type = types.nullOr types.str;
      default = null;
      example = "IT";
      description = "Workspace id or name used when a call names none. Null means the primary workspace.";
    };

    maxRecords = mkOption {
      type = types.ints.positive;
      default = 2000;
      description = "Most records one list call may return.";
    };
    maxBuckets = mkOption {
      type = types.ints.positive;
      default = 60;
      description = "Most API calls one summary (group_by, backlog, trend) may fan out to.";
    };
    toolGroups = mkOption {
      type = types.listOf (types.enum groups);
      default = groups;
      description = "Tool groups to register. `core` is always on.";
    };

    listenAddress = mkOption {
      type = types.str;
      default = "127.0.0.1";
      description = "Address to listen on. A non-loopback address requires bearerTokenFile.";
    };
    port = mkOption {
      type = types.port;
      default = 8234;
      description = "TCP port for the MCP endpoint.";
    };
    path = mkOption {
      type = types.str;
      default = "/mcp";
      description = "URL path of the MCP endpoint.";
    };
    bearerTokenFile = mkOption {
      type = types.nullOr types.str;
      default = null;
      example = "/run/secrets/freshservice-mcp-bearer";
      description = ''
        Runtime path to a shared secret HTTP clients must send as
        `Authorization: Bearer <token>`. Required for a non-loopback listener.
        `/healthz` stays open.
      '';
    };
    openFirewall = mkOption {
      type = types.bool;
      default = false;
      description = "Open port in the firewall.";
    };

    requestTimeout = mkOption {
      type = types.str;
      default = "30s";
      description = "Timeout for a single Freshservice request, as a Go duration.";
    };
    logLevel = mkOption {
      type = types.enum [
        "debug"
        "info"
        "warn"
        "error"
      ];
      default = "info";
      description = "Log verbosity. Writes are always audit-logged at info.";
    };

    user = mkOption {
      type = types.nullOr types.str;
      default = null;
      description = "Run as this user instead of a systemd DynamicUser.";
    };
    group = mkOption {
      type = types.nullOr types.str;
      default = cfg.user;
      defaultText = literalExpression "config.services.freshservice-mcp.user";
      description = "Group to run as when user is set.";
    };

    installCli = mkOption {
      type = types.bool;
      default = cfg.enable;
      defaultText = literalExpression "config.services.freshservice-mcp.enable";
      description = ''
        Put the binary on the system PATH. Set this with enable = false on a
        workstation that only spawns the server over stdio from an MCP client.
      '';
    };
    extraArgs = mkOption {
      type = types.listOf types.str;
      default = [ ];
      description = "Extra command-line arguments. Never put a secret here.";
    };
    environment = mkOption {
      type = types.attrsOf types.str;
      default = { };
      description = "Extra environment variables. Never put a secret here.";
    };
  }
  // lib.mapAttrs (
    o: help:
    mkOption {
      type = types.bool;
      default = false;
      description = "Enable the `${capabilities.${o}}` write capability: ${help}.";
    }
  ) capabilityHelp;

  config = lib.mkMerge [
    (mkIf cfg.installCli { environment.systemPackages = [ cfg.package ]; })

    (mkIf cfg.enable {
      assertions = [
        {
          assertion = cfg.apiKeyFile != null && lib.hasPrefix "/" cfg.apiKeyFile;
          message = "services.freshservice-mcp.apiKeyFile must be an absolute runtime path to the agent API key.";
        }
        {
          assertion = !(inStore cfg.apiKeyFile) && !(inStore cfg.bearerTokenFile);
          message = "services.freshservice-mcp: secret files must not live in ${builtins.storeDir}, which is world-readable. Use sops-nix, agenix or a root-owned 0400 file.";
        }
        {
          assertion = isLoopback cfg.listenAddress || cfg.bearerTokenFile != null;
          message = "services.freshservice-mcp.listenAddress is ${cfg.listenAddress} (not loopback) without bearerTokenFile; the server refuses to start unauthenticated on a network address.";
        }
        {
          assertion = cfg.bearerTokenFile == null || lib.hasPrefix "/" cfg.bearerTokenFile;
          message = "services.freshservice-mcp.bearerTokenFile must be an absolute path.";
        }
        {
          assertion = !staticUser || cfg.group != null;
          message = "services.freshservice-mcp.group must be set when user is set.";
        }
        {
          assertion = lib.hasPrefix "/" cfg.path;
          message = "services.freshservice-mcp.path must begin with a slash.";
        }
      ];

      warnings =
        optional cfg.allowTicketReplies "services.freshservice-mcp.allowTicketReplies is true: a model can email requesters and third parties."
        ++ optional cfg.allowApprovals "services.freshservice-mcp.allowApprovals is true: a model can approve and reject contracts as the API key's agent."
        ++ optional cfg.allowOps "services.freshservice-mcp.allowOps is true: a model can publish to public status pages and change on-call schedules."
        ++
          optional (cfg.openFirewall && isLoopback cfg.listenAddress)
            "services.freshservice-mcp.openFirewall has no effect while listenAddress is loopback (${cfg.listenAddress}).";

      users = mkIf staticUser {
        users.${cfg.user} = {
          isSystemUser = true;
          group = cfg.group;
        };
        groups.${cfg.group} = { };
      };

      systemd.services.freshservice-mcp = {
        description = "Freshservice MCP server";
        documentation = [ "https://github.com/lcleveland/freshservice-mcp" ];
        wantedBy = [ "multi-user.target" ];
        after = [ "network-online.target" ];
        wants = [ "network-online.target" ];
        environment = cfg.environment;
        serviceConfig = {
          Type = "exec";
          ExecStart = "${lib.getExe cfg.package} ${lib.escapeShellArgs args}";
          Restart = "on-failure";
          RestartSec = 5;
          LoadCredential = [
            "api-key:${cfg.apiKeyFile}"
          ]
          ++ optional (cfg.bearerTokenFile != null) "http-auth-token:${cfg.bearerTokenFile}";

          User = mkIf staticUser cfg.user;
          Group = mkIf staticUser cfg.group;
          DynamicUser = !staticUser;

          AmbientCapabilities = [ "" ];
          CapabilityBoundingSet = [ "" ];
          DevicePolicy = "closed";
          LockPersonality = true;
          MemoryDenyWriteExecute = true;
          NoNewPrivileges = true;
          PrivateDevices = true;
          PrivateTmp = true;
          PrivateUsers = true;
          ProcSubset = "pid";
          ProtectClock = true;
          ProtectControlGroups = true;
          ProtectHome = true;
          ProtectHostname = true;
          ProtectKernelLogs = true;
          ProtectKernelModules = true;
          ProtectKernelTunables = true;
          ProtectProc = "invisible";
          ProtectSystem = "strict";
          RemoveIPC = true;
          # AF_NETLINK: Go's pure resolver reads interface addresses.
          RestrictAddressFamilies = [
            "AF_INET"
            "AF_INET6"
            "AF_NETLINK"
          ];
          RestrictNamespaces = true;
          RestrictRealtime = true;
          RestrictSUIDSGID = true;
          SystemCallArchitectures = "native";
          SystemCallFilter = [
            "@system-service"
            "~@privileged"
            "~@resources"
          ];
          UMask = "0077";
          SocketBindDeny = "any";
          SocketBindAllow = "tcp:${toString cfg.port}";
        };
      };

      networking.firewall.allowedTCPPorts = mkIf cfg.openFirewall [ cfg.port ];
    })
  ];
}
