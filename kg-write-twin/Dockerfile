# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/kg-write-twin ./cmd/kg-write-twin

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/kg-write-twin /kg-write-twin
# Port/base-path come from PORT/BASE_PATH env (see cmd/kg-write-twin/main.go), so
# the container can run on any port without rebuilding.
ENV PORT=8030 BASE_PATH=/api-server
EXPOSE 8030
ENTRYPOINT ["/kg-write-twin"]
