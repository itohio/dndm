# Pipe based router

This router is intended to connect to peers like a pipe. It acts as a regular message queue between processes connected via reader and writer.

NOTE: Only one intent source is allowed. Meaning that only one peer can have an intent for the same route. Similarly for the interest.

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

