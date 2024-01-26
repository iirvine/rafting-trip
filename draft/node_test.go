package draft

import (
	"context"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"sync"
	"testing"
	"time"
)

var DefaultConfig = NodeConfig{
	ElectionTimeoutMS:  1000,
	HeartbeatTimeoutMS: 1000,
}

func ConnectAll(t *testing.T, nodes ...*Node) {
	for i, node := range nodes {
		for j, peer := range nodes {
			if i == j {
				continue
			}

			node.peers[peer.Id] = Peer{
				Id:       peer.Id,
				NodeAddr: peer.Addr,
			}

			peer.peers[node.Id] = Peer{
				Id:       node.Id,
				NodeAddr: node.Addr,
			}

			err := node.Transport.Connect(Peer{
				Id:       peer.Id,
				NodeAddr: peer.Addr,
			}, peer.Transport)

			err = peer.Transport.Connect(Peer{
				Id:       node.Id,
				NodeAddr: node.Addr,
			}, node.Transport)
			assert.NoError(t, err)
		}
	}
}

func ContextWithTimeout(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

func TestNode_Basic(t *testing.T) {
	trans := &MockTransport{
		inbox: make(chan RPC),
	}

	node := NewNode(1, DefaultConfig, trans, map[ServerID]Peer{})
	go node.Run()

	entries := []LogEntry{
		{Term: 1, Index: 1, Data: []byte("foo")},
	}

	respCh := make(chan RPCResult)
	trans.inbox <- RPC{
		Response: respCh,
		Envelope: Envelope{
			From: 1,
			To: Peer{
				Id:       2,
				NodeAddr: LocalAddr(),
			},
			Term: 1,
			Type: AppEntriesReq,
			Data: AppendEntriesReq{
				Term:         1,
				PrevLogIndex: 0,
				PrevLogTerm:  0,
				Entries:      entries,
				LeaderCommit: 0,
			},
		},
	}

	resp := <-respCh
	if resp.Err != nil {
		t.Fatal(resp.Err)
	}

	assert.Equal(t, node.PersistentState.log.storage, []LogEntry{
		Head,
		{Term: 1, Index: 1, Data: []byte("foo")},
	})
}

func TestNode_Connections(t *testing.T) {
	trans1 := NewMockTransport(ServerID(1), NodeAddr{Host: "1", Port: 1})
	trans2 := NewMockTransport(ServerID(2), NodeAddr{Host: "2", Port: 2})

	node1 := NewNode(1, DefaultConfig, trans1, map[ServerID]Peer{
		2: {Id: 2, NodeAddr: trans2.Addr()},
	})

	node2 := NewNode(2, DefaultConfig, trans2, map[ServerID]Peer{
		1: {Id: 1, NodeAddr: trans1.Addr()},
	})

	ConnectAll(t, node1, node2)
	assert.Equal(t, trans1.peers, map[ServerID]*MockTransport{
		trans2.Self().Id: trans2,
	})
	assert.Equal(t, trans2.peers, map[ServerID]*MockTransport{
		trans1.Self().Id: trans1,
	})
}

func TestNode_LeaderAppends(t *testing.T) {
	node := NewNode(1, DefaultConfig, NewMockTransport(ServerID(1), LocalAddr()), map[ServerID]Peer{})
	node.PersistentState.currentTerm = 1
	node.TransitionState(Leader)
	node.processCommand(Command{[]byte("foo -> bar")})
	assert.Equal(t, []LogEntry{
		Head,
		{Term: 1, Index: 1, Data: []byte("foo -> bar")},
	}, node.Log().storage)
}

func TestNode_LeaderReplicates(t *testing.T) {
	node1 := NewNode(1, DefaultConfig, NewMockTransport(ServerID(1), LocalAddr()), map[ServerID]Peer{})
	node2 := NewNode(2, DefaultConfig, NewMockTransport(ServerID(2), LocalAddr()), map[ServerID]Peer{})

	ConnectAll(t, node1, node2)
	go node1.Run()
	go node2.Run()

	node1.PersistentState.currentTerm = 1
	node1.TransitionState(Leader)
	node1.cmdCh <- Command{[]byte("foo -> bar")}
	node1.cmdCh <- Command{[]byte("car -> foo")}
	node1.cmdCh <- Command{[]byte("tar -> boo")}

	ctx, cancel := ContextWithTimeout(1 * time.Minute)
	defer cancel()
WAIT:
	for {
		select {
		case <-ctx.Done():
			t.Fatal("timeout")
		case <-time.After(100 * time.Millisecond):
			if len(node2.Log().storage) == 4 {
				assert.Equal(t, []LogEntry{
					Head,
					{Term: 1, Index: 1, Data: []byte("foo -> bar")},
					{Term: 1, Index: 2, Data: []byte("car -> foo")},
					{Term: 1, Index: 3, Data: []byte("tar -> boo")},
				}, node2.Log().storage)
				break WAIT
			}
		}
	}
}

func TestNode_LeaderReplicatesMultipleNodes(t *testing.T) {
	node1 := NewNode(1, DefaultConfig, NewMockTransport(ServerID(1), LocalAddr()), map[ServerID]Peer{})
	node2 := NewNode(2, DefaultConfig, NewMockTransport(ServerID(2), LocalAddr()), map[ServerID]Peer{})
	node3 := NewNode(3, DefaultConfig, NewMockTransport(ServerID(3), LocalAddr()), map[ServerID]Peer{})

	ConnectAll(t, node1, node2, node3)
	go node1.Run()
	go node2.Run()
	go node3.Run()

	node1.PersistentState.currentTerm = 1
	node1.TransitionState(Leader)
	node1.cmdCh <- Command{[]byte("foo -> bar")}
	node1.cmdCh <- Command{[]byte("car -> foo")}
	node1.cmdCh <- Command{[]byte("tar -> boo")}

	ctx, cancel := ContextWithTimeout(1 * time.Minute)
	defer cancel()
WAIT:
	for {
		select {
		case <-ctx.Done():
			t.Fatal("timeout")
		case <-time.After(100 * time.Millisecond):
			if len(node2.Log().storage) == 4 && len(node3.Log().storage) == 4 {
				expected := []LogEntry{
					Head,
					{Term: 1, Index: 1, Data: []byte("foo -> bar")},
					{Term: 1, Index: 2, Data: []byte("car -> foo")},
					{Term: 1, Index: 3, Data: []byte("tar -> boo")},
				}
				assert.Equal(t, expected, node2.Log().storage)
				assert.Equal(t, expected, node3.Log().storage)
				break WAIT
			}
		}
	}
}

func TestNode_ReplicationConsensus(t *testing.T) {
	// todo: test is non-deterministic - due to timings, the assertion may fail,
	//   as the leader may have replicated to both nodes but not processed the
	//   responses yet
	node1 := NewNode(1, DefaultConfig, NewMockTransport(ServerID(1), LocalAddr()), map[ServerID]Peer{})
	node2 := NewNode(2, DefaultConfig, NewMockTransport(ServerID(2), LocalAddr()), map[ServerID]Peer{})
	node3 := NewNode(3, DefaultConfig, NewMockTransport(ServerID(3), LocalAddr()), map[ServerID]Peer{})

	//slog.SetLogLoggerLevel(slog.LevelDebug)
	ConnectAll(t, node1, node2, node3)
	go node1.Run()
	go node2.Run()
	go node3.Run()

	node1.PersistentState.currentTerm = 1
	node1.TransitionState(Leader)
	node1.cmdCh <- Command{[]byte("foo -> bar")}
	node1.cmdCh <- Command{[]byte("car -> foo")}
	node1.cmdCh <- Command{[]byte("tar -> boo")}

	ctx, cancel := ContextWithTimeout(1 * time.Minute)
	defer cancel()
WAIT:
	for {
		select {
		case <-ctx.Done():
			t.Fatal("timeout")
		case <-time.After(100 * time.Millisecond):
			if len(node2.Log().storage) == 4 && len(node3.Log().storage) == 4 {
				assert.Equal(t, uint64(3), node1.VolatileState.commitIndex)
				break WAIT
			}
		}
	}
}

func TestNodeElections_RemainsFollower(t *testing.T) {
	// todo: this test is also non-deterministic - runs fine in isolation, but
	//   fails when run with other tests
	// node should remain follower while it receives heartbeats within the election timeout
	node := NewNode(1, DefaultConfig, NewMockTransport(ServerID(1), LocalAddr()), map[ServerID]Peer{})
	go node.Run()
	wg := new(sync.WaitGroup)
	//slog.SetLogLoggerLevel(slog.LevelDebug)

	readCh := make(chan RPCResult)
	iterations := 10
	wg.Add(iterations)
	go func() {
		for {
			select {
			case <-readCh:
				wg.Done()
			}
		}
	}()

	go func() {
		for i := 0; i < iterations; i++ {
			node.rpcInbox <- RPC{
				Response: readCh,
				Envelope: Envelope{
					From: 2,
					To: Peer{
						Id:       1,
						NodeAddr: node.Addr,
					},
					Term: 1,
					Type: AppEntriesReq,
					Data: AppendEntriesReq{
						Term:         1,
						PrevLogIndex: 0,
						PrevLogTerm:  0,
						Entries:      []LogEntry{},
						LeaderCommit: 0,
					},
				},
			}
			time.Sleep(time.Duration(DefaultConfig.ElectionTimeoutMS/2) * time.Millisecond)
		}
	}()

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		doneCh <- struct{}{}
	}()

	for {
		select {
		case <-doneCh:
			return
		case <-time.After(100 * time.Millisecond):
			if node.CurrentState() != Follower {
				t.Fatal("node did not remain follower")
			}
		}
	}
}

