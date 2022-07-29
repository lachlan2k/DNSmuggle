# DNSmuggle

A tool to tunnel UDP datagrams over DNS.
This is most useful when coupled with something like Wireguard, to tunnel your internet traffic in a nice and lightweight fashion.

Features:
* Bypass firewalls, such as hotspot paywalls and air-gapped networks, so long as you can resolve public DNS.
* Datagram fragmentation and re-assembly to support large datagrams.
* Avoids caching issues.
* Bypasses basic mitigations, such as case-mixing.
* Encrypted "control-channel" packets, using a pre-shared key.
* Multiple simultaneous clients and sessions per-client.

Data is not encrypted or otherwise protected over the wire -- you should use a protocol such as wireguard to provide cryptographic properties.

DNSmuggle only uses encryption when the client asks the server to open a channel. Right now, this is theoretically vulnerable to replays, but only to re-open the same destination. The encryption prevents a malicious actor from asking the server to dial arbitrary addresses.

## Build

```sh
# Make server
go build cmd/server.go

# Make client
go build cmd/client.go
```

## Set up your domain

You'll want a nice and short domain for this, as space is valuable.
Configure the root NS record to an A record that resolves to your tunnel server. Depending on your registrar and TLD, you may need to configure glue records.

## Run server
```
./server -listenAddr 0.0.0.0:53 -domain example.com -psk hunter2 --nameserver ns1.example.com
```

## Run client
```
./client -dialAddr 10.1.1.1:51820 -domain example.com -psk hunter2 -resolver 1.1.1.1:53 -listenAddr 127.0.0.1:51280
```

Now, on your client machine, a listener will open on `127.0.0.1:51820`. When it receives packets, it will make an encrypted DNS request to open a channel, asking the server to dial `10.1.1.1:51820` (server-side). Data will then be encoded and flow through the DNS tunnel bi-directionally.