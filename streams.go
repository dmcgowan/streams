// Package streams is an interface for using multiplexed transports protocols
// in a way that allows swapping between protocols without changing the higher
// level implementation.
//
// The `StreamProvider` interface is intended to abstract the underlying protocol
// in a way that is centered around creating and managing the individually
// multiplexed streams. Streams may either be created locally or remotely.
// In the case of remotely created streams, a listener must be used to accept
// new streams.
//
// The `Stream` interface allows sending and receiving bytes as easy as any
// io.ReadWriteCloser. In addition to reading and writing, the interface
// allows for retrieving the create headers and fully resetting the stream.
// The `Close` method is used to signal that no more writes will occur locally
// on the stream. This puts the stream in a half-closed state, still allowing
// for remote writes and local reads. Once the remote side calls `Close`, the
// stream will be fully closed, causing any future reads to return `io.EOF`.
// The `Reset` method is used to force the stream into a fully closed state.
// This should only be called in error cases which do not allow for any
// further action to occur on the stream. Forcing the stream into a fully
// close state may cause remote writes to fail, which should be handled by
// the remote.
//
// The `Listener` interface is used to retrieve remotely created streams for
// local use. The `Accept` method will return the new stream will require
// the application to manage the local lifecycle of that stream. If the
// retrieved stream is invalid, possibly due to a bad header configuration,
// the application should call `Reset` on the stream, indicating to the remote
// side this stream will not be used.
package streams

import (
	"io"
	"net/http"
)

// Stream is an interface to represent a single
// byte stream on a multi-plexed connection with
// plus headers and a method to force full closure.
type Stream interface {
	io.ReadWriteCloser
	Headers() http.Header
	Reset() error
}

// Listener is an interface for returning remotely
// created streams.
type Listener interface {
	Accept() (Stream, error)
}

// StreamProvider is the minimal interface for
// creating new streams and receiving remotely
// created streams.
type StreamProvider interface {
	NewStream(http.Header) (Stream, error)
	Close() error
	Listen() Listener
}
