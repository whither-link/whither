go_version  := "1.24"
go_image    := "docker.io/library/golang:" + go_version
lint_image  := "docker.io/golangci/golangci-lint:latest"
redis_image := "docker.io/library/redis:7-alpine"
binary      := "bin/whither"

_run := "podman run --rm --userns=keep-id --security-opt label=disable -v .:/workspace -w /workspace"

default:
    @just --list

build:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p bin
    {{_run}} {{go_image}} go build -o {{binary}} ./cmd/whither

test:
    #!/usr/bin/env bash
    set -euo pipefail
    {{_run}} {{go_image}} go test ./...

lint:
    #!/usr/bin/env bash
    set -euo pipefail
    {{_run}} {{lint_image}} golangci-lint run

vet:
    #!/usr/bin/env bash
    set -euo pipefail
    {{_run}} {{go_image}} go vet ./...

fmt-check:
    #!/usr/bin/env bash
    set -euo pipefail
    result=$({{_run}} {{go_image}} gofmt -l .)
    if [[ -n "$result" ]]; then
        printf 'Unformatted files:\n%s\n' "$result" >&2
        exit 1
    fi

check: vet lint fmt-check test

# Spin up a throwaway Redis container, run integration-tagged tests, then tear it down.
test-integration:
    #!/usr/bin/env bash
    set -euo pipefail
    port=16379
    name=whither-test-redis
    podman rm -f "$name" 2>/dev/null || true
    podman run -d --name "$name" -p 127.0.0.1:${port}:6379 {{redis_image}}
    trap 'podman rm -f "$name" >/dev/null 2>&1' EXIT
    until podman exec "$name" redis-cli ping 2>/dev/null | grep -q PONG; do
        sleep 0.2
    done
    podman run --rm --userns=keep-id --security-opt label=disable \
        --network=host \
        -e WHITHER_REDIS_URL="redis://127.0.0.1:${port}/0" \
        -v .:/workspace -w /workspace \
        {{go_image}} go test -tags=integration -count=1 ./...

tidy:
    #!/usr/bin/env bash
    set -euo pipefail
    {{_run}} {{go_image}} go mod tidy

dev:
    #!/usr/bin/env bash
    set -euo pipefail
    redis_name=whither-dev-redis
    podman rm -f "$redis_name" 2>/dev/null || true
    podman run -d --name "$redis_name" -p 127.0.0.1:6379:6379 {{redis_image}}
    trap 'podman rm -f "$redis_name" >/dev/null 2>&1' EXIT
    until podman exec "$redis_name" redis-cli ping 2>/dev/null | grep -q PONG; do
        sleep 0.2
    done
    podman run --rm --userns=keep-id --security-opt label=disable \
        --network=host \
        -v .:/workspace -w /workspace \
        -e WHITHER_LOG_FORMAT=text \
        -e WHITHER_LOG_LEVEL=debug \
        -e WHITHER_USER_AGENT_CONTACT=dev@whither.link \
        -e WHITHER_REDIS_URL=redis://127.0.0.1:6379/0 \
        {{go_image}} go run ./cmd/whither

gen-fixtures:
    #!/usr/bin/env bash
    set -euo pipefail
    bash scripts/gen-fixtures.sh

clean:
    #!/usr/bin/env bash
    set -euo pipefail
    rm -rf bin
