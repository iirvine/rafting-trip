package main

import (
	kv "kv-draft"
)

func main() {
	addr := ":1234"
	kv.NewKVServer().ListenAndServe(addr)
}
