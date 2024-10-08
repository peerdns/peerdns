# General TODO

- [x] Proper identity management package needs to be built, at least the basic stuff.
- [x] First phase is to fully connect validators with each other over DHT.
- [x] There should be a genesis in which, validators including initial block information is created. Should be yaml and not json...
- [x] Pinging mesh network so we can collect statistics about the network itself.
- [x] Networking needs hosts.go where actual host management, creation, etc... will be done...
- [x] Consensus, metrics separation within networking package needs to happen. 
- [x] Observability package needs to support with TLS connectivity to GRPC (traces) and there should be nice way to do this.
- [ ] Light access to the DAG database and introspection...
- [ ] TXP (TxPool) synchronization between different peers.
- 


# General Cleanups

- [ ] All types should be at one place, in pkg/types. Right now they are all over the place.
- 