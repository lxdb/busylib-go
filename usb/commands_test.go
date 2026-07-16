package usb

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestCommandsBuildFirmwareCommandLines(t *testing.T) {
	tests := []struct {
		name string
		want string
		call func(Commands) error
	}{
		{name: "uptime", want: "uptime\r\n", call: func(commands Commands) error { _, err := commands.Uptime(context.Background()); return err }},
		{name: "power", want: "power info\r\n", call: func(commands Commands) error { _, err := commands.Power(context.Background(), "info"); return err }},
		{name: "echo", want: "echo hello busy bar\r\n", call: func(commands Commands) error {
			_, err := commands.Echo(context.Background(), "hello busy bar")
			return err
		}},
		{name: "top", want: "top 1000\r\n", call: func(commands Commands) error { return commands.Top(context.Background(), io.Discard, time.Second) }},
		{name: "log", want: "log info\r\n", call: func(commands Commands) error { return commands.Log(context.Background(), io.Discard, "info") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address, done := serveOnce(t, func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.WriteString(conn, ">: ")
				if got := readLine(t, conn); got != test.want {
					t.Errorf("command = %q, want %q", got, test.want)
				}
				_, _ = io.WriteString(conn, ">: ")
			})
			if err := test.call(newTestClient(t, address).Commands()); err != nil {
				t.Fatalf("command wrapper: %v", err)
			}
			<-done
		})
	}
}

func TestRebootWritesCanonicalCommandWithoutWaitingForPrompt(t *testing.T) {
	address, done := serveOnce(t, func(conn net.Conn) {
		defer conn.Close()
		_, _ = io.WriteString(conn, ">: ")
		if got := readLine(t, conn); got != "power reboot sw\r\n" {
			t.Errorf("reboot command = %q", got)
		}
	})
	if err := newTestClient(t, address).Commands().Reboot(context.Background()); err != nil {
		t.Fatalf("Reboot: %v", err)
	}
	<-done
}
