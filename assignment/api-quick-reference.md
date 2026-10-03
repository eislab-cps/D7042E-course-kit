# API quick reference

Condensed from the Arrowhead-520-Go-Evol SPECs at the tag the kit pins
(`foundation/SPEC.md`, `core/SPEC.md`, `services/profile-ca/SPEC.md`). The SDK wraps
every call below; this page is for `curl` debugging and for understanding what the SDK
sends. When this page and the SPEC at the pinned tag disagree, the SPEC wins — tell the
course staff.

All examples assume you are in the directory holding your certificate files:

```bash
CA=ca.pem; CERT=gateway.pem; KEY=gateway.key       # from the profile-ca steps below
MTLS="--cacert $CA --cert $CERT --key $KEY"
# The foundation systems' server certificates name the container (serviceregistry,
# authentication, consumerauth), not localhost, so curl must use those names:
RESOLVE="--resolve serviceregistry:8490:127.0.0.1 --resolve authentication:8491:127.0.0.1 --resolve consumerauth:8492:127.0.0.1"
```

profile-ca's TLS port presents a certificate valid for `localhost`, so `https://localhost:8788`
works as is.

## Token status codes

Everywhere a token is checked (registration, `/mgmt/*`):

| Status | Meaning |
|---|---|
| `401` | no token, an invalid or expired token, or Authentication unreachable — log in again |
| `403` | a valid token that is not allowed: another system's token (registration) or not Sysop (`/mgmt/*`) |

## Names

| Thing | Rule | Example | Wrong → |
|---|---|---|---|
| System name | PascalCase `^[A-Z][A-Za-z0-9]{0,62}$` | `ColdChainGateway` | 400 (the registry checks the token first) |
| Service definition | camelCase `^[a-z][A-Za-z0-9]{0,62}$` | `temperatureReading` | 400 (after the token check) |
| Device name | UPPER_SNAKE_CASE | `PICO_SENSOR_1` | 400 |
| Certificate name | use the system name | `ColdChainGateway` | — |

## profile-ca — certificates (8787 plain, 8788 mTLS)

Certificates are requested **by name and profile**. profile-ca generates the key pair
and returns the private key in the response.

| Step | Call | Needs | Gives |
|---|---|---|---|
| CA certificate | `GET http://localhost:8787/ca/info` | nothing | `{"commonName","certificate"}` |
| 1 onboarding | `POST http://localhost:8787/bootstrap/onboarding-cert` | nothing | OU=`on` certificate + key |
| 2 device | `POST https://localhost:8788/ca/device-cert` | client cert with OU=`on` | OU=`de` certificate + key |
| 3 system | `POST https://localhost:8788/ca/system-cert` | client cert with OU=`de` | OU=`sy` certificate + key |

Request body for all three: `{"systemName":"ColdChainGateway"}`. Response `201`:

```json
{"systemName":"ColdChainGateway","certificate":"-----BEGIN CERTIFICATE-----\n...",
 "privateKey":"-----BEGIN EC PRIVATE KEY-----\n...","profile":"sy","issuedAt":"..."}
```

Errors: `400` empty name; `403` client certificate has the wrong profile. Without a
client certificate the TLS handshake fails on steps 2 and 3; there is no HTTP status.

```bash
curl -s http://localhost:8787/ca/info | jq -r .certificate > ca.pem
curl -s -X POST http://localhost:8787/bootstrap/onboarding-cert \
  -d '{"systemName":"ColdChainGateway"}' > on.json
jq -r .certificate on.json > on.pem; jq -r .privateKey on.json > on.key
curl -s --cacert ca.pem --cert on.pem --key on.key -X POST \
  https://localhost:8788/ca/device-cert -d '{"systemName":"ColdChainGateway"}' > de.json
# ... same for de.pem/de.key → /ca/system-cert → gateway.pem/gateway.key
```

Revocation and its state (plain port):

| Call | Result |
|---|---|
| `DELETE /ca/certificates/{cn}` | `204` revoked; `404` unknown or already revoked |
| `POST /ca/certificates/{cn}/reissue` | `204` un-revoked; `404` not revoked |
| `GET /pip/attributes/{cn}` | `{"systemName","certLevel","valid"}`; `valid` is false once revoked or expired |

The foundation systems do not consult revocation: a revoked certificate still gets 200
from the ServiceRegistry over mTLS (a lookup succeeds). Enforcement exists only where a component asks the PIP.

## Authentication — `https://authentication:8491` (mTLS)

The ServiceRegistry accepts a registration only with a token from the system being
registered; `/mgmt/*` endpoints on the foundation systems need a Sysop token. The Sysop
password is the lab-only value in the kit's `.env` (copied from `.env.example`). It is
read only when the identity store is empty, so after changing it run
`docker compose down -v` before the next start.

