package airplay

import (
	"github.com/joelgibson/go-airplay/airplay/alsa"
	"io"
	"log"
)

type alsasink struct {
	inner io.WriteCloser
}

func (a *alsasink) Start() error { return nil }
func (a *alsasink) Flush()       {}
func (a *alsasink) Write(data []byte) (int, error) {
	// Manual BE -> LE conv as my kernel didn't appear to like using BE directly
	for i := 0; i < len(data); i += 2 {
		data[i], data[i+1] = data[i+1], data[i]
	}

	return a.inner.Write(data)
}

func (a *alsasink) Close() error {
	return a.inner.Close()
}

func SupportedCodecs() string {
	return "0"
}

func CreateAudioSink(s *Session) (AudioSink, error) {
	r, err := alsa.NewAlsaWriter(alsa.Config{2, 44100})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return &alsasink{r}, nil
}
