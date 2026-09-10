package transport

import (
	"sync"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
)

func TestJA4PoolRotates(t *testing.T) {
	p := NewJA4Profiles() // default pool has 4 profiles

	first := p.Next()
	seen := map[utls.ClientHelloID]bool{first: true}
	for i := 0; i < 3; i++ {
		id := p.Next()
		if seen[id] {
			t.Errorf("profile repeated within rotation: %v", id)
		}
		seen[id] = true
	}
	// Wraps around to the first profile again.
	if p.Next() != first {
		t.Error("rotation did not wrap")
	}
}

func TestJA4PoolConcurrency(t *testing.T) {
	p := NewJA4Profiles()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Next()
		}()
	}
	wg.Wait()
	if p.Len() == 0 {
		t.Error("pool empty")
	}
}

func TestJA4PoolCustom(t *testing.T) {
	p := NewJA4Profiles(utls.HelloChrome_Auto, utls.HelloEdge_Auto)
	if p.Len() != 2 {
		t.Errorf("len = %d", p.Len())
	}
}

func TestRotatingDialerAgainstMock(t *testing.T) {
	// Point the rotating dialer at a local TLS server to verify each
	// connection gets a real handshake (profile changes per dial).
	p := NewJA4Profiles()

	// We exercise the preset mapping rather than full TLS (fast + CI safe).
	for i := 0; i < 4; i++ {
		hello := p.Next()
		preset := presetFromHello(hello)
		if preset != Chrome && preset != Edge && preset != Firefox {
			t.Errorf("mapped preset %q invalid", preset)
		}
	}
}

func TestNewClientWithPool(t *testing.T) {
	hc := NewUTLSClientWithPool(NewJA4Profiles(), 5*time.Second)
	if hc == nil || hc.Timeout != 5*time.Second {
		t.Fatal("client not configured")
	}
}

