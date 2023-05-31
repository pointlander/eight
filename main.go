// Copyright 2023 The Eight Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"os"
)

const (
	// Width is the width of the fft
	Width = 24
	// Height is the height of the fft
	Height = 24
)

// Frame is a video frame
type Frame struct {
	Frame image.Image
	DCT   [][]float64
}

func picture() {
	webcamera := NewV4LCamera()
	go webcamera.Start("/dev/video0")
	j := 0
	var wc []*image.Paletted
	for j < 32 {
		select {
		case img := <-webcamera.Images:
			opts := gif.Options{
				NumColors: 256,
				Drawer:    draw.FloydSteinberg,
			}
			bounds := img.Frame.Bounds()
			paletted := image.NewPaletted(bounds, palette.Plan9[:opts.NumColors])
			if opts.Quantizer != nil {
				paletted.Palette = opts.Quantizer.Quantize(make(color.Palette, 0, opts.NumColors), img.Frame)
			}
			opts.Drawer.Draw(paletted, bounds, img.Frame, image.Point{})
			wc = append(wc, paletted)
			fmt.Println("left", j)
			j++
		}
	}
	webcamera.Stream = false
	process := func(name string, images []*image.Paletted) {
		animation := &gif.GIF{}
		for _, paletted := range images {
			animation.Image = append(animation.Image, paletted)
			animation.Delay = append(animation.Delay, 0)
		}

		f, _ := os.OpenFile(name, os.O_WRONLY|os.O_CREATE, 0600)
		defer f.Close()
		gif.EncodeAll(f, animation)
	}
	process("webcamera.gif", wc)
}

func main() {
	picture()
}
