# Media

The media packages prepare caller-owned data for BUSY Bar endpoints. They do not send requests to a device.

| Input | Package | Result |
| --- | --- | --- |
| PNG, JPEG, or GIF image data | `convert` | Device-ready image payload |
| Encoded or raw PCM audio | `convert/audio` | Device-ready audio payload |
| Image sequence or animation archive | `convert/animation` | Encoded animation payload and metadata |
| Raw or protobuf display frame | `frame` | Standard Go image |
| Device frame snapshots | `snapshot` | Reconstructed snapshot data |

## Convert an image

```go
result, err := convert.Image(source, busylib.DisplayFront)
if err != nil {
    return err
}

log.Printf("prepared image: %dx%d", result.Width, result.Height)
```

The converter validates encoded input size and decoded pixel count before allocating the final output. Use the default limits unless the caller has a measured reason to accept more memory or processing work.

## Prepare audio

```go
result, err := audio.Convert(ctx, source, filename)
if err != nil {
    return err
}

log.Printf("prepared audio bytes: %d", len(result.Data))
```

Audio preparation accepts supported encoded formats and raw PCM input. The package bounds the source data it will process.

## Encode an animation

```go
result, err := animation.EncodeImages(frames, animation.DefaultFPS)
if err != nil {
    return err
}

log.Printf("encoded frames: %d", result.DisplayFrameCount)
```

The animation package can encode image sequences or transcode supported ZIP archives. It validates archive structure and bounds both compressed input and generated output.

## Capture and decode a display frame

```go
raw, err := client.Display().Screen(ctx, busylib.DisplayFront)
if err != nil {
    return err
}

value, err := frame.FromHTTP(busylib.DisplayFront, raw)
if err != nil {
    return err
}

decoded, err := value.RGBA()
if err != nil {
    return err
}

log.Printf("captured display bounds: %s", decoded.Bounds())
```

Use `busylib.DisplayFront` or `busylib.DisplayBack` when selecting a physical display. The equivalent shared-package values are `display.Front` and `display.Back`. Numeric selectors are not part of the v0.1.0 API.

Frame decoding requires exact dimensions and byte counts. The package rejects malformed or oversized data before decoding.

## Resource limits

The canonical default values and their configuration points are listed in [Compatibility](compatibility.md). Raising a limit increases the amount of untrusted input that the process can buffer, decode, or expand. Keep the caller’s context bounded even when a package also enforces a size limit.
