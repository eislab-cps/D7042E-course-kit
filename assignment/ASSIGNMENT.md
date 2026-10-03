# D7042E — Individual assignment

Industrial IoT on Arrowhead Framework 5: design, build, secure and evaluate a small but
complete system on the course kit. You work on it continuously from Week 1 to Week 5.
Only the final submission (code and documentation) is formally submitted; the oral
examination assesses the completed system.

Companion documents in this repository: `api-quick-reference.md` (endpoints and request
shapes), `../FRAME_FORMAT.md` (device frame contract), `../README.md` (starting the kit).

---

## 1. The brief

> Design and implement an industrial IoT system that collects data from at least two
> simulated sensors, stores it persistently, derives at least one processed result, and
> provides authorized access to both raw and derived data through the AH5 service mesh.
> Document the architecture, the security model, and evaluate the system honestly against
> production requirements.

Choose a scenario from the list or propose your own. The scenario decides what the
sensors represent and what processing is meaningful; the architecture follows the same
pattern in every scenario.

| Scenario | Sensors (frame keys) | Processing | Actuation (optional) |
|---|---|---|---|
| Cold chain monitoring | Temperature `TEMP` + humidity `HUM` | Threshold alert, drift trend | Alert relay output |
| Building energy management | Power `PWR` + occupancy `OCC` | Moving average, anomaly flag | HVAC command |
| Air quality station | CO₂ `CO2` + temperature `TEMP` | Combined air quality index | Ventilation trigger |
| Industrial press monitoring | Pressure `PRES` + cycle count `CYC` | Statistical process control | Safety interlock |
| Custom (with approval) | Your keys, registered in the proposal | Your choice | Optional |

### Project topic proposal

Due at the end of Week 1; at most one page (about 400–600 words). Feedback by the end of
Week 2, Day 1. It is a commitment to a scenario and a check that every mandatory
requirement fits it. Cover:

1. **Scenario and industrial context** — one sentence on the setting and the purpose.
2. **Data sources** — the quantities, units, realistic ranges, and how they relate to
   the context.
3. **Derived result** — what you compute, why it matters, how it differs from a raw
   reading.
4. **Access control model** — which consumers (PascalCase system names) may use which
   services (camelCase service definitions); at least one consumer denied at least one
   service; one sentence of justification per rule.
5. **Event condition** — what triggers an event, how it flows (MQTT alert service or AH5
   push subscription), and what a consumer does with it.
6. **Security focus** — the two components for your STRIDE analysis (one is the
   gateway) and the two R10 mitigations you intend to build.
7. **Actuation** *(optional)* — command path, what it represents physically, what
   authorization governs it.

---

## 2. The kit

Everything runs on your laptop. Create your own **private** repository from the kit with
GitHub's **Use this template** button (not a fork) and share it with the examiner.

| Kit part | What it gives you |
|---|---|
| `deploy/docker-compose.yml` | The Arrowhead local cloud from prebuilt, pinned images (below), plus Mosquitto and InfluxDB 2 |
| `sim/sensor_sim/` | Simulator emitting `FRAME_FORMAT.md` frames for your scenario to a named pipe or TCP socket |
| `gateway/` | Gateway stub: reads frames (`SERIAL_SOURCE` selects pipe, socket or serial device) and has marked TODOs for certificates, registration and service endpoints |
| `wokwi/pico_sensor/` | Wokwi project: Raspberry Pi Pico, BMP180 (I2C: temperature, pressure), DHT22 (single-wire: humidity), LED; C skeleton with I2C and UART initialisation |
| `FRAME_FORMAT.md` | The frame contract between device and gateway |
| Go SDK | `github.com/eislab-cps/Arrowhead-520-Evol-Go-SDK-Edu`, pinned in `go.mod`: certificates, registration, orchestration, authorization — every step an explicit call |

Services in the compose stack (from `github.com/ulfbod/Arrowhead-520-Go-Evol`):

| System | Port on your host | Transport |
|---|---|---|
| ServiceRegistry | 8490 | HTTPS, client certificate required; registration needs the registering system's token |
| Authentication | 8491 | HTTPS, client certificate required; `/mgmt/*` needs a Sysop token |
| ConsumerAuthorization | 8492 | HTTPS, client certificate required |
| DynamicOrchestration (`consumerauth` mode) | 8083 | plain HTTP; `/mgmt/*` (push trigger, history) needs a Sysop token |
| profile-ca | 8787 (bootstrap, info, PIP, revoke) / 8788 (device and system certificates) | plain HTTP / HTTPS with client certificate |
| Mosquitto | 1883 | MQTT |
| InfluxDB 2 | 8086 | HTTP, web UI |

