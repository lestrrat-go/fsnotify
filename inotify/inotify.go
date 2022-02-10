//go:build linux
// +build linux

package inotify

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/lestrrat-go/fsnotify/api"
	"golang.org/x/sys/unix"
)

type CommandOption = api.CommandOption

const (
	cmdAdd = iota + 1
	cmdRemove
)

var ErrEventOverflow = fmt.Errorf(`fsnotify queue overflow`)

// Driver is the inotify backed fsnotify driver.
// The driver itself doesn't keep state. Stateful operations
// are abstracted within the Run() method.
type Driver struct {
	control chan *api.Command
	data    chan interface{}
	pending *api.CommandQueue
}

func New() *Driver {
	d := &Driver{}
	d.pending = api.NewCommandQueue(api.CommandQueueEgressChooseFunc(func(cmd *api.Command) chan *api.Command {
		switch cmd.Type {
		case cmdAdd, cmdRemove:
			return d.control
		default:
			panic("unimplemented")
		}
	}))
	return d
}

type runCtx struct {
	mu sync.RWMutex

	epfd     int // epoll fd
	infd     int // inotify fd
	wakeupfd int // read fd for pipe used to wake up epoll
	evsink   api.EventSink
	errsink  api.ErrorSink
	paths    map[int]string
	watches  map[string]*watch
}

type watch struct {
	wd    uint32 // Watch descriptor (as returned by the inotify_add_watch() syscall)
	flags uint32 // inotify flags of this watch (see inotify(7) for the list of valid flags)
}

func epollAdd(fd, epfd int) error {
	event := unix.EpollEvent{
		Fd:     int32(fd),
		Events: unix.EPOLLIN,
	}
	if err := unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, fd, &event); err != nil {
		return fmt.Errorf(`failed to add fd to epoll: %w`, err)
	}
	return nil
}

func (rctx *runCtx) epollWake() error {
	var buf [1]byte
	n, err := unix.Write(rctx.wakeupfd, buf[:])
	if n != -1 {
		return nil
	}
	if err == unix.EAGAIN {
		// Buffer is full, poller will wake up eventually
		return nil
	}
	return err
}

func (rctx *runCtx) epollWakeClear() error {
	var buf [100]byte
	n, err := unix.Read(rctx.wakeupfd, buf[:])
	if n != -1 {
		return nil
	}

	if err == unix.EAGAIN {
		return nil
	}
	return err
}

func (driver *Driver) Run(ctx context.Context, ready chan struct{}, evsink api.EventSink, errsink api.ErrorSink) {
	driver.control = make(chan *api.Command)

	infd, err := unix.InotifyInit1(unix.IN_CLOEXEC)
	if infd == -1 {
		errsink.Error(fmt.Errorf(`failed to create inotify fd: %w`, err))
		return
	}
	defer unix.Close(infd)

	// Create epoll
	epfd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if epfd == -1 {
		errsink.Error(err)
		return
	}

	if err := epollAdd(infd, epfd); err != nil {
		errsink.Error(fmt.Errorf(`failed to register inotify fd to epoll: %w`, err))
		return
	}

	// Create pipe to interrupt/wake up the epoll_wait when
	// we detect <-ctx.Done(). This is only valid during Run() time
	var pipe [2]int
	if errno := unix.Pipe2(pipe[:], unix.O_NONBLOCK|unix.O_CLOEXEC); errno != nil {
		errsink.Error(fmt.Errorf(`failed to create pipe: %w`, errno))
		return
	}

	if err := epollAdd(pipe[0], epfd); err != nil {
		errsink.Error(fmt.Errorf(`failed to register pipe to epoll: %w`, err))
		return
	}

	rctx := runCtx{
		epfd:     epfd,
		infd:     infd,
		wakeupfd: pipe[1],
		evsink:   evsink,
		errsink:  errsink,
		paths:    make(map[int]string),
		watches:  make(map[string]*watch),
	}

	go driver.pending.Drain(ctx)
	go rctx.doEpoll(ctx)

	close(ready)
	for {
		select {
		case <-ctx.Done():
			rctx.epollWake()
			return
		case cmd := <-driver.control:
			switch cmd.Type {
			case cmdAdd:
				reply := cmd.Reply
				if err := rctx.add(cmd.Payload.(string)); err != nil {
					if reply != nil {
						select {
						case <-ctx.Done():
						case reply <- err:
						}
					}
				}
				if reply != nil {
					close(reply)
				}
			}
		}
	}
}

