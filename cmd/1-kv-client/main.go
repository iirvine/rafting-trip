package main

import (
	"log/slog"
	"net/rpc"
	"rafting-trip/internal/kv"
)

func main() {
	client, err := rpc.Dial("tcp", "localhost:1234")
	if err != nil {
		panic(err)
	}
	defer client.Close()
	doSet(client, "foo", "bar")
	doGet(client, "foo")
	doDel(client, "foo")
	doGet(client, "foo")
}

func doSet(client *rpc.Client, key, value string) {
	args := &kv.SetArgs{Key: key, Value: value}
	var reply kv.SetReply
	err := client.Call("KVServer.Set", args, &reply)
	if err != nil {
		panic(err)
	}
	slog.Info("set", "args", args, "reply", reply)
}

func doGet(client *rpc.Client, key string) {
	args := &kv.GetArgs{Key: key}
	var reply kv.GetReply
	err := client.Call("KVServer.Get", args, &reply)
	if err != nil {
		switch err.Error() {
		case kv.NotFound.Error():
			slog.Info("get", "args", args, "reply", "not found")
			return
		default:
			panic(err)
		}
	}
	slog.Info("get", "args", args, "reply", reply)
}

func doDel(client *rpc.Client, key string) {
	args := &kv.DelArgs{Key: key}
	var reply kv.DelReply
	err := client.Call("KVServer.Del", args, &reply)
	if err != nil {
		panic(err)
	}
	slog.Info("del", "args", args)
}
