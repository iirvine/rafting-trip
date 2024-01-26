package draft

type LogEntry struct {
	// Term is the term when the entry was received by the leader.
	Term uint64
	// Index is the log index of the entry.
	Index uint64
	// Data is the payload of the entry.
	Data []byte
}

type RaftLog struct {
	storage []LogEntry
}

var Head = LogEntry{Index: 0, Term: 0}

func NewRaftLog() *RaftLog {
	return &RaftLog{
		storage: []LogEntry{Head},
	}
}

func (l *RaftLog) LastIndex() uint64 {
	return uint64(len(l.storage) - 1)
}

func (l *RaftLog) LastTerm() uint64 {
	return l.storage[l.LastIndex()].Term
}

func (l *RaftLog) At(index uint64) (LogEntry, bool) {
	if index > l.LastIndex() {
		return LogEntry{}, false
	}
	return l.storage[index], true
}

func (l *RaftLog) AppendEntries(
	currentTerm uint64,
	req AppendEntriesReq,
) (*AppendEntriesResponse, error) {
	if currentTerm > req.Term {
		return &AppendEntriesResponse{
			Term:    currentTerm,
			Success: false,
		}, nil // todo: these should actually be errors
	}

	prevEntry, ok := l.At(req.PrevLogIndex)
	if !ok {
		return &AppendEntriesResponse{
			Term:    currentTerm,
			Success: false,
		}, nil
	}

	if prevEntry.Term != req.PrevLogTerm {
		return &AppendEntriesResponse{
			Term:    currentTerm,
			Success: false,
		}, nil
	}

	tail := l.LastIndex()
	next := req.PrevLogTerm + 1
	deleteFrom := tail
	var newEntries []LogEntry
	for i, entry := range req.Entries {
		if entry.Index > tail {
			newEntries = req.Entries[i:]
			break
		}

		existing, ok := l.At(next)
		if ok && existing.Term != entry.Term {
			deleteFrom = next
			newEntries = req.Entries[i:]
			break
		}
		next++
	}

	if deleteFrom < tail {
		l.storage = l.storage[:deleteFrom]
	}

	if len(newEntries) == 0 {
		return &AppendEntriesResponse{
			Term:    currentTerm,
			Success: true,
		}, nil
	}

	l.storage = append(l.storage, newEntries...)
	return &AppendEntriesResponse{Success: true, Term: currentTerm}, nil
}

func (l *RaftLog) LeaderAppend(term uint64, data []byte) {
	l.storage = append(l.storage, LogEntry{
		Term:  term,
		Index: l.LastIndex() + 1,
		Data:  data,
	})
}

func (l *RaftLog) GetEntries(lo uint64, hi uint64) []LogEntry {
	return l.storage[lo : hi+1]
}

func (l *RaftLog) GetTerm(u uint64) uint64 {
	return l.storage[u].Term
}

func (l *RaftLog) IsUpToDate(index uint64, term uint64) bool {
	lastIndex := l.LastIndex()
	lastTerm := l.LastTerm()
	return term > lastTerm || (term == lastTerm && index >= lastIndex)
}
