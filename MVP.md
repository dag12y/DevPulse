DevPulse Web Analytics MVP

1. Purpose

DevPulse Web Analytics provides developers with simple, privacy-conscious website analytics without requiring a third-party analytics platform.

The MVP answers:

How many people visited my website?

How many unique visitors did I have?

How many pages were viewed?

Which pages are most popular?

Where did visitors come from?

Which countries are visitors from?

What devices and browsers do they use?

How long do visitors stay?

How many visitors are currently online?

The system should be lightweight enough to add to a website with a single script tag.

2. MVP Scope

Included

Page-view tracking

Unique visitor tracking

Session tracking

New vs returning visitors

Referrer tracking

UTM campaign tracking

Country detection

Device type

Browser

Operating system

Screen dimensions

Top pages

Traffic sources

Visitor trends

Session duration

Bounce rate

Real-time visitors

Date-range filtering

Basic bot filtering

Privacy-conscious IP handling

Configurable data retention

Not included in MVP

Do not build these yet:

Heatmaps

Session recordings

Mouse tracking

Scroll tracking

User profiles

Advertising tracking

Cross-site tracking

Fingerprinting

Demographic inference

Individual visitor identification

A/B testing

Conversion funnels

E-commerce analytics

Google Search Console integration

Advanced attribution models

The MVP should remain focused on understanding website traffic.

3. High-Level Architecture

                  User's Website
                        │
                        │
                 analytics.js
                        │
                        │ page_view
                        ▼
              ┌────────────────────┐
              │ DevPulse Analytics  │
              │      API            │
              └─────────┬──────────┘
                        │
                 Validate event
                        │
                 Detect bot
                        │
              Resolve GeoIP metadata
                        │
              Create visitor/session
                        │
                        ▼
                 PostgreSQL
              ┌──────────────────┐
              │ visitors         │
              │ sessions         │
              │ page_views       │
              │ analytics_sites  │
              └────────┬─────────┘
                       │
                       ▼
               Analytics API
                       │
                       ▼
               Next.js Dashboard


The browser should never communicate directly with PostgreSQL.

4. Analytics Project

Each website being monitored is an analytics_project.

Example:

Project:
Dagm Portfolio

Tracking ID:
dp_01JXYZ...

Website:
https://dagmportfolio.example


A project has:

project ID

name

allowed domains

tracking status

timezone

retention period

creation date

Tracking ID

The browser receives a public tracking identifier.

Example:

<script
  src="https://analytics.devpulse.example/analytics.js"
  data-project="dp_01JXYZ">
</script>


The tracking ID is not a secret.

It only identifies the analytics project.

5. Event Model

All browser analytics begins as an event.

The MVP supports one primary browser event:

page_view


Future event types can be added later:

page_view
session_start
session_end
custom_event


But do not implement arbitrary custom events initially.

6. Page View Event Schema

The browser sends:

{
  "type": "page_view",
  "project_id": "dp_01JXYZ",
  "session_id": "01JSESSION...",
  "visitor_id": "01JVISITOR...",
  "timestamp": "2026-09-14T14:32:21.000Z",

  "page": {
    "url": "https://example.com/projects",
    "path": "/projects",
    "title": "Projects | Dagm",
    "referrer": "https://github.com/dag12y"
  },

  "screen": {
    "width": 1920,
    "height": 1080
  },

  "viewport": {
    "width": 1536,
    "height": 864
  },

  "language": "en-US",
  "timezone": "Africa/Addis_Ababa",

  "campaign": {
    "source": "github",
    "medium": "social",
    "campaign": "portfolio"
  }
}


The server should enrich this event with:

country
region
device_type
browser
browser_version
os
os_version
is_bot


The browser should not send an IP address as an analytics field.

The server may temporarily inspect the network address needed for GeoIP processing, but the raw IP must not be persisted as an analytics record.

7. Event Validation

Every incoming event must be validated.

Required:

type
project_id
timestamp
page.path


Optional:

session_id
visitor_id
page.title
page.referrer
screen
viewport
language
timezone
campaign


Limits:

URL length:        2048 characters
Path length:       1024 characters
Title length:      512 characters
Referrer length:   2048 characters
Language:          32 characters
Timezone:          64 characters
UTM values:        256 characters each


Reject:

malformed JSON

unknown project

disabled project

invalid tracking ID

oversized payload

invalid timestamp

unsupported event type

obviously malicious input

8. Visitor Identification

