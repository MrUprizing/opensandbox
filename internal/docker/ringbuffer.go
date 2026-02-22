package docker

import (
	"io"
	"sync"
)

// ringBuffer is a fixed-size circular buffer for command stdout/stderr.
// Writers append data. Readers can read from the beginning and follow new data.
type ringBuffer struct {
	mu      sync.Mutex
	buf     []byte
	size    int  // max capacity
	written int  // total bytes written (monotonic, may exceed size)
	closed  bool // set when no more writes will happen
	cond    *sync.Cond
}

const defaultRingSize = 1 << 20 // 1MB

// newRingBuffer creates a ring buffer with the given capacity.
func newRingBuffer(size int) *ringBuffer {
	r := &ringBuffer{
		buf:  make([]byte, size),
		size: size,
	}
	r.cond = sync.NewCond(&r.mu)
	return r
}

// Write appends data to the buffer, overwriting old data if capacity is exceeded.
func (r *ringBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := len(p)
	if n >= r.size {
		// Data exceeds buffer size; keep only the last `size` bytes.
		copy(r.buf, p[n-r.size:])
		r.written += n
		r.cond.Broadcast()
		return n, nil
	}

	start := r.written % r.size
	if start+n <= r.size {
		copy(r.buf[start:], p)
	} else {
		// Wrap around.
		first := r.size - start
		copy(r.buf[start:], p[:first])
		copy(r.buf, p[first:])
	}
	r.written += n
	r.cond.Broadcast()
	return n, nil
}

// Close marks the buffer as done, waking all waiting readers.
func (r *ringBuffer) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	r.cond.Broadcast()
}

// Bytes returns all buffered content (up to ring size).
func (r *ringBuffer) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.written == 0 {
		return nil
	}
	if r.written <= r.size {
		out := make([]byte, r.written)
		copy(out, r.buf[:r.written])
		return out
	}
	// Buffer has wrapped; linearize.
	out := make([]byte, r.size)
	start := r.written % r.size
	copy(out, r.buf[start:])
	copy(out[r.size-start:], r.buf[:start])
	return out
}

// NewReader returns a reader that starts from the beginning and follows new data
// until Close() is called on the buffer.
func (r *ringBuffer) NewReader() io.ReadCloser {
	return &ringReader{ring: r, pos: 0}
}

// ringReader reads from a ringBuffer, blocking for new data until the buffer is closed.
type ringReader struct {
	ring   *ringBuffer
	pos    int  // total bytes read so far (monotonic)
	closed bool // reader was closed
}

func (rr *ringReader) Read(p []byte) (int, error) {
	rr.ring.mu.Lock()
	defer rr.ring.mu.Unlock()

	for {
		if rr.closed {
			return 0, io.EOF
		}

		// If our read position has fallen behind the buffer's oldest data, skip ahead.
		if rr.ring.written > rr.ring.size && rr.pos < rr.ring.written-rr.ring.size {
			rr.pos = rr.ring.written - rr.ring.size
		}

		available := rr.ring.written - rr.pos
		if available > 0 {
			// Read as much as possible.
			n := available
			if n > len(p) {
				n = len(p)
			}

			start := rr.pos % rr.ring.size
			if start+n <= rr.ring.size {
				copy(p, rr.ring.buf[start:start+n])
			} else {
				first := rr.ring.size - start
				copy(p, rr.ring.buf[start:])
				copy(p[first:], rr.ring.buf[:n-first])
			}
			rr.pos += n
			return n, nil
		}

		if rr.ring.closed {
			return 0, io.EOF
		}

		// Wait for new data.
		rr.ring.cond.Wait()
	}
}

func (rr *ringReader) Close() error {
	rr.ring.mu.Lock()
	defer rr.ring.mu.Unlock()
	rr.closed = true
	return nil
}
