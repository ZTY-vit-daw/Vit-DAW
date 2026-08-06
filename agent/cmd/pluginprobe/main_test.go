package main

import "testing"

func TestLoopbackAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:9318", "[::1]:9318"} {
		if !loopbackAddress(address) {
			t.Fatalf("expected loopback address: %s", address)
		}
	}
	for _, address := range []string{"0.0.0.0:9318", "192.0.2.1:9318", "localhost:9318", "invalid"} {
		if loopbackAddress(address) {
			t.Fatalf("unexpected non-loopback address: %s", address)
		}
	}
}
