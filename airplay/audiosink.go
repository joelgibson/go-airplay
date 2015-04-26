package airplay

type AudioSink interface {
	Start() error
	Write(data []byte) (n int, err error)
	Close() error
	Flush()
}
