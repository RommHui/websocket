package websocket

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"time"
)

var emptyReader = &io.LimitedReader{
	R: nil,
	N: 0,
}

type rwFunc func([]byte) (int, error)

func (rw rwFunc) Read(p []byte) (int, error) {
	return rw(p)
}

func (rw rwFunc) Write(p []byte) (int, error) {
	return rw(p)
}

func maskReader(maskKey []byte, reader io.Reader) io.Reader {
	i := 0
	return rwFunc(func(p []byte) (int, error) {
		n, err := reader.Read(p)
		for k := 0; k < n; k++ {
			p[k] ^= maskKey[i]
			i = (i + 1) & 0b11
		}
		return n, err
	})
}

func contextReader(ctx context.Context, reader io.Reader) io.Reader {
	return rwFunc(func(b []byte) (int, error) {
		select {

		case <-ctx.Done():
			return 0, context.DeadlineExceeded
		default:
			return reader.Read(b)
		}
	})
}

func mustRead(ctx context.Context, reader io.Reader, p []byte) (int, error) {
	if len(p) < 1 {
		return 0, nil
	}
	read := 0
	for read < len(p) {
		select {
		case <-ctx.Done():
			return read, ctx.Err()
		default:
			l, err := reader.Read(p[read:])
			read += l
			if err != nil {
				return read, err
			}
		}
	}
	return read, nil
}

var blackhole = rwFunc(func(b []byte) (int, error) {
	return len(b), nil
})

func getSecWebsocketKey() string {
	return base64.StdEncoding.EncodeToString([]byte(time.Now().Format("20060102_150405")))
}

func getSecAcceptKey(SecWebsocketKey string) (string, error) {
	hash := sha1.New()
	for _, data := range []string{SecWebsocketKey, "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"} {
		n, err := hash.Write([]byte(data))
		if err != nil {
			return "", err
		}
		if n < len(data) {
			return "", io.ErrShortWrite
		}
	}
	return base64.StdEncoding.EncodeToString(hash.Sum(nil)), nil
}
