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
	Server       net.Addr
	ARecords     map[string]net.IP
	CNAMERecords map[string]string
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
	aRecords := map[string]net.IP{}
	cnameRecords := map[string]string{}
	for envelope := range envelopes {
		if envelope.Error != nil {
			return nil, envelope.Error
		}
		records := envelope.RR
		for _, record := range records {
			switch record := record.(type) {
			case *dns.A:
				aRecords[record.Hdr.Name] = record.A
			case *dns.AAAA:
				aRecords[record.Hdr.Name] = record.AAAA
			case *dns.CNAME:
				cnameRecords[record.Hdr.Name] = record.Target
			}
		}
	}
	return &Zone{
		Server:       dnsServer,
		ARecords:     aRecords,
		CNAMERecords: cnameRecords,
	}, nil
}
