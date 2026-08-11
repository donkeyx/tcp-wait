package tcpwait

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// probePostgres sends a StartupMessage and treats the server as ready when
// it responds with Authentication* or an ErrorResponse that is not
// "database system is starting up" / "not yet accepting connections".
//
// No credentials needed: auth challenge or "role does not exist" both mean
// the postmaster is accepting connections.
func probePostgres(ctx context.Context, addr, user, database string, timeout time.Duration) error {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}

	if err := writeStartup(conn, user, database); err != nil {
		return err
	}

	// Read messages until we can decide ready vs not.
	for {
		msgType, payload, err := readPGMessage(conn)
		if err != nil {
			return err
		}
		switch msgType {
		case 'R': // Authentication*
			return nil
		case 'E': // ErrorResponse
			fields := parsePGError(payload)
			msg := fields['M']
			if isPostgresStartingUp(msg) {
				return fmt.Errorf("postgres not ready: %s", msg)
			}
			// Other errors (auth failed, unknown role, etc.) still mean the server is up.
			return nil
		case 'N': // NoticeResponse — ignore, keep reading
			continue
		case 'S': // ParameterStatus — server is clearly up
			return nil
		case 'K': // BackendKeyData
			return nil
		case 'Z': // ReadyForQuery
			return nil
		default:
			// Unknown but got a framed message → postmaster is speaking.
			return nil
		}
	}
}

func writeStartup(w io.Writer, user, database string) error {
	// StartupMessage (no type byte): int32 len | int32 protocol | key\0val\0... | \0
	const protocol = 196608 // 3.0
	var body []byte
	body = binary.BigEndian.AppendUint32(body, protocol)
	body = appendParams(body, "user", user)
	body = appendParams(body, "database", database)
	body = append(body, 0)

	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(body)+4))
	if _, err := w.Write(lenBuf); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func appendParams(b []byte, key, val string) []byte {
	b = append(b, key...)
	b = append(b, 0)
	b = append(b, val...)
	b = append(b, 0)
	return b
}

func readPGMessage(r io.Reader) (byte, []byte, error) {
	hdr := make([]byte, 5)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return 0, nil, err
	}
	msgType := hdr[0]
	length := binary.BigEndian.Uint32(hdr[1:5])
	if length < 4 {
		return 0, nil, fmt.Errorf("invalid postgres message length %d", length)
	}
	payloadLen := int(length) - 4
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, err
		}
	}
	return msgType, payload, nil
}

func parsePGError(payload []byte) map[byte]string {
	fields := make(map[byte]string)
	i := 0
	for i < len(payload) {
		code := payload[i]
		if code == 0 {
			break
		}
		i++
		start := i
		for i < len(payload) && payload[i] != 0 {
			i++
		}
		fields[code] = string(payload[start:i])
		if i < len(payload) {
			i++ // skip NUL
		}
	}
	return fields
}

func isPostgresStartingUp(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "starting up") ||
		strings.Contains(m, "not yet accepting connections") ||
		strings.Contains(m, "the database system is in recovery mode")
}
