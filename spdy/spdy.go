// Package spdy is an implementation of the streams interface
// using spdy/3.1
package spdy

import (
	"io"
	"net"
	"net/http"

	"github.com/dmcgowan/streams"
	"github.com/docker/spdystream"
)

type spdyStream struct {
	stream *spdystream.Stream
}

type spdyStreamListener struct {
	listenChan <-chan *spdyStream
}

type spdyStreamProvider struct {
	conn       *spdystream.Connection
	closeChan  chan struct{}
	listenChan chan *spdyStream
}

// NewSpdyStreamProvider creates a stream provider by starting a spdy
// session on the given connection. The server argument is used to
// determine whether the spdy connection is the client or server side.
func NewSpdyStreamProvider(conn net.Conn, server bool) (streams.StreamProvider, error) {
	spdyConn, spdyErr := spdystream.NewConnection(conn, server)
	if spdyErr != nil {
		return nil, spdyErr
	}
	provider := &spdyStreamProvider{
		conn:       spdyConn,
		closeChan:  make(chan struct{}),
		listenChan: make(chan *spdyStream),
	}
	go spdyConn.Serve(provider.newStreamHandler)
	go func() {
		select {
		case <-spdyConn.CloseChan():
		case <-provider.closeChan:
		}
		close(provider.listenChan)
	}()

	return provider, nil
}

func (p *spdyStreamProvider) newStreamHandler(stream *spdystream.Stream) {
	s := &spdyStream{
		stream: stream,
	}
	returnHeaders := http.Header{}
	var finish bool
	select {
	case <-p.closeChan:
		returnHeaders.Set(":status", "502")
		finish = true
	case p.listenChan <- s:
		returnHeaders.Set(":status", "200")
	}
	stream.SendReply(returnHeaders, finish)
}

func (p *spdyStreamProvider) NewStream(headers http.Header) (streams.Stream, error) {
	stream, streamErr := p.conn.CreateStream(headers, nil, false)
	if streamErr != nil {
		return nil, streamErr
	}
	return &spdyStream{stream: stream}, nil
}

func (p *spdyStreamProvider) Close() error {
	close(p.closeChan)
	return p.conn.Close()
}

func (p *spdyStreamProvider) Listen() streams.Listener {
	return &spdyStreamListener{
		listenChan: p.listenChan,
	}
}

func (l *spdyStreamListener) Accept() (streams.Stream, error) {
	stream := <-l.listenChan
	if stream == nil {
		return nil, io.EOF
	}
	return stream, nil
}

func (s *spdyStream) Read(b []byte) (int, error) {
	return s.stream.Read(b)
}

func (s *spdyStream) Write(b []byte) (int, error) {
	return s.stream.Write(b)
}

func (s *spdyStream) Close() error {
	return s.stream.Close()
}

func (s *spdyStream) Reset() error {
	return s.stream.Reset()
}

func (s *spdyStream) Headers() http.Header {
	return s.stream.Headers()
}
