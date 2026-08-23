# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS build
ENV GOTOOLCHAIN=auto
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /deploybot .
RUN mkdir /specs

FROM gcr.io/distroless/static:nonroot
COPY --from=build /deploybot /deploybot
COPY --from=build /specs /specs
USER nonroot:nonroot
EXPOSE 8080
ENV DEPLOYBOT_ADDR=:8080
ENV DEPLOYBOT_SPECS_DIR=/specs
ENTRYPOINT ["/deploybot"]
CMD ["serve"]
