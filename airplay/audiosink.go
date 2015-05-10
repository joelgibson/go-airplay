package airplay

import "io"

type AudioSink interface {
	io.WriteCloser
	// Set the volume as a linear value between 0 and 1
	SetVolume(vol float32)
	Volume() float32
	Start() error
	Flush()
}
