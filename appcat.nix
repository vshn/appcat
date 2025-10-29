{
  pkgs,
  buildGoModule,
}:
buildGoModule rec {
  pname = "appcat";
  version = "TBD";
  owner = "schedar";

  src = pkgs.lib.cleanSource ./.;

  # Tests depending on Docker will need more love
  doCheck = false;

  vendorHash = "sha256-l3sFEBagQse3Kg3roGcEMeUT3OjQq32IJwO+bFespnc=";

}

