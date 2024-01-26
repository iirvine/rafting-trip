package draft

import (
	"context"
	"log/slog"
	"math/rand"
	"sort"
	"sync"
	"time"
)

type RaftState int

func (s RaftState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	}
	return "Unknown"
}

const (
	Follower RaftState = iota
	Candidate
	Leader
)

type VolatileState struct {
	// commitIndex is the index of highest log entry known to be committed
	// (initialized to 0, increases monotonically).
	commitIndex uint64
	// lastApplied is the index of highest log entry applied to state machine
	// (initialized to 0, increases monotonically).
	lastApplied uint64
	// matchIndex is the index of highest log entry known to be replicated
	// on server (initialized to 0, increases monotonically).
	matchIndex map[ServerID]uint64
}

type PersistentState struct {
	// currentTerm is the latest term server has seen (initialized to 0
	// on first boot, increases monotonically).
	currentTerm uint64
	// votedFor is the candidateId that received vote in current term (nil if none).
	votedFor *ServerID
	// log is the RaftLog; each entry contains a command for the state
	// machine, and the term when entry was received by leader.
	log *RaftLog
}

type NodeConfig struct {
	// ElectionTimeout is the timeout for election.
	ElectionTimeoutMS int
	// HeartbeatTimeout is the timeout for heartbeat.
	HeartbeatTimeoutMS int
}

type Node struct {
	mu              sync.Mutex
	Id              ServerID
	Conf            NodeConfig
	Addr            NodeAddr
	Transport       Transport
	RaftState       RaftState
	PersistentState PersistentState
	VolatileState   VolatileState
	nextElectTime   time.Time
	nextElectTimer  *time.Ticker
	peers           map[ServerID]Peer
	replicators     map[ServerID]*Replicator
	// Channel for incoming RPCs.
	rpcInbox chan RPC
	// Channel for incoming commands from the host application.
	cmdCh chan Command
	// Channel for replication signals from replicators.
	signalCh chan Signal
}

type ServerID uint64

type NodeAddr struct {
	Host string
	Port int
}

type Peer struct {
	Id       ServerID
	NodeAddr NodeAddr
}

type Command struct {
	Data []byte
}

func NewNode(id uint64, conf NodeConfig, transport Transport, peers map[ServerID]Peer) *Node {
	return &Node{
		Id:        ServerID(id),
		Conf:      conf,
		Addr:      transport.Addr(),
		Transport: transport,
		RaftState: Follower,
		VolatileState: VolatileState{
			commitIndex: 0,
			lastApplied: 0,
			matchIndex:  make(map[ServerID]uint64),
		},
		PersistentState: PersistentState{
			currentTerm: 0,
			votedFor:    nil,
			log:         NewRaftLog(),
		},
		peers:    peers,
		rpcInbox: transport.Inbox(),
		cmdCh:    make(chan Command),
		signalCh: make(chan Signal),
	}
}

func (n *Node) Run() {
	for {
		switch n.CurrentState() {
		case Follower:
			n.lockAndSetNextElectTime()
			n.loopAsFollower()
		case Leader:
			n.loopAsLeader()
		case Candidate:
			n.loopAsCandidate()
		}
	}
}

func (n *Node) loopAsLeader() {
	n.initReplication()
	ticker := time.NewTicker(1 * time.Second)
	heartbeat := time.NewTicker(time.Duration(n.Conf.HeartbeatTimeoutMS) * time.Millisecond)
	for n.CurrentState() == Leader {
		select {
		case rpc := <-n.rpcInbox:
			n.processRPC(rpc)
		case cmd := <-n.cmdCh:
			n.processCommand(cmd)
		case <-heartbeat.C:
			for _, r := range n.replicators {
				lastIdx := n.Log().LastIndex()
				r.idxCh <- lastIdx
			}
		case state := <-n.signalCh:
			if state.new != nil && state.new.matchIndex > state.old.matchIndex {
				n.advanceCommitIndex(*state.new)
			} else {
				n.replicateTo(state.old.Id, state.old.nextIndex, state.old.matchIndex)
			}
		case <-ticker.C:
			slog.Debug("Acting as leader", "node", n.Id)
		}
	}
}

