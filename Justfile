go_version := "1.24"
go_image   := "golang:" + go_version
lint_image := "golangci/golangci-lint:latest"
binary     := "bin/whither"

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

tidy:
    #!/usr/bin/env bash
    set -euo pipefail
    {{_run}} {{go_image}} go mod tidy

dev:
    #!/usr/bin/env bash
    set -euo pipefail
    {{_run}} \
        -p 8080:8080 \
        -e WHITHER_LOG_FORMAT=text \
        -e WHITHER_LOG_LEVEL=debug \
        {{go_image}} go run ./cmd/whither

gen-fixtures:
    #!/usr/bin/env bash
    set -euo pipefail
    bash scripts/gen-fixtures.sh

clean:
    #!/usr/bin/env bash
    set -euo pipefail
    rm -rf bin
