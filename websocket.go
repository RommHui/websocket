package websocket

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"golang.org/x/net/proxy"
	"io"
	"net"
	"net/http"
	"strings"
)

type OpCode byte

const (
	ContinuationFrame OpCode = iota
	TextFrame
	BinaryFrame
	ReservedNonControlFrame1
	ReservedNonControlFrame2
	ReservedNonControlFrame3
	ReservedNonControlFrame4
	ReservedNonControlFrame5
	ConnectionClose
	Ping
	Pong
	ReservedControlFrame1
	ReservedControlFrame2
	ReservedControlFrame3
	ReservedControlFrame4
	ReservedControlFrame5
)

var OpCodeName = []string{
	ContinuationFrame:        "ContinuationFrame",
	TextFrame:                "TextFrame",
	BinaryFrame:              "BinaryFrame",
	ReservedNonControlFrame1: "ReservedNonControlFrame1",
	ReservedNonControlFrame2: "ReservedNonControlFrame2",
	ReservedNonControlFrame3: "ReservedNonControlFrame3",
	ReservedNonControlFrame4: "ReservedNonControlFrame4",
	ReservedNonControlFrame5: "ReservedNonControlFrame5",
	ConnectionClose:          "ConnectionClose",
	Ping:                     "Ping",
	Pong:                     "Pong",
	ReservedControlFrame1:    "ReservedControlFrame1",
	ReservedControlFrame2:    "ReservedControlFrame2",
	ReservedControlFrame3:    "ReservedControlFrame3",
	ReservedControlFrame4:    "ReservedControlFrame4",
	ReservedControlFrame5:    "ReservedControlFrame5",
}

func (o OpCode) String() string {
	return OpCodeName[o]
}

type WebSocket interface {
	// Send 发送文本数据
	Send(text string) error

	// Ping 是用来发送一个 ping 帧，判断连接情况。
	// 如果有错误，代表连接有问题。
	// 否则就代表连接一切正常。
	Ping() error

	// Close 用于关闭 WebSocket 对象的流
	Close() error

	// Status 用于获取 WebSocket 对象的状态
	Status() uint8

	// ReadMessage 用于接收 Message 数据
	ReadMessage() (*Message, error)

	// SendMessage 用于发送 Message 数据
	SendMessage(message *Message) error
}

const (
	OPEN uint8 = iota + 1
	CLOSING
	CLOSED
)

type webSocket struct {
	writer io.WriteCloser
	reader io.ReadCloser
	mask   bool
	status uint8
}

// NewWebSocket 使用 io.WriteCloser 和 io.ReadCloser 创建一个 WebSocket 对象。
// io.WriteCloser 是输出流，io.ReadCloser 是输入流，不一定要同一条双向流的 io.WriteCloser 和 io.ReadCloser。
// 这样的好处就是，可以使用 2 条单向的流，模拟成 1 条双向的流。
// 使用 NewWebSocket 这个函数，就可以单独的去使用 WebSocket 协议，无需经过 HTTP 的 Connection Upgrade 到 WebSocket ，也就是可以让一条纯 TCP 连接去使用。
func NewWebSocket(writer io.WriteCloser, reader io.ReadCloser, mask bool) WebSocket {
	return &webSocket{
		writer: writer,
		reader: reader,
		mask:   mask,
		status: OPEN,
	}
}

var tcpDialer = proxy.Dial
var tlsDialer = func(ctx context.Context, network, address string) (net.Conn, error) {
	rawConn, err := tcpDialer(ctx, network, address)
	if err != nil {
		return nil, err
	}
	conn := tls.Client(rawConn, &tls.Config{
		ServerName: address[:strings.LastIndex(address, ":")],
	})
	err = conn.HandshakeContext(ctx)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// New 使用 url 链接来创建一个 WebSocket 对象。
// 可以通过设置环境变量 ALL_PROXY 来使用代理服务器。
//
// 例子1：wss://ws.postman-echo.com/raw/
// 例子2：http://example.com/ws
func New(url string) (WebSocket, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return Connect(context.Background(), req)
}

// Connect 使用一个 HTTP 请求来创建 WebSocket 对象。
// 可以通过设置环境变量 ALL_PROXY 来使用代理服务器。
// 传入 HTTP 请求的方法，可以用于需要验证的 WebSocket 连接，自定义添加验证信息到请求头中。
func Connect(ctx context.Context, request *http.Request) (WebSocket, error) {
	dialer := tcpDialer
	if request.URL.Scheme == "https" || request.URL.Scheme == "wss" {
		dialer = tlsDialer
	}
	return ConnectWithDialer(ctx, dialer, request)
}

// ConnectWithDialer 传入自定义 dialer，然后创建一个 WebSocket 。
// 这个函数主要考虑是用于自定义代理方法来连接目标 WebSocket。
func ConnectWithDialer(ctx context.Context, dialer func(context.Context, string, string) (net.Conn, error), request *http.Request) (WebSocket, error) {

	if len(request.RemoteAddr) < 1 {
		request.RemoteAddr = request.Host
		if len(request.URL.Port()) < 1 {
			if request.URL.Scheme == "https" || request.URL.Scheme == "wss" {
				request.RemoteAddr += ":443"
			} else {
				request.RemoteAddr += ":80"
			}
		}
	}
	conn, err := dialer(ctx, "tcp", request.RemoteAddr)
	if err != nil {
		return nil, err
	}

	request.Header.Set("sec-websocket-key", getSecWebsocketKey())
	request.Header.Set("sec-websocket-version", "13")
	request.Header.Set("connection", "upgrade")
	request.Header.Set("upgrade", "websocket")

	err = request.Write(conn)
	if err != nil {
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 101 {
		return nil, errors.New(resp.Status)
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("connection")), "upgrade") {
		return nil, fmt.Errorf("WebSocket connection to '%s' failed", request.URL)
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("upgrade")), "websocket") {
		return nil, fmt.Errorf("WebSocket connection to '%s' failed", request.URL)
	}
	secAcceptKey, err := getSecAcceptKey(request.Header.Get("sec-websocket-key"))
	if err != nil {
		return nil, err
	}
	if secAcceptKey != resp.Header.Get("sec-websocket-accept") {
		return nil, fmt.Errorf("WebSocket connection to '%s' failed", request.URL)
	}
	return NewWebSocket(conn, conn, true), nil
}

