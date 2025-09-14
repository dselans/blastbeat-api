This (rudimentary) replay was facilitated by doing the following:

1. `fhir-cli-beta download-events --prefix "user.Updated" --start "2025-09-10T08:13:00-07:00" --end "2025-09-12T00:00:00-07:00"`
  * This downloads all matching events and saves them under `./download-events-$unixTimestamp`
2. `k port-forward pod/rabbitmq-0 5672 -n rabbit` -- against prod rabbit
3. `cd ./assets/scripts/replay-20250914; PLUMBER_RABBITMQ_URL="amqp://user:$PROD_PASS@localhost:5672" ./replay.sh download-events-1757887468`

This will launch a `plumber write` for every `.json` file in `./download-events-$ts`.

