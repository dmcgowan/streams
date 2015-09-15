# Streams: Just give me a stream!

Streams is an interface for using multiplexing transport protocols in Go.
The intent is to make creating and receiving streams easy to use from an
application. Streams is designed to provide a unified implementation for
building applications which require multiplexed streaming and need to easily
swap between underlying implementations. The target implementations for Streams
is spdy/3.1, http2, and sshv2.

### Why is this useful?
Building complex networked applications which require many streams of
communication is hard. Creating and managing these streams of communication is
challenging and further complicated by the desire to support multiple protocols
and the need for debugging. Creating a simple interface for using these
protocols not only allows simplification of management, but also makes writing
debugging tools and handlers easier. Initially setting up the multiplexing
protocol is only necessary to do once per connection in a complex application.
However creating streams is a constant operation with low overhead to the
protocol but has a high management cost to the application as the streams
accumulate. A simple interface to manage these accumulating streams makes
working with streams more productive.

## Interface

~~~go
type Stream interface {
	io.ReadWriteCloser
	Headers() http.Header
	Reset() error
}

type Listener interface {
	Accept() (Stream, error)
}

type StreamProvider interface {
	NewStream(http.Header) (Stream, error)
	Close() error
	Listen(StreamHandler) Listener
}
~~~

## Multiplexing Protocols
### Spdy/3.1
Implementation using github.com/docker/spdystream
### HTTP/2
*Future implementation* using golang's http2 library
### SSH
*Future implementation* using github.com/golang/crypto/ssh
### Simple Framer
Naive framing protocol intended to implement the streams interface with minimal protocol on top of a raw byte stream
#### TBD: Simple framer definition

## Wrappers
The implementation of the stream provider may wrap another implementation to provide additional features.
These additional features may be used to span multiple connections or add debugging.
