# Core idea

dndm moves typed protobuf messages by **name**. A publisher declares an **intent** ("I can produce `Scan` on `lidar/scan`"). A subscriber declares an **interest** ("I want `Scan` on `lidar/scan`"). When both exist for the same **route** (message type plus path), dndm **matches** them, and data flows. No intent or interest means no traffic. Words and arrows: [glossary and legend](README.md#glossary).

## 1. Intent meets interest (as is)

```mermaid
---
title: "As is: a route matches one intent with one interest"
---
flowchart LR
    pub(["Publisher"]):::app -->|"Publish(path, type)"| intent["Intent<br/>route = Scan@lidar/scan"]:::core
    sub(["Subscriber"]):::app -->|"Subscribe(path, type)"| interest["Interest<br/>route = Scan@lidar/scan"]:::core
    intent -.->|"same route"| match["match (Linker)"]:::core
    interest -.->|"same route"| match
    intent -->|"Send(msg), only after the match"| interest
    interest -->|"C() channel"| sub

    classDef app fill:#dbeafe,stroke:#1d4ed8,color:#111
    classDef core fill:#fef3c7,stroke:#b45309,color:#111
```

Caption: nothing is sent until an intent and an interest with the same route meet. Before that, `Send` returns `ErrNoInterest`.

## 2. Route names (as is)

```mermaid
---
title: "As is: how a route is named"
---
flowchart LR
    type["Message type<br/>types.Scan"]:::core --> plain["Plain route<br/>Scan@lidar/scan"]:::core
    path["Path<br/>lidar/scan"]:::core --> plain
    plain -->|"or NewHashedRoute(prefix, path)"| hashed["Hashed route<br/>prefix#35;base64(sha1(name))"]:::core
    plain --> either["plain or hashed name"]:::core
    hashed --> either
    either ==>|"sent as text in every frame header"| wire{{"connection"}}:::link

    classDef core fill:#fef3c7,stroke:#b45309,color:#111
    classDef link fill:#dcfce7,stroke:#15803d,color:#111
```

Caption: today the route name, plain or hashed, travels as a string in the header of every message (`route.go`, `types/core` `Header.route`).

## 3. Inside one process (as is)

```mermaid
---
title: "As is: the router asks every endpoint for an intent and an interest"
---
flowchart TB
    app(["Application<br/>Publish / Subscribe"]):::app --> router["Router<br/>dndm.go"]:::core
    router -->|"one per route"| ir["IntentRouter<br/>sends to every endpoint"]:::core
    ir --> direct["direct endpoint<br/>same process, own Linker"]:::core
    ir --> remote["remote endpoint<br/>one per connection, own Linker"]:::core
    ir --> mesh["mesh Handshaker<br/>wraps a remote endpoint once the handshake is done"]:::core
    direct --> sr["InterestRouter, one per route<br/>collects from every endpoint, copies to each subscriber"]:::core
    remote --> sr
    mesh --> sr
    sr --> subs(["Subscribers"]):::app
    remote ---|"uses"| conn{{"network.Conn<br/>TCP, serial"}}:::link

    classDef app fill:#dbeafe,stroke:#1d4ed8,color:#111
    classDef core fill:#fef3c7,stroke:#b45309,color:#111
    classDef link fill:#dcfce7,stroke:#15803d,color:#111
```

Caption: one `Publish` creates an intent in **every** endpoint, and one `Subscribe` an interest in every endpoint. Each endpoint's Linker matches its own intents and interests.

## 4. Announcing over a connection (as is)

```mermaid
---
title: "As is: intents and interests are announced as messages"
---
flowchart LR
    subgraph A["Node A"]
        apub(["Publisher"]):::app --> ali["remote LocalIntent"]:::core
        ali -->|"matched in A's Linker"| arint["RemoteInterest<br/>(B wants it)"]:::core
    end
    conn{{"TCP or serial connection"}}:::link
    subgraph B["Node B"]
        brint["RemoteIntent<br/>(A has it)"]:::core -->|"matched in B's Linker"| bli["remote LocalInterest"]:::core
        bli --> bsub(["Subscriber"]):::app
    end
    ali -.->|"Intent: route, register"| conn
    conn -.-> brint
    bli -.->|"Interest: route, register"| conn
    conn -.-> arint
    arint ==>|"MESSAGE frames"| conn
    conn ==> brint

    classDef app fill:#dbeafe,stroke:#1d4ed8,color:#111
    classDef core fill:#fef3c7,stroke:#b45309,color:#111
    classDef link fill:#dcfce7,stroke:#15803d,color:#111
    classDef group fill:#f9fafb,stroke:#9ca3af,color:#111
    class A,B group
```

Caption: each side mirrors the other's announcements as Remote objects, and each side's Linker matches a local object with a remote one. One route can't yet flow both ways on one connection (DND-020).

## 5. What one data frame carries (as is vs target)

```mermaid
---
title: "As is vs Target: bytes around each message"
---
flowchart TB
    subgraph asis["As is: 86–94 B for EasyRobot's routes"]
        direction LR
        m1["magic<br/>4 B"]:::core ~~~ s1["sizes<br/>4 + 4 + 4 B"]:::core ~~~ h1["Header protobuf<br/>timestamp, type, route name text<br/>70–78 B"]:::core ~~~ p1["message protobuf"]:::field
    end
    subgraph tgt["Target (not built yet): 4 B"]
        direction LR
        f2["face framing<br/>COBS/CRC, UDP datagram"]:::linkT ~~~ r2["route_id field<br/>2 B"]:::coreT ~~~ l2["payload tag + length<br/>2 B"]:::coreT ~~~ p2["message protobuf"]:::fieldT
    end
    asis ~~~ tgt

    classDef core fill:#fef3c7,stroke:#b45309,color:#111
    classDef field fill:#ffffff,stroke:#6b7280,color:#111
    classDef coreT fill:#fef3c7,stroke:#b45309,color:#111,stroke-dasharray:5 4
    classDef fieldT fill:#ffffff,stroke:#6b7280,color:#111,stroke-dasharray:5 4
    classDef linkT fill:#dcfce7,stroke:#15803d,color:#111,stroke-dasharray:5 4
    classDef group fill:#f9fafb,stroke:#9ca3af,color:#111
    classDef groupT fill:#f9fafb,stroke:#9ca3af,color:#111,stroke-dasharray:5 4
    class asis group
    class tgt groupT
```

Caption: today each EasyRobot message carries 86–94 B around it, mostly the route name. In the target it carries 4 B of protobuf (route id plus the payload's tag and length, for payloads under 128 B), because the name was bound once, when the link started. Secured links add a counter and an 8 B tag (about 15 B in total); on CAN the route id moves into the CAN identifier.

## 6. Link context (target)

```mermaid
---
title: "Target (not built yet): a link fixes zone and security once"
---
flowchart LR
    subgraph node["SBC"]
        app(["Application<br/>Publish lidar/scan in zone lab"]):::appT --> table["zone lab route table"]:::coreT
        table --> disp["dispatcher<br/>only path between links and routes"]:::coreT
    end
    disp ==>|"DATA: route_id 3 + payload, 4 B"| l1{{"Link 1: UART<br/>zone lab, OnBoard, no crypto"}}:::linkT
    disp ==>|"DATA: route_id 7 + payload + tag"| l2{{"Link 2: Wi-Fi<br/>zone lab, Secure AEAD"}}:::linkT
    l1 ==> mcu["MCU"]:::device
    l2 ==> host["Host"]:::device
    disp -.->|"BIND once: lidar/scan = 3"| l1
    disp -.->|"BIND once: lidar/scan = 7"| l2

    classDef appT fill:#dbeafe,stroke:#1d4ed8,color:#111,stroke-dasharray:5 4
    classDef coreT fill:#fef3c7,stroke:#b45309,color:#111,stroke-dasharray:5 4
    classDef linkT fill:#dcfce7,stroke:#15803d,color:#111,stroke-dasharray:5 4
    classDef device fill:#f9fafb,stroke:#9ca3af,color:#111,stroke-dasharray:5 4
    classDef groupT fill:#f9fafb,stroke:#9ca3af,color:#111,stroke-dasharray:5 4
    class node groupT
```

Caption: the same route gets a different id on each link. Ids mean nothing outside their link, so a frame cannot name another link's routes or another zone.

## 7. Zones and crossing (target)

```mermaid
---
title: "Target (not built yet): zones are crossed only through a bridge with a grant"
---
flowchart LR
    subgraph lab["Zone lab"]
        lp(["pose publisher"]):::appT --> lt["lab route table"]:::coreT
        la(["arm/target publisher"]):::appT --> lt
    end
    bridge{"Bridge<br/>grant: lab pose to fleet robots/r1/pose"}:::coreT
    subgraph fleet["Zone fleet"]
        ft["fleet route table"]:::coreT --> fs(["fleet dashboard"]):::appT
    end
    lt -->|"pose"| bridge
    bridge -->|"re-encoded, new route id"| ft
    lt --x|"arm/target: no grant"| bridge

    classDef appT fill:#dbeafe,stroke:#1d4ed8,color:#111,stroke-dasharray:5 4
    classDef coreT fill:#fef3c7,stroke:#b45309,color:#111,stroke-dasharray:5 4
    classDef groupT fill:#f9fafb,stroke:#9ca3af,color:#111,stroke-dasharray:5 4
    class lab,fleet groupT
```

Caption: a zone is a separate route table, not a prefix on every name. The only way out is a bridge, and it passes only routes its grant names.

## 8. Build-time stages (target)

```mermaid
---
title: "Target (not built yet): stages run as a flat list, chosen at build time"
---
flowchart LR
    send(["Send(msg)"]):::appT --> s1["stage: latest or buffered"]:::coreT --> s2["stage: fragment<br/>only on small links"]:::coreT
    s2 --> s3["stage: Secure AEAD"]:::coreT --> face{{"face write<br/>UART, UDP, CAN"}}:::linkT
    s2 --> ins["Insecure(reason)<br/>network link without Secure:<br/>loud log, advertised as unauthenticated"]:::flagT --> face

    classDef appT fill:#dbeafe,stroke:#1d4ed8,color:#111,stroke-dasharray:5 4
    classDef coreT fill:#fef3c7,stroke:#b45309,color:#111,stroke-dasharray:5 4
    classDef linkT fill:#dcfce7,stroke:#15803d,color:#111,stroke-dasharray:5 4
    classDef flagT fill:#fef3c7,stroke:#b91c1c,color:#111,stroke-dasharray:5 4
```

Caption: stages look like middleware in the API but run as a fixed list in one loop, so call depth stays constant. A network link uses either Secure or a declared `Insecure(reason)`, never neither. Unused stages are not linked into a small build.

[Diagrams index](README.md) · [Link flows](link-flows.md) · [Message paths](message-paths.md)