| Call | Body (essentials) | Result |
|---|---|---|
| `POST /authentication/identity/login` | `{"systemName":"Sysop","credentials":{"password":"<SYSOP_PASSWORD>"}}` | `201 {"token","systemName","expirationTime","sysop"}`; `401` wrong name or password; `400` `credentials` not `{"password":...}` |
| `POST /authentication/mgmt/identities` (header `Authorization: Bearer <sysop token>`) | `{"authenticationMethod":"PASSWORD","identities":[{"systemName":"ColdChainGateway","credentials":{"password":"<pick one>"},"sysop":false}]}` | `201` created identities |
| `POST /authentication/identity/login` | `{"systemName":"ColdChainGateway","credentials":{"password":"..."}}` | `201` the system's token |
| `GET /authentication/identity/verify/{token}` | — | `200 {"verified","systemName","expirationTime","sysop"}` |
| `POST /authentication/identity/logout` (Bearer token) | — | `200` |

```bash
SYSOP=$(curl -s $MTLS $RESOLVE -X POST https://authentication:8491/authentication/identity/login \
  -d '{"systemName":"Sysop","credentials":{"password":"'"$SYSOP_PASSWORD"'"}}' | jq -r .token)
curl -s $MTLS $RESOLVE -H "Authorization: Bearer $SYSOP" -X POST \
  https://authentication:8491/authentication/mgmt/identities -d '{"authenticationMethod":"PASSWORD",
  "identities":[{"systemName":"ColdChainGateway","credentials":{"password":"gw-lab-pw"},"sysop":false}]}'
TOKEN=$(curl -s $MTLS $RESOLVE -X POST https://authentication:8491/authentication/identity/login \
  -d '{"systemName":"ColdChainGateway","credentials":{"password":"gw-lab-pw"}}' | jq -r .token)
```

Tokens expire (`expirationTime`); log in again when the registry answers 401.

## ServiceRegistry — `https://serviceregistry:8490` (mTLS)

Both register calls need `Authorization: Bearer $TOKEN` from the system named in the body:
no, invalid or expired token → `401`; another system's token → `403`. The token is
checked before the body, so a bad name only shows as `400` once the token is right.
Lookup needs no token.

| Call | Body (essentials) | Result |
|---|---|---|
| `POST /serviceregistry/system-discovery/register` | `{"name":"ColdChainGateway","addresses":[{"type":"HOSTNAME","address":"localhost"}],"metadata":{}}` | `201` new, `200` updated |
| `POST /serviceregistry/service-discovery/register` | see below | `201` new, `200` updated, `400` bad name or policy, `401` no or invalid token, `403` token of another system |
| `POST /serviceregistry/service-discovery/lookup` | `{"serviceDefinitionNames":["temperatureReading"]}` | `200 {"entries":[...],"count":n}` |
| `DELETE /serviceregistry/service-discovery/revoke/{instanceId}` | — (no token checked) | `200` removed, `204` not found |

Service registration:

```json
{
  "systemName": "ColdChainGateway",
  "serviceDefinitionName": "temperatureReading",
  "version": "1",
  "metadata": {"unit": "degC"},
  "interfaces": [
    {"templateName": "HTTP-SECURE-JSON", "protocol": "http", "policy": "CERT_AUTH",
     "properties": {"accessAddresses": "localhost", "accessPort": "9443",
                    "basePath": "/temperature"}}
  ]
}
```

Interface property values are always JSON strings; an array or a number is rejected.
The orchestrator builds its pull result from three properties of the first interface
that has them. **All three are required for orchestration to work (R3):**

| Property | Example | Becomes in the pull result | If missing |
|---|---|---|---|
| `accessAddresses` | `"localhost"` (comma-separated list allowed; the first is used) | `provider.address` | first system address, else `""` |
| `accessPort` | `"9443"` (numeric string) | `provider.port` | `0` (also for a non-numeric value) |
| `basePath` | `"/temperature"` (leading slash) | `service.serviceUri` | `""` |

Register the address your **consumers** use to reach the provider; the orchestrator
never contacts providers, it only passes the address on. If your consumers run on your
laptop (the usual case), use `localhost`. `host.docker.internal` and compose service names
such as `mosquitto` resolve only inside containers: use them only for consumers that run
in a container (with `extra_hosts: host.docker.internal:host-gateway` on Linux). The flat form
`"interfaces": ["HTTP-INSECURE-JSON"]` is accepted by the registry but carries no
properties, so orchestration returns port 0 and an empty `serviceUri`: do not use it.
Policies: `NONE`, `CERT_AUTH`, and token policies you will not need.

An MQTT alert service (R5) is registered the same way with an MQTT interface from the
foundation MQTT profile. The same three properties apply; `basePath` carries the topic, so
the pull result's `serviceUri` is the topic to subscribe to:

