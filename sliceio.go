// Copyright 2022 Giuseppe Calabrese.
// Distributed under the terms of the ISC License.

// Package sliceio provides a wrapper that turns a byte slice into
// an I/O stream of bounded capacity.
package sliceio

import (
	"errors"
	"io"
	"math"
)

// ErrCapacity means that the operation failed because the
// length of the underlying slice is insufficient to accommodate it.
var ErrCapacity = errors.New("insufficient capacity")

// Wrapper implements stream-like I/O on a byte slice.
//
// Reading and writing happen at a common offset, and never cross the
// end of the slice. In particular, write operations past the end of
// the slice fail with ErrCapacity.
type Wrapper struct {
	slice  []byte // The underlying slice.
	offset int    // The read/write offset.
}

// Constructors.

// Wrap returns a wrapper with the given underlying slice and offset.
func Wrap(slice []byte, offset int) *Wrapper {
	return &Wrapper{slice, offset}
}

// New returns a wrapper of the given capacity and an offset of 0.
func New(capacity int) *Wrapper {
	return &Wrapper{make([]byte, capacity), 0}
}

// Accessors.

// Offset returns the read/write offset.
func (rw *Wrapper) Offset() int { return rw.offset }

// Content returns the underlying slice.
func (rw *Wrapper) Content() []byte { return rw.slice }

// Head returns a slice of the content up to the offset (exclusive).
func (rw *Wrapper) Head() []byte { return rw.slice[:rw.offset] }

// Rest returns a slice of the content from the offset (inclusive) onward.
func (rw *Wrapper) Rest() []byte { return rw.slice[rw.offset:] }

// Cap returns the capacity of the wrapper, that is, the length
// of its underlying slice.
func (rw *Wrapper) Cap() int { return len(rw.slice) }

// RestLen returns the number of remaining readable or writable bytes
// given the current offset; i.e., the length of Rest().
func (rw *Wrapper) RestLen() int { return rw.Cap() - rw.offset }

// Content operations.

// Copy returns a copy of the content.
//
// Uses mem as the backing capacity for the copy if cap(mem) is enough
// to hold the content; otherwise allocates a new slice.
func (rw *Wrapper) Copy(mem []byte) []byte {
	return append(mem[0:], rw.slice...)
}

// CopyHead returns a copy of the content up to the offset (exclusive).
//
// Uses mem as the backing capacity for the copy if cap(mem) is enough
// to hold the head; otherwise allocates a new slice.
func (rw *Wrapper) CopyHead(mem []byte) []byte {
	return append(mem[0:], rw.slice[:rw.offset]...)
}

// CopyRest returns a copy of the content from the offset (inclusive) onward.
//
// Uses mem as the backing capacity for the copy if cap(mem) is enough
// to hold the tail; otherwise allocates a new slice.
func (rw *Wrapper) CopyRest(mem []byte) []byte {
	return append(mem[0:], rw.slice[rw.offset:]...)
}

// Wrapper operations.

// NewShared returns a wrapper sharing the same underlying slice
// but with an independent, initially identical offset.
func (rw *Wrapper) NewShared() *Wrapper {
	return &Wrapper{rw.slice, rw.offset}
}

// NewCopy returns an independent wrapper with the same initial content
// and offset.
//
// Uses mem as the backing capacity for the new underlying slice
// if cap(mem) is enough to hold the content; otherwise allocates
// a new slice.
func (rw *Wrapper) NewCopy(mem []byte) *Wrapper {
	return &Wrapper{append(mem[:0], rw.slice...), rw.offset}
}

// Basic I/O.

// Read reads as many bytes as possible to buf.
//
// Returns the number of bytes read.
//
// Fails with io.EOF if less than len(buf) bytes were read.
func (rw *Wrapper) Read(buf []byte) (int, error) {
	n := copy(buf, rw.slice[rw.offset:])
	rw.offset += n
	if n < len(buf) {
		return n, io.EOF
	}
	return n, nil
}

// Write writes as many bytes as possible from buf.
//
// Returns the number of bytes written.
//
// Fails with ErrCapacity if less than len(buf) bytes were written.
func (rw *Wrapper) Write(buf []byte) (int, error) {
	n := copy(rw.slice[rw.offset:], buf)
	rw.offset += n
	if n < len(buf) {
		return n, ErrCapacity
	}
	return n, nil
}

