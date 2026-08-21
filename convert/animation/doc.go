// Package animation creates firmware-native BUSY Bar bicycle0 animations.
//
// EncodeRGB888 accepts device-ready pixels. The firmware calls the format
// RGB888 but stores each pixel in B, G, R byte order. EncodeImages accepts
// equal-sized standard-library images and performs that channel conversion.
// ConvertZIP accepts the firmware source layout: a root directory matching the
// ZIP filename, meta.json, and contiguous frame_N.png files.
//
// This package produces deterministic raw RGB888 records in one default
// section. It does not resize images or support RLE, custom sections, Gray4,
// or ARGB8888.
package animation
