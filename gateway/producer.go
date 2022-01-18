package main

import "time"

type Message struct {
	Chain     string
	Project   string
	Potocol   string
	IP        string
	Method    string
	Code      int
	Length    int64
	Timestamp time.Time
}
