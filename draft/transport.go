package draft

import "context"

type Transport interface {
	Inbox() chan RPC
	Connect(peer Peer, t Transport) error
	AppendEntries(
		ctx context.Context,
		env Envelope,
		args *AppendEntriesReq,
		resp *AppendEntriesResponse,
	) error
	RequestVote(
		ctx context.Context,
		env Envelope,
		args *RequestVoteReq,
		resp *RequestVoteResponse,
	) error
	Addr() NodeAddr
	Self() Peer
}

type RPC struct {
	Response chan RPCResult
	// The request message.
	Envelope Envelope
}

func (r *RPC) Respond(resp interface{}, err error) {
	r.Response <- RPCResult{resp, err}
}

type RPCResult struct {
	// The response message.
	Resp interface{}
	Err  error
}

type MsgType int

const (
	AppEntriesReq MsgType = iota
	AppEntriesResp
	VoteReq
	VoteResp
	HeartbeatReq
	HeartbeatResp
)

type Envelope struct {
	From uint64
	To   Peer
	// Term is the term of the sender.
	Term uint64
	// Type is the type of the message.
	Type MsgType
	// Data is the content of the message.
	Data interface{}
}

type AppendEntriesReq struct {
	// Leader's term.
	Term uint64
	// prevLogIndex is the index of the log entry immediately preceding
	// the new ones.
	PrevLogIndex uint64
	// prevLogTerm is the term of the log entry immediately preceding
	// the new ones.
	PrevLogTerm uint64
	// Entries is the log entries to store (empty for heartbeat).
	Entries []LogEntry
	// Leader's commitIndex.
	LeaderCommit uint64
}

type AppendEntriesResponse struct {
	// Term is the term of the follower.
	Term uint64
	// Success is true if follower contained entry matching
	// prevLogIndex and prevLogTerm.
	Success       bool
	ConflictIndex uint64
	ConflictTerm  uint64
}

type RequestVoteReq struct {
	// Candidate's term.
	Term uint64
	// Candidate requesting vote.
	CandidateId uint64
	// Index of candidate's last log entry.
	LastLogIndex uint64
	// Term of candidate's last log entry.
	LastLogTerm uint64
}

type RequestVoteResponse struct {
	// Term is the term of the follower.
	Term uint64
	// VoteGranted is true if the follower granted the vote.
	VoteGranted bool
}
