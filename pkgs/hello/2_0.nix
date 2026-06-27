# Trivial example package — second version of `hello`, shows how multiple
# versions of one package coexist as distinct flat attrs (hello_1_0 vs
# hello_2_0). Safe to delete once real packages exist.
{ lib, runCommand }:

runCommand "hello-2.0"
{
  meta = {
    description = "Trivial example package (v2.0) demonstrating the auto-patcher NUR layout";
    license = lib.licenses.mit;
    platforms = lib.platforms.all;
    maintainers = [ ];
  };
} ''
  mkdir -p "$out/bin"
  cat > "$out/bin/hello-example" <<'SCRIPT'
  #!/bin/sh
  echo "Hello from the auto-patcher NUR — version 2.0"
  SCRIPT
  chmod +x "$out/bin/hello-example"
''
