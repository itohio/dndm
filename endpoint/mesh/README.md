# Mesh

Mesh router transport connects nodes into a full mesh. This is achieved by sharing peers and/or
address book with other peers when then successfully handshake.

## Protocol

1. Connection completed
2. Periodic ping-pong is sent and connection measured
  2.1. No restrictions on the rate and size of ping-pong is emposed
3. Messages can be exchanged
  3.1. Only messages that match interest/intent can be shared
  3.1. `Intent` / `Interest` / `Peers` / `Addrbook` messages can be shared

### Connection

1. Handshake initiated
2. Connected Peers shared
3. Interests shared
4. Intents shared

The goal of this handshake is to learn about each other FQN. For example a node's configured FQN
can be `tcp://0.0.0.0:1234/some.path`, however, the interface on which it is actually reachable can be
`tcp://127.0.0.1:1234/some.path` or maybe some public IP that is behind NAT. Then both the IP and the address
will be different.

In order for the node's address book to be useful, we need to be able to determine what is the `ip:port` that the node connecting to us
is reachable by. Handshake shares this information how it sees the peer with the peer and the peer can adjust its actual address.

The handshake is as follows:

1. Peer sends `Handshake` message with `Initial` state
  1.1. Peer is initially in `INIT` state
  1.2. Node is in `WAITING` state
  1.3. `Me` field is peer's FQN (e.g. tcp://0.0.0.0:1234/some.path)
  1.4. `You` field is the node's FQN as known by the peer (e.g. tcp://some.hostname.com:1234/remote.path)
  1.5. `Intents` and `Interests` will contain routes the peer is interested in
  1.6. Peer transitions to `WAITING` state
2. Node receives handshake, updates peer information, validates
  2.1. sends `Handshake` message with `Initial` state to the peer
  2.2. Node sets `Me` to its own FQN
  2.3. `You` will contain the peer's FQN as seen by the node
  2.4. `Intents` and `Interests` will contain routes the node is interested in
  2.4. Node transitions to `DONE` state
3. Peer receives handshake, validates, updates peer information
  3.1. sends `Handshake` message with `Final` state confirming `Me` & `You` fields
  3.2. `Intents` and `Interests` will contain node's routes the peer successfully registered
  3.3. Peer transitions to `DONE` state
4. Node sends `Handshake` message with `Final` state
  4.1. `Me` and `You` must match advertised respective FQNs
  4.2. `Intents` and `Interests` will contain peer's routes the node successfully registered
  4.3. Node sends `Peers` message with peers it is connected to
5. Peer receives `Peers` and adds them to the addrbook
  5.1. It will try to connect to these peers if it has no active connection to them yet


### Intent

### Interest

## TODO

- pinging
  - how to get that info?
  - Why though?

## FIXME

- transform peer's `ip:port` so that it can be reachable if it is configured to listen on `0.0.0.0`
- do not reset connection when another connection comes in and is rejected 
  - reject that new connection sooner
- share new peer with the network when it connects
  - can be periodic
