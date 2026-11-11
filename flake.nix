{
  description = "Explain Nix error messages offline";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      inherit (nixpkgs) lib;
      version = "0.2.3-hotfix";
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      eachSystem =
        f:
        lib.genAttrs systems (
          system:
          f {
            inherit system;
            pkgs = nixpkgs.legacyPackages.${system};
          }
        );
    in
    {
      packages = eachSystem (
        { pkgs, ... }:
        rec {
          nix-explain = pkgs.buildGoModule {
            pname = "nix-explain";
            inherit version;
            src = self;
            subPackages = [ "." ];
            vendorHash = "sha256-g+yaVIx4jxpAQ/+WrGKxhVeliYx7nLQe/zsGpxV4Fn4=";
            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
            ];
            meta = {
              description = "Explain Nix error messages offline";
              homepage = "https://github.com/nepoet/nix-explain";
              license = lib.licenses.mit;
              mainProgram = "nix-explain";
            };
          };
          default = nix-explain;
        }
      );

      apps = eachSystem (
        { pkgs, ... }:
        {
          default = {
            type = "app";
            program = "${self.packages.${pkgs.stdenv.hostPlatform.system}.default}/bin/nix-explain";
          };
        }
      );

      devShells = eachSystem (
        { pkgs, ... }:
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              pkgs.gopls
              pkgs.staticcheck
            ];
          };
        }
      );

      formatter = eachSystem ({ pkgs, ... }: pkgs.nixfmt-rfc-style);

      checks = eachSystem ({ pkgs, ... }: {
        default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      });
    };
}
