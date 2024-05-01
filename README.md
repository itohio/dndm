# dndm

DNDM stands for Decentralized Named Data Messaging. The inspiration for such a system comes from LibP2P, Named Data Networks, general Pub/Sub architectures, and ROS.
The main purpose of this library is to provide a framework for seamless decentralized, distributed, and modular architectures mainly for robotic applications.

This is the initial version. It focuses on efficient delivery within the same process, multiple processes using sockets, and LAN using UDP/TCP.

The main goal is to be able to connect multiple independent modules that can stream typed data from each other, communicate with each other using named and typed channels. 
The connection between these modules should happen automagically. For example, the networked version should either be able to discover each node and self-arrange into a full-mesh network.
Socket communicatino would be more restricted since you'd have to specify what nodes communicate with what other nodes.

## Architecture

DNDM consists of Intents and Interests. These are communicated and managed via Endpoints. Each Intent and Interest is marked with a Route that uniquely identifies the data
by combining the stream name and message type. A Linker used by Endpoint connects an Intent with an Interest by routing messages from an Intent to an Interest as efficiently
as possible.

Interest is very similar to Subscription in Pub/Sub systems. When an entity Subscribes to a certain named stream it indicates an Interest in that named steram and certain
message type. This Interest is propagated through the system until a corresponding Intent is detected and the two are linked.

An Intent is registered by Publishing a certain route.

Once an Intent is linked with a matching Interest, it receives a notification that contains the Route it was linked with. The data source can start sending data to that link.

### Routes

A Route is combination of a path and a type. For example, if some example data source sends `Foo` messages on `example` path, then the Route will be `Foo@example`.
`Foo@example` is the PlainRoute that has the type and the path in plain text, however, in some situations is it desirable to hide the type and the path. In this case
the path is hashed. This allows for Object-Capability security model.

TBD: Hashed Routes and Peer Paths matching?

### Direct Endpoint

Direct Endpoint manages Intents and Interests created within the same process and boils down to a simple channel. There is no additional overhead once Interest and Intent is linked.
There can be multiple senders and multiple receivers of the data, however, care must be taked to not modify the message since only the copy of the pointer and not the data itself
is transmitted to other Interests.

### Remote Endpoint

Remote Endpoint manages Intents and Interests created on different processes or even systems. There have to be two Endpoints for the communication to take place.
Endpoints are connected via a connection that wraps a ReadWriter interface, therefore any transport protocol can be implemented.

Intents and Interests are registered either locally or remotely by a connected peer. Remote Intents and Interests are registered with accordance with Peer Path and Route Path matching.

### Mesh Endpoint

Mesh Endpoint manages a collection of Remote Endpoints, and implements a protocol that allows all these endpoints to ultimately connect into a full-mesh.
Intents and Interests are routed based on the Self/Remote Peer path and the message path. Peer Path is matched as a prefix to the message path, and if the prefix is matched
then the message is routed accordingly to the Interest/Intent.

#### Peers

Each peer is identified by a transport schema, address, and path. Peer can also contain parameters that help the connection establishment.
The address is used for peer identification. Path is used to control how the intents and interests are routed.

For example, if the A peer path is `example.foo` and B peer path is `example.bar`, then peer A Intents that start with `example.foo` are relayed to peer B, and
peer's B interests that start with `example.foo` are routed to peer A.

It is very helpful for each peer to have unique paths that they export to the remote peers.

##### Collision resolution

TBD: When two peers have the same path, but different addresses and also send the same data.

