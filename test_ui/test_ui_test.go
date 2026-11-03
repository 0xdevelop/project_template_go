package test_ui

import (
	"net"
	"testing"
)

func TestTestUIPortConflict(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	listener, err := listen("127.0.0.1", occupied.Addr().(*net.TCPAddr).Port)
	if err == nil {
		defer listener.Close()
		t.Fatalf("occupied configured port must fail, listener moved to %s", listener.Addr())
	}
}
