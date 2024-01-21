FROM golang:1.21.6-alpine as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /rokim_cd

FROM alpine:3.19.0

# Copy only the binary from the build stage to the final image
COPY --from=builder /rokim_cd /


ENTRYPOINT ["/rokim_cd"]