# Robotics scenarios

Scenarios S1–S9 from [research-2](../research/ideation/2026-11-08-ndn-delivery-auth-2.md) §9. All of them are **target designs (not built yet)**: today's dndm has no zones, link profiles, history or broadcast. Frame sizes follow the [link-context wire design](../design/link-context-wire.md): a data frame is 4 B plus the payload, and a secured one about 15 B. The rest are estimates from research-2 §6. Legend and glossary: [README](README.md#legend).

## S1. Lidar to planner across three devices

```mermaid
---
title: "Target (not built yet): S1 lidar, SLAM, planner and motors in zone robot1"
---
flowchart TB
    subgraph mcu["RP2040"]
        lidar(["lidar driver"]):::appT
        motor(["motor controller"]):::appT
        fs["local motor failsafe<br/>stops on a stale target"]:::outside
    end
    uart{{"UART link, OnBoard<br/>4 B + payload, COBS/CRC"}}:::linkT
    subgraph sbc["SBC"]
        slam(["SLAM"]):::appT --> hist[("pose history")]:::storeT --> plan(["planner"]):::appT
    end
    wifi{{"Wi-Fi link, static-DH, AEAD<br/>about 15 B + payload"}}:::linkT
    subgraph host["Host"]
        view(["map viewer"]):::appT
    end
    lidar ==>|"scan: latest"| uart ==> slam
    plan ==>|"target: buffered"| uart ==> motor
    motor ---|"watched by"| fs
    hist ==>|"pose: history"| wifi ==> view

    classDef appT fill:#dbeafe,stroke:#1d4ed8,color:#111,stroke-dasharray:5 4
    classDef storeT fill:#ede9fe,stroke:#6d28d9,color:#111,stroke-dasharray:5 4
    classDef linkT fill:#dcfce7,stroke:#15803d,color:#111,stroke-dasharray:5 4
    classDef outside fill:#f3f4f6,stroke:#6b7280,color:#111
    classDef groupT fill:#f9fafb,stroke:#9ca3af,color:#111,stroke-dasharray:5 4
    class mcu,sbc,host groupT
```

Caption: one zone spans all three devices, and each route picks its own delivery: latest for scans, history for poses, buffered for targets. The motor failsafe is local to the MCU and does not depend on dndm.

## S2. Teleop over ELRS with a failsafe

```mermaid
---
title: "Target (not built yet): S2 RC control stays outside dndm"
---
flowchart LR
    subgraph ground["Ground (fpv-arena rig)"]
        pilot(["pilot input"]):::outside --> wd["failsafe watchdog<br/>fresh up to 150 ms, hold 100 ms more,<br/>then neutral values"]:::outside
        wd --> tx{{"ELRS TX module<br/>CRSF RC channels"}}:::outside
        tel(["telemetry view"]):::appT
    end
    subgraph robot["Robot"]
        rx{{"ELRS receiver<br/>own failsafe if the radio drops"}}:::outside --> drive["drive controller"]:::outside
        sensors(["battery, IMU"]):::appT
    end
    tx ==>|"RC frames, native CRSF"| rx
    sensors ==>|"telemetry: latest, zone key, 4 B tag"| crsf{{"dndm link inside CRSF frames<br/>60 B payload"}}:::linkT
    crsf ==> tel

    classDef appT fill:#dbeafe,stroke:#1d4ed8,color:#111,stroke-dasharray:5 4
    classDef linkT fill:#dcfce7,stroke:#15803d,color:#111,stroke-dasharray:5 4
    classDef outside fill:#f3f4f6,stroke:#6b7280,color:#111
    classDef groupT fill:#f9fafb,stroke:#9ca3af,color:#111,stroke-dasharray:5 4
    class ground,robot groupT
```

Caption: steering and throttle never go through dndm, so if dndm telemetry stops, only the view goes stale and nothing moves. The watchdog numbers are fpv-arena's settings (`drive-stale-ms` 150, `drive-hold-ms` 100, ADR 0022), and that watchdog runs on the rig, before the TX module, not on the robot.

## S3. Three zones with a bridge

