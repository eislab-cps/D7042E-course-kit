# ARCHITECTURE.md — D7042E-course-kit

## System overview

The kit gives students a running Arrowhead 5.2 local cloud plus simulated field
devices. Students build the rest: services, consumers, authorization rules, an event
path, storage and a derived result.

```
Wokwi Pico (C) ──UART──┐
                       ├──▶ gateway (Go, SDK) ──▶ AH5 services ──▶ consumers
sim/sensor_sim (Go) ───┘          │
                                  ├──▶ MQTT (Mosquitto) ──▶ alert subscriber
                                  └──▶ collector ──▶ InfluxDB ──▶ analysis service
```

## Directory tree

```
D7042E-course-kit/
├── deploy/               docker-compose.yml (slim stack, pinned image tags)
├── sim/sensor_sim/       simulator
├── gateway/              gateway stub
├── wokwi/pico_sensor/    diagram.json, wokwi.toml, src/main.c
├── assignment/           ASSIGNMENT.md, api-quick-reference.md
└── FRAME_FORMAT.md
```

## Technology choices

| Concern | Choice | Reason |
|---|---|---|
| Stack | Arrowhead-520-Go-Evol, pinned tag, prebuilt GHCR images | No source builds on student laptops |
| Orchestrator mode | `AUTH_BACKEND=consumerauth` | Access-control story of the assignment |
| Client library | Arrowhead-520-Evol-Go-SDK-Edu, pinned tag | Removes boilerplate, keeps concepts explicit |
| Messaging | Mosquitto (MQTT) | Simplicity; RabbitMQ PEP from Go-Evol is optional advanced material |
| Storage | InfluxDB 2 | Time-series model taught in Lecture 5 |
| Device simulation | Wokwi + Go simulator | No install; identical frame format to hardware |

## Design decisions

The key design decisions are summarized below.

| ID | Decision | Summary |
|---|---|---|
| D1 | Simulated hardware lives in the kit | Changes with the assignment each round; no reuse yet |
| D2 | Slim compose, not the full Go-Evol stack | Seventeen services incl. Kafka and AuthzForce are too heavy for a distance course |
| D3 | Blacklist thread dropped | No Blacklist service in Go-Evol |
| D4 | Certificates by name and profile | profile-ca chain onboarding → device → system; server-generated key; CSR flow later |
| D5 | TLS-only foundation ports | Client certificate needed from Phase 1; orchestrator and CA bootstrap stay plain HTTP this round |
| D6 | R10 mitigation menu | Two of: mTLS on own providers, HMAC frames, MQTT broker auth, revocation with a PIP check |
| D7 | Identity tokens for registration | `REGISTER_AUTH_URL` on the registry, `MGMT_AUTH_URL` on the foundation systems; lab-only Sysop password |
