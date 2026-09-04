package busylib_test

import (
	"context"
	"log"
	"time"

	busylib "github.com/lxdb/busylib-go"
)

func ExampleSystemService_Status() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.System().Status(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("firmware version: %s", status.Firmware.Version)
}

func ExampleSettingsService_Name() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	name, err := client.Settings().Name(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("device name: %s", name.Name)
}

func ExampleDisplayService_Brightness() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	brightness, err := client.Display().Brightness(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("display brightness: %s", brightness.Value)
}

func ExampleAudioService_Volume() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	volume, err := client.Audio().Volume(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("audio volume: %d", volume.Volume)
}

func ExampleAssetsService_UploadFile() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Assets().UploadFile(ctx, "example", "icon.png", "icon.png"); err != nil {
		log.Print(err)
		return
	}
}

func ExampleStorageService_Status() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.Storage().Status(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("free bytes: %d", status.FreeBytes)
}

func ExampleBusyService_Snapshot() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snapshot, err := client.Busy().Snapshot(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("timer type: %s", snapshot.Snapshot.Type)
}

func ExampleAccountService_Status() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.Account().Status(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("account state: %s", status.Status)
}

func ExampleBLEService_Status() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.BLE().Status(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("BLE state: %s", status.State)
}

func ExampleWiFiService_Status() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.WiFi().Status(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("Wi-Fi state: %s", status.State)
}

func ExampleInputService_SendKey() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Input().SendKey(ctx, busylib.InputKeyOK); err != nil {
		log.Print(err)
		return
	}
}

func ExampleSmartHomeService_PairingStatus() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.SmartHome().PairingStatus(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("pairing state: %s", status.LatestPairingStatus.Value)
}

func ExampleTimeService_Now() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now, err := client.Time().Now(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("device time: %s", now.Timestamp)
}

func ExampleUpdateService_Status() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.Update().Status(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("available version: %s", status.Check.AvailableVersion)
}
