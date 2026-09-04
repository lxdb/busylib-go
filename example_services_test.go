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

func ExampleSettingsService_MintAccessToken() {
	bootstrapClient, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	minted, err := bootstrapClient.Settings().MintAccessToken(ctx, "example")
	if err != nil {
		log.Print(err)
		return
	}
	client, err := busylib.NewClient(busylib.WithLocalAccessToken(minted.Token))
	if err != nil {
		log.Print(err)
		return
	}
	if err := client.Settings().RevokeAccessToken(ctx, minted.ShortID); err != nil {
		log.Print(err)
	}
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

func ExampleDisplayService_ClearElements() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const xpm = `! XPM2
4 4 2 1
. c #000000
+ c #FFFFFF
....
.++.
.++.
....`
	zero, ten := 0, 10
	background := busylib.NewRectangleElement("background", 16, 16)
	background.ZIndex = &zero
	bitmap := busylib.NewXPMBitmapElement("bitmap", xpm)
	bitmap.ZIndex = &ten
	if err := client.Display().Draw(ctx, busylib.NewDisplayElements("example", background, bitmap)); err != nil {
		log.Print(err)
		return
	}
	if err := client.Display().ClearElements(ctx, busylib.ClearDisplayElementsRequest{
		ApplicationName: "example",
		ElementIDs:      []string{"bitmap"},
	}); err != nil {
		log.Print(err)
	}
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

func ExampleStorageService_Write_append() {
	client, err := busylib.NewClient()
	if err != nil {
		log.Print(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	appendContent := true
	if err := client.Storage().Write(ctx, busylib.WriteStorageFileRequest{
		Path:   "/ext/example/events.log",
		Body:   busylib.BytesBody([]byte("started\n"), "text/plain"),
		Append: &appendContent,
	}); err != nil {
		log.Print(err)
	}
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
