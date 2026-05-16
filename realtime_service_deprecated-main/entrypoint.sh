#!/bin/sh
# Entrypoint for Realtime Gateway (Erlang release)
cd /app
exec ./bin/realtime_service "$@"
