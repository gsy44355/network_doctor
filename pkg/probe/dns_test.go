package probe

import (
	"context"
	"net"
	"testing"
)

func TestDNSProbe_SkipsForIP(t *testing.T) {
	p := &DNSProbe{}
	target := &Target{Raw: "1.2.3.4:80", IP: "1.2.3.4", Port: 80, IsIP: true, Scheme: "http"}
	result := p.Run(context.Background(), target, nil)
	if result.Status != StatusSkipped {
		t.Errorf("expected Skipped for IP target, got %v", result.Status)
	}
}

func TestDNSProbe_ResolvesHostname(t *testing.T) {
	p := &DNSProbe{}
	target := &Target{Raw: "google.com", Host: "google.com", Port: 443, Scheme: "https"}
	result := p.Run(context.Background(), target, nil)
	if result.Status == StatusError {
		t.Skipf("DNS resolution failed (network issue): %s", result.Message)
	}
	if result.DNS == nil {
		t.Fatal("DNS details should be populated")
	}
	if len(result.DNS.IPv4) == 0 && len(result.DNS.IPv6) == 0 {
		t.Error("expected at least one IP address")
	}
}

func TestDNSConsistencyTreatsPublicResolverTransportErrorAsUnknown(t *testing.T) {
	details := &DNSDetails{}
	err := &net.DNSError{Err: "timeout", Name: "internal.example", IsTimeout: true}

	applyPublicDNSConsistency(nil, err, details)

	if details.InternalDomain {
		t.Fatal("transport errors should not be classified as internal domains")
	}
	if details.PublicDNSError == "" {
		t.Fatal("public DNS error should be recorded")
	}
}

func TestDNSConsistencyTreatsPublicNXDomainAsInternalDomain(t *testing.T) {
	details := &DNSDetails{}
	err := &net.DNSError{Err: "no such host", Name: "internal.example", IsNotFound: true}

	applyPublicDNSConsistency(nil, err, details)

	if !details.InternalDomain {
		t.Fatal("NXDOMAIN from public DNS should indicate an internal-only domain")
	}
}
