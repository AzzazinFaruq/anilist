# syntax=docker/dockerfile:1
# Three stages: build the React app, build (and test) the Go server with the
# app embedded, then ship only the static binary on an empty base image.

FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS server
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-buildvcs=false
COPY go.mod ./
COPY cmd cmd
COPY internal internal
RUN go vet ./... && go test ./...
COPY --from=web /web/dist internal/web/dist
RUN go build -trimpath -ldflags="-s -w" -o /out/tonton ./cmd/tonton

FROM scratch
COPY --from=server /out/tonton /tonton
USER 1000:1000
ENV TONTON_ADDR=:8080 TONTON_DATA=/data TONTON_CATALOG=/catalog TZ=Asia/Jakarta
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s CMD ["/tonton", "healthcheck"]
ENTRYPOINT ["/tonton"]
CMD ["serve"]
