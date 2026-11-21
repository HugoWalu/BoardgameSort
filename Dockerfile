FROM golang:1.27 AS build


WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/boardgamesort .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/boardgamesort /app/boardgamesort
COPY --from=build /src/web /app/web
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/boardgamesort"]