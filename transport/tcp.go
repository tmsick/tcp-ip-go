package transport

import (
	"context"
	"fmt"
	"log"

	"github.com/kawa1214/tcp-ip-go/link"
	"github.com/kawa1214/tcp-ip-go/network"
)

const (
	QUEUE_SIZE = 10
)

type TcpPacket struct {
	IpHeader  *network.Header
	TcpHeader *Header
	Packet    link.Packet
}

type TcpPacketQueue struct {
	manager       *ConnectionManager
	outgoingQueue chan link.Packet
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewTcpPacketQueue() *TcpPacketQueue {
	ConnectionManager := NewConnectionManager()
	return &TcpPacketQueue{
		manager:       ConnectionManager,
		outgoingQueue: make(chan link.Packet, QUEUE_SIZE),
	}
}

func (tcp *TcpPacketQueue) ManageQueues(ip *network.IpPacketQueue) {
	tcp.ctx, tcp.cancel = context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-tcp.ctx.Done():
				return
			default:
				ipPkt, err := ip.Read()
				if err != nil {
					log.Printf("read error: %s", err.Error())
				}
				tcpHeader, err := Parse(ipPkt.Packet.Buf[ipPkt.IpHeader.IHL*4 : ipPkt.Packet.N])
				if err != nil {
					log.Printf("parse error: %s", err)
					continue
				}
				tcpPacket := TcpPacket{
					IpHeader:  ipPkt.IpHeader,
					TcpHeader: tcpHeader,
					Packet:    ipPkt.Packet,
				}

				tcp.manager.recv(tcp, tcpPacket)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-tcp.ctx.Done():
				return
			default:
				select {
				case pkt := <-tcp.outgoingQueue:
					err := ip.Write(pkt)
					if err != nil {
						log.Printf("write error: %s", err.Error())
					}
				}
			}
		}
	}()
}

func (tcp *TcpPacketQueue) Close() {
	tcp.cancel()
}

func (tcp *TcpPacketQueue) Write(from, to TcpPacket, data []byte) {

	ipHdr := to.IpHeader.Marshal()
	tcpHdr := to.TcpHeader.Marshal(from.IpHeader, data)

	pkt := append(ipHdr, tcpHdr...)
	pkt = append(pkt, data...)

	tcp.outgoingQueue <- link.Packet{
		Buf: pkt,
		N:   uintptr(len(pkt)),
	}
}

func (tcp *TcpPacketQueue) ReadConnection() (Connection, error) {
	pkt, ok := <-tcp.manager.ConnectionQueue
	if !ok {
		return Connection{}, fmt.Errorf("connection queue is closed")
	}

	return pkt, nil
}
