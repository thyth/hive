package xform

import (
	"github.com/miekg/dns"
	"github.com/thyth/hive/conf"

	"fmt"
)

func StartServer(config conf.Configuration, key conf.TsigKey) {
	server := &dns.Server{
		Addr: config.BindAddress.String() + ":53",
		Net: "udp",
		TsigSecret: map[string]string{key.ZoneName: key.Key},
	}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
	dns.HandleFunc(".", handlerGenerator(config, key))
}

func handlerGenerator(config conf.Configuration, key conf.TsigKey) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		fmt.Println(r.String())
		msg := &dns.Msg{}
		msg.SetReply(r)
		w.WriteMsg(msg)

		// TODO peer requests will be AXFRs when they initialize, and update records from DHCP servers
	}
}
