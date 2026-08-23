{
  description = "feedlr-yt - alternative frontend for YouTube and podcasts";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            # Go toolchain; templ and sqlboiler run via `go tool` (see go.mod)
            go
            # Frontend build
            nodejs
            # Live reload
            air
            # Task runner
            go-task
            # Database migrations (task migrate / task migrate-apply)
            atlas
            # Inspecting the SQLite database by hand
            sqlite-interactive
          ];
        };
      });
    };
}
