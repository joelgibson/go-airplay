package alsa

import "unsafe"

const (
	SNDRV_MASK_MAX                  = 256
	SNDRV_PCM_FORMAT_S16_LE         = 2
	SNDRV_PCM_FORMAT_S16_BE         = 3
	SNDRV_PCM_TSTAMP_NONE           = 0
	SNDRV_PCM_ACCESS_RW_INTERLEAVED = 3
)
const (
	FLAG_OPENMIN = 1 << iota
	FLAG_OPENMAX
	FLAG_INTEGER
	FLAG_EMPTY
)

const (
	SNDRV_PCM_HW_PARAM_ACCESS      = 0 /* Access type */
	SNDRV_PCM_HW_PARAM_SAMPLE_BITS = 8 /* Bits per sample */
)
const (
	SNDRV_PCM_HW_PARAM_FIRST_MASK = SNDRV_PCM_HW_PARAM_ACCESS + iota
	SNDRV_PCM_HW_PARAM_FORMAT     /* Format */
	SNDRV_PCM_HW_PARAM_SUBFORMAT  /* Subformat */
	SNDRV_PCM_HW_PARAM_LAST_MASK  = SNDRV_PCM_HW_PARAM_SUBFORMAT
)
const (
	SNDRV_PCM_HW_PARAM_FIRST_INTERVAL = SNDRV_PCM_HW_PARAM_SAMPLE_BITS + iota
	SNDRV_PCM_HW_PARAM_FRAME_BITS     /* Bits per frame */
	SNDRV_PCM_HW_PARAM_CHANNELS       /* Channels */
	SNDRV_PCM_HW_PARAM_RATE           /* Approx rate */
	SNDRV_PCM_HW_PARAM_PERIOD_TIME    /* Approx distance between interrupts in us */
	SNDRV_PCM_HW_PARAM_PERIOD_SIZE    /* Approx frames between interrupts */
	SNDRV_PCM_HW_PARAM_PERIOD_BYTES   /* Approx bytes between interrupts */
	SNDRV_PCM_HW_PARAM_PERIODS        /* Approx interrupts per buffer */
	SNDRV_PCM_HW_PARAM_BUFFER_TIME    /* Approx duration of buffer in us */
	SNDRV_PCM_HW_PARAM_BUFFER_SIZE    /* Size of buffer in frames */
	SNDRV_PCM_HW_PARAM_BUFFER_BYTES   /* Size of buffer in bytes */
	SNDRV_PCM_HW_PARAM_TICK_TIME      /* Approx tick duration in us */
	SNDRV_PCM_HW_PARAM_LAST_INTERVAL  = SNDRV_PCM_HW_PARAM_TICK_TIME
)

type (
	sndrv_pcm_sframes_t int32
	sndrv_pcm_uframes_t uint32
	sndrv_xferi         struct {
		result sndrv_pcm_sframes_t
		buf    unsafe.Pointer
		frames sndrv_pcm_uframes_t
	}

	sndrv_interval struct {
		min, max, flags uint32
	}

	sndrv_ctl_card_info struct {
		card       int32     /* card number */
		pad        int32     /* reserved for future (was type) */
		id         [16]byte  /* ID of card (user selectable) */
		driver     [16]byte  /* Driver name */
		name       [32]byte  /* Short name of soundcard */
		longname   [80]byte  /* name + info text about soundcard */
		reserved_  [16]byte  /* reserved for future (was ID of mixer) */
		mixername  [80]byte  /* visual mixer identification */
		components [128]byte /* card components / fine identification, delimited with one space (AC97 etc..) */
	}
	sndrv_pcm_sync_id [16]byte
	sndrv_pcm_info    struct {
		device           int32    /* RO/WR (control): device number */
		subdevice        int32    /* RO/WR (control): subdevice number */
		stream           int32    /* RO/WR (control): stream number */
		card             int32    /* R: card number */
		id               [64]byte /* ID (user selectable) */
		name             [80]byte /* name of this device */
		subname          [32]byte /* subdevice name */
		dev_class        int32    /* SNDRV_PCM_CLASS_* */
		dev_subclass     int32    /* SNDRV_PCM_SUBCLASS_* */
		subdevices_count uint32
		subdevices_avail uint32
		sync             sndrv_pcm_sync_id /* hardware synchronization ID */
		reserved         [64]byte          /* reserved for future... */
	}

	sndrv_mask struct {
		bits [(SNDRV_MASK_MAX + 31) / 32]uint32
	}

	sndrv_pcm_hw_params struct {
		flags     uint32
		masks     [SNDRV_PCM_HW_PARAM_LAST_MASK - SNDRV_PCM_HW_PARAM_FIRST_MASK + 1]sndrv_mask
		mres      [5]sndrv_mask /* reserved masks */
		intervals [SNDRV_PCM_HW_PARAM_LAST_INTERVAL - SNDRV_PCM_HW_PARAM_FIRST_INTERVAL + 1]sndrv_interval
		ires      [9]sndrv_interval   `json:"-"` /* reserved intervals */
		rmask     uint32              /* W: requested masks */
		cmask     uint32              /* R: changed masks */
		info      uint32              /* R: Info flags for returned setup */
		msbits    uint32              /* R: used most significant bits */
		rate_num  uint32              /* R: rate numerator */
		rate_den  uint32              /* R: rate denominator */
		fifo_size sndrv_pcm_uframes_t /* R: chip FIFO size in frames */
		reserved  [64]byte            `json:"-"` /* reserved for future */
	}
	sndrv_pcm_sw_params struct {
		tstamp_mode       int32 /* timestamp mode */
		period_step       uint32
		sleep_min         uint32              /* min ticks to sleep */
		avail_min         sndrv_pcm_uframes_t /* min avail frames for wakeup */
		xfer_align        sndrv_pcm_uframes_t /* xfer size need to be a multiple */
		start_threshold   sndrv_pcm_uframes_t /* min hw_avail frames for automatic start */
		stop_threshold    sndrv_pcm_uframes_t /* min avail frames for automatic stop */
		silence_threshold sndrv_pcm_uframes_t /* min distance from noise for silence filling */
		silence_size      sndrv_pcm_uframes_t /* silence block size */
		boundary          sndrv_pcm_uframes_t /* pointers wrap point */
		reserved          [60]byte            /* reserved for future */
		period_event      uint32              /* for alsa-lib implementation */
	}
)

