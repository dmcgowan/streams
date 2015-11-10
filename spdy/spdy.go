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
}

type spdyStreamProvider struct {
	conn       *spdystream.Connection
	closeChan  chan struct{}
	listenChan chan acceptStream
	handler    StreamHandler
}

// StreamHandler is a function to handle a new stream
// by adding response headers and whether the stream
// should be accepted. If a stream is not accepted, it
// will not be returned by an accept on the listener.
// Any error returned by the handler should be returned
// on accept.
type StreamHandler func(http.Header) (http.Header, bool, error)

// NewSpdyStreamProvider creates a stream provider by starting a spdy
// session on the given connection. The server argument is used to
// determine whether the spdy connection is the client or server side.
func NewSpdyStreamProvider(conn net.Conn, server bool, handler StreamHandler) (streams.StreamProvider, error) {
	spdyConn, spdyErr := spdystream.NewConnection(conn, server)
	if spdyErr != nil {
		return nil, spdyErr
	}
	if handler == nil {
		handler = nilHandler
	}
	provider := &spdyStreamProvider{
		conn:       spdyConn,
		closeChan:  make(chan struct{}),
		listenChan: make(chan acceptStream),
		handler:    handler,
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
	as := acceptStream{
		s: &spdyStream{
			stream: stream,
		},
	}

	replyHeaders, accept, err := p.handler(as.s.Headers())
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
	} else {
		as.err = as.s.stream.SendReply(replyHeaders, false)
		// Do no clear original stream since it is stil open on reply error
	}

	select {
	case <-p.closeChan:
		// Close stream if provided
		if as.s != nil {
			as.s.stream.Close()
		}
	case p.listenChan <- as:
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

func (p *spdyStreamProvider) Listen() streams.Listener {
	return &spdyStreamListener{
		listenChan: p.listenChan,
	}
}

func (l *spdyStreamListener) Accept() (streams.Stream, error) {
	a, ok := <-l.listenChan
	if !ok {
		return nil, io.EOF
	}
	return a.s, a.err
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
