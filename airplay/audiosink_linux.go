package airplay

import (
	"bufio"
	"github.com/joelgibson/go-airplay/airplay/alsa"
	"io"
	"log"
	"reflect"
	"unsafe"
)

var aw io.WriteCloser

func init() {
	r, err := alsa.NewAlsaWriter(alsa.Config{2, 44100})
	if err != nil {
		log.Fatalln(err)
	}
	aw = r
}

type alsasink struct {
	inner io.WriteCloser
	bw    *bufio.Writer
	vol   float32
}

func (a *alsasink) SetVolume(vol float32) { a.vol = vol }
func (a *alsasink) Volume() float32       { return a.vol }
func (a *alsasink) Start() error          { return nil }
func (a *alsasink) Flush()                {}
func (a *alsasink) Write(data []byte) (int, error) {
	var d2 []int16
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&d2))
	sh.Data = uintptr(unsafe.Pointer(&data[0]))
	sh.Cap = len(data) / 2
	sh.Len = sh.Cap
	for i, v := range d2 {
		// Manual BE -> LE conv as my kernel didn't appear to like using BE directly
		v = int16(int16(v&0xff)<<8 | (int16(v>>8) & 0xff))
		d2[i] = int16(float32(v) * a.vol)
	}

	return a.bw.Write(data)
}

func (a *alsasink) Close() error {
	return nil //a.inner.Close(
}

func SupportedCodecs() string {
	return "0"
}

func CreateAudioSink(s *Session) (AudioSink, error) {
	return &alsasink{aw, bufio.NewWriterSize(aw, 1024), 1}, nil
}