type cmdRequest struct {
	Type  int
	Arg   interface{}
	reply chan error
}

// Add adds a new path to be watched by the driver
func (driver *Driver) Add(path string, options ...api.CommandOption) error {
	cmd := &api.Command{
		Type:    cmdAdd,
		Payload: path,
	}
	return driver.pending.SendCmd(cmd, options...)
}

func (driver *Driver) Remove(path string, options ...api.CommandOption) error {
	cmd := &api.Command{
		Type:    cmdRemove,
		Payload: path,
	}
	return driver.pending.SendCmd(cmd, options...)
}

func (rctx *runCtx) add(path string) error {
	const agnosticEvents = unix.IN_MOVED_TO | unix.IN_MOVED_FROM |
		unix.IN_CREATE | unix.IN_ATTRIB | unix.IN_MODIFY |
		unix.IN_MOVE_SELF | unix.IN_DELETE | unix.IN_DELETE_SELF
	var flags uint32 = agnosticEvents

	rctx.mu.Lock()
	defer rctx.mu.Unlock()
	watchEntry := rctx.watches[path]
	if watchEntry != nil {
		flags |= watchEntry.flags | unix.IN_MASK_ADD
	}
	wd, errno := unix.InotifyAddWatch(rctx.infd, path, flags)
	if wd == -1 {
		return errno
	}

	if watchEntry == nil {
		rctx.watches[path] = &watch{wd: uint32(wd), flags: flags}
		rctx.paths[wd] = path
	} else {
		watchEntry.wd = uint32(wd)
		watchEntry.flags = flags
	}

	return nil
}

