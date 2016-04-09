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

type acceptStream struct {
	s   *spdyStream
	err error
}

type spdyStreamListener struct {
	listenChan <-chan acceptStream
	closeChan  <-chan struct{}
}

type spdyStreamProvider struct {
	conn      *spdystream.Connection
	closeChan chan struct{}
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
		conn:      spdyConn,
		closeChan: make(chan struct{}),
	}
	go func() {
		<-spdyConn.CloseChan()
		close(provider.closeChan)
	}()

	return provider, nil
}

func (p *spdyStreamProvider) newStreamHandler(listenChan chan acceptStream, handler streams.StreamHandler) spdystream.StreamHandler {
	if handler == nil {
		handler = nilHandler
	}
	return func(stream *spdystream.Stream) {
		as := acceptStream{
			s: &spdyStream{
				stream: stream,
			},
		}

		replyHeaders, accept, err := handler(as.s.Headers())
		if err != nil {
			// Send reply but only return original handler error, ignore reply error
			as.s.stream.SendReply(replyHeaders, !accept)
			// Do not return original stream since it has been closed
			as.s = nil
			as.err = err
		}
		if !accept {
			// Not accepted, error will also be ignored
			as.s.stream.SendReply(replyHeaders, true)
			return
		}
		as.err = as.s.stream.SendReply(replyHeaders, false)
		// Do no clear original stream since it is stil open on reply error

		select {
		case <-p.closeChan:
			// Close stream if provided
			if as.s != nil {
				as.s.stream.Close()
			}
		case listenChan <- as:
		}
	}
}

func (p *spdyStreamProvider) NewStream(headers http.Header) (streams.Stream, error) {
	stream, streamErr := p.conn.CreateStream(headers, nil, false)
	if streamErr != nil {
		return nil, streamErr
	}
	return &spdyStream{stream: stream}, nil
}

func (p *spdyStreamProvider) Close() error {
	return p.conn.Close()
}

func nilHandler(http.Header) (http.Header, bool, error) {
	return http.Header{}, true, nil
}

func (p *spdyStreamProvider) Listen(handler streams.StreamHandler) streams.Listener {
	// TODO: Check if already listening
	listenChan := make(chan acceptStream)
	go p.conn.Serve(p.newStreamHandler(listenChan, handler))

	return &spdyStreamListener{
		listenChan: listenChan,
		closeChan:  p.closeChan,
	}
}

func (l *spdyStreamListener) Accept() (streams.Stream, error) {
	select {
	case <-l.closeChan:
		return nil, io.EOF
	case a, ok := <-l.listenChan:
		if !ok {
			return nil, io.EOF
		}
		return a.s, a.err
	}
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
