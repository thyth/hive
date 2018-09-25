package xform

import (
	"github.com/miekg/dns"
	"github.com/thyth/hive/conf"

	"net"
	"time"
)

func WriteUpdate(dnsServer net.Addr, config *conf.Configuration, key *conf.TsigKey, mapping *Mapping) error {
	msg := &dns.Msg{}
	msg.Opcode = dns.OpcodeUpdate
	msg.SetQuestion(config.SearchSuffix, dns.TypeSOA)
	var rr dns.RR
	if mapping.IP != nil {
		if len(mapping.IP) == net.IPv4len {
			rr = &dns.A{
				Hdr: dns.RR_Header{
					Name:   mapping.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    config.TTL,
				},
				A: mapping.IP,
			}
		} else {
			rr = &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   mapping.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    config.TTL,
				},
				AAAA: mapping.IP,
			}
		}
	} else {
		rr = &dns.CNAME{
			Hdr: dns.RR_Header{
				Name:   mapping.Name,
				Rrtype: dns.TypeCNAME,
				Class:  dns.ClassINET,
				Ttl:    config.TTL,
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
