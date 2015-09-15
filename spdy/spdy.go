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
	handler    streams.StreamHandler
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

	select {
	case <-p.closeChan:
		// Do not set handlers on closed providers, new streams
		// should not be handled by the provider
		stream.SendReply(http.Header{}, true)
	case p.listenChan <- s:
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
	close(p.closeChan)
	return p.conn.Close()
}

func nilHandler(http.Header) (http.Header, bool, error) {
	return http.Header{}, true, nil
}

func (p *spdyStreamProvider) Listen(handler streams.StreamHandler) streams.Listener {
	if handler == nil {
		handler = nilHandler
	}
	return &spdyStreamListener{
		listenChan: p.listenChan,
		handler:    handler,
	}
}

func (l *spdyStreamListener) Accept() (streams.Stream, error) {
	for {
		s := <-l.listenChan
		if s == nil {
			return nil, io.EOF
		}
		replyHeaders, accept, err := l.handler(s.Headers())
		if err != nil {
			// Send reply but only return original handler error
			s.stream.SendReply(replyHeaders, !accept)
			return nil, err
		}
		if !accept {
			if err := s.stream.SendReply(replyHeaders, true); err != nil {
				return nil, err
			}
			continue
		} else {
			if err := s.stream.SendReply(replyHeaders, false); err != nil {
				return nil, err
			}
		}
		return s, nil
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
