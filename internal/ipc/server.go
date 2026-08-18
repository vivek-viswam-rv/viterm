// Package ipc implements the Unix-domain-socket transport shared by the
// viterm TUI (server) and the viterm CLI (client). See internal/protocol for
// the wire schema exchanged over the socket.
package ipc

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/vivekviswam/viterm/internal/protocol"
)

// Handler processes a decoded Command and returns the raw reply bytes to
// write back to the client, or nil to send no reply at all.
type Handler func(protocol.Command) []byte

// Server accepts connections on a per-session Unix domain socket and
// dispatches each request to a Handler.
type Server struct {
	path     string
	ln       net.Listener
	handler  Handler
	done     chan struct{}
	closeErr error
	wg       sync.WaitGroup
	once     sync.Once
}

var staleSockRe = regexp.MustCompile(`^viterm-(\d+)-[0-9a-f]+\.sock$`)

// NewServer creates and binds a new session socket at
// /tmp/viterm-<pid>-<hex8>.sock. The caller must call Start to begin
// accepting connections and Close to release resources.
func NewServer() (*Server, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return nil, fmt.Errorf("ipc: generating socket suffix: %w", err)
	}
	name := fmt.Sprintf("viterm-%d-%s.sock", os.Getpid(), hex.EncodeToString(buf[:]))
	path := filepath.Join(os.TempDir(), name)

	// Remove any leftover socket file at this exact path (extremely unlikely
	// given the random suffix, but net.Listen fails if it already exists).
	_ = os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("ipc: listening on %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		os.Remove(path)
		return nil, fmt.Errorf("ipc: chmod %s: %w", path, err)
	}

	return &Server{
		path: path,
		ln:   ln,
		done: make(chan struct{}),
	}, nil
}

// Path returns the filesystem path of the server's Unix domain socket.
func (s *Server) Path() string {
	return s.path
}

// Start begins accepting connections in a background goroutine, dispatching
// each decoded Command to h.
func (s *Server) Start(h Handler) error {
	s.handler = h
	s.wg.Add(1)
	go s.acceptLoop()
	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				// Close was called; this Accept error is expected.
				return
			default:
				// Unexpected accept error; keep the loop from spinning hot
				// while remaining ready for a subsequent Close.
				select {
				case <-s.done:
					return
				case <-time.After(10 * time.Millisecond):
					continue
				}
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	data, readErr := io.ReadAll(conn)
	if readErr != nil && len(data) == 0 {
		// Nothing usable was read (e.g. deadline exceeded before any bytes
		// arrived); there's no request to respond to.
		return
	}

	var cmd protocol.Command
	if err := json.Unmarshal(data, &cmd); err != nil {
		reply, merr := json.Marshal(protocol.Reply{OK: false, Error: "malformed request: " + err.Error()})
		if merr == nil {
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			_, _ = conn.Write(reply)
		}
		return
	}

	reply := s.dispatch(cmd)
	if reply != nil {
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, _ = conn.Write(reply)
	}
}

// dispatch calls the handler, recovering from any panic and converting it
// into an error Reply so a misbehaving handler can never crash the server or
// hang a client waiting for a response.
func (s *Server) dispatch(cmd protocol.Command) (reply []byte) {
	defer func() {
		if r := recover(); r != nil {
			b, err := json.Marshal(protocol.Reply{OK: false, Error: fmt.Sprintf("handler panic: %v", r)})
			if err == nil {
				reply = b
			}
		}
	}()
	if s.handler == nil {
		return nil
	}
	return s.handler(cmd)
}

// Close stops the accept loop, closes the listener, and unlinks the socket
// file. It is safe to call multiple times.
func (s *Server) Close() error {
	s.once.Do(func() {
		close(s.done)
		s.closeErr = s.ln.Close()
		s.wg.Wait()
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			if s.closeErr == nil {
				s.closeErr = err
			}
		}
	})
	return s.closeErr
}

// CleanStale removes viterm session socket files left behind by processes
// that are no longer running. It never touches a socket belonging to a live
// pid, even if that pid belongs to an unrelated process that has since
// reused the number in a way this check cannot detect.
func CleanStale() error {
	dir := os.TempDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("ipc: reading %s: %w", dir, err)
	}

	var firstErr error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := staleSockRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		pid, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if pidAlive(pid) {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// pidAlive reports whether pid refers to a currently running process,
// determined by sending it the null signal.
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if err == syscall.ESRCH {
		return false
	}
	// Any other error (e.g. EPERM: process exists but is owned by someone
	// else) means we cannot prove it's dead, so treat it as alive and leave
	// its socket alone.
	return true
}
