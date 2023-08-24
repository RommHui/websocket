package websocket

import (
	"context"
	"fmt"
	"io"
	"math/rand"
)

type Frame struct {
	Payload *io.LimitedReader
	Fin     bool
	Mask    bool
	OpCode  OpCode
}

func (f *Frame) String() string {
	if f.Payload == nil {
		f.Payload = emptyReader
	}
	return fmt.Sprintf("Frame(%s){Fin:%v Mask:%v PayloadLen:%d}", f.OpCode, f.Fin, f.Mask, f.Payload.N)
}

// Decode 用于从 io.Reader 中反序列化到 Frame
func (f *Frame) Decode(ctx context.Context, reader io.Reader) error {
	buf := make([]byte, 8)
	_, err := mustRead(ctx, reader, buf[:2])
	if err != nil {
		return err
	}
	f.Fin = buf[0]&0b10000000 > 0
	f.OpCode = OpCode(buf[0] & 0b00001111)
	f.Mask = buf[1]&0b10000000 > 0
	f.Payload = &io.LimitedReader{}
	f.Payload.N = int64(buf[1] & 0b01111111)
	extendPayloadLength := 0
	if f.Payload.N == 126 {
		extendPayloadLength, err = mustRead(ctx, reader, buf[:2])
		if err != nil {
			return err
		}
	} else if f.Payload.N == 127 {
		extendPayloadLength, err = mustRead(ctx, reader, buf[:8])
		if err != nil {
			return err
		}
	}
	if extendPayloadLength > 0 {
		f.Payload.N = int64(bigEndianUint64Unpack(buf[:extendPayloadLength]))
	}
	maskKey := buf[:4]
	if f.Mask {
		_, err = mustRead(ctx, reader, maskKey)
		if err != nil {
			return err
		}
		reader = maskReader(maskKey, reader)
	}
	f.Payload.R = reader
	return nil
}

// Encode 用于从 Frame 中把数据序列化
func (f *Frame) Encode() io.Reader {
	buf := make([]byte, 14)
	headerLen := 2
	if f.Fin {
		buf[0] |= 0b10000000
	}
	buf[0] |= byte(f.OpCode)

	maskKey := []byte{byte(rand.Int()), byte(rand.Int()), byte(rand.Int()), byte(rand.Int())}
	extendedPayloadLen := 0
	if f.Payload == nil {
		f.Payload = emptyReader
	}
	if f.Payload.N < 125 {
		buf[1] |= byte(f.Payload.N)
	} else if f.Payload.N < 1<<16 {
		buf[1] |= 126
		extendedPayloadLen = 2
	} else {
		buf[1] |= 127
		extendedPayloadLen = 8
	}
	if extendedPayloadLen > 0 {
		bigEndianUint64Pack(buf[2:extendedPayloadLen+2], uint64(f.Payload.N))
		headerLen += extendedPayloadLen
	}
	if f.Mask {
		buf[1] |= 0b10000000
		headerLen += copy(buf[2+extendedPayloadLen:], maskKey)
		f.Payload.R = maskReader(maskKey, f.Payload.R)
	}

	return io.MultiReader(newBytesBuffer(buf[:headerLen]), f.Payload)
}