```mermaid
---
title: "Target (not built yet): S3 robots r1 and r2, base station, one grant"
---
flowchart LR
    subgraph r1["Zone r1"]
        r1pose(["r1 pose"]):::appT
    end
    subgraph r2["Zone r2"]
        r2arm(["r2 arm/target subscriber"]):::appT
    end
    bridge{"Bridge<br/>grant: r1 pose to base robots/r1/pose<br/>no grant for r2"}:::coreT
    subgraph base["Zone base"]
        dash(["fleet dashboard"]):::appT
        nats["NATS gateway<br/>to the fleet's NATS"]:::outside
    end
    r1pose -->|"pose"| bridge
    bridge -->|"robots/r1/pose"| dash
    dash -->|"robots/r1/pose"| nats
    dash --x|"command r2 arm/target: no grant"| bridge
    bridge ~~~ r2arm

    classDef appT fill:#dbeafe,stroke:#1d4ed8,color:#111,stroke-dasharray:5 4
    classDef coreT fill:#fef3c7,stroke:#b45309,color:#111,stroke-dasharray:5 4
    classDef outside fill:#f3f4f6,stroke:#6b7280,color:#111
    classDef groupT fill:#f9fafb,stroke:#9ca3af,color:#111,stroke-dasharray:5 4
    class r1,r2,base groupT
```

Caption: every crossing goes through the bridge, and each needs its own grant. The base station can watch r1's pose, but its command for r2's arm stops at the bridge because no grant names it.

## S4. Sensor history replay

```mermaid
sequenceDiagram
    title Target (not built yet): S4 a late consumer catches up from history
    participant P as Producer ring (MCU)
    participant H as History store near the consumer (SBC)
    participant O as Other holder (peer SBC)
    participant C as Late consumer
    C->>H: fetch pose, seq 100 to 200
    H-->>C: seq 150 to 200 (partial hit)
    C->>O: fetch seq 100 to 149
    O-->>C: seq 100 to 139, signed by the producer
    Note over C: copies served by others must carry the producer's signature
    C->>P: fetch seq 140 to 149
    P-->>C: seq 140 to 149 from its ring
    alt a copy fails the signature check
        Note over C: dropped, ask the next holder or the producer
    end
```

Caption: history can be served by anyone who holds a copy. The consumer trusts the producer's signature, not the holder.

## S5. Link bring-up

Drawn once, in [link-flows.md](link-flows.md):

