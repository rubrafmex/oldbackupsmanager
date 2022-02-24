FROM scratch

LABEL maintainer="ruben.rafaelmartinez@cm.com"

# Variables used in config.ini
ENV BACKUPSMGR_WORKINGDIR="/var/data"
ENV BACKUPSMGR_BASEURL="http://localhost:31000"
ENV BACKUPSMGR_DB_HOST="localhost"
ENV BACKUPSMGR_DB_USER="root"
ENV BACKUPSMGR_DB_NAME="pg_commonapi"
ENV BACKUPSMGR_DB_OPTIONS="sslmode=disable"
ENV BACKUPSMGR_GCP_BASE64_ENCODED_JSON_KEY="ewogICJ0ZXN0IjogInJlcGxhY2UgdGhpcyBqc29uIHdpdGggdGhlIGNvcnJlY3QgZ2NwIGpzb24gY3JlZGVudGlhbHMgZGVwZW5kaW5nIG9uIHRoZSBlbnYiCn0="
ENV BACKUPSMGR_GCS_BUCKET_NAME="uniquebucketname"

# Root certificates
# This contains all the regular ones plus our own ones (ClubMessage, CMgroep)
COPY --from=gitlabregistry.cmpayments.local/cicd/docker/go:v1.17 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy config
COPY configs/docker.ini /config.ini

# Copy binary
COPY /bin/app /app

EXPOSE 31000

ENTRYPOINT ["/app"]