DevPulse needs to distinguish unique visitors.

The MVP should use a random first-party visitor identifier.

Example:

dp_visitor=01JVISITORXYZ


This identifier is stored in the website's first-party browser storage.

Preferred approach:

Cookie:
dp_visitor=<random-id>


The cookie should:

be random

contain no personal information

be scoped to the website

have a configurable lifetime

never contain email/name/user ID

never be shared across domains

Do not fingerprint visitors.

Do not use:

IP + User-Agent + Screen + Font + Canvas


or similar fingerprinting techniques.

9. Visitor Table

CREATE TABLE analytics_visitors (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES analytics_projects(id),

    visitor_key TEXT NOT NULL,

    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,

    first_country TEXT,
    last_country TEXT,

    first_referrer TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(project_id, visitor_key)
);


visitor_key is the random first-party identifier.

Do not store:

name
email
phone
account_id
raw_ip


10. Session Identification

A visitor can have multiple sessions.

Example:

Visitor A
 ├── Session 1
 │    ├── /
 │    ├── /projects
 │    └── /about
 │
 └── Session 2
      ├── /
      └── /contact


A new session begins when:

there is no existing session, or

the previous activity is older than 30 minutes.

Default session timeout:

30 minutes


A session ends after 30 minutes of inactivity.

11. Session Table

