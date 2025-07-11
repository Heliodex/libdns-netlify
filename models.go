package netlify

import (
	"time"

	"github.com/libdns/libdns"
	"github.com/netlify/open-api/v2/go/models"
)

type netlifyZone struct {
	*models.DNSZone
}

type netlifyDNSRecord struct {
	*models.DNSRecord
}

func (r netlifyDNSRecord) libdnsRecord(zone string) (libdns.Record, error) {
	return libdns.RR{
		Type:     r.Type,
		Name:     libdns.RelativeName(r.Hostname, zone),
		Data:    r.Value,
		TTL:      time.Duration(r.TTL) * time.Second,
	}.Parse()
}

func netlifyRecord(r libdns.Record) netlifyDNSRecord {
	rr := r.RR()

	return netlifyDNSRecord{
		&models.DNSRecord{
			Type:     rr.Type,
			Hostname: rr.Name,
			Value:    rr.Data,
			TTL:      int64(rr.TTL.Seconds()),
		},
	}
}

type netlifyDNSDeleteError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
