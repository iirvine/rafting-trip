package draft

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ReplicationState is the state of a follower's replication of a log entry.
type ReplicationState struct {
	Id ServerID
	// nextIndex is the index of the next log entry to send to that server
	// (initialized to leader last log index + 1).
	nextIndex uint64
	// matchIndex is the index of highest log entry known to be replicated
	// on server (initialized to 0, increases monotonically).
	matchIndex uint64
}

type Replicator struct {
	followerId ServerID
	state      ReplicationState
	addr       NodeAddr
	transport  Transport
	mu         sync.Mutex
	idxCh      chan uint64
	rpcOutbox  chan Envelope
	signal     chan Signal
}

type Signal struct {
	old ReplicationState
	new *ReplicationState
}

func NewReplicator(
	id ServerID,
	addr NodeAddr,
	transport Transport,
	signal chan Signal,
	max uint64,
) *Replicator {
	return &Replicator{
		followerId: id,
		state: ReplicationState{
			Id:         id,
			nextIndex:  max + 1,
			matchIndex: 0,
		},
		addr:      addr,
		transport: transport,
		signal:    signal,
		idxCh:     make(chan uint64, 50),
		rpcOutbox: make(chan Envelope),
	}
}

func (r *Replicator) Run() {
	for {
		select {
		case <-r.idxCh:
			asyncNotify(r.signal, Signal{r.state, nil})
		case env := <-r.rpcOutbox:
			r.dispatch(env)
		}
	}
}

func (r *Replicator) dispatch(env Envelope) {
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()

	state := r.state

	req := env.Data.(AppendEntriesReq)
	resp := new(AppendEntriesResponse)
	if len(req.Entries) > 0 {
		slog.Info("Replicator dispatching AppendEntries RPC", "replicator", r.followerId)
	} else {
		slog.Info("Replicator dispatching AppendEntries heartbeat", "replicator", r.followerId)
	}

	err := r.transport.AppendEntries(ctx, env, &req, resp)
	if err != nil {
		slog.Error("Replicator failed to send AppendEntries RPC", "err", err)
		return
	}

	if resp.Success {
		r.state.matchIndex = req.PrevLogIndex + uint64(len(req.Entries))
		r.state.nextIndex = r.state.matchIndex + 1
	} else {
		r.state.nextIndex--
	}

	newState := r.state

	if len(req.Entries) > 0 {
		r.signal <- Signal{state, &newState}
	}
}

func asyncNotify(signal chan Signal, state Signal) {
	select {
	case signal <- state:
	default:
	}
}
