// Copyright 2023 The Eight Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"math"
	"os"
	"time"
)

const (
	// Width is the width of the fft
	Width = 24
	// Height is the height of the fft
	Height = 24
	// EmbeddingWidth is the width of the embedding
	EmbeddingWidth = 24
	// EmbeddingHeight is the height of the embedding
	EmbeddingHeight = 24
)

// Frame is a video frame
type Frame struct {
	Frame image.Image
	DCT   [][]float64
}

// Point is a point
type Point struct {
	Name  string
	Point []float64
}

// Points is a set of points
type Points map[string]Point

func picture() {
	webcamera := NewV4LCamera()
	go webcamera.Start("/dev/video0")
	var wc []*image.Paletted
	for j := 0; j < 32; j++ {
		img := <-webcamera.Images

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

var (
	// FlagLearn a point
	FlagLearn = flag.String("learn", "", "learn a point")
	// FlagInfer
	FlagInfer = flag.Bool("infer", false, "inference mode")
	// FlagPicture take a picture
	FlagPicture = flag.Bool("picture", false, "take a picture")
)

func main() {
	flag.Parse()

	if *FlagPicture {
		picture()
		return
	}
	if *FlagLearn != "" {
		fmt.Println("wait 5 seconds")
		time.Sleep(5 * time.Second)
		input, err := os.Open("points.gob")
		points := make(Points)
		if err == nil {
			decoder := gob.NewDecoder(input)
			err = decoder.Decode(&points)
			if err != nil {
				panic(err)
			}
		}
		input.Close()
		webcamera := NewV4LCamera()
		go webcamera.Start("/dev/video0")
		image := <-webcamera.Images
		webcamera.Stream = false
		values, index := make([]float64, EmbeddingHeight*EmbeddingWidth), 0
		for i := 0; i < EmbeddingHeight; i++ {
			for j := 0; j < EmbeddingWidth; j++ {
				values[index] = image.DCT[i][j]
				index++
			}
		}
		points[*FlagLearn] = Point{
			Name:  *FlagLearn,
			Point: values,
		}

		output, err := os.Create("points.gob")
		if err != nil {
			panic(err)
		}
		defer output.Close()
		encoder := gob.NewEncoder(output)
		err = encoder.Encode(points)
		if err != nil {
			panic(err)
		}
		return
	}
	if *FlagInfer {
		input, err := os.Open("points.gob")
		points := make(Points)
		if err != nil {
			panic(err)
		}
		decoder := gob.NewDecoder(input)
		err = decoder.Decode(&points)
		if err != nil {
			panic(err)
		}
		defer input.Close()

		/*for i := range points {
			length := 0.0
			for _, value := range points[i].Point {
				length += value * value
			}
			length = math.Sqrt(length)
			for j, value := range points[i].Point {
				points[i].Point[j] = value / length
			}
		}*/

		webcamera := NewV4LCamera()
		go webcamera.Start("/dev/video0")
		for {
			image := <-webcamera.Images
			vector, index, length := make([]float64, EmbeddingHeight*EmbeddingWidth), 0, 0.0
			for i := 0; i < EmbeddingHeight; i++ {
				for j := 0; j < EmbeddingWidth; j++ {
					value := image.DCT[i][j]
					vector[index] = value
					length += value * value
					index++
				}
			}
			length = math.Sqrt(length)

			name, min := "", math.MaxFloat64
			for _, point := range points {
				sum := 0.0
				for key, value := range vector {
					diff := point.Point[key] - value
					sum += diff * diff
				}
				if sum < min {
					min, name = sum, point.Name
				}
			}
			fmt.Println("\t\t\t\t\t"+name, min)
		}
		//webcamera.Stream = false
	}
}
