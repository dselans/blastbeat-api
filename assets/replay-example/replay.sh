#!/bin/sh
# Usage: ./replay.sh /path/to/dir
#
# Example: PLUMBER_RABBITMQ_URL=amqp://guest:guest@localhost:5672 ./replay.sh ./assets/replay-20250914
#

set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 /path/to/dir"
    exit 1
fi

if [ -z "${PLUMBER_RABBITMQ_URL:-}" ]; then
    echo "Variable not set"
    export PLUMBER_RABBITMQ_URL="amqp://guest:guest@localhost:5672"
fi

DIR="$1"

if [ ! -d "$DIR" ]; then
    echo "Directory $DIR does not exist."
    exit 1
fi

for file in $(ls "$DIR"/*.json | awk -F. '{print $(NF-1), $0}' | sort -n | cut -d' ' -f2-); do
    echo "Replaying $file to $PLUMBER_RABBITMQ_URL"
  	plumber write rabbit \
      --protobuf-descriptor-set ../events/events.protoset \
      --protobuf-root-message common.Event \
      --address ${PLUMBER_RABBITMQ_URL} \
      --exchange-name events-replay-go-merch \
      --routing-key user.Updated \
      --encode-type jsonpb \
      --input-file "${file}"
done
