#!/bin/sh
set -e

# Export the CA cert to the shared volume so backend can verify the server cert
if [ -d /certs-out ]; then
  cp /pg-ssl/ca.crt /certs-out/ca.crt
  chmod 644 /certs-out/ca.crt
fi

# Hand off to the original postgres entrypoint, enabling SSL with baked-in certs
exec docker-entrypoint.sh "$@" \
  -c ssl=on \
  -c ssl_cert_file=/pg-ssl/server.crt \
  -c ssl_key_file=/pg-ssl/server.key
