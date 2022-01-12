package main

import "math/rand"

const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyz"

func RandStringBytesRemainder(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}
