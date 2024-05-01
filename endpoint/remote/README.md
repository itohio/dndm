# Remote Endpoint

Remote Endpoint connects another Remote Endpoint via a network.Conn interface.

## Intents

Local Intent is registered only if LocalPeer path matches the prefix of the Route.

Remote Intent is only registered if RemotePeer path matches the prefix of the Route.

## Interests

Local Interest is registered only if RemotePeer path matches the prefix of the Route.

Remote Interest is registered only if LocalPeer path matches the prefix of the Route.

## Messages

TBD

### Ping-Pong

A Ping-Pong is exchanged with a configured period. Each peer is sending Ping and Pong back.

TODO: Perhaps only exchange OWL messages and estimate TTL from that saving some traffic(Might have issues with patents).

## Basic protocol idea draft

Publish: broadcasts intent to publish data
Subscribe: broadcasts interest in data

At the beginning router tries to connect to peers

When a peer is connected:

- share peers
- share interests
- share intents

When a peer is disconnected:

- remove interests and intents

When router encounters Intent message:

- register intent

When router encounters Interest message:

- register interest

What to do if intent/interest exists locally and we receive one remotely?

- TBD

We have Interest and Intent match:

- register a link only if intent and interest source IDs do not match (allow only remote communication)
- if intent is local and interest remote -> send data to remote
- if intent is remote and interest is local -> receive data from remote
- if both are remote - ignore (relay not supported)