func (n *Node) loopAsFollower() {
	ticker := time.NewTicker(1 * time.Second)
	electionTimer := time.NewTimer(n.getElectTimeout())
	defer func() {
		electionTimer.Stop()
		ticker.Stop()
	}()

	for n.CurrentState() == Follower {
		select {
		case rpc := <-n.rpcInbox:
			slog.Debug("Node received RPC", "node", n.Id)
			n.processRPC(rpc)
		case electTimeout := <-electionTimer.C:
			slog.Debug("Checking election timeout", "node", n.Id)
			if newTimeout := n.maybeStartElection(electTimeout); newTimeout > 0 {
				slog.Debug("Resetting election timeout", "node", n.Id, "newTimeout", newTimeout)
				electionTimer.Stop()
				electionTimer.Reset(newTimeout)
			} else {
				slog.Debug("Starting election", "node", n.Id)
				n.startElection()
				return
			}
		case <-ticker.C:
			slog.Debug("Acting as follower", "node", n.Id)
		}
	}
}

func (n *Node) loopAsCandidate() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	votesNeeded := len(n.peers)/2 + 1
	votesGranted := 0
	votesCh := n.beginVote()

	for n.CurrentState() == Candidate {
		select {
		case rpc := <-n.rpcInbox:
			n.processRPC(rpc)
		case vote := <-votesCh:
			if vote.VoteGranted {
				slog.Info("Got vote", "node", n.Id, "term", vote.Term)
				votesGranted++
				if votesGranted >= votesNeeded {
					slog.Info("Got majority of votes", "node", n.Id, "votes", votesGranted)
					n.TransitionState(Leader)
					return
				}
			}
		case <-ticker.C:
			slog.Debug("Acting as candidate", "node", n.Id)
		}
	}
}

func (n *Node) processRPC(rpc RPC) {
	switch rpc.Envelope.Type {
	case AppEntriesReq:
		n.HandleAppendEntries(rpc)
	case VoteReq:
		n.HandleRequestVote(rpc)
	default:
		slog.Error("Unknown RPC type", "node", n.Id, "type", rpc.Envelope.Type)
	}
}

func (n *Node) processCommand(cmd Command) {
	n.PersistentState.log.LeaderAppend(n.PersistentState.currentTerm, cmd.Data)
	idx := n.Log().LastIndex()
	// Notify replicators of new entry.
	for _, replicator := range n.replicators {
		replicator.idxCh <- idx
	}
}

func (n *Node) maybeStartElection(electTimeout time.Time) time.Duration {
	n.mu.Lock()
	defer n.mu.Unlock()
	if electTimeout.After(n.nextElectTime) {
		return 0
	}
	return n.nextElectTime.Sub(electTimeout)
}

func (n *Node) startElection() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.TransitionState(Candidate)
	n.PersistentState.currentTerm++
	n.PersistentState.votedFor = &n.Id
	n.nextElectTime = time.Now().Add(n.getElectTimeout())
}

func (n *Node) beginVote() chan RequestVoteResponse {
	votesCh := make(chan RequestVoteResponse, len(n.peers))
	// start by voting for self
	votesCh <- RequestVoteResponse{
		Term:        n.PersistentState.currentTerm,
		VoteGranted: true,
	}

	for _, peer := range n.peers {
		if peer.Id == n.Id {
			continue
		}
		n.sendRequestVote(peer, votesCh)
	}

	return votesCh
}

