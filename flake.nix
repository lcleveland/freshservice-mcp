{
  description = "MCP server for the Freshservice API v2";

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

      packages = forAllSystems (system: rec {
        freshservice-mcp = (pkgsFor system).callPackage ./pkgs/freshservice-mcp.nix { };
        default = freshservice-mcp;
      });

      checks = forAllSystems (system: {
        # Runs the Go test suite in checkPhase.
        package = self.packages.${system}.freshservice-mcp;
      });

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
