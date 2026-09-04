# Display and media

Prepare media in the caller process, upload application-owned assets, and then tell the device what to render or play.

## Draw text

Build a display update with typed elements. The application name owns the rendered elements and determines which content `Clear` removes.

```go
text := busylib.NewTextElement("status", "In a meeting", busylib.FontNormal)
text.Color = "#FFFFFF"

request := busylib.NewDisplayElements("calendar", text)
if warnings := request.Warnings(); len(warnings) > 0 {
	log.Printf("display warnings: %v", warnings)
}
if err := client.Display().Draw(ctx, request); err != nil {
	return err
}
```

`Draw` validates the request before transport use. Warnings report suspicious but valid values; they do not prevent drawing.

Use `client.Display().Clear(ctx, "calendar")` to remove only that application's elements. An empty application name removes all rendered elements.

## Draw an inline XPM bitmap

`NewXPMBitmapElement` accepts XPM2 data. `ZIndex` is a pointer, so set it only when the application needs an explicit layer.

```go
const badgeXPM = `! XPM2
4 4 2 1
. c #000000
+ c #FFFFFF
....
.++.
.++.
....`

zero, ten, twenty := 0, 10, 20
background := busylib.NewRectangleElement("background", 16, 16)
background.ZIndex = &zero
badge := busylib.NewXPMBitmapElement("badge", badgeXPM)
badge.ZIndex = &ten
label := busylib.NewTextElement("label", "Now", busylib.FontNormal)
label.ZIndex = &twenty

if err := client.Display().Draw(
	ctx,
	busylib.NewDisplayElements("calendar", background, badge, label),
); err != nil {
	return err
}

if err := client.Display().ClearElements(ctx, busylib.ClearDisplayElementsRequest{
	ApplicationName: "calendar",
	ElementIDs:      []string{"badge", "label"},
}); err != nil {
	return err
}
```

Set `ApplicationName` to limit removal to elements owned by that application. If it is empty, the firmware applies the `ElementIDs` filter across all applications.

When `ZIndex` is omitted, firmware sequences elements in request order with default layers `0`, `10`, and `20` for the first three elements.

> [!WARNING]
> Firmware 1.2.3 has an unterminated internal element-ID list. `ClearElements` can behave incorrectly or restart the device. The library sends the application name in the query to avoid the separate body-application bug, but it cannot fix the pointer-list bug. Verify selective clear on a physical device before relying on it.

## Convert, upload, and draw an image

The `convert` package prepares image bytes but does not contact the device.

```go
prepared, err := convert.ImageFile("logo.png", busylib.DisplayFront)
if err != nil {
	return err
}

if err := client.Assets().Upload(ctx, busylib.UploadAssetRequest{
	ApplicationName: "calendar",
	File:            "logo.png",
	Body:            busylib.BytesBody(prepared.Data, "image/png"),
}); err != nil {
	return err
}

image := busylib.NewAssetImageElement("logo", "logo.png")
if err := client.Display().Draw(
	ctx,
	busylib.NewDisplayElements("calendar", image),
); err != nil {
	return err
}
```

Use `busylib.DisplayFront` or `busylib.DisplayBack` to select a physical display. The equivalent shared-package values are `display.Front` and `display.Back`.

## Prepare and play audio

`audio.Convert` accepts device-ready PCM or invokes `ffmpeg` for supported encoded input. The converter returns raw device audio and does not upload it.

```go
prepared, err := audio.ConvertFile(ctx, "notification.mp3")
if err != nil {
	return err
}

assetName := "notification" + prepared.Extension
if err := client.Assets().Upload(ctx, busylib.UploadAssetRequest{
	ApplicationName: "calendar",
	File:            assetName,
	Body:            busylib.BytesBody(prepared.Data, "application/octet-stream"),
}); err != nil {
	return err
}

if err := client.Audio().PlayAsset(ctx, "calendar", assetName); err != nil {
	return err
}
```

Use `client.Audio().Stop(ctx)` to stop playback. `SetVolume` can produce device feedback; use `SetVolumeSilently` when that feedback is not wanted.

## Encode an animation

Use `animation.EncodeImages` for an in-memory image sequence or `animation.ConvertZIP` for a supported firmware archive.

```go
result, err := animation.EncodeImages(frames, animation.DefaultFPS)
if err != nil {
	return err
}

log.Printf(
	"animation: %dx%d frames=%d",
	result.Width,
	result.Height,
	result.DisplayFrameCount,
)
```

The animation package validates dimensions, frame consistency, archive layout, and configured input and output limits. It does not resize images.

## Capture and decode a display

```go
raw, err := client.Display().Screen(ctx, busylib.DisplayFront)
if err != nil {
	return err
}

captured, err := frame.FromHTTP(display.Front, raw)
if err != nil {
	return err
}

decoded, err := captured.RGBA()
if err != nil {
	return err
}
log.Printf("captured display bounds: %s", decoded.Bounds())
```

Frame decoding requires exact dimensions and byte counts. It rejects malformed, unsupported, or oversized data before rendering an image.

## Send virtual input

```go
if err := client.Input().SendKey(ctx, busylib.InputKeyOK); err != nil {
	return err
}
```

Sending a key can change the current device screen or operation. Read the current state first when the application must avoid an unexpected transition.

## Resource limits

Image, audio, animation, and frame packages bound untrusted input or output. Keep the defaults unless the application has a measured requirement. See [Compatibility](../reference/compatibility.md#default-safety-limits).
