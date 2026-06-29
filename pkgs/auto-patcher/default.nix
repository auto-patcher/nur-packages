{ lib, buildGoModule }:

buildGoModule {
  pname = "auto-patcher";
  version = "0.1.0";

  src = ./.;

  vendorHash = "sha256-yfEZmSLqhFJf1fqC5S5L2x5U4V4O7N8Q9R0S1T2U3V=";

  meta = {
    description = "AI-powered patch automation tool";
    homepage = "https://github.com/auto-patcher/nur-packages";
    license = lib.licenses.mit;
    platforms = lib.platforms.all;
    mainProgram = "auto-patcher";
  };
}
