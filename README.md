# WebSocket implementation for Go

## Description

A simple websocket protocol implementation for Golang.

## Installation

```bash
go get -u github.com/RommHui/websocket
```

## Examples

### 0x01 New

use the method `New` to connect to the websocket server through a URL

```go
package main

import (
	"bytes"
	"fmt"
	"github.com/Rommhui/websocket"
)

func main() {
	ws, err := websocket.New("http://example.com/ws")
	if err != nil {
		panic(err)
	}
	defer ws.Close()
	
	err = ws.Ping() // try to detect if the connection is normal.
	if err != nil {
		panic(err)
    }
	
	err = ws.Send("Hello World") // send a text to server.
	if err != nil {
		panic(err)
	}
	
	message, err := ws.ReadMessage() // receive a message from server.
	if err != nil {
		panic(err)
	}
	buf := &bytes.Buffer{}
	_, err = buf.ReadFrom(message)
	if err != nil {
		panic(err)
	}
	fmt.Println(buf.String())
}
```

### 0x02 SendMessage

use the method `SendMessage` to send a frame package as you wish

```go
package main

import (
	"bytes"
	"github.com/RommHui/websocket"
)

func main() {
	ws, err := websocket.New("http://example.com/ws")
	if err != nil {
		panic(err)
	}
	defer ws.Close()

	err = ws.SendMessage(&websocket.Message{
		Reader: bytes.NewBufferString("Hello"),
		OpCode: websocket.TextFrame,
	})
	if err != nil {
		panic(err)
    }
}
```

### 0x03 Pair

use the method `Pair` to pair a websocket connection from client

```go
package main

import (
	"github.com/RommHui/websocket"
	"net/http"
)

func main() {
	http.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
		ws,err := websocket.Pair(writer,request)
		if err != nil {
			return
        }
		defer ws.Close()
		
		err = ws.Send("Hi")
		if err != nil {
			return
        }
	})
}
```

### 0x04 NewWebSocket

use `NewWebSocket` can using WebSocket protocol as a independence application layer protocol to transmit data

```go
// server
package main

import (
	"github.com/RommHui/websocket"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", "0.0.0.0:8080")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	for {
		clientConn, acceptErr := listener.Accept()
		if acceptErr != nil {
			panic(acceptErr)
		}
		go func(conn net.Conn) {
			ws := websocket.NewWebSocket(conn,conn,true)
			defer ws.Close()
			err := ws.Ping()
			if err != nil {
				return
            }
			err = ws.Send("HI")
			if err != nil {
				return
            }
		}(clientConn)
	}
}
```

```go
// client
package main

import (
	"bytes"
	"fmt"
	"github.com/RommHui/websocket"
	"net"
)

func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		panic(err)
	}

	ws := websocket.NewWebSocket(conn, conn, true)
	defer ws.Close()

	message, err := ws.ReadMessage()
	if err != nil {
		panic(err)
	}

	buf := &bytes.Buffer{}
	_, err = buf.ReadFrom(message)
	if err != nil {
		panic(err)
	}
	fmt.Println(buf.String())
}
```

### 0x05 Connect

use the method `Connect` to addition some authorization information to the http request

```go
package main

import (
	"context"
	"github.com/RommHui/websocket"
	"net/http"
)

func main() {
	request, err := http.NewRequest("GET", "http://example.com/ws", nil)
	if err != nil {
		panic(err)
	}
	request.Header.Set("authorization", "Bearer a1b2c3d4")
	request.Header.Set("user-agent", "testing websocket client")

	ws,err := websocket.Connect(context.Background(),request)
	if err != nil {
		panic(err)
    }
	defer ws.Close()
	
	err = ws.Send("Hello World")
	if err != nil {
		panic(err)
    }
}
```

### 0x06 ServerPair

use `ServerPair` to enable websocket support in your raw TCP listening program

```go
package main

import (
	"github.com/RommHui/websocket"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", "0.0.0.0:8080")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	for {
		clientConn, acceptErr := listener.Accept()
		if acceptErr != nil {
			panic(acceptErr)
		}
		go func(conn net.Conn) {
			ws,err := websocket.ServerPair(conn,conn)
			if err != nil {
				return
            }
			defer ws.Close()
			err = ws.Ping()
			if err != nil {
				return
			}
			err = ws.Send("HI")
			if err != nil {
				return
			}
		}(clientConn)
	}
}
```

### 0x07 Two in One

use the method `NewWebSocket` to make tow HTTP request as a websocket connection

```go
package main

import (
	"github.com/RommHui/websocket"
	"io"
	"net/http"
)

func write(id string, url string) (io.WriteCloser, error) {
	reader, writer := io.Pipe()
	req, err := http.NewRequest("PUT", url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("connection", id)
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return writer, nil
}

func read(id string, url string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("connection", id)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func main() {
	writer, err := write("123456", "http://127.0.0.1:8080")
	if err != nil {
		panic(err)
	}
	reader, err := read("123456", "http://127.0.0.1:8080")
	if err != nil {
		panic(err)
	}
	ws := websocket.NewWebSocket(writer,reader,false)
	defer ws.Close()
	
	err = ws.Ping()
	if err != nil {
		panic(err)
    }
}
```