Facts you will need from day one:

- **You need a client certificate before you can talk to the registry.** profile-ca
  issues it in three steps, by name and profile: onboarding certificate (no
  authentication) → device certificate → system certificate. profile-ca generates the
  key pair and sends you the private key.
- **You need an identity token before you can register.** As Sysop (lab password in
  the kit's `.env.example`, never a real secret) create an identity for each of your
  systems, then log in as that system. The registry accepts a registration only with
  the registering system's own token: no, invalid or expired token → 401 (log in
  again); a valid token of another system → 403.
- **Names are checked.** System names are PascalCase (`ColdChainGateway`), service
  definitions camelCase (`temperatureReading`). A wrong name gives HTTP 400 — but the
  registry checks your token first, so you see the 400 only once the token is right.
  Use the system name as the certificate name.
- **Register how to reach you.** Each service interface needs the properties
  `accessAddresses`, `accessPort` and `basePath`; without them orchestration returns
  port 0 and an empty URI (see `api-quick-reference.md`).
- **Denied is not an error.** A consumer without a ConsumerAuthorization rule gets HTTP
  200 with an empty `response` from the orchestrator.
- **Two links are plain text on purpose:** the profile-ca bootstrap port and the
  orchestrator. The orchestrator believes the `requesterSystem` you send it, while the
  registry checks a token for the same kind of claim. And no token is checked when
  anyone with a certificate grants or revokes a ConsumerAuthorization rule or revokes a
  service in the registry. Your STRIDE analysis must deal with both.

---

## 3. Requirements

Every project addresses all mandatory requirements, in the implementation and in the oral
examination. R8 is optional.

| ID | Requirement |
|----|-------------|
| R1 | At least two distinct simulated physical quantities, each exposed as a separate AH5 service with a defined interface contract (camelCase serviceDefinition, interface name, JSON field names and units) |
| R2 | A gateway system that holds a profile-ca system certificate and its own Authentication identity, reads from the device simulator, and on startup logs in and registers itself and all sensor services in the ServiceRegistry over mTLS with its own token |
| R3 | At least one consumer that discovers services via DynamicOrchestration — no hardcoded provider endpoints |
| R4 | ConsumerAuthorization rules for all services; at least one access denial demonstrable live (a consumer that lacks a rule receives HTTP 200 with an empty orchestration `response`) |
| R5 | At least one event-driven path triggered by a configurable threshold or condition, consumable as a registered AH5 service: either MQTT alerts from a service registered with an MQTT interface (`MQTT-INSECURE-JSON` or `MQTT-SECURE-JSON`, topic under `ah5/<ProviderSystem>/`) and discovered via orchestration, or an AH5 push subscription whose notification makes the consumer pull |
| R6 | All sensor readings stored persistently in InfluxDB; data survives an AH5 stack restart |
| R7 | At least one derived result computed from stored data (aggregation, detection, or classification), exposed as a new AH5 service with its own ConsumerAuthorization rule |
| R8 | *(optional)* At least one actuation path: a consumer sends a command that results in a simulated physical output, authorized through ConsumerAuthorization |
| R9 | STRIDE analysis for at least two components (one is the gateway), each of the six categories assessed as mitigated, partially mitigated, or accepted; it addresses the server-generated private key, the orchestrator's unauthenticated requester, and the unauthenticated ConsumerAuthorization grant/revoke and service revoke |
| R10 | At least two of the mitigations below implemented, each demonstrated **refusing** something. mTLS to the foundation systems is the baseline and does not count. |
| R11 | Written evaluation: production readiness gaps; the highest-priority unmitigated security risk and why it was accepted; the component that fails first under 10× sensor load |
| R12 | The complete system starts from `docker compose up` plus documented commands; all requirements above are demonstrable in the oral examination |

R10 menu:

