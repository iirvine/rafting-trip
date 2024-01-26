package draft

import (
	"context"
	"github.com/dustinkirkland/golang-petname"
	"sync"
)

// assert MockTransport implements Transport
var _ Transport = &MockTransport{}

func LocalAddr() NodeAddr {
	return NodeAddr{
		Host: petname.Generate(2, "-") + ".com",
		Port: 1234,
	}
}

type MockTransport struct {
	mu    sync.Mutex
	inbox chan RPC
	id    ServerID
	peers map[ServerID]*MockTransport
	addr  NodeAddr
}

func (m *MockTransport) RequestVote(
	ctx context.Context,
	env Envelope,
	args *RequestVoteReq,
	resp *RequestVoteResponse,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	peer := m.peers[env.To.Id]
	respCh := make(chan RPCResult)
	rpc := RPC{
		Envelope: env,
		Response: respCh,
	}
	peer.inbox <- rpc
	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-respCh:
		out := result.Resp.(*RequestVoteResponse)
		*resp = *out
	}
	return nil
}

func (m *MockTransport) AppendEntries(
	ctx context.Context,
	env Envelope,
	args *AppendEntriesReq,
	resp *AppendEntriesResponse,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	peer := m.peers[env.To.Id]
	respCh := make(chan RPCResult)
	rpc := RPC{
		Envelope: env,
		Response: respCh,
	}

	peer.inbox <- rpc

	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-respCh:
		out := result.Resp.(*AppendEntriesResponse)
		*resp = *out
	}
	return nil
}

func (m *MockTransport) Self() Peer {
	return Peer{
		Id:       m.id,
		NodeAddr: m.addr,
	}
}

func NewMockTransport(id ServerID, addr NodeAddr) *MockTransport {
	return &MockTransport{
		id:    id,
		inbox: make(chan RPC),
		peers: map[ServerID]*MockTransport{},
		addr:  addr,
	}
}

func (m *MockTransport) Addr() NodeAddr {
	return m.addr
}

func (m *MockTransport) Connect(peer Peer, t Transport) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tran := t.(*MockTransport)
	m.peers[peer.Id] = tran
	return nil
}

func (m *MockTransport) Inbox() chan RPC {
	return m.inbox
}
