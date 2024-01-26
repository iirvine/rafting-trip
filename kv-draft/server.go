package kv_draft

import (
	"draft"
)

type RaftKV struct {
	kv       *KVServer
	raftNode *draft.RaftNode
}

func NewRaftKV() *RaftKV {
	return &RaftKV{
		kv: NewKVServer(),
		//raftNode: draft.NewRaftNode(),
	}
}
