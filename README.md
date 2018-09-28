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
the local IP address range (e.g. `10.0.0.0/16`), and the local authoritative DNS master (e.g. `10.0.0.1`). It presents
itself as a DNS server that listens for RFC1996 zone update notifications from the local DNS master, initiates zone
transfers from that master (using RFC1995 IFXRs when possible), and listens for RFC2136 dynamic DNS update commands from
the Hive instances of other sites.

These host device records are transformed to the rendezvous DNS search path suffix (e.g. `rdvu.example.com`), and then
forwarded as CNAME mappings via RFC2136 updates to the site's primary DNS server. Host address mappings from the local
master will supersede any remote peer mappings.

The role of each Hive instance is to augment the local DNS master records and communicate the necessary information to
its peers at other sites. Dynamic update queries from e.g. DHCP servers, and all client requests shall be served only
by that DNS master. This minimizes the complexity of rendezvous coordination, best interoperates with typical dynamic
DNS/DHCP site configurations, and assures minimal network disruption in the event of a Hive instance failure.

## Result

- For `foo` on site `west.example.com` only, all clients will resolve `foo.rdvu.example.com` &rarr;
  `foo.west.example.com` &rarr; `10.0.0.100`
- For `foo` tunneled to both `west.example.com` and `east.example.com`, clients on the `west` site will resolve
  `foo.rdvu.example.com` &rarr; `foo.west.example.com` &rarr; `10.0.0.100`, but clients on the `east` site will resolve
  e.g. `foo.rdvu.example.com` &rarr; `foo.east.example.com` &rarr; `10.1.0.103`
- For `foo` and `bar` both tunneled to the `west` and `east` sites trying to connect to each other, both clients will
  resolve based on their own tunnel priority order, but should achieve connectivity regardless. In practice, multi-site
  tunneling clients should order their tunnels from highest to lowest tunnel link bandwidth for best performance.

## Site Configuration

- DHCP Server (e.g. ISC dhcpd) configured to update A and PTR records on the local DNS master (i.e. `zone` designating 
  the local DNS master server as `primary` for `west.example.com`). ISC dhcpd configuration permits only one DNS server
  for updates (sent as RFC2136 update commands).
- DNS Server (e.g. ISC BIND) configured as the authoritative master for both the local site namespace (e.g.
  `west.example.com`) and the rendezvous namespace (e.g. `rdvu.example.com`). The local site namespace shall be
  configured to notify the Hive instance as a "slave" DNS server (e.g. `notify explicit;` and `also-notify { <hive
  instance IP> };`). The local Hive instance must be permitted to perform zone transfers (e.g. `allow-transfer { ... };`
  ).
- For security reasons, both the update commands and zone transfer should be TSIG authenticated.
