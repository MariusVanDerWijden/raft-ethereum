# Raft Ethereum

This package contains an implementation of the [RAFT consensus protocol](https://raft.github.io/) that drives Ethereum nodes via the Engine API.
It's supposed to be an easy way to spin up a multi-node test network.
It's meant to be easily expandable for other use cases and other consensus protocols (like clique etc.)

**This project is very alpha and has not been tested or reviewed properly.**

**Do not use in production yet!**

![Ethereum on a Raft](./image.jpg)

# Structure

The consensus package contains the RAFT algorithm. It interacts with the nodes via the interfaces defined in `engine/api.go` and with the network via `network/api.go`. This abstraction allows to implement different network or engine adapters based on different use cases. 
The current implementations are:

|Interface |Implementation| Purpose |
|-----|--------|------|
| engine | local.go       | locally spin up a node from code, used for testing |
| engine | remote.go      | (Not implemented yet) Connect to a standalone remote node via the engine api |
| network | local.go | spin up a network of pipes between different threads, used for testing |
| network | static.go | Connect to a list of statically defined nodes | 


# Contributing

Contributions are very welcome! 
The code is distributed under MIT, so feel free to take it and add to it.

Good first issues would be to add:
- `engine/remote.go` to connect to a standalone node via the engine api and JWT
- `network/dynamic.go` dynamically define a network where nodes can join or leave
- `consensus/clique.go` implement the clique consensus algorithm

etc. 