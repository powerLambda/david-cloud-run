FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags timetzdata -o /out/app ./

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/app /app
ENV PORT=8080
ENV CALDAV_USERNAME="" \
	CALDAV_PASSWORD=""
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/app"]
