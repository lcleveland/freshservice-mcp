# Module evaluation checks: no VM, just the generated unit.
{
  pkgs,
  self,
  lib,
}:
let
  evalModule =
    module:
    (lib.nixosSystem {
      inherit (pkgs.stdenv.hostPlatform) system;
      modules = [
        self.nixosModules.freshservice-mcp
        {
          services.freshservice-mcp.package = lib.mkForce (
            pkgs.writeShellScriptBin "freshservice-mcp" "exit 0"
          );
          boot.loader.grub.enable = false;
          fileSystems."/" = {
            device = "none";
            fsType = "tmpfs";
          };
          system.stateVersion = lib.trivial.release;
        }
        module
      ];
    }).config;

  failed = config: map (a: a.message) (builtins.filter (a: !a.assertion) config.assertions);

  base = extra: {
    services.freshservice-mcp = {
      enable = true;
      domain = "acme.freshservice.com";
      apiKeyFile = "/persist/secrets/freshservice-api-key";
    }
    // extra;
  };

  # Evaluates cleanly and the generated unit passes the grep checks.
  unitCheck =
    name: extra: checks:
    let
      config = evalModule (base extra);
      broken = failed config;
      svc = config.systemd.services.freshservice-mcp;
    in
    assert broken == [ ] || throw "${name}: ${lib.concatStringsSep "; " broken}";
    pkgs.runCommand "freshservice-mcp-${name}" { } ''
      cat > cmd <<'EOF'
      ${svc.serviceConfig.ExecStart}
      EOF
      cat > env <<'EOF'
      ${lib.concatStringsSep "\n" (lib.mapAttrsToList (k: v: "${k}=${v}") svc.environment)}
      EOF
      cat > creds <<'EOF'
      ${lib.concatStringsSep "\n" svc.serviceConfig.LoadCredential}
      EOF
      check() { grep -qF -- "$1" "$2" || { echo "missing from $2: $1"; cat "$2"; exit 1; }; }
      refute() { if grep -qF -- "$1" "$2"; then echo "unexpected in $2: $1"; cat "$2"; exit 1; fi; }
      ${checks}
      touch $out
    '';

  # Evaluation must trip an assertion containing `want`.
  mustFail =
    name: extra: want:
    let
      broken = failed (evalModule (base extra));
    in
    assert
      lib.any (lib.hasInfix want) broken
      || throw "${name}: expected assertion '${want}', got: ${toString broken}";
    pkgs.runCommand "freshservice-mcp-${name}" { } "touch $out";

  warns =
    name: extra: want:
    let
      config = evalModule (base extra);
    in
    assert lib.any (lib.hasInfix want) config.warnings || throw "${name}: no warning '${want}'";
    pkgs.runCommand "freshservice-mcp-${name}" { } "touch $out";
in
{
  module-eval = unitCheck "module-eval" { } ''
    check "--http" cmd
    check "--addr 127.0.0.1:8234" cmd
    check "--domain acme.freshservice.com" cmd
    check "--tool-groups core,itil,assets,knowledge,catalog,projects,ops,custom" cmd
    check "--max-records 2000" cmd
    check "--max-buckets 60" cmd
    check "api-key:/persist/secrets/freshservice-api-key" creds
    refute "/persist/secrets/freshservice-api-key" cmd
    refute "freshservice-api-key" env
    refute "--allow-" cmd
    refute "--default-workspace" cmd
  '';

  module-full =
    unitCheck "module-full"
      {
        allowTickets = true;
        allowPeople = true;
        allowCustomObjects = true;
        defaultWorkspace = "IT";
        listenAddress = "0.0.0.0";
        bearerTokenFile = "/run/secrets/bearer";
        toolGroups = [ "itil" ];
        maxRecords = 500;
      }
      ''
        check "--allow-tickets" cmd
        check "--allow-people" cmd
        check "--allow-custom-objects" cmd
        refute "--allow-approvals" cmd
        refute "--allow-ticket-replies" cmd
        check "--default-workspace IT" cmd
        check "--addr 0.0.0.0:8234" cmd
        check "--tool-groups itil" cmd
        check "--max-records 500" cmd
        check "http-auth-token:/run/secrets/bearer" creds
        refute "/run/secrets/bearer" cmd
      '';

  module-replies-warn = warns "replies-warn" {
    allowTicketReplies = true;
  } "allowTicketReplies is true";
  module-approvals-warn = warns "approvals-warn" { allowApprovals = true; } "allowApprovals is true";
  module-ops-warn = warns "ops-warn" { allowOps = true; } "allowOps is true";

  module-no-key = mustFail "no-key" { apiKeyFile = null; } "apiKeyFile";
  module-key-in-store = mustFail "key-in-store" {
    apiKeyFile = "${builtins.storeDir}/abc-key";
  } "world-readable";
  module-open-listener = mustFail "open-listener" {
    listenAddress = "0.0.0.0";
  } "without bearerTokenFile";
}
