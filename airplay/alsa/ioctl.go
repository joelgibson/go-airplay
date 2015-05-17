package alsa

import (
	"syscall"
	"unsafe"
)

const (
	_IOC_NRBITS    = 8
	_IOC_TYPEBITS  = 8
	_IOC_SIZEBITS  = 14
	_IOC_NRSHIFT   = 0
	_IOC_TYPESHIFT = (_IOC_NRSHIFT + _IOC_NRBITS)
	_IOC_SIZESHIFT = (_IOC_TYPESHIFT + _IOC_TYPEBITS)
	_IOC_DIRSHIFT  = (_IOC_SIZESHIFT + _IOC_SIZEBITS)
	_IOC_NONE      = 0
	_IOC_WRITE     = 1
	_IOC_READ      = 2
)

func _IOC(dir, typ, nr, size uintptr) uintptr {
	return (((dir) << _IOC_DIRSHIFT) |
		((typ) << _IOC_TYPESHIFT) |
		((nr) << _IOC_NRSHIFT) |
		((size) << _IOC_SIZESHIFT))
}
func _IOR(typ, nr, size uintptr) uintptr {
	return _IOC(_IOC_READ, typ, nr, size)
}

func _IO(typ, nr uintptr) uintptr {
	return _IOC(_IOC_NONE, (typ), (nr), 0)
}
func _IOW(typ, nr, size uintptr) uintptr {
	return _IOC(_IOC_WRITE, (typ), (nr), size)
}
func _IOWR(typ, nr, size uintptr) uintptr {
	return _IOC(_IOC_READ|_IOC_WRITE, (typ), (nr), size)
}

func ioctl(device int, cmd uintptr, data unsafe.Pointer) (int, error) {
	a, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(device), cmd, uintptr(data))
	var err error
	if errno != 0 {
		err = errno
	}
	return int(a), err
}
