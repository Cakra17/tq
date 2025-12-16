#!/usr/bin/env bash

case "$1" in
  up | down)
    migrate -verbose -path=./db/migrations -database postgres://admin:adminsecret@localhost:5432/tq?sslmode=disable $1
    ;;
  *)
    echo "you have to pass 'up' or 'down' as an argument"
    exit 1
    ;;
esac