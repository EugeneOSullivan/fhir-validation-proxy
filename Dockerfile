# Start from the official Golang image for building
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o fhir-validation-proxy ./cmd/server

# Use a minimal image for running
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/fhir-validation-proxy ./
COPY configs ./configs
EXPOSE 8080
ENV FHIR_SERVER_URL=
CMD ["./fhir-validation-proxy"] 