# Hive

Dynamic DNS update transformation for consistent multi-site hostname rendezvous

## Why?

Network sites using DHCP driven dynamic DNS updates are easily configured to provide automatic resolution of client
device hostnames in some DNS namespace.

For a `west.example.com` site, hosts `foo`, `bar`, and `baz` may be mapped as follows:
- `foo.west.example.com` &rarr; `10.0.0.100`
- `bar.west.example.com` &rarr; `10.0.0.101`
- `baz.west.example.com` &rarr; `10.0.0.102`

For an `east.example.com` site, hosts `qux` and `corge` may be mapped as follows:
- `qux.east.example.com` &rarr; `10.1.0.100`
- `corge.east.example.com` &rarr; `10.1.0.101`

If the `west.example.com` and `east.example.com` sites are linked (e.g. by VPN tunnels), communication between e.g.
`foo.west.example.com` and `qux.east.example.com` could be achieved by `foo` querying the A record for
`qux.east.example.com` on the `east.example.com` authoritative DNS server and receiving `10.1.0.100` as an answer, then
establishing a connection over the inter-site tunnel.

However, if clients can move between sites, the host `qux` may at different times be mapped to `qux.east.example.com` or
`qux.west.example.com`. Furthermore, off-site clients may establish their own VPN tunnels to one or more sites
simultaneously to access resources under e.g. both `west.example.com` and `east.example.com`, and where unnecessary
utilization of the VPN link between `west` and `east` is undesirable (e.g. multiple slow WAN link crossings).

In this situation, ideally hosts would be identified with a virtual rendezvous identifier, e.g. `qux.rdvu.example.com`,
where `rdvu.example.com` contains CNAME records mapping `qux.rdvu.example.com` &rarr; `qux.west.example.com` and/or to
`qux.east.example.com` depending on network locality (either by direct connectivity or off-site tunnels).

Hive manages these rendezvous sub-domain namespaces, and prioritizes mappings to network local identifiers before
mapping to off-site identifiers.

## How?

A Hive instance is configured to run on each site with knowledge of the local DNS search path (e.g. `west.example.com`),
and the local IP address range (e.g. `10.0.0.0/16`). It presents itself as an RFC2136 compatible DNS server, listening
for DHCP driven dynamic DNS update commands from both the local DHCP server, and the Hive instances of other sites.

These host device records are transformed to the rendezvous DNS search path suffix (e.g. `rdvu.example.com`), and then
forwarded as CNAME mappings via RFC2136 updates to the site's primary DNS server. Host address mappings from the local
IP address range will supersede any remote IP address range. Host address mappings from remote IP address ranges will be
dropped if a mapping from a local IP address range is active.

## Result

- For `foo` on site `west.example.com` only, all clients will resolve `foo.rdvu.example.com` &rarr;
  `foo.west.example.com` &rarr; `10.0.0.100`
- For `foo` tunneled to both `west.example.com` and `east.example.com`, clients on the `west` site will resolve
  `foo.rdvu.example.com` &rarr; `foo.west.example.com` &rarr; `10.0.0.100`, but clients on the `east` site will resolve
  e.g. `foo.rdvu.example.com` &rarr; `foo.east.example.com` &rarr; `10.1.0.103`
- For `foo` and `bar` both tunneled to the `west` and `east` sites trying to connect to each other, both clients will
  resolve based on their own tunnel priority order, but should achieve connectivity regardless. In practice, multi-site
  tunneling clients should order their tunnels from highest to lowest tunnel link bandwidth for best performance.
