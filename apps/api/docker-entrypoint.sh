#!/bin/sh
set -e

migrate -dir /app/migrations
seed
api
