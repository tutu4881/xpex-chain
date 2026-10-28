# Hermes IBC relayer for XPEX Chain. Deployed as its own Railway service (root directory: relayer/).
FROM debian:bookworm-slim

ARG HERMES_VERSION=v1.13.3

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl jq \
 && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /tmp/hermes \
 && curl -fsSL -o /tmp/hermes.tgz "https://github.com/informalsystems/hermes/releases/download/${HERMES_VERSION}/hermes-${HERMES_VERSION}-x86_64-unknown-linux-gnu.tar.gz" \
 && tar -xzf /tmp/hermes.tgz -C /tmp/hermes \
 && install -m 0755 "$(find /tmp/hermes -type f -name hermes | head -n1)" /usr/local/bin/hermes \
 && rm -rf /tmp/hermes /tmp/hermes.tgz \
 && hermes version

RUN useradd -m -u 2000 hermes
USER hermes
WORKDIR /home/hermes

COPY --chown=hermes:hermes entrypoint.sh /home/hermes/entrypoint.sh
ENTRYPOINT ["/bin/bash", "/home/hermes/entrypoint.sh"]
