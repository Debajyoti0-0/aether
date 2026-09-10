package transport

import (
	"sync"

	utls "github.com/refraction-networking/utls"
)

// JA4Pool rotates TLS client fingerprints across requests so defenders
// cannot pin and block a single JA4/JA3 hash mid-engagement.
type JA4Pool struct {
	mu       sync.Mutex
	profiles []utls.ClientHelloID
	current  int
}

// DefaultJA4Profiles is the rotation set: three real browser stacks
// plus the MSAL/broker profiles Entra clients actually present.
func DefaultJA4Profiles() []utls.ClientHelloID {
	return []utls.ClientHelloID{
		utls.HelloChrome_Auto,
		utls.HelloEdge_Auto,
		utls.HelloFirefox_Auto,
		utls.HelloRandomizedALPN,
	}
}

// NewJA4Pool builds a rotation pool (default profiles when empty).
func NewJA4Profiles(profiles ...utls.ClientHelloID) *JA4Pool {
	if len(profiles) == 0 {
		profiles = DefaultJA4Profiles()
	}
	return &JA4Pool{profiles: profiles}
}

// Next returns the next fingerprint in rotation.
func (p *JA4Pool) Next() utls.ClientHelloID {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := p.profiles[p.current%len(p.profiles)]
	p.current = (p.current + 1) % len(p.profiles)
	return id
}

// Current reports the profile that will be used on the next Next().
func (p *JA4Pool) Current() utls.ClientHelloID {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.profiles[p.current%len(p.profiles)]
}

// Len reports the pool size.
func (p *JA4Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.profiles)
}
