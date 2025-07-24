FROM golang:1.24.0-alpine AS builder
WORKDIR /app
COPY go.mod .
COPY main.go .
COPY helpers/ ./helpers/
COPY ui/ ./ui/

RUN <<EOF
go mod tidy 
go build
EOF

FROM scratch
WORKDIR /app
COPY --from=builder /app/dm-agent .
COPY instructions.md .
COPY character_sheet.md .

ENTRYPOINT ["./dm-agent"]
