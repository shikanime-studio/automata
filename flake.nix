{
  inputs = {
    devenv.url = "github:cachix/devenv";
    devlib.url = "github:shikanime-studio/devlib";
    flake-parts.url = "github:hercules-ci/flake-parts";
    git-hooks.url = "github:cachix/git-hooks.nix";
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    treefmt-nix.url = "github:numtide/treefmt-nix";
  };

  nixConfig = {
    extra-substituters = [
      "https://cachix.cachix.org"
      "https://devenv.cachix.org"
      "https://shikanime.cachix.org"
    ];
    extra-trusted-public-keys = [
      "cachix.cachix.org-1:eWNHQldwUO7G2VkjpnjDbWwy4KQ/HNxht7H4SSoMckM="
      "devenv.cachix.org-1:w1cLUi8dv3hnoSPGAuibQv+f9TZLr6cv/Hm9XgU50cw="
      "shikanime.cachix.org-1:OrpjVTH6RzYf2R97IqcTWdLRejF6+XbpFNNZJxKG8Ts="
    ];
  };

  outputs =
    inputs@{
      devenv,
      devlib,
      flake-parts,
      git-hooks,
      treefmt-nix,
      ...
    }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        devenv.flakeModule
        devlib.flakeModule
        git-hooks.flakeModule
        treefmt-nix.flakeModule
      ];
      perSystem =
        {
          self',
          pkgs,
          lib,
          ...
        }:
        {
          devenv.shells.default = {
            imports = [
              devlib.devenvModules.docs
              devlib.devenvModules.formats
              devlib.devenvModules.github
              devlib.devenvModules.go
              devlib.devenvModules.nix
              devlib.devenvModules.opentofu
              devlib.devenvModules.shell
              devlib.devenvModules.shikanime-studio
            ];

            automata.package = self'.packages.default;
          };

          packages.default = pkgs.buildGoModule {
            pname = "automata";
            version = "v0.1.0";
            src = lib.cleanSource ./.;
            subPackages = [ "cmd/automata" ];
            vendorHash = null;
            meta = {
              description = "Automata CLI";
              homepage = "https://github.com/shikanime-studio/automata";
              license = lib.licenses.asl20;
              mainProgram = "automata";
            };
          };
        };
      systems = [
        "x86_64-linux"
        "x86_64-darwin"
        "aarch64-linux"
        "aarch64-darwin"
      ];
    };
}
