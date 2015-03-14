## Usage
```go
package main

import (
	"fmt"
	"github.com/xgdapg/xconn"
	"net"
	"time"
)

func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:8000")
	if err != nil {
		return
	}
	c := xconn.NewConn(conn)
	h := &Handler{conn: c}
	c.SetMsgHandler(h)
	c.SetCloseHandler(h)
	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		c.SendData([]byte("hello!"))
	}
	time.Sleep(10 * time.Second)
}

type Handler struct {
	conn *xconn.Conn
}

func (this *Handler) HandleMsg(msg *xconn.MsgData) {
	fmt.Println("RECV: " + string(msg.Data))
}

func (this *Handler) HandleClose() {
	fmt.Println("CLOSED")
}
```