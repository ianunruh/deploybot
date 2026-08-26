// Package valkey is a tiny RESP GET/SET client. No connection pool: each
// call dials, runs one command, and closes. Enough for off-request-path
// hydrate and persist of cluster live snapshots.
package valkey

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout = 2 * time.Second
	maxBulk        = 16 << 20
)

// Client talks to a Valkey/Redis server at Addr.
type Client struct {
	Addr    string
	Timeout time.Duration
}

func (c *Client) timeout() time.Duration {
	if c != nil && c.Timeout > 0 {
		return c.Timeout
	}
	return defaultTimeout
}

func (c *Client) addr() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Addr)
}

// Get returns the bulk value for key, or nil if it is missing.
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	reply, err := c.do(ctx, []string{"GET", key})
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// Set writes value at key.
func (c *Client) Set(ctx context.Context, key string, value []byte) error {
	_, err := c.do(ctx, []string{"SET", key, string(value)})
	return err
}

func (c *Client) do(ctx context.Context, args []string) ([]byte, error) {
	if c.addr() == "" {
		return nil, fmt.Errorf("valkey: empty addr")
	}
	d := net.Dialer{Timeout: c.timeout()}
	conn, err := d.DialContext(ctx, "tcp", c.addr())
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	deadline := time.Now().Add(c.timeout())
	if t, ok := ctx.Deadline(); ok && t.Before(deadline) {
		deadline = t
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if err := writeCommand(conn, args); err != nil {
		return nil, err
	}
	return readReply(bufio.NewReader(conn))
}

func writeCommand(w io.Writer, args []string) error {
	var b strings.Builder
	b.WriteString("*")
	b.WriteString(strconv.Itoa(len(args)))
	b.WriteString("\r\n")
	for _, a := range args {
		b.WriteString("$")
		b.WriteString(strconv.Itoa(len(a)))
		b.WriteString("\r\n")
		b.WriteString(a)
		b.WriteString("\r\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func readReply(r *bufio.Reader) ([]byte, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	switch prefix {
	case '+':
		return []byte(line), nil
	case '-':
		return nil, fmt.Errorf("valkey: %s", line)
	case ':':
		return []byte(line), nil
	case '$':
		n, err := strconv.Atoi(line)
		if err != nil {
			return nil, fmt.Errorf("valkey bulk: %w", err)
		}
		if n < 0 {
			return nil, nil
		}
		if n > maxBulk {
			return nil, fmt.Errorf("valkey bulk: %d bytes", n)
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		if buf[n] != '\r' || buf[n+1] != '\n' {
			return nil, fmt.Errorf("valkey bulk: missing CRLF")
		}
		return buf[:n], nil
	default:
		return nil, fmt.Errorf("valkey: unexpected prefix %q", prefix)
	}
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}
