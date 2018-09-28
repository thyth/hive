package xform

import (
	"github.com/miekg/dns"
	"github.com/thyth/hive/conf"

	"fmt"
	"net"
	"time"
)

type CNAMECallback func(proposer net.Addr, name string, target string)
type ACallback func(proposer net.Addr, name string, target net.IP)
type SerialCallback func(zone string) uint32
type TransferCallback func(zone string) []*Mapping

type Mapping struct {
	Name   string
	Target string
	IP     net.IP
}

type PeerCallbacks struct {
	CNAME    CNAMECallback
	A        ACallback
	AAAA     ACallback
	Serial   SerialCallback
	Transfer TransferCallback
}

func StartServer(config *conf.Configuration, key *conf.TsigKey, callbacks *PeerCallbacks) {
	tsig := map[string]string{key.ZoneName: key.Key}

	// run both UDP and TCP, since TCP is usually used for zone transfers
	serverUdp := &dns.Server{
		Addr:       config.BindAddress.String() + ":53",
		Net:        "udp",
		TsigSecret: tsig,
	}
	serverTcp := &dns.Server{
		Addr:       config.BindAddress.String() + ":53",
		Net:        "tcp",
		TsigSecret: tsig,
	}

	go func() {
		if err := serverUdp.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
	go func() {
		if err := serverTcp.ListenAndServe(); err != nil {
			panic(err)
		}
	}()

	dns.HandleFunc(".", handlerGenerator(config, key, callbacks))
}

func handlerGenerator(config *conf.Configuration, key *conf.TsigKey, callbacks *PeerCallbacks) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, request *dns.Msg) {
		msg := &dns.Msg{}
		msg.SetReply(request)

		// if tsig is absent or invalid...
		if request.IsTsig() == nil || w.TsigStatus() != nil {
			// ... abort further processing
			w.WriteMsg(msg)
			return
		}

		if request.Opcode == dns.OpcodeUpdate {
			// add/delete records
			validZoneUpdate := false
			for _, question := range request.Question {
				if question.Qtype == dns.TypeSOA {
					validZoneUpdate = true
				}
			}
			if validZoneUpdate {
				for _, authority := range request.Ns {
					proposerHost, _, err := net.SplitHostPort(w.RemoteAddr().String())
					if err != nil {
						continue
					}
					proposer := &net.IPAddr{IP: net.ParseIP(proposerHost)}

					switch authority := authority.(type) {
					case *dns.CNAME:
						if callbacks != nil && callbacks.CNAME != nil {
							callbacks.CNAME(proposer, authority.Hdr.Name, authority.Target)
						}
					case *dns.A:
						if callbacks != nil && callbacks.A != nil {
							callbacks.A(proposer, authority.Hdr.Name, authority.A)
						}
					case *dns.AAAA:
						if callbacks != nil && callbacks.AAAA != nil {
							callbacks.AAAA(proposer, authority.Hdr.Name, authority.AAAA)
						}
					}
				}
				// sign the reply
				msg.SetTsig(key.ZoneName, key.Algorithm, 300, time.Now().Unix())
			}
		} else if request.Opcode == dns.OpcodeQuery {
			// zone transfers
			for _, question := range request.Question {
				if question.Qclass == dns.ClassINET &&
					question.Qtype == dns.TypeAXFR {
					zone := question.Name
					var records []*Mapping
					if callbacks != nil && callbacks.Transfer != nil {
						records = callbacks.Transfer(zone)
					}
					if len(records) == 0 {
						// not authoritative for this zone
						continue
					}
					fmt.Printf("Transferring zone '%v'\n", zone)
					ch := make(chan *dns.Envelope)
					tr := &dns.Transfer{}
					go tr.Out(w, request, ch)
					soa := &dns.SOA{
						Hdr: dns.RR_Header{
							Name:   zone,
							Rrtype: dns.TypeSOA,
							Class:  dns.ClassINET,
							Ttl:    config.TTL,
						},
						Ns:      "ns." + zone,
						Mbox:    "ns." + zone,
						Serial:  callbacks.Serial(zone),
						Refresh: config.TTL,
						Retry:   config.TTL / 10,
						Expire:  config.TTL * 2,
						Minttl:  config.TTL * 2,
					}
					ch <- &dns.Envelope{RR: []dns.RR{
						soa,
						&dns.A{
							Hdr: dns.RR_Header{
								Name:   "ns." + zone,
								Rrtype: dns.TypeA,
								Class:  dns.ClassINET,
								Ttl:    config.TTL,
							},
							A: net.ParseIP(config.BindAddress.String()),
						},
					}}
					// send records from callback
					for _, record := range records {
						var rr dns.RR
						if record.IP != nil {
							if len(record.IP) == net.IPv4len {
								rr = &dns.A{
									Hdr: dns.RR_Header{
										Name:   record.Name,
										Rrtype: dns.TypeA,
										Class:  dns.ClassINET,
										Ttl:    config.TTL,
									},
									A: record.IP,
								}
							} else {
								rr = &dns.AAAA{
									Hdr: dns.RR_Header{
										Name:   record.Name,
										Rrtype: dns.TypeAAAA,
										Class:  dns.ClassINET,
										Ttl:    config.TTL,
									},
									AAAA: record.IP,
								}
							}
						} else {
							rr = &dns.CNAME{
								Hdr: dns.RR_Header{
									Name:   record.Name,
									Rrtype: dns.TypeCNAME,
									Class:  dns.ClassINET,
									Ttl:    config.TTL,
								},
								Target: record.Target,
							}
						}
						ch <- &dns.Envelope{RR: []dns.RR{rr}}
					}
					ch <- &dns.Envelope{RR: []dns.RR{soa}}
					close(ch)
					w.Hijack()
				}
			}
		}
		w.WriteMsg(msg)
	}
}
