# Deploys the autopatcher dispatcher binary.
# The dispatcher clones repos and invokes claude directly as a subprocess.
# Build with podman: podman build -t ghcr.io/auto-patcher/dispatcher:latest .
#
# Assumes the dispatcher binary has been compiled and is in the build context.
# Build the binary first: nix build .#dispatcher && cp result/bin/dispatcher .

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    curl git ca-certificates nodejs npm \
    && rm -rf /var/lib/apt/lists/*

# Install GitHub CLI
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg && \
    chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    | tee /etc/apt/sources.list.d/github-cli.list > /dev/null && \
    apt-get update && apt-get install -y gh && \
    rm -rf /var/lib/apt/lists/*

# Install claude-code.
# TODO: replace with auto-patcher fork once published:
#   RUN npm install -g https://github.com/auto-patcher/claude-code
RUN npm install -g @anthropic-ai/claude-code

# Copy the compiled dispatcher binary.
COPY dispatcher /usr/local/bin/dispatcher

ENTRYPOINT ["dispatcher"]
