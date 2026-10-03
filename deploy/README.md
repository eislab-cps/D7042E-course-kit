# deploy/ — the kit's compose stack

`deploy/docker-compose.yml` implements the design on this page (compose project name
`d7042e-kit`). The environment variable names were verified against the pinned `v0.1.1`
images.

One command starts a small Arrowhead 5.2 local cloud plus the messaging and storage the
assignment needs:

```bash
cp deploy/.env.example deploy/.env      # lab-only values; never real secrets
docker compose -f deploy/docker-compose.yml up -d --wait
docker compose -f deploy/docker-compose.yml down -v   # stop and clear all state
```

Rules for the file:

- Every Arrowhead image comes from `ghcr.io/ulfbod/<name>:<tag>` at one pinned
  Go-Evol tag. No `build:` anywhere.
- The foundation systems are published on their **TLS ports only**; a client
  certificate from profile-ca is required for every call.
- Secrets come from `deploy/.env`, created from `.env.example`. The values in
  `.env.example` are lab-only and documented as such.

## Services

| Service | Image | Published port(s) | Role |
|---|---|---|---|
| `profile-ca` | `ghcr.io/ulfbod/profile-ca` | 8787 (plain HTTP: CA info, onboarding, revoke, PIP), 8788 (mTLS: device and system certificates) | Local cloud certificate authority; its CA key persists in a volume |
| `cert-provisioner` | `ghcr.io/ulfbod/cert-provisioner` | none (init container, exits 0) | Writes the CA certificate and server certificates for the foundation systems into the shared `certs` volume |
| `serviceregistry` | `ghcr.io/ulfbod/serviceregistry` | 8490 (mTLS) | AH5 system and service discovery |
| `authentication` | `ghcr.io/ulfbod/authentication` | 8491 (mTLS) | Identities and tokens; Sysop bootstrap |
| `consumerauth` | `ghcr.io/ulfbod/consumerauth` | 8492 (mTLS) | Authorization policies; `verify` for the orchestrator |
| `dynamicorch-xacml` | `ghcr.io/ulfbod/dynamicorch-xacml` | 8083 (plain HTTP) | Pull and push orchestration, `consumerauth` backend |
| `mosquitto` | `eclipse-mosquitto:2` | 1883 | MQTT broker for alert services |
| `influxdb` | `influxdb:2.7` | 8086 | Time-series storage and web UI |

Not included from Go-Evol: the XACML backend (authz-pdp, AuthzForce, PAP), RabbitMQ and
Kafka enforcement points, the dashboard.

## Settings per service

Internal URLs use the compose service names; the plain ports 8080–8082 are reachable
only inside the compose network.

**profile-ca**: `PORT=8787`, `TLS_PORT=8788`, `CA_KEY_FILE=/data/ca.key`; volume
`profile-ca-data:/data`. Healthcheck on `/health`.

**cert-provisioner**: `CA_URL=http://profile-ca:8787`, `CERTS_DIR=/certs`; volume
`certs:/certs`; starts when profile-ca is healthy; `restart: "no"`.

**serviceregistry**, **authentication**, **consumerauth** (common):

| Variable | Value | Why |
|---|---|---|
| `PORT` | 8080 / 8081 / 8082 | internal plain listener, used by the orchestrator and for token checks |
| `TLS_PORT` | 8490 / 8491 / 8492 | the only published port |
| `TLS_CERT_FILE`, `TLS_KEY_FILE` | `/certs/<service>.crt`, `/certs/<service>.key` | server certificate from cert-provisioner; its only name is the service name |
| `TLS_CA_FILE` | `/certs/ca.crt` | makes the server require and verify a client certificate |
| `DB_PATH` | `/data/<service>.db` on a named volume | state survives a restart of the container; `docker compose down -v` clears it |
| `MGMT_AUTH_URL` | `http://authentication:8081` | `/mgmt/*` endpoints require a Sysop token |

Additionally:

| Service | Variable | Value | Why |
|---|---|---|---|
| serviceregistry | `REGISTER_AUTH_URL` | `http://authentication:8081` | registration requires the registering system's own token (401 without or invalid, 403 for another system's) |
| serviceregistry | `SR_AUTH_URL` | `http://authentication:8081` | token checks for system removal |
| authentication | `SYSOP_PASSWORD` | from `.env` | creates the Sysop identity when the identity store is empty; a later change needs `docker compose down -v` |

`HTTPS_ONLY` stays unset: the orchestrator reaches the registry and ConsumerAuthorization
on their internal plain ports.

**dynamicorch-xacml**:

| Variable | Value | Why |
|---|---|---|
| `AUTH_BACKEND` | `consumerauth` | authorization through ConsumerAuthorization `verify`; no XACML backend |
| `CA_URL` | `http://consumerauth:8082` | ConsumerAuthorization base URL (the name is historical, not the certificate authority) |
| `SR_URL` | `http://serviceregistry:8080` | registry lookups |
| `ENABLE_AUTH` | `true` | a consumer without a rule gets `200` with an empty `response` |
| `MGMT_AUTH_URL` | `http://authentication:8081` | `/mgmt/*` (push trigger and query, locks, history) require a Sysop token |
| `PORT` | 8083 | published as plain HTTP; the orchestrator has no TLS listener |
| `extra_hosts` | `host.docker.internal:host-gateway` | push notifications reach a consumer's notify endpoint on the student's host (needed on Linux) |

Healthcheck on `/status`; starts when the registry and ConsumerAuthorization are healthy.

**mosquitto**: a mounted `mosquitto/mosquitto.conf` with `listener 1883` and
`allow_anonymous true` (Mosquitto 2 otherwise accepts local connections only). Students
who choose broker authentication replace it with a password file and an ACL.

**influxdb**: `DOCKER_INFLUXDB_INIT_MODE=setup` with user, password, organisation,
bucket and admin token from `.env`; volume `influxdb-data` so data survives a restart of
the Arrowhead services.

## Deliberately not set

- `MQTT_BROKER_URL` on any Arrowhead service. The Arrowhead systems do not listen on
  MQTT in the pinned stack; the MQTT profile is used by student services through its
  interface names and topic convention.
- `LOOKUP_AUTH_URL` and `SERVICE_DISCOVERY_POLICY`: lookups stay open.

## Known properties students analyse

These are intended, documented, and part of the security analysis (R9), not defects of
the kit:

- The orchestrator and profile-ca's bootstrap port are plain HTTP.
- The orchestrator takes `requesterSystem` from the request body.
- ConsumerAuthorization `grant`/`revoke`/`lookup` and service revoke check no token.
- The orchestrator's `general/mgmt` log buffer and configuration are readable without
  a token.
- The Sysop password in `.env` is a shared lab secret.

## `.env.example`

```bash
# Lab-only values. Do not reuse anywhere real.
SYSOP_PASSWORD=change-me-lab-only
INFLUXDB_USERNAME=admin
INFLUXDB_PASSWORD=change-me-lab-only
INFLUXDB_ORG=d7042e
INFLUXDB_BUCKET=sensors
INFLUXDB_ADMIN_TOKEN=change-me-lab-only-token
```
