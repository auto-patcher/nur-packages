# Trivial example package — demonstrates the pkgs/<name>/<version>.nix layout
# and a fully offline, reproducible build. Safe to delete once the
# auto-patcher has populated real packages.
{ lib, runCommand }:

runCommand "hello-1.0"
{
  meta = {
    description = "Trivial example package (v1.0) demonstrating the auto-patcher NUR layout";
    license = lib.licenses.mit;
    platforms = lib.platforms.all;
    maintainers = [ ];
  };
} ''
  mkdir -p "$out/bin"
  cat > "$out/bin/hello-example" <<'SCRIPT'
  #!/bin/sh
  echo "Hello from the auto-patcher NUR — version 1.0"
  SCRIPT
  chmod +x "$out/bin/hello-example"
''
