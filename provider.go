package netlify

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/libdns/libdns"
)

// Provider implements the libdns interfaces for Netlify.
type Provider struct {
	// Personal Access Token is required to Authenticate
	// yourself to Netlify's API
	PersonalAccessToken string `json:"personal_access_token,omitempty"`
	zones               map[string]netlifyZone
	zonesMu             sync.Mutex
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	zoneInfo, err := p.getZoneInfo(ctx, zone)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/dns_zones/%s/dns_records", baseURL, zoneInfo.ID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	var result []netlifyDNSRecord
	err = p.doAPIRequest(req, false, false, true, false, &result)
	if err != nil {
		return nil, err
	}

	recs := make([]libdns.Record, 0, len(result))
	for _, rec := range result {
		libdnsRec, err := rec.libdnsRecord(zone)
		if err != nil {
			return nil, fmt.Errorf("parsing Netlify DNS record %+v: %w", rec, err)
		}
		recs = append(recs, libdnsRec)
	}

	return recs, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	zoneInfo, err := p.getZoneInfo(ctx, zone)
	if err != nil {
		return nil, err
	}

	var created []libdns.Record
	for _, rec := range records {
		result, err := p.createRecord(ctx, zoneInfo, rec)
		if err != nil {
			return nil, err
		}
		libdnsRec, err := result.libdnsRecord(zone)
		if err != nil {
			return nil, fmt.Errorf("parsing Netlify DNS record %+v: %w", rec, err)
		}
		created = append(created, libdnsRec)
	}

	return created, nil
}

// DeleteRecords deletes the records from the zone. If a record does not have an ID,
// it will be looked up. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	zoneInfo, err := p.getZoneInfo(ctx, zone)
	if err != nil {
		return nil, err
	}

	var recs []libdns.Record
	for _, rec := range records {
		// record ID is required; try to find it with what was provided
		exactMatches, err := p.getDNSRecords(ctx, zoneInfo, rec, true)
		if err != nil {
			return nil, err
		}

		for _, nRec := range exactMatches {
			reqURL := fmt.Sprintf("%s/dns_zones/%s/dns_records/%s", baseURL, zoneInfo.ID, nRec.ID)

			req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
			if err != nil {
				return nil, err
			}

			var result netlifyDNSRecord
			err = p.doAPIRequest(req, false, false, true, true, &result)
			if err != nil {
				return nil, err
			}

			libdnsRec, err := result.libdnsRecord(zone)
			if err != nil {
				return nil, fmt.Errorf("parsing Netlify DNS record %+v: %w", result, err)
			}
			recs = append(recs, libdnsRec)

			req, err = http.NewRequestWithContext(ctx, "DELETE", reqURL, nil)
			if err != nil {
				return nil, err
			}

			err = p.doAPIRequest(req, false, true, false, true, &result)
			if err != nil {
				return nil, err
			}
		}

	}

	return recs, nil
}

// SetRecords sets the records in the zone, either by updating existing records
// or creating new ones. It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	zoneInfo, err := p.getZoneInfo(ctx, zone)
	if err != nil {
		return nil, err
	}

	var results []libdns.Record
	for _, rec := range records {
		oldRec := netlifyRecord(rec)
		oldRec.DNSZoneID = zoneInfo.ID

		// the record might already exist, even if we don't know the ID yet
		matches, err := p.getDNSRecords(ctx, zoneInfo, rec, false)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			// record doesn't exist; create it
			result, err := p.createRecord(ctx, zoneInfo, rec)
			if err != nil {
				return nil, err
			}
			libdnsRec, err := result.libdnsRecord(zone)
			if err != nil {
				return nil, fmt.Errorf("parsing Netlify DNS record %+v: %w", result, err)
			}
			results = append(results, libdnsRec)
			continue
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("unexpectedly found more than 1 record for %v", rec)
		}
		// record does exist, fill in the ID so that we can update it
		oldRec.ID = matches[0].ID

		// record exists; update it
		result, err := p.updateRecord(ctx, oldRec, netlifyRecord(rec))
		if err != nil {
			return nil, err
		}
		libdnsRec, err := result.libdnsRecord(zone)
		if err != nil {
			return nil, fmt.Errorf("parsing Netlify DNS record %+v: %w", result, err)
		}
		results = append(results, libdnsRec)
	}

	return results, nil
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
