package xform

import (
	"github.com/miekg/dns"
	"github.com/thyth/hive/conf"

	"net"
	"time"
)

func WriteUpdate(dnsServer net.Addr, ttl uint32, key *conf.TsigKey, mapping *Mapping, zone string) error {
	msg := &dns.Msg{}
	msg.Opcode = dns.OpcodeUpdate
	msg.SetQuestion(zone, dns.TypeSOA)
	var rr dns.RR
	class := uint16(dns.ClassINET)
	if mapping.IP == nil && mapping.Target == "" {
		class = uint16(dns.ClassANY)
		ttl = 0
	}
	if mapping.IP != nil {
		if len(mapping.IP) == net.IPv4len {
			rr = &dns.A{
				Hdr: dns.RR_Header{
					Name:   mapping.Name,
					Rrtype: dns.TypeA,
					Class:  class,
					Ttl:    ttl,
				},
				A: mapping.IP,
			}
		} else {
			rr = &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   mapping.Name,
					Rrtype: dns.TypeAAAA,
					Class:  class,
					Ttl:    ttl,
				},
				AAAA: mapping.IP,
			}
		}
	} else {
		rr = &dns.CNAME{
			Hdr: dns.RR_Header{
				Name:   mapping.Name,
				Rrtype: dns.TypeCNAME,
				Class:  class,
				Ttl:    ttl,
			},
			Target: mapping.Target,
		}
	}
	msg.Ns = []dns.RR{rr}

	cli := &dns.Client{}
	cli.TsigSecret = map[string]string{key.ZoneName: key.Key}
	msg.SetTsig(key.ZoneName, key.Algorithm, 300, time.Now().Unix())
	_, _, err := cli.Exchange(msg, dnsServer.String()+":53")
	// read success status from reply?
	return err
}
