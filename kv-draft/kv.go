package kv_draft

import (
	"errors"
	"log/slog"
	"net"
	"net/rpc"
	"sync"
)

type KVServer struct {
	addr string
	data map[string]string
	mu   sync.RWMutex
}

type SetArgs struct {
	Key   string
	Value string
}

type SetReply struct {
	Value string
}

type GetArgs struct {
	Key string
}

type GetReply struct {
	Value string
}

type DelArgs struct {
	Key string
}

type DelReply struct{}

var NotFound = errors.New("not found")

func NewKVServer() *KVServer {
	return &KVServer{
		data: make(map[string]string),
	}
}

func (s *KVServer) Set(args *SetArgs, reply *SetReply) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[args.Key] = args.Value
	reply.Value = args.Value
	return nil
}

func (s *KVServer) Get(args *GetArgs, reply *GetReply) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[args.Key]
	if !ok {
		return NotFound
	}
	reply.Value = value
	return nil
}

func (s *KVServer) Del(args *DelArgs, reply *DelReply) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, args.Key)
	return nil
}

func (s *KVServer) ListenAndServe(addr string) error {
	s.addr = addr
	rpc.Register(s)
	listener, err := net.Listen("tcp", addr)
	slog.Info("listening", "addr", addr)
	if err != nil {
		return err
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go rpc.ServeConn(conn)
	}
}
