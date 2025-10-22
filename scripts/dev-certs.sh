#!/usr/bin/env bash
set -euo pipefail

OUT=${1:-configs/dev}
mkdir -p "$OUT"

echo "Generating dev CA..."
openssl genrsa -out "$OUT/ca.key" 2048 >/dev/null 2>&1
openssl req -x509 -new -nodes -key "$OUT/ca.key" -sha256 -days 3650 \
  -subj "/CN=croupier-dev-ca" -out "$OUT/ca.crt" >/dev/null 2>&1

cat > "$OUT/core-openssl.cnf" <<EOF
[req]
distinguished_name=req_distinguished_name
[req_distinguished_name]
[ v3_req ]
subjectAltName = @alt_names
[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
EOF

echo "Generating Core cert..."
openssl genrsa -out "$OUT/server.key" 2048 >/dev/null 2>&1
openssl req -new -key "$OUT/server.key" -subj "/CN=croupier-core" -out "$OUT/server.csr" -config "$OUT/core-openssl.cnf" >/dev/null 2>&1
openssl x509 -req -in "$OUT/server.csr" -CA "$OUT/ca.crt" -CAkey "$OUT/ca.key" -CAcreateserial \
  -out "$OUT/server.crt" -days 365 -sha256 -extensions v3_req -extfile "$OUT/core-openssl.cnf" >/dev/null 2>&1

echo "Generating Agent cert..."
openssl genrsa -out "$OUT/agent.key" 2048 >/dev/null 2>&1
openssl req -new -key "$OUT/agent.key" -subj "/CN=croupier-agent" -out "$OUT/agent.csr" >/dev/null 2>&1
openssl x509 -req -in "$OUT/agent.csr" -CA "$OUT/ca.crt" -CAkey "$OUT/ca.key" -CAcreateserial \
  -out "$OUT/agent.crt" -days 365 -sha256 >/dev/null 2>&1

echo "Done. Files in $OUT:"
ls -1 "$OUT" | sed 's/^/  - /'

