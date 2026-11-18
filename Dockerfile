# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/todo ./cmd/todo
# A placeholder keeps the owned data directory present in the runtime image.
RUN mkdir -p /out/data && touch /out/data/.keep

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/todo /app/todo
COPY --from=build --chown=65532:65532 /out/data /data
ENV ADDR=:8080
ENV DATABASE_PATH=/data/todos.db
EXPOSE 8080
VOLUME ["/data"]
USER 65532:65532
ENTRYPOINT ["/app/todo"]