func (n *Node) sendRequestVote(peer Peer, votesCh chan RequestVoteResponse) {
	req := RequestVoteReq{
		Term:         n.PersistentState.currentTerm,
		CandidateId:  uint64(n.Id),
		LastLogIndex: n.Log().LastIndex(),
		LastLogTerm:  n.Log().LastTerm(),
	}

	env := Envelope{
		From: uint64(n.Id),
		To:   peer,
		Term: n.PersistentState.currentTerm,
		Type: VoteReq,
		Data: req,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	go func() {
		resp := new(RequestVoteResponse)
		err := n.Transport.RequestVote(ctx, env, &req, resp)
		if err != nil {
			cancel()
			slog.Error("Error sending RequestVote RPC", "node", n.Id, "peer", peer.Id, "err", err)
			return
		}
		votesCh <- *resp
	}()
}

func (n *Node) lockAndSetNextElectTime() {
	n.mu.Lock()
	n.nextElectTime = time.Now().Add(n.getElectTimeout())
	n.mu.Unlock()
}

func (n *Node) getElectTimeout() time.Duration {
	// get an election timeout, a random time between
	// ElectionTimeout and 2 * ElectionTimeout.
	timeout := time.Duration(rand.Intn(n.Conf.ElectionTimeoutMS)+n.Conf.ElectionTimeoutMS) * time.Millisecond
	return timeout
}

func (n *Node) initReplication() {
	n.replicators = make(map[ServerID]*Replicator)
	for _, peer := range n.peers {
		replicator := NewReplicator(
			peer.Id,
			peer.NodeAddr,
			n.Transport,
			n.signalCh,
			n.PersistentState.log.LastIndex(),
		)
		n.replicators[peer.Id] = replicator
		go replicator.Run()
	}
}

func (n *Node) replicateTo(peerID ServerID, nextIndex uint64, matchIndex uint64) {
	req := AppendEntriesReq{
		Term:         n.PersistentState.currentTerm,
		Entries:      n.PersistentState.log.GetEntries(nextIndex, n.Log().LastIndex()),
		PrevLogIndex: nextIndex - 1,
		PrevLogTerm:  n.Log().GetTerm(nextIndex - 1),
	}

	p := n.peers[peerID]

	env := Envelope{
		From: uint64(n.Id),
		To:   p,
		Term: n.PersistentState.currentTerm,
		Type: AppEntriesReq,
		Data: req,
	}

	replicator := n.replicators[peerID]
	replicator.rpcOutbox <- env
}

func (n *Node) advanceCommitIndex(state ReplicationState) {
	// If there exists an N such that N > commitIndex, a majority
	// of matchIndex[i] ≥ N, and log[N].term == currentTerm:
	// set commitIndex = N (§5.3, §5.4).
	n.VolatileState.matchIndex[state.Id] = state.matchIndex
	matchIndices := make([]uint64, 0, len(n.VolatileState.matchIndex))
	for _, idx := range n.VolatileState.matchIndex {
		matchIndices = append(matchIndices, idx)
	}
	// Sort matchIndices in descending order.
	sort.Slice(matchIndices, func(i, j int) bool {
		return matchIndices[i] > matchIndices[j]
	})

	// Find the index of the majority matchIndex.
	majorityIdx := len(matchIndices) / 2

	// If the majority matchIndex is greater than the commitIndex,
	// and the term of the log entry at that index is equal to the
	// current term, then update the commitIndex.
	if majorityIdx > int(n.VolatileState.commitIndex) &&
		n.Log().GetTerm(matchIndices[majorityIdx]) == n.PersistentState.currentTerm {
		n.VolatileState.commitIndex = matchIndices[majorityIdx]
	}
}

func (n *Node) Log() *RaftLog {
	return n.PersistentState.log
}

func (n *Node) CurrentState() RaftState {
	return n.RaftState
}

func (n *Node) TransitionState(state RaftState) {
	slog.Info("Transitioning state", "node", n.Id, "state", state)
	if n.RaftState == state {
		return
	}
	n.RaftState = state
}

func (n *Node) HandleAppendEntries(rpc RPC) {
	req := rpc.Envelope.Data.(AppendEntriesReq)
	// If RPC request or response contains term T > currentTerm:
	// set currentTerm = T, convert to follower (§5.1)
	if req.Term > n.PersistentState.currentTerm {
		n.PersistentState.currentTerm = req.Term
		n.TransitionState(Follower)
	}

	resp, err := n.Log().AppendEntries(
		n.PersistentState.currentTerm,
		req)

	if resp.Success {
		n.lockAndSetNextElectTime()
	}

	rpc.Respond(resp, err)
}

func (n *Node) HandleRequestVote(rpc RPC) {
	req := rpc.Envelope.Data.(RequestVoteReq)
	resp := &RequestVoteResponse{
		VoteGranted: true,
		Term:        n.PersistentState.currentTerm,
	}
	if n.PersistentState.currentTerm > req.Term {
		resp.VoteGranted = false
		rpc.Respond(resp, nil)
		return
	}

	if !n.Log().IsUpToDate(req.LastLogIndex, req.LastLogTerm) {
		resp.VoteGranted = false
		rpc.Respond(resp, nil)
		return
	}

	n.lockAndSetNextElectTime()
	rpc.Respond(resp, nil)
}
