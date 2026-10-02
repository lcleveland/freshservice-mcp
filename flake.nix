{
  description = "MCP server for the Freshservice API v2, packaged with a NixOS module";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      inherit (nixpkgs) lib;
      forAllSystems = lib.genAttrs [
        "x86_64-linux"
        "aarch64-linux"
      ];
      pkgsFor = system: nixpkgs.legacyPackages.${system};
    in
    {
      overlays.default = import ./overlay.nix;

      nixosModules = {
        freshservice-mcp =
          { pkgs, ... }:
          {
            imports = [ ./modules/freshservice-mcp.nix ];
            services.freshservice-mcp.package =
              lib.mkDefault
                self.packages.${pkgs.stdenv.hostPlatform.system}.freshservice-mcp;
          };
        default = self.nixosModules.freshservice-mcp;
      };

      packages = forAllSystems (system: rec {
        freshservice-mcp = (pkgsFor system).callPackage ./pkgs/freshservice-mcp.nix { };
        default = freshservice-mcp;
      });

      checks = forAllSystems (
        system:
        {
          # Runs the Go test suite in checkPhase.
          package = self.packages.${system}.freshservice-mcp;
        }
        // import ./tests/eval.nix {
          inherit self lib;
          pkgs = pkgsFor system;
        }
        // {
          # Boots a VM with a stub Freshservice; about a minute with KVM.
          vm = import ./tests/vm.nix {
            inherit self;
            pkgs = pkgsFor system;
          };
        }
      );

      formatter = forAllSystems (system: (pkgsFor system).nixfmt-tree);

      devShells = forAllSystems (
        system:
        let
          pkgs = pkgsFor system;
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              pkgs.gopls
              pkgs.gotools
              pkgs.nixfmt
              pkgs.jq
            ];
            env.CGO_ENABLED = "0";
          };
        }
      );
    };
}
