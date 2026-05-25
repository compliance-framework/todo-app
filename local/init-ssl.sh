#!/bin/sh
# Generate self-signed SSL certificates for PostgreSQL
mkdir -p /var/lib/postgresql/certs
cd /var/lib/postgresql/certs

# Generate server key and certificate
openssl req -new -x509 -days 365 -nodes -text -out server.crt \
  -keyout server.key -subj "/CN=postgres" \
  -addext "subjectAltName=DNS:postgres,DNS:localhost,IP:127.0.0.1"

# Set permissions
chmod 600 server.key
chown postgres:postgres server.key server.crt

# Copy for client verification
cp server.crt /var/lib/postgresql/certs/ca.crt
