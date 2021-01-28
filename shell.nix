{pkgs ? import <nixpkgs> {}}:
pkgs.mkShell {
  buildInputs = with pkgs; [
    nodePackages.vercel
    gopls
    go
  ];
}
