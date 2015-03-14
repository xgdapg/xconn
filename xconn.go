package xconn

import (
	"encoding/binary"
	"net"
	"time"
)

type Conn struct {
	conn         net.Conn
	send         chan *MsgData
	msgHandler   MsgHandler
	recvBuffer   []byte
	msgPacker    MsgPacker
	pingInterval uint
	pingHandler  PingHandler
	pingStop     chan bool
	closeHandler CloseHandler
}

type MsgHandler interface {
	HandleMsg(*MsgData)
}

type MsgPacker interface {
	PackMsg(*MsgData) []byte
	UnpackMsg([]byte) *MsgData
}

type MsgData struct {
	Data []byte
	Ext  interface{}
}

type PingHandler interface {
	HandlePing()
}

type CloseHandler interface {
	HandleClose()
}

func NewConn(conn net.Conn) *Conn {
	c := &Conn{
		conn:         conn,
		send:         make(chan *MsgData, 64),
		msgHandler:   nil,
		recvBuffer:   []byte{},
		msgPacker:    nil,
		pingInterval: 0,
		pingHandler:  nil,
		pingStop:     nil,
		closeHandler: nil,
	}

	go c.recvLoop()
	go c.sendLoop()

	return c
}

func recoverPanic() {
	if err := recover(); err != nil {
		//fmt.Println(err)
	}
}

func (this *Conn) SetMsgHandler(hdlr MsgHandler) {
	this.msgHandler = hdlr
}

func (this *Conn) SetMsgPacker(packer MsgPacker) {
	this.msgPacker = packer
}

func (this *Conn) SetPing(sec uint, hdlr PingHandler) {
	this.pingInterval = sec
	this.pingHandler = hdlr
	if this.pingStop == nil {
		this.pingStop = make(chan bool)
	}
	if sec > 0 {
		go this.pingLoop()
	}
}

func (this *Conn) SetCloseHandler(hdlr CloseHandler) {
	this.closeHandler = hdlr
}

func (this *Conn) pingLoop() {
	defer recoverPanic()
	for {
		select {
		case <-this.pingStop:
			return
		case <-time.After(time.Duration(this.pingInterval) * time.Second):
			this.Ping()
		}
	}
}

func (this *Conn) RawConn() net.Conn {
	return this.conn
}

func (this *Conn) recvLoop() {
	defer recoverPanic()
	defer this.Close()
	buffer := make([]byte, 2048)
	for {
		bytesRead, err := this.conn.Read(buffer)
		if err != nil {
			return
		}

		this.recvBuffer = append(this.recvBuffer, buffer[0:bytesRead]...)
		for len(this.recvBuffer) > 4 {
			length := binary.BigEndian.Uint32(this.recvBuffer[0:4])
			readToPtr := length + 4
			if uint32(len(this.recvBuffer)) < readToPtr {
				break
			}
			if length == 0 {
				if this.pingHandler != nil {
					this.pingHandler.HandlePing()
				}
			} else {
				buf := this.recvBuffer[4:readToPtr]
				go this.recvMsg(buf)
			}
			this.recvBuffer = this.recvBuffer[readToPtr:]
		}
	}
}

func (this *Conn) recvMsg(data []byte) {
	defer recoverPanic()
	msg := &MsgData{
		Data: data,
		Ext:  nil,
	}
	if this.msgPacker != nil {
		msg = this.msgPacker.UnpackMsg(data)
	}
	if this.msgHandler != nil {
		this.msgHandler.HandleMsg(msg)
	}
}

func (this *Conn) sendLoop() {
	defer recoverPanic()
	for {
		msg, ok := <-this.send
		if !ok {
			break
		}

		go this.sendMsg(msg)
	}
}

func (this *Conn) sendMsg(msg *MsgData) {
	defer recoverPanic()
	sendBytes := make([]byte, 4)
	if msg != nil {
		data := msg.Data
		if this.msgPacker != nil {
			data = this.msgPacker.PackMsg(msg)
		}
		length := len(data)
		binary.BigEndian.PutUint32(sendBytes, uint32(length))
		sendBytes = append(sendBytes, data...)
	}
	this.conn.Write(sendBytes)
}

func (this *Conn) Close() {
	defer recoverPanic()
	this.conn.Close()
	close(this.send)
	if this.pingStop != nil {
		close(this.pingStop)
	}
	if this.closeHandler != nil {
		this.closeHandler.HandleClose()
	}
}

func (this *Conn) SendMsg(msg *MsgData) {
	this.send <- msg
}

func (this *Conn) SendData(data []byte) {
	this.SendMsg(&MsgData{Data: data, Ext: nil})
}

func (this *Conn) Ping() {
	go this.sendMsg(nil)
}
