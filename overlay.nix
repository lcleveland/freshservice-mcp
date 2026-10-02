# nixpkgs.overlays = [ freshservice-mcp.overlays.default ]; gives pkgs.freshservice-mcp.
final: _prev: {
  freshservice-mcp = final.callPackage ./pkgs/freshservice-mcp.nix { };
}