```json
{"templateName": "MQTT-INSECURE-JSON", "protocol": "mqtt", "policy": "NONE",
 "properties": {"accessAddresses": "localhost", "accessPort": "1883",
                "basePath": "ah5/ColdChainGateway/temperatureAlert"}}
```

## ConsumerAuthorization — `https://consumerauth:8492` (mTLS)

| Call | Body (essentials) | Result |
|---|---|---|
| `POST /consumerauthorization/authorization/grant` | `{"provider":"ColdChainGateway","targetType":"SERVICE_DEF","target":"temperatureReading","defaultPolicy":{"policyType":"WHITELIST","policyList":["MonitorApp"]}}` | `201` the stored policy with `instanceId`; `409` exists |
| `DELETE /consumerauthorization/authorization/revoke/{instanceId}` | — (encode `|` as `%7C`) | `200` or `404` |
| `POST /consumerauthorization/authorization/lookup` | `{"targetNames":["temperatureReading"]}` | `200 {"policies":[...],"count","totalCount"}` |
| `POST /consumerauthorization/authorization/verify` | `{"consumer","provider","target","targetType"}` | `200` bare `true` or `false` |

`grant`, `revoke`, `lookup` and `verify` check no token: any client with a profile-ca
certificate can call them. `/mgmt/*` needs a Sysop token.

`instanceId` has the form `PR|LOCAL|<provider>|SERVICE_DEF|<target>`. Policy types you
use: `WHITELIST` (only listed consumers) and `ALL`.

## DynamicOrchestration — `http://localhost:8083` (plain HTTP, `consumerauth` mode)

Pull:

```bash
curl -s -X POST http://localhost:8083/serviceorchestration/orchestration/pull -d '{
  "requesterSystem": {"systemName": "MonitorApp"},
  "requestedService": {"serviceDefinition": "temperatureReading"}}'
```

```json
{"response": [
  {"provider": {"systemName": "ColdChainGateway", "address": "...", "port": 0},
   "service": {"serviceDefinition": "temperatureReading", "serviceUri": "...",
               "interfaces": ["..."], "version": 1}}
]}
```

No rule for the requester → `200 {"response": []}`. The orchestrator takes
`requesterSystem` from the body without checking who is calling (no token, no
certificate), unlike the registry.

Push. `/mgmt/*` endpoints need `Authorization: Bearer $SYSOP` (`401` no or invalid token,
`403` valid but not Sysop); subscribe, unsubscribe and pull need none:

| Call | Body | Result |
|---|---|---|
| `POST /serviceorchestration/orchestration/subscribe` | `{"ownerSystemName":"MonitorApp","targetSystemName":"MonitorApp","orchestrationRequest":{"requesterSystem":{"systemName":"MonitorApp"},"requestedService":{"serviceDefinition":"temperatureReading"}},"notifyInterface":{"address":"host.docker.internal","port":9100,"path":"/notify"}}` | `201` new / `200` replaced; the subscription with its `id` |
| `POST /serviceorchestration/orchestration/mgmt/push/trigger` (Sysop Bearer) | `{"subscriptionId":"<id>"}` | `200 {"status":"triggered"}`; `404` unknown id |
| `POST /serviceorchestration/orchestration/mgmt/push/query` (Sysop Bearer) | `{}` | all subscriptions |
| `DELETE /serviceorchestration/orchestration/unsubscribe/{id}` | — | `200` removed, `204` not found |
| `POST /serviceorchestration/orchestration/mgmt/history/query` (Sysop Bearer) | `{}` | history; push entries go `PENDING` → `DELIVERED` or `FAILED` |

```bash
curl -s -H "Authorization: Bearer $SYSOP" -X POST \
  http://localhost:8083/serviceorchestration/orchestration/mgmt/push/trigger \
  -d '{"subscriptionId":"<id from subscribe>"}'
```

The notify address is the opposite case: the orchestrator's container calls it, so a
notify endpoint on your laptop is `host.docker.internal` (the kit maps it for the
orchestrator). `notifyInterface` also accepts `{"notifyUri":"http://host.docker.internal:9100/notify"}`.
The orchestrator POSTs `{"subscriptionId","ownerSystemName","targetSystemName"}` to it
over plain HTTP; any 2xx counts as delivered. The notification contains no provider
list — pull after you receive it.

## MQTT profile (foundation `SPEC.md` section 11)

| Item | Value |
|---|---|
| Interface names | `MQTT-INSECURE-JSON`, `MQTT-SECURE-JSON` (TLS to the broker) |
| Topic prefix | `ah5/<SystemName>/` |
| Broker in the kit | `tcp://localhost:1883` from the host, `tcp://mosquitto:1883` inside compose |

In the pinned stack the profile is a naming convention and a library; the Arrowhead
systems themselves do not listen on MQTT. Your services use the convention.
