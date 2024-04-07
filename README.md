# dndm

DNDM stands for Decentralized Named Data Messaging. The inspiration for such a system comes from LibP2P, Named Data Networks, general Pub/Sub architectures, and ROS.
The main purpose of this library is to provide a framework for seamless decentralized, distributed, and modular architectures mainly for robotic applications.

This is the initial version. It focuses on efficient delivery within the same process, multiple processes using sockets, and LAN using UDP/TCP.

The main goal is to be able to connect multiple independent modules that can stream data from each other, communicate with each other using named and typed channels. 
The connection between these modules should happen automagically. For example, the networked version should either be able to discover each node and self-arrange into a full-mesh network.
Socket communicatino would be more restricted since you'd have to specify what nodes communicate with what other nodes.
