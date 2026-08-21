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
		{name: "storage", want: "storage info\r\n", call: func(commands Commands) error { _, err := commands.Storage(context.Background(), "info"); return err }},
		{name: "update", want: "update status\r\n", call: func(commands Commands) error { _, err := commands.Update(context.Background(), "status"); return err }},
		{name: "input", want: "input ok\r\n", call: func(commands Commands) error { _, err := commands.Input(context.Background(), "ok"); return err }},
		{name: "loader", want: "loader list\r\n", call: func(commands Commands) error { _, err := commands.Loader(context.Background(), "list"); return err }},
		{name: "echo", want: "echo hello busy bar\r\n", call: func(commands Commands) error {
			_, err := commands.Echo(context.Background(), "hello busy bar")
			return err
		}},
		{name: "top", want: "top 1000\r\n", call: func(commands Commands) error { return commands.Top(context.Background(), io.Discard, time.Second) }},
		{name: "free", want: "free\r\n", call: func(commands Commands) error { _, err := commands.Free(context.Background()); return err }},
		{name: "free blocks", want: "free_blocks\r\n", call: func(commands Commands) error { _, err := commands.FreeBlocks(context.Background()); return err }},
		{name: "log", want: "log info\r\n", call: func(commands Commands) error { return commands.Log(context.Background(), io.Discard, "info") }},
		{name: "device info", want: "device_info\r\n", call: func(commands Commands) error { _, err := commands.DeviceInfo(context.Background()); return err }},
		{name: "date", want: "date get\r\n", call: func(commands Commands) error { _, err := commands.Date(context.Background(), "get"); return err }},
		{name: "timezone", want: "timezone get\r\n", call: func(commands Commands) error { _, err := commands.Timezone(context.Background(), "get"); return err }},
		{name: "matter", want: "matter info\r\n", call: func(commands Commands) error { _, err := commands.Matter(context.Background(), "info"); return err }},
		{name: "audio", want: "audio stop\r\n", call: func(commands Commands) error { _, err := commands.Audio(context.Background(), "stop"); return err }},
		{name: "display", want: "display clear\r\n", call: func(commands Commands) error { _, err := commands.Display(context.Background(), "clear"); return err }},
		{name: "sysctl", want: "sysctl get test\r\n", call: func(commands Commands) error {
			_, err := commands.Sysctl(context.Background(), "get", "test")
			return err
		}},
		{name: "log dump", want: "log_dump create\r\n", call: func(commands Commands) error { _, err := commands.LogDump(context.Background(), "create"); return err }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address, done := serveOnce(t, func(conn net.Conn) {
				defer func() { _ = conn.Close() }()
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
		defer func() { _ = conn.Close() }()
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
