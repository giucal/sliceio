// Copyright 2022 Giuseppe Calabrese.
// Distributed under the terms of the ISC License.

package sliceio_test

import (
	"encoding/binary"
	"fmt"

	"github.com/giucal/sliceio"
)

type Color struct {
	R uint8
	G uint8
	B uint8
}

// Encode a value to a slice using binary.Write.
func ExampleWrapper_encodeToASlice() {
	// Some RGB colors to encode.
	fuchsia := Color{255, 0, 255}
	cyan := Color{0, 255, 255}

	// Allocate and wrap a suitable buffer.
	colorSize := binary.Size(Color{})
	w := sliceio.New(colorSize)

	// Use binary.Write to encode fuchsia to w.
	binary.Write(w, binary.LittleEndian, fuchsia)

	// w.Head() now holds the binary representation of fuchsia.
	fmt.Println(w.Head())
	fmt.Println(w.Content()) // Same, because we allocated colorSize bytes.

	// Also encode cyan to w.
	old := w.CopyHead(nil) // Preserve the current value.
	w.Rewind()             // Seek back to the front.
	binary.Write(w, binary.LittleEndian, cyan)

	fmt.Println("Encoded fuchsia:", old)
	fmt.Println("Encoded cyan:", w.Head())
	// Output:
	// [255 0 255]
	// [255 0 255]
	// Encoded fuchsia: [255 0 255]
	// Encoded cyan: [0 255 255]
}