| Option | What you build | What you must show refused |
|---|---|---|
| A. mTLS on your own providers | Sensor and analysis services serve HTTPS, require a client certificate from profile-ca, register with `HTTP-SECURE-JSON` and policy `CERT_AUTH`; consumers verify the provider by its system name (`api-quick-reference.md`, "Calling a provider over mTLS") | A consumer without a certificate fails the handshake |
| B. HMAC frame integrity | `FRAME_FORMAT.md` section 5 in the producer (Wokwi C or simulator) and the gateway, `FRAME_HMAC=required`; `sim/sensor_sim -hmac -tamper-every N` produces tampered frames to test with | A tampered frame is dropped and logged |
| C. MQTT broker authentication | Mosquitto with passwords or profile-ca client certificates, plus a topic ACL | An unauthorized publish on your alert topic is rejected |
| D. Revocation with enforcement | Your provider checks `GET /pip/attributes/{cn}` on profile-ca per request or per connection and treats `404` as not valid (fail closed); revoke with `DELETE /ca/certificates/{cn}`. Revocations survive a profile-ca restart. Revocation is per name, not per certificate: anyone who can reach profile-ca's plain port can restore a revoked name (onboard it again, or `POST …/reissue`), and the original revoked certificate is then accepted again | The same consumer is served before revocation and refused after. Also show that the revoked certificate still gets 200 from the ServiceRegistry over mTLS (for example a lookup), and explain why. Also show or explain how the revoked name can be restored |

### Where each requirement is built

| Req | Phase | Kit components involved |
|---|---|---|
| R1 | 1 | `sim/sensor_sim`, `FRAME_FORMAT.md`, `gateway/`, ServiceRegistry |
| R2 | 1 | `gateway/`, profile-ca, Authentication (8491), ServiceRegistry (8490), SDK |
| R3 | 1 | DynamicOrchestration (8083), SDK |
| R4 | 1 | ConsumerAuthorization (8492), DynamicOrchestration |
| R5 | 2 | Mosquitto, `gateway/`, ServiceRegistry; or DynamicOrchestration push |
| R6 | 3 | InfluxDB, your collector |
| R7 | 3 | InfluxDB, ServiceRegistry, ConsumerAuthorization, your analysis service |
| R8 | 3 (optional) | `wokwi/pico_sensor` LED, `gateway/` |
| R9 | 2, refined in 4 | your architecture; profile-ca, DynamicOrchestration, `gateway/` |
| R10 | 2 | A: profile-ca + your services; B: `FRAME_FORMAT.md`, `wokwi/`, `sim/`, `gateway/`; C: Mosquitto config in `deploy/`; D: profile-ca PIP + your services |
| R11 | 4 | documentation |
| R12 | 4 | `deploy/docker-compose.yml`, your `README.md` |

---

## 4. Phases

Four internal phases pace the work. Weeks and days count from the first lecture.

### Phase 1 — Identity and core integration (Weeks 1–2)

1. **Certificates.** Start the stack. Get a system certificate for your gateway through
   profile-ca's three steps, using its PascalCase system name; fetch the CA certificate
   from `GET /ca/info`.
   **Identity.** Log in as Sysop, create an identity for the gateway, log in as the
   gateway. Register a test system and service on the ServiceRegistry with the gateway's
   token and look it up. Record what happens without a client certificate, without a
   token, and with another system's token.
2. Run `sim/sensor_sim` for your scenario; check the frames on the pipe or socket against
   `FRAME_FORMAT.md`.
3. Extend the gateway stub: parse both keys, expose two HTTP endpoints, register the
   gateway system (the SDK has no call for this yet: send the raw
   `system-discovery/register` request from `api-quick-reference.md`) and both services. Write down the interface contract (fields, units,
   format).
4. Get a certificate and an identity for a consumer system. Write a consumer that discovers each service
   through DynamicOrchestration and reads a live value. Grant a ConsumerAuthorization rule;
   revoke it and show the empty orchestration result.
5. Wokwi: implement the BMP180 I2C read (with its datasheet compensation), the DHT22
   single-wire read and the UART frame output in C. Check the frames in
   the Wokwi console. Write down what would change on real hardware and which security
   properties cannot be added at this layer, and why.

**Done when:** gateway and consumer use profile-ca certificates and their own identities;
both sensor services are registered over mTLS with the gateway's token; the consumer gets authorized data and is denied without a rule;
Wokwi produces correct frames.

### Phase 2 — Security and messaging (Week 3)

1. STRIDE for the gateway and one more component (the orchestrator is a good choice).
   Cover the server-generated private key (what would a CSR flow change, what would it
   not?) and try a pull with another consumer's `requesterSystem`; compare with what the
   registry does when you register with another system's token, and grant your consumer
   access to a service it should not see — who stopped you? Name your two
   highest-priority unmitigated threats.
2. MQTT alerts: when a value crosses a configurable threshold, the gateway publishes to a
   topic under `ah5/<GatewaySystem>/`. Register the alert stream as its own service with
   an MQTT interface whose `accessAddresses`/`accessPort` name the broker and whose
   `basePath` is the topic. Write a subscriber that finds
   it through orchestration, logs alerts and serves the last 10 as an HTTP service
   (`alertFeed`) with its own ConsumerAuthorization rule.