CREATE TABLE analytics_sessions (
    id UUID PRIMARY KEY,

    project_id UUID NOT NULL REFERENCES analytics_projects(id),
    visitor_id UUID NOT NULL REFERENCES analytics_visitors(id),

    started_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,

    landing_page TEXT,
    exit_page TEXT,

    page_views INTEGER NOT NULL DEFAULT 0,

    referrer TEXT,
    country TEXT,

    device_type TEXT,
    browser TEXT,
    browser_version TEXT,
    os TEXT,
    os_version TEXT,

    is_bounce BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


12. Page View Table

CREATE TABLE analytics_page_views (
    id BIGSERIAL PRIMARY KEY,

    project_id UUID NOT NULL REFERENCES analytics_projects(id),
    visitor_id UUID NOT NULL REFERENCES analytics_visitors(id),
    session_id UUID NOT NULL REFERENCES analytics_sessions(id),

    path TEXT NOT NULL,
    page_title TEXT,

    referrer TEXT,

    country TEXT,
    region TEXT,

    device_type TEXT,
    browser TEXT,
    browser_version TEXT,
    os TEXT,
    os_version TEXT,

    screen_width INTEGER,
    screen_height INTEGER,

    viewport_width INTEGER,
    viewport_height INTEGER,

    language TEXT,
    timezone TEXT,

    utm_source TEXT,
    utm_medium TEXT,
    utm_campaign TEXT,
    utm_term TEXT,
    utm_content TEXT,

    occurred_at TIMESTAMPTZ NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


13. Analytics Project Table

CREATE TABLE analytics_projects (
    id UUID PRIMARY KEY,

    workspace_id UUID NOT NULL REFERENCES workspaces(id),

    name TEXT NOT NULL,

    tracking_id TEXT NOT NULL UNIQUE,

    allowed_domains TEXT[] NOT NULL DEFAULT '{}',

    timezone TEXT NOT NULL DEFAULT 'UTC',

    retention_days INTEGER NOT NULL DEFAULT 90,

    enabled BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


Example:

name:
Dagm Portfolio

tracking_id:
dp_01JXYZ

allowed_domains:
[
  "dagmportfolio.vercel.app",
  "www.dagmportfolio.com"
]

timezone:
Africa/Addis_Ababa

retention:
90 days


14. Database Indexes

The main query patterns are date-range analytics.

Create indexes on:

CREATE INDEX idx_page_views_project_time
ON analytics_page_views(project_id, occurred_at DESC);

CREATE INDEX idx_page_views_project_path
ON analytics_page_views(project_id, path);

CREATE INDEX idx_page_views_session
ON analytics_page_views(session_id);

CREATE INDEX idx_sessions_project_time
ON analytics_sessions(project_id, started_at DESC);

CREATE INDEX idx_sessions_visitor
ON analytics_sessions(visitor_id);

CREATE INDEX idx_visitors_project_last_seen
ON analytics_visitors(project_id, last_seen_at DESC);


Do not prematurely create dozens of indexes.

15. API Endpoints

Public ingestion API

POST /v1/analytics/events

Receives browser analytics events.

Request:

POST /v1/analytics/events
Content-Type: application/json


Body:

{
  "type": "page_view",
  "project_id": "dp_01JXYZ",
  "visitor_id": "01JVISITOR",
  "session_id": "01JSESSION",
  "timestamp": "2026-09-14T14:32:21Z",
  "page": {
    "path": "/projects",
    "title": "Projects",
    "url": "https://example.com/projects",
    "referrer": "https://github.com/"
  }
}


Response:

{
  "accepted": true
}


The endpoint should return quickly.

The browser should not need to wait for dashboard aggregation.

16. Project Management API

These require authenticated DevPulse dashboard access.

POST /v1/analytics/projects

Create analytics project.

{
  "name": "My Portfolio",
  "allowed_domains": [
    "example.com"
  ],
  "timezone": "Africa/Addis_Ababa"
}


GET /v1/analytics/projects

List analytics projects.

GET /v1/analytics/projects/:id

Get project configuration.

PATCH /v1/analytics/projects/:id

Update:

name

domains

timezone

retention

enabled

DELETE /v1/analytics/projects/:id

Delete the analytics project and associated analytics data.

17. Analytics Query API

Overview

GET /v1/analytics/projects/:id/overview


Query:

?from=2026-09-01
&to=2026-09-14


Response:

{
  "visitors": 1284,
  "sessions": 1643,
  "page_views": 3721,
  "bounce_rate": 42.1,
  "avg_session_duration": 184
}


18. Visitors Over Time

GET /v1/analytics/projects/:id/timeseries


Parameters:

from
to
interval=hour|day


Response:

{
  "data": [
    {
      "timestamp": "2026-09-14T00:00:00Z",
      "visitors": 82,
      "sessions": 97,
      "page_views": 241
    }
  ]
}


19. Top Pages

GET /v1/analytics/projects/:id/pages


Response:

{
  "data": [
    {
      "path": "/",
      "page_views": 1842,
      "unique_visitors": 931
    },
    {
      "path": "/projects",
      "page_views": 923,
      "unique_visitors": 601
    }
  ]
}


20. Traffic Sources

GET /v1/analytics/projects/:id/sources


Response:

{
  "data": [
    {
      "source": "Google",
      "sessions": 691,
      "percentage": 42.1
    },
    {
      "source": "Direct",
      "sessions": 509,
      "percentage": 31.0
    },
    {
      "source": "GitHub",
      "sessions": 148,
      "percentage": 9.0
    }
  ]
}


21. Countries

GET /v1/analytics/projects/:id/countries


Response:

{
  "data": [
    {
      "country": "Ethiopia",
      "visitors": 4821
    },
    {
      "country": "United States",
      "visitors": 1932
    }
  ]
}


The API should return country codes internally where appropriate:

ET
US
DE
GB
KE


The frontend can convert these into display names.

22. Devices

GET /v1/analytics/projects/:id/devices


Example:

{
  "device_types": [
    {
      "type": "desktop",
      "visitors": 712
    },
    {
      "type": "mobile",
      "visitors": 492
    },
    {
      "type": "tablet",
      "visitors": 80
    }
  ],

  "browsers": [
    {
      "name": "Chrome",
      "visitors": 831
    }
  ],

  "operating_systems": [
    {
      "name": "Windows",
      "visitors": 612
    }
  ]
}


23. Real-Time API

GET /v1/analytics/projects/:id/realtime


Returns currently active visitors.

A visitor is considered active if their last event occurred within:

5 minutes


Example:

{
  "active_visitors": 17,
  "pages": [
    {
      "path": "/projects",
      "visitors": 5
    },
    {
      "path": "/",
      "visitors": 7
    }
  ]
}


For true live updates, use WebSockets later:

/ws/analytics/:project_id


The initial implementation can poll every 15–30 seconds.

24. Browser Tracking Script

The tracking script should be extremely small.

Example:

<script
  src="https://analytics.devpulse.example/analytics.js"
  data-project="dp_01JXYZ">
</script>


The script should:

Read the project ID.

Validate that it exists.

Create/retrieve visitor ID.

Create/retrieve session ID.

Capture current page.

Capture referrer.

Capture screen information.

Capture UTM parameters.

Send a page-view event.

Never interfere with the host website.

25. Script Failure Behavior

This is extremely important.

If DevPulse is down:

Website must continue working normally.


The analytics script must never:

throw uncaught errors

block page rendering

freeze the website

retry forever

consume excessive memory

repeatedly hammer the DevPulse API

Use:

timeout
+
limited retry
+
silent failure


Analytics is optional functionality.

The user's website is the priority.

26. Sending Events

Use:

navigator.sendBeacon()


when appropriate.

Otherwise:

fetch()


with:

keepalive: true


The browser should send analytics asynchronously.

Example:

navigator.sendBeacon(
  "https://analytics.devpulse.example/v1/analytics/events",
  JSON.stringify(event)
);


The script should use:

Content-Type: application/json


where supported.

27. SPA Navigation

Modern websites often use React/Next.js.

A traditional script only sees the initial page load.

Therefore the MVP should support SPA navigation.

Detect:

history.pushState
history.replaceState
popstate


When the URL changes:

send page_view


Example:

/
/projects
/projects/saferun
/contact


Each should become a separate page view.

Avoid duplicate events for the same navigation.

28. UTM Tracking

Support:

utm_source
utm_medium
utm_campaign
utm_term
utm_content


Example:

https://example.com/?utm_source=linkedin&utm_medium=social&utm_campaign=launch


Store these on the first session/page view.

The dashboard can then display:

Campaign
Source
Medium


Do not build advanced attribution yet.

29. Referrer Processing

The browser sends:

document.referrer


DevPulse should normalize it.

Examples:

https://www.google.com/search?q=...
→ Google

https://github.com/dag12y
→ GitHub

https://www.linkedin.com/...
→ LinkedIn


Do not store unnecessary query parameters from external URLs.

In particular, avoid retaining potentially sensitive search queries.

Prefer storing:

google.com


rather than:

https://google.com/search?q=private-information


30. Traffic Source Classification

Initial source categories:

Direct
Organic Search
Social
Referral
Campaign
Other


Known sources can be recognized:

Google
Bing
GitHub
LinkedIn
Reddit
Facebook
Instagram
YouTube
X


Example:

github.com
→ Social / GitHub

google.com
→ Organic Search / Google

example.com
→ Referral / example.com

empty referrer
→ Direct


31. Device Detection

Use the browser User-Agent on the server.

Classify into:

desktop
mobile
tablet
bot
unknown


Do not attempt invasive fingerprinting.

32. Browser Detection

Initially support common browsers:

Chrome
Firefox
Safari
Edge
Opera
Samsung Internet
Other


Store:

browser
browser_version


33. Operating System Detection

Initially support:

Windows
macOS
Linux
Android
iOS
Other


Store:

os
os_version


34. Geographic Detection

The server may use the request's network address to determine approximate geography.

Possible output:

country
region


Example:

country = ET
region = Oromia


The raw network address must not become part of the permanent analytics dataset.

Recommended processing:

Incoming request
       │
       ▼
Temporary IP
       │
       ▼
GeoIP lookup
       │
       ├── country
       └── region
       │
       ▼
Discard raw IP
       │
       ▼
Store geographic metadata


Do not promise precise location.

Analytics geography should be considered approximate.

35. Privacy Rules

DevPulse Web Analytics should follow a data minimization principle.

Never intentionally collect:

Name
Email
Phone
Password
Authentication token
Full IP address
Precise GPS location
Clipboard data
Mouse movements
Keystrokes
Camera
Microphone
Browser fingerprint


Do not allow arbitrary user-provided PII to become an analytics identifier.

36. Cookies

The MVP uses a first-party visitor identifier.

Example:

dp_visitor


The value should be random:

01JXYZ...


It must not encode:

user ID
email
IP
location


Session information can use:

sessionStorage


or an equivalent short-lived mechanism.

37. Privacy Controls

Each analytics project should eventually support:

Analytics enabled
Cookie-based visitor identification
Retention period
Respect Do Not Track


For the MVP, make the privacy behavior explicit in project settings.

A site owner should be able to disable persistent visitor identification and use session/page-view analytics only.

38. Data Retention

Default:

90 days


Allow:

30 days
90 days
180 days
365 days


Do not offer unlimited retention by default.

When data expires:

delete old page views
delete expired sessions
delete orphaned visitor records where appropriate


Retention cleanup should run as a scheduled background job.

39. Rate Limiting

The ingestion endpoint is public.

Therefore it must be protected.

Initial limits can be based on:

project
network address
request rate
payload size


Example:

Maximum:
100 events/minute/project/IP


The exact limits can be tuned after testing.

The goal is to prevent:

spam
accidental loops
malicious event flooding
database exhaustion


Do not rely on the public tracking ID as authentication.

40. Domain Validation

Each project has:

allowed_domains


Example:

example.com
www.example.com


The ingestion API should validate the request's origin/referer where possible.

However:

Do not treat Origin/Referer as a strong authentication mechanism.

The tracking ID is public and can technically be copied.

Therefore abuse prevention must also include:

rate limits

payload limits

anomaly detection later

project-level event quotas later

41. Bot Filtering

The MVP should identify obvious bots using:

User-Agent

known crawler patterns

known monitoring agents

Examples:

Googlebot
Bingbot
Twitterbot
facebookexternalhit


Bot events should not count toward normal visitor metrics.

Store is_bot internally if useful for analysis.

Do not attempt perfect bot detection.

42. Dashboard

The main navigation:

DevPulse
│
├── Overview
├── Pages
├── Sources
├── Countries
├── Devices
├── Real-time
└── Settings


43. Overview Dashboard

The dashboard should answer:

"How is my website performing from a visitor perspective?"

Top cards:

Visitors
1,284

Sessions
1,643

Page Views
3,721

Bounce Rate
42.1%

Avg. Session Duration
3m 04s


Comparison:

+18.4% vs previous period


44. Visitor Chart

Primary chart:

Visitors over time


Allow:

24 hours
7 days
30 days
90 days
Custom


Depending on range:

hour
day
week


Example:

Visitors
  │
300│                 ╭─╮
250│       ╭──╮    ╭╯ ╰╮
200│   ╭──╯  ╰────╯    ╰─
150│───╯
   └────────────────────────
     Mon Tue Wed Thu Fri Sat


Also provide:

Page Views
Sessions


as selectable metrics.

45. Top Pages

Display:

Page                 Views      Visitors
------------------------------------------
/                    1,842       931
/projects              923       601
/about                 421       310
/contact               213       181


Sort by:

Page Views
Unique Visitors


46. Traffic Sources Dashboard

Display:

Traffic Sources

Google       42%
Direct       31%
GitHub        9%
LinkedIn      7%
Reddit        4%
Other         7%


Also provide:

Source
Medium
Campaign
Sessions
Visitors


47. Countries Dashboard

Display:

Countries

Ethiopia        4,821
United States   1,932
Germany           421
United Kingdom    318
Kenya             287


The MVP can use a table rather than building a world map.

A map can be added later.

48. Devices Dashboard

Sections:

Device Type
Desktop
Mobile
Tablet

Browser
Chrome
Safari
Firefox
Edge

Operating System
Windows
Android
macOS
iOS
Linux


Use percentages and visitor counts.

49. Real-Time Dashboard

Display:

17 people online


Example:

Current Visitors

Country       Page
---------------------------
Ethiopia      /projects
US            /
Germany       /about
Kenya         /projects


Show:

Last seen
Current page
Country
Device


Do not expose the visitor identifier.

50. Bounce Rate

For MVP:

A session is considered a bounce if:

page_views = 1


Therefore:

bounce_rate =
single_page_sessions / total_sessions * 100


This definition should be documented clearly.

51. Session Duration

For sessions with multiple page views:

duration =
last_seen_at - started_at


For a single-page session, duration is difficult to know reliably without another event.

Therefore the MVP should either:

report it as zero/unknown, or

add a lightweight session heartbeat later.

Recommended MVP behavior:

Single-page session duration = unknown


Do not pretend that the duration is accurate.

52. Unique Visitor Metric

For a selected time range:

COUNT(DISTINCT visitor_id)


Important:

A visitor returning multiple times is still:

1 unique visitor


while potentially producing:

multiple sessions
multiple page views


53. Analytics Query Rules

Dashboard APIs should never fetch every raw event and calculate everything in the browser.

Bad:

Browser → download 2 million events → calculate


Good:

Browser
   ↓
Analytics API
   ↓
PostgreSQL aggregation
   ↓
small response


For example:

SELECT
    COUNT(*) AS page_views,
    COUNT(DISTINCT visitor_id) AS visitors
FROM analytics_page_views
WHERE project_id = $1
AND occurred_at >= $2
AND occurred_at < $3;


54. Time Zones

Events should always be stored in:

UTC


The project can have a display timezone:

Africa/Addis_Ababa


The dashboard converts UTC timestamps to the project's selected timezone.

Never store analytics timestamps as local server time.

55. Duplicate Events

The browser may occasionally send the same event twice.

The MVP should include an event ID:

{
  "event_id": "01JXYZEVENT..."
}


The server can use it for deduplication.

Add:

event_id TEXT UNIQUE


or a unique index scoped appropriately to the project.

This prevents retries from inflating analytics.

56. Recommended Final Event Schema

The final MVP event should therefore look like:

{
  "event_id": "01JEVENT123",
  "type": "page_view",

  "project_id": "dp_01PROJECT",

  "visitor_id": "01JVISITOR",
  "session_id": "01JSESSION",

  "timestamp": "2026-09-14T14:32:21.000Z",

  "page": {
    "url": "https://example.com/projects",
    "path": "/projects",
    "title": "Projects",
    "referrer": "https://github.com/"
  },

  "screen": {
    "width": 1920,
    "height": 1080
  },

  "viewport": {
    "width": 1536,
    "height": 864
  },

  "language": "en-US",
  "timezone": "Africa/Addis_Ababa",

  "campaign": {
    "source": "github",
    "medium": "social",
    "campaign": "portfolio",
    "term": null,
    "content": null
  }
}


The server enriches it with:

country
region
device_type
browser
browser_version
os
os_version
is_bot


57. MVP Security Requirements

The analytics system must:

validate every input

limit request body size

rate-limit ingestion

authenticate dashboard APIs

authorize project ownership

prevent cross-project data access

sanitize displayed values

prevent SQL injection

use parameterized queries

enforce allowed domains

protect WebSocket project access

avoid logging raw analytics payloads unnecessarily

never expose database credentials

never expose dashboard authentication tokens to analytics scripts

58. MVP Performance Requirements

Initial targets:

Analytics ingestion:
<100ms server processing target

Dashboard queries:
<500ms for normal date ranges

Browser script:
asynchronous
non-blocking
small payload

Database:
PostgreSQL only


Do not optimize for millions of events before having real usage data.

When scale eventually requires it, consider:

aggregation tables
partitioning
Redis
ClickHouse
queue-based ingestion


But these are post-MVP decisions.

59. MVP Definition of Done

Web Analytics MVP is complete when a developer can:

Step 1

Create a DevPulse analytics project.

Step 2

Receive:

<script
  src="https://analytics.devpulse.example/analytics.js"
  data-project="PROJECT_ID">
</script>


Step 3

Put that script on a real website.

Step 4

Visit the website.

Step 5

DevPulse records:

visitor
session
page view
country
referrer
device
browser
OS


Step 6

Open the DevPulse dashboard.

Step 7

See:

Visitors
Sessions
Page Views
Bounce Rate
Session Duration


Step 8

See:

Visitor trends
Top pages
Traffic sources
Countries
Devices
Browsers
Operating systems


Step 9

Open Real-time and see the current visitor.

Step 10

Verify privacy behavior:

No raw IP stored
No fingerprinting
No PII collected
No analytics failure breaks the website


Step 11

Verify security:

Rate limiting
Payload limits
Project isolation
Domain validation
Authentication


Step 12

Verify retention:

Old analytics data is automatically removed


60. Suggested Implementation Order

Build it in this exact order:

1. Database migrations
        ↓
2. Analytics project CRUD
        ↓
3. POST /v1/analytics/events
        ↓
4. Event validation
        ↓
5. Visitor/session creation
        ↓
6. Page-view persistence
        ↓
7. Browser analytics.js
        ↓
8. SPA navigation tracking
        ↓
9. Referrer + UTM processing
        ↓
10. GeoIP enrichment
        ↓
11. Bot detection
        ↓
12. Overview API
        ↓
13. Pages API
        ↓
14. Sources API
        ↓
15. Countries API
        ↓
16. Devices API
        ↓
17. Next.js dashboard
        ↓
18. Real-time analytics
        ↓
19. Retention worker
        ↓
20. Security/performance testing


61. What NOT to Build Yet

Do not let the MVP turn into Google Analytics.

Avoid:

❌ Heatmaps
❌ Session replay
❌ Custom event builder
❌ Funnels
❌ User segmentation engine
❌ Marketing attribution engine
❌ E-commerce
❌ Machine-learning insights
❌ Complex maps
❌ Click tracking
❌ Scroll tracking
❌ Hundreds of dashboard filters
❌ ClickHouse
❌ Kafka
❌ Kubernetes


The first milestone should prove one thing:

A developer can put one script on their website and immediately understand their traffic.

That is the core DevPulse Web Analytics product.