func TestNodeElections_BecomesCandidate(t *testing.T) {
	// node should become candidate when it does not receive heartbeats within the election timeout
	node := NewNode(1, DefaultConfig, NewMockTransport(ServerID(1), LocalAddr()), map[ServerID]Peer{})
	go node.Run()
	slog.SetLogLoggerLevel(slog.LevelDebug)

	// the max election timeout possible is 2x the election timeout
	timer := time.NewTimer(time.Duration(DefaultConfig.ElectionTimeoutMS*2) * time.Millisecond)
	for {
		select {
		case <-timer.C:
			if node.CurrentState() != Candidate {
				t.Fatal("node did not become candidate within max timeout")
			}
			return
		case <-time.After(100 * time.Millisecond):
			if node.CurrentState() == Candidate {
				return
			}
		}
	}
}

func TestNodeElections_BecomesLeader(t *testing.T) {
	// node should become leader when it receives a majority of votes
	conf := NodeConfig{
		ElectionTimeoutMS:  10000,
		HeartbeatTimeoutMS: 1000,
	}

	// with default config, node1 should become candidate before other nodes in cluster
	node1 := NewNode(1, DefaultConfig, NewMockTransport(ServerID(1), LocalAddr()), map[ServerID]Peer{})
	node2 := NewNode(2, conf, NewMockTransport(ServerID(2), LocalAddr()), map[ServerID]Peer{})
	node3 := NewNode(3, conf, NewMockTransport(ServerID(3), LocalAddr()), map[ServerID]Peer{})

	slog.SetLogLoggerLevel(slog.LevelDebug)
	peers := map[ServerID]Peer{
		1: {Id: 1, NodeAddr: node1.Addr},
		2: {Id: 2, NodeAddr: node2.Addr},
		3: {Id: 3, NodeAddr: node3.Addr},
	}

	node1.peers = peers
	node2.peers = peers
	node3.peers = peers

	ConnectAll(t, node1, node2, node3)

	node1.PersistentState.currentTerm = 1
	node1.TransitionState(Candidate)

	go node1.Run()
	go node2.Run()
	go node3.Run()

	ctx, cancel := ContextWithTimeout(1 * time.Minute)
	defer cancel()
WAIT:
	for {
		select {
		case <-ctx.Done():
			t.Fatal("timeout")
		case <-time.After(100 * time.Millisecond):
			if node1.CurrentState() == Leader {
				break WAIT
			}
		}
	}
}
