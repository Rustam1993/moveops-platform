# New Estimate Analytics Events

This document defines tenant-safe analytics events for the New Estimate workflow.

## PII policy

`analytics_events.properties` must not include direct PII.

Disallowed keys/patterns (case-insensitive):
- `email`
- `phone`
- `address`
- `notes`
- `customer`

Do not send free-form customer notes, addresses, names, phone numbers, or email addresses in properties.

## Event schema

Stored fields:
- `tenant_id` (required)
- `event_name` (required)
- `estimate_id` (optional)
- `user_id` (optional)
- `properties` (JSON object; PII-sanitized)
- `created_at`

## Allowed event names

- `estimate.entry_started`
- `estimate.inventory_updated`
- `estimate.inventory_link_sent`
- `estimate.charges_updated`
- `estimate.quote_sent`
- `estimate.sign_requested`
- `estimate.sign_completed`
- `estimate.booked`
- `estimate.follow_up_set`

## Recommended properties by event

### `estimate.entry_started`
- `source`: `new_estimate_page` | `duplicate` (optional)

### `estimate.inventory_updated`
- `item_count` (integer)
- `total_cf` (number)
- `via`: `internal` | `public_link`

### `estimate.inventory_link_sent`
- `via`: `email_center` | `manual`

### `estimate.charges_updated`
- `mode`: `local` | `long_distance`
- `total_cents` (integer)

### `estimate.quote_sent`
- `via`: `email_center`
- `delivery_mode`: `smtp` | `log`

### `estimate.sign_requested`
- `via`: `email_center` | `signature_request_endpoint`

### `estimate.sign_completed`
- `signature_type`: `typed`

### `estimate.booked`
- `from_status` (optional)

### `estimate.follow_up_set`
- `has_datetime`: boolean

## Emission points

Server-side emission is preferred and implemented in handlers for:
- estimate create
- inventory updates (internal/public)
- charges updates
- email send actions (quote/inventory/sign)
- signature completion
- booking/follow-up workflow updates

Optional client emission can use:
- `POST /analytics/events` (authenticated only)

## Aggregations used by dashboard

`GET /admin/new-estimate/metrics` computes:
- median time-to-quote using `entry_started` and `quote_sent`
- quote->sign conversion
- sign->book conversion
- inventory completion rate from `inventory_link_sent` -> `inventory_updated`
- stuck estimate count from estimate status/age
