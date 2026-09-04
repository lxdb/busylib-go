// Package convert prepares PNG, JPEG, and single-frame GIF images for a BUSY
// Bar display.
//
// Image and ImageFile bound encoded and decoded input, resize only when an
// image exceeds the selected display, center-crop it, and return owned PNG
// data. The package adds no dependencies to the core HTTP client.
package convert