func (p *sndrv_pcm_hw_params) SetMask(n, bit uint32) {
	if bit >= SNDRV_MASK_MAX || !isMask(n) {
		return
	}
	m := p.mask(n)
	m.bits[0] = 0
	m.bits[1] = 0
	m.bits[bit>>5] |= (1 << uint32(bit&31))

}
func isMask(p uint32) bool {
	return (p >= SNDRV_PCM_HW_PARAM_FIRST_MASK) &&
		(p <= SNDRV_PCM_HW_PARAM_LAST_MASK)
}

func isInterval(p uint32) bool {
	return (p >= SNDRV_PCM_HW_PARAM_FIRST_INTERVAL) &&
		(p <= SNDRV_PCM_HW_PARAM_LAST_INTERVAL)
}

func (p *sndrv_pcm_hw_params) interval(n uint32) *sndrv_interval {
	return &p.intervals[n-SNDRV_PCM_HW_PARAM_FIRST_INTERVAL]
}

func (p *sndrv_pcm_hw_params) mask(n uint32) *sndrv_mask {
	return &p.masks[n-SNDRV_PCM_HW_PARAM_FIRST_MASK]
}

func (p *sndrv_pcm_hw_params) SetMin(n, val uint32) {
	if !isInterval(n) {
		return
	}
	i := p.interval(n)
	i.min = val
}
func (p *sndrv_pcm_hw_params) SetMax(n, val uint32) {
	if !isInterval(n) {
		return
	}
	i := p.interval(n)
	i.max = val
}

func (p *sndrv_pcm_hw_params) SetInt(n, val uint32) {
	if !isInterval(n) {
		return
	}
	i := p.interval(n)
	i.min = val
	i.max = val
	i.flags = FLAG_INTEGER
}

func (p *sndrv_pcm_hw_params) Int(n uint32) uint32 {
	if !isInterval(n) {
		return 0
	}
	i := p.interval(n)
	return i.max
}
func (p *sndrv_pcm_hw_params) Init() {
	for n := uint32(SNDRV_PCM_HW_PARAM_FIRST_MASK); n <= SNDRV_PCM_HW_PARAM_LAST_MASK; n++ {
		m := p.mask(n)
		m.bits[0] = 0xffffffff
		m.bits[1] = 0xffffffff
	}
	for n := uint32(SNDRV_PCM_HW_PARAM_FIRST_INTERVAL); n <= SNDRV_PCM_HW_PARAM_LAST_INTERVAL; n++ {
		i := p.interval(n)
		i.min = 0
		i.max = 0xffffffff
		i.flags = 0
	}
	p.rmask = 0xffffffff
	p.info = 0xffffffff
}

var (
	SNDRV_CTL_IOCTL_CARD_INFO       = _IOR('U', 0x01, unsafe.Sizeof(sndrv_ctl_card_info{}))
	SNDRV_CTL_IOCTL_PCM_NEXT_DEVICE = _IOR('U', 0x30, 4)
	SNDRV_CTL_IOCTL_PCM_INFO        = _IOWR('U', 0x31, unsafe.Sizeof(sndrv_pcm_info{}))
	SNDRV_PCM_IOCTL_PVERSION        = _IOR('A', 0x00, 4)
	SNDRV_PCM_IOCTL_INFO            = _IOR('A', 0x01, unsafe.Sizeof(sndrv_pcm_info{}))
	SNDRV_PCM_IOCTL_HW_REFINE       = _IOWR('A', 0x10, unsafe.Sizeof(sndrv_pcm_hw_params{}))
	SNDRV_PCM_IOCTL_HW_PARAMS       = _IOWR('A', 0x11, unsafe.Sizeof(sndrv_pcm_hw_params{}))
	SNDRV_PCM_IOCTL_SW_PARAMS       = _IOWR('A', 0x13, unsafe.Sizeof(sndrv_pcm_sw_params{}))
	SNDRV_PCM_IOCTL_DELAY           = _IOR('A', 0x21, unsafe.Sizeof(sndrv_pcm_sframes_t(0)))
	SNDRV_PCM_IOCTL_PREPARE         = _IO('A', 0x40)
	SNDRV_PCM_IOCTL_START           = _IO('A', 0x42)
	SNDRV_PCM_IOCTL_DROP            = _IO('A', 0x43)
	SNDRV_PCM_IOCTL_DRAIN           = _IO('A', 0x44)
	SNDRV_PCM_IOCTL_PAUSE           = _IOW('A', 0x45, 4)

	SNDRV_PCM_IOCTL_WRITEI_FRAMES = _IOW('A', 0x50, unsafe.Sizeof(sndrv_xferi{}))
)