- [4a](link-flows.md#4a-handshake-per-link-profile-target): the handshake per link profile, including a wrong zone key and a profile mismatch;
- [4b](link-flows.md#4b-bind-outcomes-target): bind outcomes (ok, denied, table full) and then data.

## S6. Wi-Fi broadcast with repair

```mermaid
sequenceDiagram
    title Target (not built yet): S6 one receiver misses a frame and repairs it
    participant S as Feature publisher (sender slot 4)
    participant A as Receiver A
    participant B as Receiver B
    S->>A: broadcast seq 40
    S->>B: broadcast seq 40
    S->>A: broadcast seq 41
    S-xB: seq 41 lost in the air
    S->>A: broadcast seq 42
    S->>B: broadcast seq 42
    Note over B: gap after 40 in sender 4's window
    B->>S: repair request (sender 4, seq 41)
    S-->>B: resend seq 41
    Note over A,B: each receiver keeps a replay window per sender slot
```

Caption: only receivers that miss a frame ask for it, and only on routes with a buffer. Replay windows are kept per sender, so a resent frame is accepted once.

## S7. CAN actuator bus

```mermaid
---
title: "Target (not built yet): S7 CAN inside one robot, CAN gateway to the SBC"
---
flowchart LR
    sbc(["SBC planner"]):::appT
    gw["CAN gateway<br/>same zone robot1, not a bridge"]:::coreT
    bus{{"CAN bus link, zone robot1, OnBoard<br/>route id in the CAN identifier"}}:::linkT
    subgraph j1["joint 1 MCU"]
        c1(["joint controller"]):::appT
    end
    subgraph j2["joint 2 MCU"]
        c2(["joint controller"]):::appT
    end
    sbc -->|"arm/target"| gw
    gw ==>|"command, authenticated:<br/>classic CAN leaves 1 B of data, CAN FD about 50 B"| bus
    bus ==> c1
    bus ==> c2
    c1 ==>|"joint state, periodic, not authenticated:<br/>6 B of data in 8"| bus
    bus ==> gw
    gw -->|"joint state"| sbc

    classDef appT fill:#dbeafe,stroke:#1d4ed8,color:#111,stroke-dasharray:5 4
    classDef coreT fill:#fef3c7,stroke:#b45309,color:#111,stroke-dasharray:5 4
    classDef linkT fill:#dcfce7,stroke:#15803d,color:#111,stroke-dasharray:5 4
    classDef groupT fill:#f9fafb,stroke:#9ca3af,color:#111,stroke-dasharray:5 4
    class j1,j2 groupT
```

Caption: the route id rides in the CAN identifier, so a frame spends only 2 B on protobuf. An authenticated command also needs a counter and a 3 B MAC, which leaves 1 B of data in classic CAN's 8 bytes. That stays true for every frame, because only the counter's low 7 bits are sent. That is why authenticated commands need CAN FD in practice; the owner's rule applies once a target is chosen. Sizes measured with code generated from the draft schema.

## S8. Same app, three builds

```mermaid
---
title: "Target (not built yet): S8 which stages each build links in"
---
flowchart TB
    code(["same app code:<br/>Publish scan, Subscribe target"]):::appT
    subgraph rp["RP2040 build"]
        rp1["core"]:::coreT --> rp2["stage: latest"]:::coreT --> rp3{{"serial face"}}:::linkT
    end
    subgraph esp["ESP32-C5 build"]
        e1["core"]:::coreT --> e2["stage: buffered"]:::coreT --> e3["stage: Secure AEAD"]:::coreT --> e4{{"UDP face"}}:::linkT
    end
    subgraph hst["Host build"]
        h1["core"]:::coreT --> h2["stage: history"]:::coreT --> h3["stage: end-to-end sign"]:::coreT --> h4["stage: Secure AEAD"]:::coreT --> h5{{"TCP face"}}:::linkT
        h1 --> h6["NATS gateway"]:::outside
    end
    code --> rp1
    code --> e1
    code --> h1

    classDef appT fill:#dbeafe,stroke:#1d4ed8,color:#111,stroke-dasharray:5 4
    classDef coreT fill:#fef3c7,stroke:#b45309,color:#111,stroke-dasharray:5 4
    classDef linkT fill:#dcfce7,stroke:#15803d,color:#111,stroke-dasharray:5 4
    classDef outside fill:#f3f4f6,stroke:#6b7280,color:#111
    classDef groupT fill:#f9fafb,stroke:#9ca3af,color:#111,stroke-dasharray:5 4
    class rp,esp,hst groupT
```

Caption: the app code does not change; the build picks the stages. Stages a build does not use are not linked in. Whether that holds for TinyGo or C++ binaries is still to be measured (DND-015).

## S9. Attack paths

```mermaid
---
title: "Target (not built yet): S9 where each attacker is stopped"
---
flowchart LR
    a1["attacker clipped onto the serial wire"]:::deny
    a2["attacker on the Wi-Fi network"]:::deny
    a3["compromised device inside the zone"]:::deny
    o1["OnBoard wire: no check"]:::coreT
    o2{"link AEAD:<br/>zone key or static-DH"}:::coreT
    o5{"end-to-end<br/>producer signature"}:::coreT
    route(["protected route"]):::appT
    a1 -->|"can inject: accepted risk on board"| o1 --> route
    a2 --x|"guessed route ids and injected frames fail the tag"| o2
    a3 -->|"inside the zone: passes link AEAD"| o2
    o2 --> o5
    o5 --x|"not the producer's key: rejected"| route

    classDef appT fill:#dbeafe,stroke:#1d4ed8,color:#111,stroke-dasharray:5 4
    classDef coreT fill:#fef3c7,stroke:#b45309,color:#111,stroke-dasharray:5 4
    classDef deny fill:#fee2e2,stroke:#b91c1c,color:#111
```

Caption: each protection stops a different attacker.

- Link AEAD stops outsiders.
- End-to-end signatures stop insiders who forge a producer.
- Nothing stops a device physically on an on-board wire. That is why such wires must be declared `OnBoard()`.

Full table: research-2 §8.

[Diagrams index](README.md) · [Core](core.md) · [Link flows](link-flows.md) · [Message paths](message-paths.md)
