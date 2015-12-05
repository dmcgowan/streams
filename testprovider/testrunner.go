package testprovider

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/dmcgowan/streams"
)

// RunTestReset tests that the connection may be reset from
// either end and future writes will be disallowed.
func RunTestReset(t *testing.T, e1, e2 streams.StreamProvider) {
	server := func(provider streams.StreamProvider) error {
		listener := provider.Listen()
		for {
			stream, err := listener.Accept()
			if err != nil {
				if err != io.EOF {
					return err
				}
				return nil
			}
			if err := stream.Reset(); err != nil {
				return err
			}

			// Fails due to spdystream bug
			// https://github.com/docker/spdystream/issues/45
			if _, err := stream.Write([]byte("some value")); err == nil {
				return fmt.Errorf("Expected error writing after reset")
			}
		}
	}
	client := func(provider streams.StreamProvider) error {
		stream, err := provider.NewStream(nil)
		if err != nil {
			return err
		}

		b := make([]byte, 10)
		if n, err := stream.Read(b); err != nil && err != io.EOF {
			return err
		} else if err == nil && n > 0 {
			return fmt.Errorf("Expected read of %d bytes", n)
		} else if err == nil {
			return fmt.Errorf("Expected error reading from stream")
		}
		return nil
	}
	runTest(t, e1, e2, client, server)
}

func checkHeaderValue(h http.Header, key, expected string) error {
	actual := h.Get(key)
	if actual != expected {
		return fmt.Errorf("Unexpected header value for %q: %q, expected %q", key, actual, expected)
	}
	return nil
}

// RunTestHeader tests that headers are sent and received by the provider
func RunTestHeader(t *testing.T, e1, e2 streams.StreamProvider) {
	server := func(provider streams.StreamProvider) error {
		listener := provider.Listen()
		for i := 0; ; i++ {
			stream, err := listener.Accept()
			if err != nil {
				if err != io.EOF {
					return err
				}
				return nil
			}
			headers := stream.Headers()
			if err := checkHeaderValue(headers, "Test-Header-Origin", "client"); err != nil {
				return err
			}
			expected := fmt.Sprintf("value-%d", i)
			if err := checkHeaderValue(headers, "Counter", expected); err != nil {
				return err
			}

			if err := stream.Close(); err != nil {
				return err
			}
		}
	}
	client := func(provider streams.StreamProvider) error {
		for i := 0; i < 11; i++ {
			headers := http.Header{}
			headers.Add("test-header-origin", "client")
			headers.Add("counter", fmt.Sprintf("value-%d", i))
			stream, err := provider.NewStream(headers)
			if err != nil {
				return err
			}
			b := make([]byte, 1)
			if _, err := stream.Read(b); err != nil && err != io.EOF {
				return err
			}
			if err := stream.Close(); err != nil {
				return err
			}
		}
		return nil
	}
	runTest(t, e1, e2, client, server)
}

// RunTestReadWrite ensures that streams created by the provider can
// be read and written to.
func RunTestReadWrite(t *testing.T, e1, e2 streams.StreamProvider) {
	server := func(provider streams.StreamProvider) error {
		stream, err := provider.Listen().Accept()
		if err != nil {
			return err
		}

		for i := 1; ; i++ {
			b := make([]byte, i)
			n, err := stream.Read(b)
			if err != nil {
				if err == io.EOF {
					if n, err := stream.Write([]byte("goodbye")); err != nil {
						return err
					} else if n != 7 {
						return fmt.Errorf("Unexpected number of bytes written: %d, expected 7", n)
					}
					return stream.Close()
				}
				return err
			}
			if n != i {
				return fmt.Errorf("Unexpected number of bytes read: %d, expected %d", n, i)
			}
			for j := 0; j < i; j++ {
				if b[j] != byte(j) {
					return fmt.Errorf("Unexpected byte value: %x, expected %x", b[j], j)
				}
			}
		}
	}
	client := func(provider streams.StreamProvider) error {
		stream, err := provider.NewStream(nil)
		if err != nil {
			return err
		}
		for i := 1; i < 254; i++ {
			b := make([]byte, i)
			for j := 0; j < i; j++ {
				b[j] = byte(j)
			}
			n, err := stream.Write(b)
			if err != nil {
				return err
			}
			if n != i {
				return fmt.Errorf("Unexpected number of bytes written: %d, expected %d", n, i)
			}
		}
		// Half close
		if err := stream.Close(); err != nil {
			return err
		}

		b := make([]byte, 7)
		n, err := stream.Read(b)
		if n != 7 {
			return fmt.Errorf("Unexpected number of bytes read: %d, expected %d", n, 7)
		}
		if expected := "goodbye"; string(b) != expected {
			return fmt.Errorf("Unexpected value read: %s, expected %s", b, expected)
		}
		return nil
	}
	runTest(t, e1, e2, client, server)
}

type endpointHandler func(streams.StreamProvider) error

func runEndpoint(provider streams.StreamProvider, f endpointHandler, errChan chan error) {
	defer close(errChan)
	defer provider.Close()
	if err := f(provider); err != nil {
		errChan <- err
	}
}

func runTest(t *testing.T, client, server streams.StreamProvider, clientFunc, serverFunc endpointHandler) {
	serverDone := make(chan error, 1)
	clientDone := make(chan error, 1)
	go runEndpoint(server, serverFunc, serverDone)
	go runEndpoint(client, clientFunc, clientDone)
	timeout := time.After(50 * time.Millisecond)
	for clientDone != nil || serverDone != nil {
		select {
		case err := <-clientDone:
			if err != nil {
				t.Fatalf("Client error: %s", err)
			}
			t.Logf("Client done")
			clientDone = nil
		case err := <-serverDone:
			if err != nil {
				t.Fatalf("Server error: %s", err)
			}
			t.Logf("Server done")
			serverDone = nil
		case <-timeout:
			pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
			t.Fatalf("Timeout!")
		}
	}
}