3. Push orchestration: run an HTTP notify endpoint the orchestrator container can reach;
   subscribe; trigger with the push-trigger endpoint (Sysop token); see the notification arrive and the
   history entry go `PENDING` → `DELIVERED`. Stop the consumer, trigger again, see
   `FAILED`. The notification carries only the subscription ID and system names, not a
   provider list: your consumer pulls when notified. Write down what should trigger a
   push in production and what happens to a notification sent while the consumer is down.
4. Build two R10 mitigations and demonstrate each one refusing something.

**Done when:** STRIDE table complete for two components; alerts flow over MQTT and are
discoverable as an AH5 service; a push notification delivered and a failed delivery
observed; two mitigations demonstrated.

### Phase 3 — Storage and processing (Week 4)

1. Check InfluxDB (web UI on port 8086); create a bucket and a token, or use the bucket and
   admin token from `deploy/.env` (see `deploy/README.md`).
2. Collector: discovers both sensor services through orchestration, polls every 10 s,
   writes readings to InfluxDB; MQTT alerts go to a separate measurement. Watch data
   accumulate for 10 minutes.
3. Analysis service: queries the last N readings, computes a derived value that fits your
   scenario, returns JSON, registered as its own service (e.g. `temperatureAnalysis`).
4. Grant the monitoring consumer access to the analysis service; the final consumer
   combines the raw value and the derived result into one status summary.
5. Restart the AH5 systems. Record what survived and what did not: InfluxDB data,
   registrations, authorization rules, certificates, revocations and the CA key, subscriptions.
   Re-register what was lost; write down what this means for production.

**Done when:** data accumulates in InfluxDB across restarts; the analysis service returns
a derived result; the pipeline sensor → gateway → AH5 → collector → InfluxDB → analysis →
consumer runs end to end.

### Phase 4 — Documentation and evaluation (Week 5, Days 1–3)

Produce the documentation in section 5 and submit (section 6).

---

## 5. Documentation

Keep it short: 4–8 pages including diagrams. Tables and diagrams beat prose.

1. **Architecture diagram** — every component and link; each link labelled with its
   protocol and whether it is mTLS, TLS or plain; each component labelled with its role
   (sensor producer, gateway, registry, orchestrator, policy decision point, certificate
   authority, broker, storage, analysis, consumer). Readable without the text.
2. **Service catalogue**

   | serviceDefinition | Provider system | Endpoint URI or topic | Interface | Who has access | Why |
   |---|---|---|---|---|---|

3. **Security analysis** — STRIDE tables for two components; a trust boundary diagram
   (what is inside and outside, which links are plain text); the gateway identity problem
   (why the Pico cannot hold its own AH5 identity, what the gateway holds for it, what a
   compromised gateway means); the certificate flow (who generated each private key, how
   it travelled, what a CSR flow would change); your two R10 mitigations — what each
   stops, what it does not, and the refusal you demonstrated.
4. **Evaluation** — one paragraph each:
   - *Production readiness:* what must change for a real plant? Consider persistence on
     restart, what triggers push notifications, revocation enforcement, hardware identity
     for the sensor node, operational monitoring.
   - *Security risk:* your biggest remaining risk, and why you accepted it.
   - *Scalability:* what breaks first at 10× sensor nodes, and what change fixes it.

---

## 6. Submission and oral examination

**Submission:** end of Week 5, Day 3. Your private repository created from the kit,
shared with the examiner, containing:

| Artifact | Requirement |
|---|---|
| Source code | Starts with `docker compose up` plus documented commands |
| Architecture diagram (PDF or PNG) | All components and links labelled; readable standalone |
| Security analysis | STRIDE for two components, trust boundary diagram, certificate flow, two mitigations |
| Evaluation | Production readiness, highest-priority risk, scalability limit |
| `README.md` | Exact steps to start the system and trigger each of R1–R12 |

**Oral examination:** Week 5, Days 4–5; individual, 30 minutes.

| Part | Time | Content |
|---|---|---|
| Live demo | 12 min | Data flowing through the gateway; an orchestrated request; a consumer blocked, a rule granted, the same consumer succeeding; data in InfluxDB; the derived result; an alert event; both R10 mitigations refusing something |
| Architecture walkthrough | 8 min | Your diagram, each component's role, and why you chose this protocol, this service boundary, this access rule |
| Examiner questions | 10 min | Three to four questions probing what you built and what lies next to it |

Simulation and real hardware are assessed identically. What earns credit: correct and
deep explanations, the ability to trace a reading or command through every component
including where authorization is checked, and an honest account of the remaining
weaknesses. Claiming the system is production-ready does not.
