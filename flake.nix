{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs";
  };

  outputs = { self, nixpkgs }:
    let
    system = "x86_64-linux";
  pkgs = import nixpkgs { inherit system; };
  in {
    packages.x86_64-linux = {
      default = pkgs.buildGoModule {
        pname = "postfix-queue-exporter";
        version = "0.1.0";
        src = ./.;
        vendorHash = "sha256-oeCSKwDKVwvYQ1fjXXTwQSXNl/upDE3WAAk680vqh3U=";
      };
    };

  };
}
