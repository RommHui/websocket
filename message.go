package websocket

import (
	"bytes"
	"context"
	"errors"
	"io"
)

type Message struct {
	io.Reader
	OpCode OpCode
}

func (w *webSocket) sendMessage(message *Message) error {
	ctx := context.Background()
	frame := &Frame{
		Payload: nil,
		Fin:     false,
		Mask:    w.mask,
		OpCode:  message.OpCode,
	}
	buf := make([]byte, 2048)
	offset := 0
	if message.Reader == nil {
		message.Reader = emptyReader
	}
	for {
		n, err := message.Read(buf[offset:])
		if err != nil && err != io.EOF {
			return err
		}
		offset += n
		if err == nil && n < len(buf) {
			continue
		}
		frame.Payload = &io.LimitedReader{
			R: bytes.NewBuffer(buf[:offset]),
			N: int64(offset),
		}
		frame.Fin = err != nil
		err = w.sendFrame(ctx, frame)
		if err != nil {
			return err
		}
		if frame.Fin {
			return nil
		}
		offset = 0
		frame.OpCode = ContinuationFrame
	}
}

func (w *webSocket) SendMessage(message *Message) error {
	return w.sendMessage(message)
}

var ErrPreviousMessageNotReadToCompletion = errors.New("previous message not read to completion")

func (w *webSocket) readMessage() (*Message, error) {
	ctx := context.Background()
	frame, err := w.readFrame(ctx)
	if err != nil {
		return nil, err
	}
	return &Message{
		Reader: rwFunc(func(b []byte) (int, error) {
			for {
				if frame != nil {
					n, readErr := frame.Payload.Read(b)
					if readErr == io.EOF && frame.Fin != true {
						readErr = nil
						frame = nil
					}
					return n, readErr
				}
				frame, err = w.readFrame(ctx)
				if err != nil {
					return 0, err
				}
				if frame.OpCode != ContinuationFrame {
					return 0, ErrPreviousMessageNotReadToCompletion
				}
			}
		}),
		OpCode: frame.OpCode,
	}, nil
}

func (w *webSocket) ReadMessage() (*Message, error) {
	for {
		message, err := w.readMessage()
		if err != nil {
			return nil, err
		}
		if message.OpCode == Ping {
			err = w.responsePong(message)
			if err != nil {
				return nil, err
			}
		} else if message.OpCode == ConnectionClose {
			err = w.Close()
			if err != nil {
				return nil, err
			}
		}
		return message, nil
	}
}
