FROM docker.io/library/golang:1.24 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /whither \
    ./cmd/whither

FROM gcr.io/distroless/static:nonroot

COPY --from=build /whither /whither

USER nonroot
EXPOSE 8080
ENTRYPOINT ["/whither"]