func (rctx *runCtx) doEpoll(ctx context.Context) {
	events := make([]unix.EpollEvent, 7)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Note: if the context is marked done while waiting for EpollWait,
		// the main Driver loop should send us a notification.
		n, err := unix.EpollWait(rctx.epfd, events, -1)
		if n == -1 {
			if err == unix.EINTR {
				continue
			}
		}

		if n == 0 {
			continue
		}

		if n > 6 {
			rctx.errsink.Error(fmt.Errorf(`epoll_wait returned %d events (expected less than 6)`, n))
			continue
		}

		var eventsReady bool
		for _, event := range events[:n] {
			switch event.Fd {
			case int32(rctx.infd):
				// inotify fd
				if event.Events&unix.EPOLLHUP != 0 || event.Events&unix.EPOLLERR != 0 || event.Events&unix.EPOLLIN != 0 {
					// EPOLLHUP: something's wrong, but wake up and handle the next event
					// EPOLLERR: something's wrong, but wake up and let unix.Read pick up the error
					// EPOLLIN:  there's something to read.
					eventsReady = true
				}
			case int32(rctx.wakeupfd):
				if event.Events&unix.EPOLLIN != 0 {
					if err := rctx.epollWakeClear(); err != nil {
						rctx.errsink.Error(err)
					}
				}
			}
		}

		if !eventsReady {
			continue
		}

		var buf [unix.SizeofInotifyEvent * 4096]byte
		n, err = unix.Read(rctx.infd, buf[:])
		// If a signal interrupted execution, see if we've been asked to close, and try again.
		// http://man7.org/linux/man-pages/man7/signal.7.html :
		// "Before Linux 3.8, reads from an inotify(7) file descriptor were not restartable"
		if err == unix.EINTR {
			continue
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		if n < unix.SizeofInotifyEvent {
			if n == 0 {
				rctx.errsink.Error(io.EOF)
			} else if n < 0 {
				rctx.errsink.Error(err)
			} else {
				rctx.errsink.Error(fmt.Errorf(`short read while reading events`))
			}
			continue
		}

		var offset uint32
		for max := uint32(n - unix.SizeofInotifyEvent); offset <= max; {
			raw := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			rawMask := uint32(raw.Mask)
			nameLen := uint32(raw.Len)

			if rawMask&unix.IN_Q_OVERFLOW != 0 {
				rctx.errsink.Error(ErrEventOverflow)
			}

			// If the event happened to the watched directory or the watched file, the kernel
			// doesn't append the filename to the event, but we would like to always fill the
			// the "Name" field with a valid filename. We retrieve the path of the watch from
			// the "paths" map.
			rctx.mu.Lock()
			name, ok := rctx.paths[int(raw.Wd)]
			// IN_DELETE_SELF occurs when the file/directory being watched is removed.
			// This is a sign to clean up the maps, otherwise we are no longer in sync
			// with the inotify kernel state which has already deleted the watch
			// automatically.
			if ok && rawMask&unix.IN_DELETE_SELF == unix.IN_DELETE_SELF {
				delete(rctx.paths, int(raw.Wd))
				delete(rctx.watches, name)
			}
			rctx.mu.Unlock()

			if nameLen > 0 {
				// Point "bytes" at the first byte of the filename
				bytes := (*[unix.PathMax]byte)(unsafe.Pointer(&buf[offset+unix.SizeofInotifyEvent]))[:nameLen:nameLen]
				// The filename is padded with NULL bytes. TrimRight() gets rid of those.
				name += "/" + strings.TrimRight(string(bytes[0:nameLen]), "\000")
			}

			mask := newOpMask(rawMask)
			if !ignoreLinux(name, mask, rawMask) {
				event := api.NewEvent(name, mask)
				rctx.evsink.Event(event)
			}

			// Move to the next event in the buffer
			offset += unix.SizeofInotifyEvent + nameLen
		}
	}
}

func newOpMask(rawMask uint32) api.OpMask {
	var mask api.OpMask
	if rawMask&unix.IN_CREATE == unix.IN_CREATE || rawMask&unix.IN_MOVED_TO == unix.IN_MOVED_TO {
		mask.Set(api.OpCreate)
	}
	if rawMask&unix.IN_DELETE_SELF == unix.IN_DELETE_SELF || rawMask&unix.IN_DELETE == unix.IN_DELETE {
		mask.Set(api.OpRemove)
	}
	if rawMask&unix.IN_MODIFY == unix.IN_MODIFY {
		mask.Set(api.OpWrite)
	}
	if rawMask&unix.IN_MOVE_SELF == unix.IN_MOVE_SELF || rawMask&unix.IN_MOVED_FROM == unix.IN_MOVED_FROM {
		mask.Set(api.OpRename)
	}
	if rawMask&unix.IN_ATTRIB == unix.IN_ATTRIB {
		mask.Set(api.OpChmod)
	}
	return mask
}

// Certain types of events can be "ignored" and not sent over the Events
// channel. Such as events marked ignore by the kernel, or MODIFY events
// against files that do not exist.
func ignoreLinux(name string, mask api.OpMask, rawMask uint32) bool {
	if rawMask&unix.IN_IGNORED != 0 {
		return true
	}

	// If the event is not a DELETE or RENAME, the file must exist.
	// Otherwise the event is ignored.
	// *Note*: this was put in place because it was seen that a MODIFY
	// event was sent after the DELETE. This ignores that MODIFY and
	// assumes a DELETE will come or has come if the file doesn't exist.
	if !mask.IsSet(api.OpRemove) && !mask.IsSet(api.OpRename) {
		_, statErr := os.Lstat(name)
		return os.IsNotExist(statErr)
	}
	return false
}
