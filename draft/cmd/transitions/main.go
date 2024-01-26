package main

import (
	"draft"
	"github.com/nsf/termbox-go"
	"log/slog"
	"os"
)

func main() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}

	slog.SetLogLoggerLevel(slog.LevelDebug)

	node := draft.NewNode(1, draft.NewMockTransport(draft.NodeAddr{}), map[uint64]draft.Peer{})
	go node.Run()

	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyEnter:
				node.TransitionState(draft.Leader)
			default:
				os.Exit(0)
			}
		}
	}
}
