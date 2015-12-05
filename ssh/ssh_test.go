package sshstream

import (
	"net"
	"os"
	"runtime/pprof"
	"sync"
	"testing"
	"time"

	"github.com/dmcgowan/streams"
	"github.com/dmcgowan/streams/testprovider"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/testdata"
)

func TestSSHConnection(t *testing.T) {
	clientConn, serverConn, err := netPipe()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Creating server config")
	serverConfig := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	s, err := ssh.ParsePrivateKey(testdata.PEMBytes["dsa"])
	if err != nil {
		t.Fatal(err)
	}
	serverConfig.AddHostKey(s)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		t.Logf("Starting server")
		server, err := NewSSHServerStreamProvider(serverConn, serverConfig)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Close server")
		if err := server.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	go func() {
		defer wg.Done()
		t.Logf("Connecting from Client")
		client, err := NewSSHClientStreamProvider(clientConn, "", &ssh.ClientConfig{})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Close client")
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	wg.Wait()
}

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
	c1, c2, err := netPipe()
	if err != nil {
		t.Fatal(err)
	}
	done1 := make(chan error, 1)
	done2 := make(chan error, 1)

	serverConfig := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	s, err := ssh.ParsePrivateKey(testdata.PEMBytes["dsa"])
	if err != nil {
		t.Fatal(err)
	}
	serverConfig.AddHostKey(s)

	go func() {
		var err error
		p2, err = NewSSHServerStreamProvider(c2, serverConfig)
		if err != nil {
			done1 <- err
		}
		close(done1)
	}()
	go func() {
		var err error
		p1, err = NewSSHClientStreamProvider(c1, "", &ssh.ClientConfig{})
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

// netPipe is analogous to net.Pipe, but it uses a real net.Conn, and
// therefore is buffered (net.Pipe deadlocks if both sides start with
// a write.)
func netPipe() (net.Conn, net.Conn, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	defer listener.Close()
	c1, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		return nil, nil, err
	}

	c2, err := listener.Accept()
	if err != nil {
		c1.Close()
		return nil, nil, err
	}

	return c1, c2, nil
}
