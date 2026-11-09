FROM node:24-alpine AS client-build
WORKDIR /client

COPY src/client/package.json src/client/pnpm-lock.yaml src/client/pnpm-workspace.yaml ./
RUN corepack enable pnpm \
    && pnpm config set fetch-timeout 600000 \
    && pnpm config set fetch-retries 5 \
    && pnpm config set fetch-retry-maxtimeout 120000 \
    && pnpm install --frozen-lockfile

COPY src/client/ ./
RUN pnpm build

FROM golang:1.26-alpine AS server-build
WORKDIR /src/server

COPY src/server/go.mod src/server/go.sum ./
RUN for attempt in 1 2 3 4 5; do \
      go mod download && break; \
      if [ "$attempt" = "5" ]; then exit 1; fi; \
      sleep 10; \
    done

COPY src/server/ ./
RUN CGO_ENABLED=0 go build -o /out/server .

FROM alpine:3.22
WORKDIR /app

RUN apk add --no-cache ca-certificates wget

COPY --from=server-build /out/server ./server
COPY --from=client-build /client/out ./pb_public
COPY src/server/.env.example ./.env.example

EXPOSE 8090
CMD ["./server", "serve", "--http=0.0.0.0:8090"]
