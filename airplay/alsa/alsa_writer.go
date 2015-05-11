package alsa

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	period_size  = 512
	period_count = 8
	buffers      = 256
	max          = period_size * 2 * 2
)

var (
	initial_buffer_size = 44100 * 2 * 2 * 2
)

func init() {
	flag.IntVar(&initial_buffer_size, "ia", initial_buffer_size, "Amount of buffering before starting playback")
}

type (
	Config struct {
		Channels, Freq uint32
	}
	alsaWriter struct {
		wg            sync.WaitGroup
		work, done    chan int
		buf           [][]byte
		fd            int
		running       bool
		bytesPerFrame int
		underruns     int
		accepted      int
	}
)

func NewAlsaWriter(c Config) (io.WriteCloser, error) {
	a := &alsaWriter{bytesPerFrame: int(c.Channels * 2)}
	var par sndrv_pcm_hw_params
	par.Init()
	par.SetMask(SNDRV_PCM_HW_PARAM_ACCESS, SNDRV_PCM_ACCESS_RW_INTERLEAVED)
	par.SetMask(SNDRV_PCM_HW_PARAM_FORMAT, SNDRV_PCM_FORMAT_S16_LE)
	par.SetMask(SNDRV_PCM_HW_PARAM_SUBFORMAT, 0)
	par.SetMin(SNDRV_PCM_HW_PARAM_PERIOD_SIZE, period_size)
	par.SetInt(SNDRV_PCM_HW_PARAM_SAMPLE_BITS, 16)
	par.SetInt(SNDRV_PCM_HW_PARAM_FRAME_BITS, 16*c.Channels)
	par.SetInt(SNDRV_PCM_HW_PARAM_CHANNELS, c.Channels)
	par.SetInt(SNDRV_PCM_HW_PARAM_PERIODS, period_count)
	par.SetInt(SNDRV_PCM_HW_PARAM_RATE, c.Freq)

	log.Println("Opening /dev/snd/pcmC0D0p")
	f, err := syscall.Open("/dev/snd/pcmC0D0p", syscall.O_RDWR, 0644)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	log.Println("hw_params ioctl")

	sparams := sndrv_pcm_sw_params{}
	if _, err = ioctl(f, SNDRV_PCM_IOCTL_HW_PARAMS, unsafe.Pointer(&par)); err != nil {
		log.Println(err)
		goto cleanup
	}
	log.Printf("fifo_size: %d", par.fifo_size)

	sparams = sndrv_pcm_sw_params{
		tstamp_mode:     SNDRV_PCM_TSTAMP_NONE,
		period_step:     1,
		avail_min:       1,
		start_threshold: 1,                                               //sndrv_pcm_uframes_t(par.interval(SNDRV_PCM_HW_PARAM_BUFFER_SIZE).max - 1), // start when we almost have the buffer full
		stop_threshold:  sndrv_pcm_uframes_t(period_count * period_size), // stop when we have an underrun
		//boundary:          sndrv_pcm_uframes_t(par.interval(SNDRV_PCM_HW_PARAM_BUFFER_SIZE).max),
		//xfer_align:        period_size / 2,
		silence_size:      0,
		silence_threshold: 0,
	}
	if _, err = ioctl(f, SNDRV_PCM_IOCTL_SW_PARAMS, unsafe.Pointer(&sparams)); err != nil {
		log.Println(err)
		goto cleanup
	}

	log.Printf("\n%+v\n%+v\n%+v", par.interval(SNDRV_PCM_HW_PARAM_BUFFER_SIZE), par.interval(SNDRV_PCM_HW_PARAM_BUFFER_BYTES), par.interval(SNDRV_PCM_HW_PARAM_TICK_TIME))
	log.Println(par.fifo_size)

	a.fd = f
	a.buf = make([][]byte, buffers)
	a.work = make(chan int, len(a.buf))
	a.done = make(chan int, len(a.buf))
	for i := range a.buf {
		a.done <- i
	}
	go a.thread()
	return a, nil

cleanup:
	syscall.Close(f)
	return nil, err
}
func (a *alsaWriter) write(buf []byte) (n int, err error) {
	i := 0
	for i < len(buf) {
		if !a.running {
			// if len(a.work) == 0 {
			// 	for len(a.work) < buffers/2 {
			// 		time.Sleep(time.Millisecond)
			// 	}
			// }
			_ = time.May
			if _, err := ioctl(a.fd, SNDRV_PCM_IOCTL_PREPARE, nil); err != nil {
				log.Println(err)
				return i, err
			}
			a.running = true
		}
		left := len(buf) - i
		if left > max {
			left = max
		}
		var x sndrv_xferi
		x.buf = unsafe.Pointer(&buf[i])
		x.frames = sndrv_pcm_uframes_t(left / a.bytesPerFrame)
		ii, err := ioctl(a.fd, SNDRV_PCM_IOCTL_WRITEI_FRAMES, unsafe.Pointer(&x))
		if err == nil {
			i += int(x.result) * a.bytesPerFrame
			//time.Sleep(time.Millisecond)
			a.accepted++
			continue
		} else if err == syscall.EPIPE {
			//log.Println("underrun", ii, err, x.result)
			a.running = false
			a.underruns++
			continue
		}
		log.Println(ii, err)
		a.running = false // try restarting
		return i, nil
	}
	return i, nil
}

func (a *alsaWriter) thread() {
	a.wg.Add(1)
	var buf bytes.Buffer
	for i := range a.work {
		var fr sndrv_pcm_sframes_t
		ioctl(a.fd, SNDRV_PCM_IOCTL_DELAY, unsafe.Pointer(&fr))
		fmt.Printf("queued buffers: %2d driver latency: %.2f    %d/%d                      \r", len(a.work), float32(fr)/44100.0, a.underruns, a.accepted)
		if (a.underruns > 30) && len(a.work) == 0 {
			a.running = false
			a.underruns = 0
			a.accepted = 0
		}
		if !a.running {
			// for len(a.work) < buffers/2 {
			// 	time.Sleep(time.Millisecond)
			// }

			buf.Write(a.buf[i])
			a.done <- i
			if buf.Len() >= initial_buffer_size {
				n, _ := a.write(buf.Bytes())
				buf.Next(n)
			}
			continue
		}
		for buf.Len() > 0 {
			n, _ := a.write(buf.Bytes())
			buf.Next(n)
			log.Println("emptying buf...", n, buf.Len())
		}

		n, _ := a.write(a.buf[i])
		if n >= 0 && n < len(a.buf[i]) {
			buf.Write(a.buf[i][n:])
		}
		a.done <- i
	}
	a.wg.Done()
}

func (a *alsaWriter) Write(buf []byte) (n int, err error) {
	l := len(buf)
	i := <-a.done
	if len(a.buf[i]) < l {
		if cap(a.buf[i]) < l {
			a.buf[i] = make([]byte, l)
		}
		a.buf[i] = a.buf[i][:l]
	}
	copy(a.buf[i], buf)
	a.work <- i

	return len(buf), nil
}

func (a *alsaWriter) Close() error {
	close(a.work)
	a.wg.Wait()
	close(a.done)
	return syscall.Close(a.fd)
}
