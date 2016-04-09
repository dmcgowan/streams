package spdy

import (
	"net"
	"os"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/dmcgowan/streams"
	"github.com/dmcgowan/streams/testprovider"
)

func TestReset(t *testing.T) {
	e1, e2 := providerPair(t)
	testprovider.RunTestReset(t, e1, e2)
}

func TestHeader(t *testing.T) {
	e1, e2 := providerPair(t)
	testprovider.RunTestHeader(t, e1, e2)
}

func TestReadWrite(t *testing.T) {
	e1, e2 := providerPair(t)
	testprovider.RunTestReadWrite(t, e1, e2)
}

func providerPair(t *testing.T) (p1 streams.StreamProvider, p2 streams.StreamProvider) {
	c1, c2 := net.Pipe()
	done1 := make(chan error, 1)
	done2 := make(chan error, 1)
	go func() {
		var err error
		p2, err = NewSpdyStreamProvider(c1, true)
		if err != nil {
			done1 <- err
		}
		close(done1)
	}()
	go func() {
		var err error
		p1, err = NewSpdyStreamProvider(c2, false)
		if err != nil {
			done2 <- err
		}
		close(done2)
	}()
	timeout := time.After(50 * time.Millisecond)
	for done1 != nil || done2 != nil {
		select {
		case err := <-done1:
			if err != nil {
				t.Fatalf("Client error: %s", err)
			}
			done1 = nil
		case err := <-done2:
			if err != nil {
				t.Fatalf("Server error: %s", err)
			}
			done2 = nil
		case <-timeout:
			pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
			t.Fatalf("Timeout!")
		}
	}

	return
}
