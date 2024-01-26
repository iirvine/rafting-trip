package draft

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRaftLog_LastIndex(t *testing.T) {
	log := NewRaftLog()
	if log.LastIndex() != 0 {
		t.Errorf("LastIndex() should return 0 for empty log, returned %d", log.LastIndex())
	}

	log.storage = append(log.storage, LogEntry{Index: 1})
	if log.LastIndex() != 1 {
		t.Errorf("LastIndex() should return 1 for log with one entry, returned %d", log.LastIndex())
	}
}

func TestRaftLog_LastTerm(t *testing.T) {
	log := NewRaftLog()
	if log.LastTerm() != 0 {
		t.Errorf("LastTerm() should return 0 for empty log, returned %d", log.LastTerm())
	}

	log.storage = append(log.storage, LogEntry{Term: 1})
	if log.LastTerm() != 1 {
		t.Errorf("LastTerm() should return 1 for log with one entry, returned %d", log.LastTerm())
	}
}

func TestRaftLog_51(t *testing.T) {
	//	 Reply false if term < currentTerm (§5.1)
	log := NewRaftLog()
	req := AppendEntriesReq{
		Term:         1,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      nil,
		LeaderCommit: 0,
	}

	response, _ := log.AppendEntries(2, req)
	if response.Success {
		t.Errorf("AppendEntries() should return false if term < currentTerm")
	}
}

func TestRaftLog_53_2(t *testing.T) {
	// Reply false if log doesn’t contain an entry at prevLogIndex
	// whose term matches prevLogTerm (§5.3.2)
	{
		log := NewRaftLog()
		log.storage = append(log.storage, LogEntry{Term: 1})
		req := AppendEntriesReq{
			Term:         1,
			PrevLogIndex: 2,
			PrevLogTerm:  1,
			Entries:      nil,
			LeaderCommit: 0,
		}

		response, _ := log.AppendEntries(1, req)
		if response.Success {
			t.Errorf("AppendEntries() should return false if log doesn’t contain an entry at prevLogIndex")
		}
	}
}

func TestRaftLog_53_3(t *testing.T) {
	// If an existing entry conflicts with a new one (same index
	// but different terms), delete the existing entry and all that
	// follow it (§5.3.3)
	{
		log := NewRaftLog()
		log.storage = append(log.storage, LogEntry{Term: 1}, LogEntry{Term: 1})
		req := AppendEntriesReq{
			Term:         1,
			PrevLogIndex: 1,
			PrevLogTerm:  2,
			Entries:      nil,
			LeaderCommit: 0,
		}
		response, _ := log.AppendEntries(1, req)
		if response.Success {
			t.Errorf("AppendEntries() should return false if an existing entry conflicts with prevLogTerm")
		}
	}
	{
		log := NewRaftLog()
		log.storage = append(log.storage,
			LogEntry{Term: 1}, // <-- prevLogIndex=1
			LogEntry{Term: 1}, // all that follow it should be deleted
			LogEntry{Term: 1})

		rpcEntries := []LogEntry{
			{Term: 2, Data: []byte("foo")},
			{Term: 2, Data: []byte("bar")},
		}

		req := AppendEntriesReq{
			Term:         1,
			PrevLogIndex: 1,
			PrevLogTerm:  1,
			Entries:      rpcEntries,
			LeaderCommit: 0,
		}
		response, _ := log.AppendEntries(1, req)

		if !response.Success {
			t.Errorf("AppendEntries() should return true if an existing entry conflicts with a new one")
		}

		if log.LastIndex() != 3 {
			t.Errorf("AppendEntries() should delete the existing entry and all that follow it and append new entries")
		}

		assert.Equal(t, log.storage, []LogEntry{
			Head,
			{Term: 1},
			{Term: 2, Data: []byte("foo")},
			{Term: 2, Data: []byte("bar")},
		})
	}
}

func TestRaftLog_AppendEntriesShouldSkipDuplicates(t *testing.T) {
	log := NewRaftLog()
	log.storage = append(log.storage,
		LogEntry{Term: 1, Index: 1}, // <-- prevLogIndex
		LogEntry{Term: 2, Index: 2},
	)
	rpcEntries := []LogEntry{
		{Term: 2, Data: []byte("foo"), Index: 2}, // <-- duplicate
		{Term: 2, Data: []byte("bar"), Index: 3},
	}

	req := AppendEntriesReq{
		Term:         2,
		PrevLogIndex: 1,
		PrevLogTerm:  1,
		Entries:      rpcEntries,
		LeaderCommit: 0,
	}

	response, _ := log.AppendEntries(2, req)

	if !response.Success {
		t.Errorf("AppendEntries() returned false, expected true")
	}

	assert.Equal(t, log.storage, []LogEntry{
		Head,
		{Term: 1, Index: 1},
		{Term: 2, Index: 2},
		{Term: 2, Index: 3, Data: []byte("bar")},
	})
}