// Seeking.

// Only offsets in the range [0..math.MaxInt] make sense for a
// slice-backed stream.

// ErrOffsetExceedsMaxInt means that an offset is too large to fit into
// an int.
var ErrOffsetExceedsMaxInt = errors.New("offset does not fit into an int")

// ErrSeekBeforeStart means that an attempt has been made to seek before
// the start.
var ErrSeekBeforeStart = errors.New("seek before the start")

// Seek sets the read/write offset.
//
// Fails with ErrSeekBeforeStart if the resolved offset would be negative.
// Fails with ErrCapacity if the resolved offset exceeds the capacity.
// Fails with ErrOffsetExceedsMaxInt if the resolved offset exceeds
// the maximum representable capacity of a slice (i.e. math.MaxInt).
func (rw *Wrapper) Seek(offset uint64, whence int) (uint64, error) {
	current := uint64(rw.offset) // rw.offset >= 0
	capacity := uint64(rw.Cap()) // Cap() >= 0
	var resolved uint64
	switch whence {
	case io.SeekStart:
		resolved = offset
	case io.SeekCurrent:
		resolved = current + offset
	case io.SeekEnd:
		// No seek before the start.
		if offset > capacity {
			return current, ErrSeekBeforeStart
		}
		resolved = capacity - offset
	default:
		panic("bad whence value")
	}

	if resolved > capacity {
		return current, ErrCapacity
	}
	if resolved > math.MaxInt {
		return current, ErrOffsetExceedsMaxInt
	}

	rw.offset = int(resolved) // resolved <= math.MaxInt
	return resolved, nil
}

// Rewind seeks to the start.
func (rw *Wrapper) Rewind() {
	rw.Seek(0, io.SeekStart)
}

// Other I/O stuff.

// ReadFrom reads as many bytes as possible from r.
//
// Returns the number of bytes read and any error encountered.
func (rw *Wrapper) ReadFrom(r io.Reader) (int64, error) {
	n, err := r.Read(rw.slice[rw.offset:])
	rw.offset += n
	return int64(n), err
}

// WriteTo writes as many bytes as possible bytes to w.
//
// Returns the number of bytes written and any error encountered.
func (rw *Wrapper) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(rw.slice[rw.offset:])
	rw.offset += n
	return int64(n), err
}

// ReadAt reads as many bytes as possible to buf, but starting at the given
// absolute offset instead of the current, which will remain unaffected.
//
// Returns the number of bytes read.
//
// Fails with io.EOF if less than len(buf) bytes were read.
// Fails with ErrSeekBeforeStart if the given offset is negative.
// Fails with ErrCapacity if the offset exceeds the capacity.
// Fails with ErrOffsetExceedsMaxInt if the resolved offset exceeds
// the maximum representable capacity of a slice (i.e. math.MaxInt).
func (rw *Wrapper) ReadAt(buf []byte, offset int64 /* sic */) (int, error) {
	if offset < 0 {
		return 0, ErrSeekBeforeStart
	}
	if offset > math.MaxInt {
		return 0, ErrOffsetExceedsMaxInt
	}
	if offset > int64(rw.Cap()) {
		return 0, ErrCapacity
	}

	c := rw.NewShared()
	c.offset = int(offset)
	return c.Read(buf)
}

// WriteAt writes as many bytes as possible from buf, but starting at the
// given absolute offset instead of the current, which will remain unaffected.
//
// Returns the number of bytes written.
//
// Fails with ErrCapacity if less than len(buf) were written, or
// if the offset exceeds the capacity.
// Fails with ErrSeekBeforeStart if the given offset is negative.
// Fails with ErrOffsetExceedsMaxInt if the resolved offset exceeds
// the maximum representable capacity of a slice (i.e. math.MaxInt).
func (rw *Wrapper) WriteAt(buf []byte, offset int64 /* sic */) (int, error) {
	if offset < 0 {
		return 0, ErrSeekBeforeStart
	}
	if offset > math.MaxInt {
		return 0, ErrOffsetExceedsMaxInt
	}
	if offset > int64(rw.Cap()) {
		return 0, ErrCapacity
	}

	c := rw.NewShared()
	c.offset = int(offset)
	return c.Write(buf)
}
