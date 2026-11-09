# Link flows

One link over time: how it starts, how routes get onto it, how data moves, and how it ends. In these sequence diagrams:

- solid arrows are data or requests;
- dashed arrows are replies;
- an arrow ending in ✕ is lost in transit;
- `alt` boxes are the failure branches;
- grey boxes only group the participants of one device.

Legend and glossary: [README](README.md#legend).

## 1. Bring-up today (as is)

```mermaid
sequenceDiagram
    title As is: mesh connection bring-up (endpoint/mesh)
    box rgb(243,244,246) Node A (dials)
        participant AB as A address book
        participant AH as A Handshaker
    end
    box rgb(243,244,246) Node B (accepts)
        participant BH as B Handshaker
        participant BC as B Container
    end
    AB->>AH: dialer loop takes a peer and dials TCP
    AH->>BH: Handshake INITIAL (me, you as text)
    Note over BH: identity is whatever text "me" says
    BH-->>AH: Handshake FINAL
    Note over BH: B is ready
    BH->>BC: OnReady: register every current route
    AH-->>BH: Handshake FINAL (B ignores it)
    Note over AH: A is ready and registers its routes too
    opt B is a seed node
        BH->>AH: Peers (other active peers)
    end
    loop every ping period
        AH->>BH: Ping
        BH-->>AH: Pong
    end
```

Caption: the handshake swaps self-declared names and nothing else, with no key, zone or permission check. Routes are registered only after it finishes (DND-004). The **address book** is each node's list of known peers and their connections. A **seed node** is one that tells newcomers about other peers.

## 2. Announce, then data (as is)

```mermaid
sequenceDiagram
    title As is: a remote subscriber appears, then data flows
    box rgb(243,244,246) Node A
        participant P as Producer
        participant AL as A remote endpoint
    end
    box rgb(243,244,246) Node B
        participant BL as B remote endpoint
        participant C as Consumer
    end
    P->>AL: Publish: LocalIntent
    AL->>BL: Intent (route name, register)
    Note over BL: B makes a RemoteIntent
    C->>BL: Subscribe: LocalInterest
    BL->>AL: Interest (route name, register)
    Note over AL: A makes a RemoteInterest, matches it with the LocalIntent, notifies P
    P->>AL: Send(msg)
    AL->>BL: MESSAGE frame: header with route name text, then msg
    BL->>C: RemoteIntent.Send into the matched interest
    C->>BL: Unsubscribe
    BL->>AL: Interest (route name, register = false)
    Note over AL: A closes that RemoteInterest, P's Send gets ErrNoInterest again
```

Caption: every MESSAGE frame repeats the full route name in its header, and nothing checks whether A may publish or B may subscribe.

## 3. Losing the connection and redialling (as is)

```mermaid
sequenceDiagram
    title As is: the connection drops, the address book redials
    box rgb(243,244,246) Node A
        participant AE as A remote endpoint
        participant AB as A address book
    end
    participant B as Node B
    B--xAE: connection drops (read error)
    Note over AE: shutdown: close the conn and the Linker, every match closes
    AE->>AB: OnClose: entry has no connection now
    loop every 10 s
        AB->>AB: monitor queues inactive outbound peers
    end
    alt dial works
        AB->>B: new TCP connection, new handshake (flow 1)
    else dial fails
        AB->>AB: back off (grows up to a max), queue again
    end
```

Caption: a reconnect is a brand-new connection that registers every route again. What goes wrong with simultaneous connections, and how generations fix it, is in [design/close-reconnect.md](../design/close-reconnect.md).

## 4a. Handshake per link profile (target)

```mermaid
sequenceDiagram
    title Target (not built yet): link bring-up, the handshake depends on the link profile
    box rgb(243,244,246) MCU
        participant M as MCU core
    end
    box rgb(243,244,246) SBC
        participant S as SBC core
    end
    M->>S: HELLO (profile, nonce, session)
    Note over M,S: until the handshake completes, every other frame is dropped
    alt OnBoard wire (I2C, SPI, on-board UART)
        S-->>M: HELLO
        Note over M,S: no keys, zone fixed by build config
    else radio, CAN FD, exposed serial: zone key (PSK)
        S-->>M: HS 1 (nonce)
        M->>S: HS 2 (proof made with keys from the zone key and both nonces)
        alt wrong zone key
            Note over S: proof does not match: CLOSE (auth), no route is ever bound
        else right zone key
            S-->>M: HS 3 (proof)
        end
    else IP link: static-DH bound to the zone anchor
        S-->>M: HS 1 (ephemeral key, static key signed by the zone anchor)
        M->>S: HS 2 (ephemeral key, own signed static key)
        alt static key not signed by this zone's anchor
            Note over M,S: CLOSE (auth)
        end
    end
    alt profiles differ (one side Secure, the other not)
        S-->>M: CLOSE (profile mismatch), never negotiated down
    end
```

Caption: a wrong zone key is caught **at the handshake**, because the proof fails, before any route is bound. The per-frame tag is a second line of defence. Only broadcast links, which have no handshake, catch a wrong zone key at data time, by tag failures.

## 4b. Bind outcomes (target)

```mermaid
sequenceDiagram
    title Target (not built yet): binding one route on a ready link
    participant M as MCU core (sends data)
    participant S as SBC core
    M->>S: BIND (route_id 3, path, full type name, stages)
    alt route exists in the link's zone and is permitted
        S-->>M: BIND-ACK ok
        M->>S: DATA (route_id 3, payload), 4 B + payload
    else route not permitted in this zone, or no grant
        S-->>M: BIND-ACK denied
        M->>S: DATA (route_id 3, payload)
        Note over S: id 3 was never bound: dropped and counted
    else route table full or BIND rate too high
        S-->>M: BIND-ACK table full
    end
```

Caption: each route name is sent once, in BIND, and data frames then carry only the route id. The receiver drops data for an id that was never bound.

## 5. Rebind after a reset (target)

```mermaid
sequenceDiagram
    title Target (not built yet): MCU reboots, the link starts a new session
    participant M as MCU core
    participant S as SBC core
    M->>S: DATA (route_id 3, counter 812)
    Note over M: power cycle, all link state lost
    M->>S: HELLO (new session)
    Note over S: old keys, counters and route ids are dropped
    M->>S: handshake (flow 4a), fresh keys
    M->>S: BIND lidar/scan as route_id 40
    S-->>M: BIND-ACK ok
    M->>S: DATA (route_id 40, counter 1)
    alt attacker replays an old frame (route_id 3, counter 812)
        Note over S: tag fails under the new keys, and id 3 is not bound: dropped
    end
```

Caption: fresh keys on every session make it safe to restart counters, and new route ids continue after the old ones, so a replayed or late frame from before the reset matches nothing.

## 6. Close and reconnect (target)

```mermaid
sequenceDiagram
    title Target (not built yet): orderly close and a reconnect that cannot revive old state
    participant A as Node A
    participant B as Node B
    A->>B: CLOSE (reason, session 7)
    Note over A,B: both drop keys, route ids and replay windows of session 7
    A->>B: HELLO (session 8)
    A->>B: handshake, then BIND every route again
    alt a delayed frame from session 7 arrives
        Note over B: no key and no route id for it: dropped and counted
    else a delayed CLOSE for session 7 arrives
        Note over B: not the current session: ignored
    end
```

Caption: a **session** is one life of a link, from HELLO to CLOSE. Ending it drops all of its state, so stale frames and a stale CLOSE have nothing to act on. Address-book generations are in [design/close-reconnect.md](../design/close-reconnect.md).

[Diagrams index](README.md) · [Core](core.md) · [Message paths](message-paths.md)
