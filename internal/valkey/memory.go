package valkey

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
)

// Memory is an in-process GET/SET server for tests.
type Memory struct {
	mu sync.Mutex
	kv map[string]string
}

// Listen serves GET/SET on ln until it is closed.
func (m *Memory) Listen(ln net.Listener) {
	if m.kv == nil {
		m.kv = map[string]string{}
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go m.serve(conn)
	}
}

func (m *Memory) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	args, err := readArray(r)
	if err != nil || len(args) == 0 {
		return
	}
	switch args[0] {
	case "GET":
		if len(args) < 2 {
			return
		}
		m.mu.Lock()
		v, ok := m.kv[args[1]]
		m.mu.Unlock()
		if !ok {
			_, _ = io.WriteString(conn, "$-1\r\n")
			return
		}
		_, _ = fmt.Fprintf(conn, "$%d\r\n%s\r\n", len(v), v)
	case "SET":
		if len(args) < 3 {
			return
		}
		m.mu.Lock()
		m.kv[args[1]] = args[2]
		m.mu.Unlock()
		_, _ = io.WriteString(conn, "+OK\r\n")
	default:
		_, _ = io.WriteString(conn, "-ERR unknown\r\n")
	}
}

func readArray(r *bufio.Reader) ([]string, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if prefix != '*' {
		return nil, fmt.Errorf("prefix %q", prefix)
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(trimCRLF(line))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, n)
	for range n {
		if _, err := r.ReadByte(); err != nil {
			return nil, err
		}
		lenLine, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(trimCRLF(lenLine))
		if err != nil {
			return nil, err
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		out = append(out, string(buf[:size]))
	}
	return out, nil
}

func trimCRLF(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	return s
}
