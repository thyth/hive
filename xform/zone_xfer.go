package xform

import (
	"github.com/miekg/dns"
	"github.com/thyth/hive/conf"

	"net"
	"sync"
	"time"
)

type Zone struct {
	sync.Mutex
	Server net.Addr
	M      map[string]net.IP
}

// ReadZoneEntries will zone transfer and look at A and AAAA records.
func ReadZoneEntries(dnsServer net.Addr, key *conf.TsigKey, zone string) (*Zone, error) {
	axfr := &dns.Transfer{
		TsigSecret: map[string]string{
			key.ZoneName: key.Key,
		},
	}
	msg := &dns.Msg{}
	msg.SetAxfr(zone)
	msg.SetTsig(key.ZoneName, key.Algorithm, 300, time.Now().Unix())
	envelopes, err := axfr.In(msg, dnsServer.String()+":53")
	if err != nil {
		return nil, err
	}
	m := map[string]net.IP{}
	for envelope := range envelopes {
		if envelope.Error != nil {
			return nil, envelope.Error
		}
		records := envelope.RR
		for _, record := range records {
			switch record := record.(type) {
			case *dns.A:
				m[record.Hdr.Name] = record.A
			case *dns.AAAA:
				m[record.Hdr.Name] = record.AAAA
			}
		}
	}
	return &Zone{Server: dnsServer, M: m}, nil
}