// Pair 用于 HTTP 服务端接收一个 WebSocket 对象
//
// 使用例子：
//
//	http.HandleFunc("/ws",func(w http.ResponseWriter,request *http.Request){
//		ws,err := websocket.Pair(w,request)
//		if err != nil {
//			return
//		}
//		fmt.Println(ws)
//	})
//	http.ListenAndServe("0.0.0.0:8080")
func Pair(w http.ResponseWriter, req *http.Request) (WebSocket, error) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return nil, err
	}
	return pair(conn, conn, req)
}

// ServerPair 用于传入 io.WriteCloser 和 io.ReadCloser 来创建 WebSocket。
// 可以用于自己编写的 WEB 服务来创建一个 WebSocket 对象。
func ServerPair(writer io.WriteCloser, reader io.ReadCloser) (WebSocket, error) {
	req, err := http.ReadRequest(bufio.NewReader(reader))
	if err != nil {
		return nil, err
	}
	return pair(writer, reader, req)
}

func pair(writer io.WriteCloser, reader io.ReadCloser, request *http.Request) (WebSocket, error) {
	if !strings.Contains(strings.ToLower(request.Header.Get("connection")), "upgrade") {
		return nil, errors.New("request header `connection` is not equal to 'upgrade'")
	}
	if !strings.Contains(strings.ToLower(request.Header.Get("upgrade")), "websocket") {
		return nil, errors.New("request header `upgrade` is not equal to 'websocket'")
	}
	if request.Header.Get("sec-websocket-version") != "13" {
		return nil, errors.New("request header `sec-websocket-version` is not equal to '13'")
	}

	secAcceptKey, err := getSecAcceptKey(request.Header.Get("sec-websocket-key"))
	if err != nil {
		return nil, err
	}
	response := []string{
		"HTTP/1.1 101 Switching Protocols",
		"Sec-Websocket-Accept: " + secAcceptKey,
		"Upgrade: websocket",
		"Connection: upgrade",
		"\r\n",
	}
	_, err = writer.Write([]byte(strings.Join(response, "\r\n")))
	if err != nil {
		return nil, err
	}
	return NewWebSocket(writer, reader, false), nil
}

func (w *webSocket) Send(text string) error {
	return w.SendMessage(&Message{
		Reader: bytes.NewBufferString(text),
		OpCode: TextFrame,
	})
}

func (w *webSocket) Ping() error {
	return w.ping()
}

func (w *webSocket) Close() error {
	err := w.sendMessage(&Message{
		OpCode: ConnectionClose,
	})
	w.status = CLOSING
	if err != nil {
		return err
	}
	for _, closeFn := range []func() error{w.writer.Close, w.reader.Close} {
		if closeErr := closeFn(); closeErr != nil && errors.Is(err, net.ErrClosed) {
			return closeErr
		}
	}
	w.status = CLOSED
	return nil
}

func (w *webSocket) Status() uint8 {
	return w.status
}

var (
	ErrClosedStatus = errors.New("WebSocket is already in CLOSING or CLOSED state")
)

func (w *webSocket) ping() error {
	message := &Message{
		OpCode: Ping,
	}
	err := w.SendMessage(message)
	if err != nil {
		return err
	}
	for {
		message, err = w.ReadMessage()
		if err != nil {
			return err
		}
		if message.OpCode != Pong {
			continue
		} else if _, err = io.Copy(blackhole, message); err != nil {
			return err
		} else {
			return nil
		}
	}
}

func (w *webSocket) responsePong(ping *Message) error {
	ping.OpCode = Pong
	return w.SendMessage(ping)
}

func (w *webSocket) sendFrame(ctx context.Context, frame *Frame) error {
	if w.status > OPEN {
		return ErrClosedStatus
	}
	_, err := io.Copy(w.writer, contextReader(ctx, frame.encodeFrame()))
	return err
}

func (w *webSocket) readFrame(ctx context.Context) (*Frame, error) {
	if w.status > OPEN {
		return nil, ErrClosedStatus
	}
	frame := &Frame{}
	err := frame.decodeFrame(ctx, w.reader)
	if err != nil {
		return nil, err
	}
	return frame, nil
}
