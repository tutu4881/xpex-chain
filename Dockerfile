FROM golang:1.25-alpine AS build

RUN apk add --no-cache make git libc-dev bash gcc linux-headers

WORKDIR /app

COPY go.mod go.sum ./
RUN cp go.sum /tmp/go.sum.orig && cp go.mod /tmp/go.mod.orig
RUN go mod download

COPY . .

# DIAGNOSTIC (temporary): repository go.sum is missing entries for x/upgrade's
# dependencies. Regenerate here and print the difference so it can be committed.
RUN go mod tidy 2>&1 | tail -5; \
    echo "=== GO.SUM NEW LINES ==="; \
    sort /tmp/go.sum.orig > /tmp/a.txt; sort go.sum > /tmp/b.txt; comm -13 /tmp/a.txt /tmp/b.txt; \
    echo "=== GO.MOD DIFF ==="; diff /tmp/go.mod.orig go.mod || true; \
    echo "=== END ==="

RUN go build -o /xpexd ./exampled


FROM alpine:3.22

RUN apk add --no-cache bash curl jq sed python3

COPY --from=build /xpexd /usr/local/bin/xpexd

RUN mkdir -p /data/.exampleapp/config /data/.exampleapp/data /opt/xpex

COPY railway-config/config/genesis.json /opt/xpex/genesis.json
COPY railway-config/config/genesis.staging.json /opt/xpex/genesis.staging.json
COPY railway-start.sh /usr/bin/railway-start.sh

RUN chmod +x /usr/bin/railway-start.sh

EXPOSE 26656
EXPOSE 26657
EXPOSE 1317
EXPOSE 9090

WORKDIR /root

ENTRYPOINT ["/usr/bin/railway-start.sh"]
