#!/bin/sh
# Starts ttyd serving the KeroAgile TUI on port 7433.
#
# Anyone who can open this port gets the whole board: they can read every
# project and edit or delete tickets. Which network interfaces the port is
# published on is decided outside the container (see docker-compose.yml);
# these settings control what a visitor needs once they can reach it.
#
#   TTYD_CREDENTIAL    user:password. When set, the browser must log in.
#   TTYD_MAX_CLIENTS   most browser sessions allowed at once (default: no limit)
#   TTYD_CHECK_ORIGIN  set to "false" to turn off the same-origin check, which
#                      some reverse proxies need. On by default, so a page on
#                      another site cannot open a session in your browser.
set -eu

set -- --port 7433 --writable
if [ "${TTYD_CHECK_ORIGIN:-true}" != "false" ]; then
	set -- "$@" --check-origin
fi
if [ -n "${TTYD_CREDENTIAL:-}" ]; then
	set -- "$@" --credential "$TTYD_CREDENTIAL"
fi
if [ -n "${TTYD_MAX_CLIENTS:-}" ]; then
	set -- "$@" --max-clients "$TTYD_MAX_CLIENTS"
fi

exec ttyd "$@" KeroAgile
