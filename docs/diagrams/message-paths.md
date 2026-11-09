# Message paths

How one message travels from a publisher to its subscribers. The first four diagrams show code that exists today; the last two show the target design. Legend and glossary: [README](README.md#legend).

## 1. Inside one process (as is)

```mermaid
---
title: "As is: one Send inside one process (direct endpoint)"
---
flowchart LR
    prod(["Producer.Send"]):::app --> wrap["intent wrapper<br/>Router.Publish handle"]:::core
    wrap --> fan["FanOutIntent<br/>one goroutine per endpoint per Send"]:::core
    fan --> li["direct LocalIntent"]:::core
    li -->|"waits while full"| q1[("interest channel<br/>size from router options")]:::store
    q1 --> sr["InterestRouter<br/>copies to each subscriber"]:::core
    sr --> q2[("subscriber 1 channel")]:::store --> c1(["Consumer 1"]):::app
    sr --> q3[("subscriber 2 channel")]:::store --> c2(["Consumer 2"]):::app
    sr --x|"subscriber channel full: dropped, not counted"| lost["lost message"]:::deny

    classDef app fill:#dbeafe,stroke:#1d4ed8,color:#111
    classDef core fill:#fef3c7,stroke:#b45309,color:#111
    classDef store fill:#ede9fe,stroke:#6d28d9,color:#111
    classDef deny fill:#fee2e2,stroke:#b91c1c,color:#111
```

Caption: the sender waits while the interest channel is full, but a full subscriber channel silently loses the message (making that visible is DND-006/007). All subscribers share the very same message object, which is why it must not change after Send (DND-002).

## 2. To another node (as is)

```mermaid
---
title: "As is: one Send to a subscriber on another node (TCP)"
---
flowchart LR
    subgraph A["Node A"]
        prod(["Producer.Send"]):::app --> rli["remote LocalIntent"]:::core
        rli --> rq[("RemoteInterest channel")]:::store
        rq --> fwd["forwarder goroutine<br/>the only reader (DND-003)"]:::core
        fwd --> enc["encode frame<br/>header + route name + msg"]:::core
    end
    enc ==> tcp{{"TCP stream"}}:::link
    subgraph B["Node B"]
        dec["read and decode frame<br/>size capped (DND-001)"]:::core --> hm["handleMsg<br/>find RemoteIntent by route name"]:::core
        hm --> bq[("local interest channel")]:::store --> bsr["InterestRouter"]:::core --> cons(["Consumer"]):::app
    end
    tcp ==> dec

    classDef app fill:#dbeafe,stroke:#1d4ed8,color:#111
    classDef core fill:#fef3c7,stroke:#b45309,color:#111
    classDef store fill:#ede9fe,stroke:#6d28d9,color:#111
    classDef link fill:#dcfce7,stroke:#15803d,color:#111
    classDef group fill:#f9fafb,stroke:#9ca3af,color:#111
    class A,B group
```

Caption: the message is encoded once per remote peer, and the receiving side finds its route by the name text in the header.

## 3. Through a middle node (as is)

```mermaid
---
title: "As is: no automatic relay through a middle node"
---
flowchart LR
    subgraph A["Node A"]
        ap(["Publisher of pose"]):::app
    end
    subgraph B["Node B (middle)"]
        bra["endpoint to A<br/>holds RemoteIntent pose"]:::core
        brc["endpoint to C<br/>holds RemoteInterest pose"]:::core
        relay(["only if written by hand:<br/>app on B subscribes to pose<br/>and publishes it again"]):::app
    end
    subgraph C["Node C"]
        cs(["Subscriber of pose"]):::app
    end
    ap -.->|"Intent pose"| bra
    brc ~~~ cs
    cs -.->|"Interest pose"| brc
    bra --x|"not matched: the router does not match<br/>remote objects of different endpoints"| brc
    bra --> relay --> brc

    classDef app fill:#dbeafe,stroke:#1d4ed8,color:#111
    classDef core fill:#fef3c7,stroke:#b45309,color:#111
    classDef group fill:#f9fafb,stroke:#9ca3af,color:#111
    class A,B,C group
```

Caption: today a message crosses only one connection. To go further, an app on the middle node must subscribe and publish again (`container.go` refuses to re-announce remote intents and interests; `Router.addInterest` skips remote interests).

## 4. Request and reply (as is, x/bus)

```mermaid
sequenceDiagram
    title As is: Caller and Service with reply nonces (x/bus, DND-005)
    participant Ca as Caller
    participant Sv as Service
    box rgb(254,226,226) Untrusted
        participant X as Other participant on the reply path
    end
    Note over Ca: nonce from the NonceSource (crypto/rand by default, counter on TinyGo)
    Ca->>Sv: request on the request path (nonce n)
    X-->>Ca: reply with a guessed nonce
    Note over Ca: no live call with that nonce: dropped, Dropped() + 1
    Sv-->>Ca: reply on the reply path (nonce n)
    Note over Ca: matches call n, delivered once
    alt X copies n from the request and answers first
        X-->>Ca: forged reply (nonce n)
        Note over Ca: accepted: a nonce is not authentication (needs DND-009)
    end
```

Caption: nonces keep replies with the right call, even when several Callers share one reply path. They do not prove who answered.

## 5. Through a bridge between zones (target)

```mermaid
---
title: "Target (not built yet): a pose leaves robot zone r1 for zone base"
---
flowchart LR
    subgraph r1["Zone r1"]
        mcu(["pose estimator on MCU"]):::appT ==> l1{{"UART link, zone r1<br/>route_id 2"}}:::linkT ==> sbc["SBC dispatcher<br/>r1 table"]:::coreT
    end
    br{"Bridge on SBC<br/>grant: r1 pose to base robots/r1/pose"}:::coreT
    subgraph base["Zone base"]
        bt["base table"]:::coreT ==> l2{{"Wi-Fi link, zone base<br/>route_id 9, AEAD"}}:::linkT ==> host(["fleet dashboard"]):::appT
    end
    sbc -->|"pose"| br
    br -->|"re-encoded with base's route id"| bt
    host --x|"subscribe r1 arm/target: no grant"| br

    classDef appT fill:#dbeafe,stroke:#1d4ed8,color:#111,stroke-dasharray:5 4
    classDef coreT fill:#fef3c7,stroke:#b45309,color:#111,stroke-dasharray:5 4
    classDef linkT fill:#dcfce7,stroke:#15803d,color:#111,stroke-dasharray:5 4
    classDef groupT fill:#f9fafb,stroke:#9ca3af,color:#111,stroke-dasharray:5 4
    class r1,base groupT
```

Caption: the message keeps its payload but gets a new route id and a new zone on the far side of the bridge. Without a grant, nothing crosses.

## 6. Broadcast to many receivers (target)

```mermaid
---
title: "Target (not built yet): one Wi-Fi broadcast, many receivers"
---
flowchart LR
    pub(["feature publisher"]):::appT --> enc["frame: route id, sender slot, seq,<br/>sealed with the zone group key"]:::coreT
    enc ==> air{{"UDP broadcast"}}:::linkT
    air ==> r1["receiver 1<br/>replay window for this sender"]:::coreT
    air ==> r2["receiver 2"]:::coreT
    air ==> r3["receiver 3"]:::coreT
    outsider["device without the zone key"]:::deny --x|"tag fails"| r1

    classDef appT fill:#dbeafe,stroke:#1d4ed8,color:#111,stroke-dasharray:5 4
    classDef coreT fill:#fef3c7,stroke:#b45309,color:#111,stroke-dasharray:5 4
    classDef linkT fill:#dcfce7,stroke:#15803d,color:#111,stroke-dasharray:5 4
    classDef deny fill:#fee2e2,stroke:#b91c1c,color:#111,stroke-dasharray:5 4
```

Caption: one transmission reaches every receiver. A group key keeps outsiders out, but any zone member could pose as another member unless the route is end-to-end signed. Repairing a missed frame is [S6](robotics.md#s6-wi-fi-broadcast-with-repair).

[Diagrams index](README.md) · [Core](core.md) · [Link flows](link-flows.md) · [Robotics](robotics.md)
