#!/bin/bash
set -e

# an alias to connect to mysql with the given host, port, user and password
function hanabi_mysql() {
    mysql --host=$HANABI_DB_HOST --port=$HANABI_DB_PORT --user=$HANABI_DB_USER --password=$HANABI_DB_PASS "$HANABI_DB_NAME" "$@"
}

# wait for mysql to come up (wait at most 30 times)
TRIES=30
echo "Waiting for Database to become available ..."
until hanabi_mysql -e "select 1;"  > /dev/null 2>&1 || [ $TRIES -eq 0 ]; do
    echo "Database not yet available. $((TRIES--)) tries left, or starting anyways. "
    sleep 1
done

# generate .env
echo "Writing '/app/.env'. "
cat << EOF > /app/.env
DOMAIN="${HANABI_DOMAIN}"
PORT=8080

SESSION_SECRET="${HANABI_SESSION_SECRET}"

DB_HOST="${HANABI_DB_HOST}"
DB_PORT=${HANABI_DB_PORT}
DB_USER="${HANABI_DB_USER}"
DB_PASS="${HANABI_DB_PASS}"
DB_NAME="${HANABI_DB_NAME}"

DISCORD_TOKEN=
DISCORD_LISTEN_CHANNEL_IDS=
DISCORD_LOBBY_CHANNEL_ID=
DISCORD_BOT_CHANNEL_ID=

GA_TRACKING_ID=
SENTRY_DSN=

TLS_CERT_FILE=
TLS_KEY_FILE=

EOF

if ! hanabi_mysql -e "use $HANABI_DB_NAME; SELECT 1 FROM users LIMIT 1; " > /dev/null 2>&1; then
    echo "Installing database schema becacuse 'users' tables does not exist. "
    hanabi_mysql < "/app/install/database_schema.sql"
else
    echo "Skipping installing database schema. "
fi


# startup with whatever command was provided
echo "Container initialization finished, handing control over to script. "
"$@"