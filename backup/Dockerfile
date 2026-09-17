# Based on postgres:16-alpine so pg_dump matches the server version exactly
# (avoids client/server version-mismatch dump failures).
FROM postgres:16-alpine

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
