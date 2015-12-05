package sshstream

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"

	"github.com/Sirupsen/logrus"
	"github.com/dmcgowan/streams"
	"golang.org/x/crypto/ssh"
)

type sshStream struct {
	channel ssh.Channel
	header  http.Header
}

type sshStreamListener struct {
	listenChan <-chan *sshStream
	closeChan  chan struct{}
}

type sshStreamProvider struct {
	conn        ssh.Conn
	listenChan  chan *sshStream
	closeChan   chan struct{}
	channelName string
}

type SSHProviderConfig struct {
	Conn           ssh.Conn
	NewChannelChan <-chan ssh.NewChannel
	ChannelName    string
}

func NewSSHStreamProvider(conf SSHProviderConfig) (streams.StreamProvider, error) {
	listenChan := make(chan *sshStream)
	p := &sshStreamProvider{
		conn:        conf.Conn,
		listenChan:  listenChan,
		closeChan:   make(chan struct{}),
		channelName: conf.ChannelName,
	}
	// Handle new requests
	go func() {
		for nc := range conf.NewChannelChan {
			// Parse Headers
			r := textproto.NewReader(bufio.NewReader(bytes.NewReader(nc.ExtraData())))
			h, err := r.ReadMIMEHeader()
			if err != nil && err != io.EOF {
				if err := nc.Reject(ssh.ConnectionFailed, err.Error()); err != nil {
					// TODO handle reject error
				}
				continue
			}
			c, _, err := nc.Accept()
			if err != nil {
				// TODO handle non-EOF accept error
				logrus.Printf("Received error accepting channel: %v", err)
				continue
			}
			s := &sshStream{
				channel: c,
				header:  http.Header(h),
			}
			select {
			case <-p.closeChan:
				// Reply
				// Teardown
				break
			case p.listenChan <- s:
			}
		}
	}()
	go func() {
		// TODO handle error
		p.conn.Wait()
		close(p.closeChan)
	}()

	return p, nil
}

func NewSSHClientStreamProvider(c net.Conn, addr string, conf *ssh.ClientConfig) (streams.StreamProvider, error) {
	conn, nChan, _, err := ssh.NewClientConn(c, addr, conf)
	if err != nil {
		return nil, fmt.Errorf("Error creating client: %v", err)
	}
	return NewSSHStreamProvider(SSHProviderConfig{
		Conn:           conn,
		NewChannelChan: nChan,
		ChannelName:    "stream",
	})
}

func NewSSHServerStreamProvider(c net.Conn, conf *ssh.ServerConfig) (streams.StreamProvider, error) {
	conn, nChan, _, err := ssh.NewServerConn(c, conf)
	if err != nil {
		return nil, err
	}
	// TODO handle permissions
	return NewSSHStreamProvider(SSHProviderConfig{
		Conn:           conn,
		NewChannelChan: nChan,
		ChannelName:    "stream",
	})
}

func (p *sshStreamProvider) NewStream(headers http.Header) (streams.Stream, error) {
	buf := bytes.NewBuffer(nil)
	if err := headers.Write(buf); err != nil {
		return nil, fmt.Errorf("Error writing header: %v", err)
	}
	channel, _, err := p.conn.OpenChannel(p.channelName, buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("Error opening channel: %v", err)
	}
	// TODO handle return headers
	return &sshStream{
		channel: channel,
	}, nil
}

func (p *sshStreamProvider) Close() error {
	return p.conn.Close()
}

func (p *sshStreamProvider) Listen() streams.Listener {
	return &sshStreamListener{
		listenChan: p.listenChan,
		closeChan:  p.closeChan,
	}
}

func (l *sshStreamListener) Accept() (streams.Stream, error) {
	select {
	case <-l.closeChan:
		return nil, io.EOF
	case a, ok := <-l.listenChan:
		if !ok {
			return nil, io.EOF
		}
		return a, nil
	}
}

func (s *sshStream) Read(data []byte) (int, error) {
	return s.channel.Read(data)
}

func (s *sshStream) Write(data []byte) (int, error) {
	return s.channel.Write(data)
}

func (s *sshStream) Close() error {
	return s.channel.CloseWrite()
}

func (s *sshStream) Headers() http.Header {
	return s.header
}

func (s *sshStream) Reset() error {
	return s.channel.Close()
}
