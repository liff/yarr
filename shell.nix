{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  #env.CGO_ENABLED = "0";

  packages = with pkgs; [
    go
    gotest
    gotestsum
    gotools
    delve
    go-swag
    gopls
    mage
  ];
}
