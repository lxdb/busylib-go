package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalapi "github.com/lxdb/busylib-go/internal/api"
)

func TestCheckFramesVerifiesCanonicalFactsAndDetectsBGRDrift(t *testing.T) {
	contract, err := internalapi.LoadContractFile("../../testdata/firmware-contract.json")
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	root := t.TempDir()
	files := map[string]string{
		"applications/services/web_server/http_api/api_streaming.c": `
http_api_streaming_single_frame_callback
FRONT_DISPLAY_BUF_SIZE
BACK_DISPLAY_BUF_SIZE / 2
color_buf_l8_to_l4
`,
		"applications/services/state_publisher/screen_streamer.c": `
frame_data_init
get_frame
ScreenStreamerPixelFormatR8G8B8
ScreenStreamerPixelFormatL4
FRONT_DISPLAY_W
FRONT_DISPLAY_H
BACK_DISPLAY_W
BACK_DISPLAY_H
const uint8_t blk_size = instance->display_id == GuiDisplayIdFront ? 3 : 2;
ScreenStreamerCompressionRLE
ScreenStreamerCompressionPlain
`,
		"applications/services/state_publisher/subscriptions.c": `
collect_frame
ScreenStreamerCompressionPlain] = BSB_Frame_Encoding_PLAIN
ScreenStreamerCompressionRLE] = BSB_Frame_Encoding_RUN_LENGTH
ScreenStreamerPixelFormatR8G8B8] = BSB_Frame_PixelFormat_RGB888
ScreenStreamerPixelFormatL8] = BSB_Frame_PixelFormat_L8
ScreenStreamerPixelFormatL4] = BSB_Frame_PixelFormat_L4
GuiDisplayIdFront] = BSB_Frame_Screen_FRONT
GuiDisplayIdBack] = BSB_Frame_Screen_BACK
`,
		"lib/toolbox/color.c": `
color_buf_l8_to_l4
(src_u8[src_i] >> 4) | (src_u8[src_i + 1] & 0xF0)
`,
		"applications/services/front_display/front_display.h": `
#define FRONT_DISPLAY_W (72)
#define FRONT_DISPLAY_H (16)
#define FRONT_DISPLAY_BPP (24)
`,
		"applications/services/back_display/back_display.h": `
#define BACK_DISPLAY_W (160)
#define BACK_DISPLAY_H (80)
#define BACK_DISPLAY_BPP (8)
`,
		"applications/services/gui/modules/canvas.c": `
lv_canvas_set_px_no_invalidate
data[2] = color.red;
data[1] = color.green;
data[0] = color.blue;
`,
	}
	writeFirmwareFixture(t, root, files)

	if err := checkFrames(root, contract.Frames, make(map[string][]byte)); err != nil {
		t.Fatalf("checkFrames: %v", err)
	}

	canvasPath := filepath.Join(root, "applications/services/gui/modules/canvas.c")
	canvas := strings.ReplaceAll(files["applications/services/gui/modules/canvas.c"], "data[0] = color.blue;", "data[0] = color.red;")
	if err := os.WriteFile(canvasPath, []byte(canvas), 0o600); err != nil {
		t.Fatalf("rewrite canvas fixture: %v", err)
	}
	if err := checkFrames(root, contract.Frames, make(map[string][]byte)); err == nil {
		t.Fatal("checkFrames accepted changed RGB888 byte order")
	}
}

func writeFirmwareFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
}
