# Vendored MaxMind conformance test database

`GeoLite2-City-Test.mmdb` is MaxMind's published client-conformance test
data, taken from <https://github.com/maxmind/MaxMind-DB> (`test-data/`).
It contains only synthetic test networks (e.g. `2.125.160.216` → GB)
and exists so GeoIP unit tests run hermetically without network access.

Production deployments must supply a real GeoLite2-City (or GeoIP2-City)
database — see `docs/tracking.md` ("Geographic detection") and the
`GEOIP_DB_PATH` setting in `.env.example`.
