# DevPulse Privacy Principles

DevPulse is designed to provide useful website analytics while minimizing the collection of personal data. These are product requirements for the tracker and analytics API as they are implemented.

## Data Minimization

DevPulse should collect only information needed for analytics. It must not collect:

- Names, email addresses, phone numbers, passwords, or authentication tokens
- Raw IP addresses for permanent storage
- GPS coordinates, clipboard contents, keystrokes, camera or microphone data, or mouse movement
- Browser fingerprints

## Visitor Identification

DevPulse will use a random, first-party visitor identifier. It must not contain personal information, be shared across unrelated domains, or be generated through browser fingerprinting. Website owners may disable persistent visitor identification.

## IP Addresses and Geography

The tracker must not send a visitor's IP address. The API may temporarily inspect an incoming request's network address for approximate GeoIP enrichment, but it must not permanently store raw IP addresses.

Stored geographic data may include country and region. Precise GPS location is not collected.

## Retention

Analytics data must have configurable retention periods. The default retention period is 90 days, and expired analytics data should be deleted periodically.

## Scope

DevPulse is intended for first-party website analytics. It does not provide cross-site tracking, advertising profiles, user profiling, fingerprinting, heatmaps, or session recordings.

## Data Protection

The API should validate incoming events, apply rate limits, avoid logging sensitive analytics payloads, isolate projects, protect dashboard APIs, and use parameterized database queries. Privacy requirements apply throughout development.
