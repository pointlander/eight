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
	"math/cmplx"
	"os"
	"time"

	"github.com/pointlander/image-segmentation/graph"
	"github.com/pointlander/image-segmentation/segmentation"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

const (
	// Width is the width of the fft
	Width = 24
	// Height is the height of the fft
	Height = 24
	// EmbeddingWidth is the width of the embedding
	EmbeddingWidth = 8
	// EmbeddingHeight is the height of the embedding
	EmbeddingHeight = 8
)

// Frame is a video frame
type Frame struct {
	Frame *image.YCbCr
	DCT   [][]complex128
}

// Point is a point
type Point struct {
	Name   string
	Points [][]complex128
}

// Points is a set of points
type Points map[string]Point

// Color is a color
type Color struct {
	R, G, B, A uint32
}

// Index is an index of colors to image sides
type Index map[Color][4]bool

func Segment(img *image.YCbCr) image.Image {
	// 0 to 1
	sigma := .8
	graphType := graph.KINGSGRAPH
	weightfn := segmentation.NNWeight
	segmenter := segmentation.New(img, graphType, weightfn)
	segmenter.SetRandomColors(true)
	// 0 to 15
	minWeight := 5.0
	segmenter.SegmentHMSF(sigma, minWeight)
	result := segmenter.GetResultImage()

	index, bounds := make(Index), result.Bounds()
	for i := 0; i < bounds.Max.X; i++ {
		r, g, b, a := result.At(i, 0).RGBA()
		color := Color{
			R: r,
			G: g,
			B: b,
			A: a,
		}
		sides := index[color]
		sides[0] = true
		index[color] = sides
	}
	for i := 0; i < bounds.Max.Y/2; i++ {
		r, g, b, a := result.At(bounds.Max.X-1, i).RGBA()
		color := Color{
			R: r,
			G: g,
			B: b,
			A: a,
		}
		sides := index[color]
		sides[1] = true
		index[color] = sides
	}
	/*{
		r, g, b, a := result.At(bounds.Max.X/2, bounds.Max.Y-1).RGBA()
		color := Color{
			R: r,
			G: g,
			B: b,
			A: a,
		}
		sides := index[color]
		sides[2] = true
		index[color] = sides
	}*/
	for i := 0; i < bounds.Max.Y/2; i++ {
		r, g, b, a := result.At(0, i).RGBA()
		color := Color{
			R: r,
			G: g,
			B: b,
			A: a,
		}
		sides := index[color]
		sides[3] = true
		index[color] = sides
	}

	cp := image.NewRGBA(img.Bounds())
	draw.Draw(cp, cp.Bounds(), img, img.Bounds().Min, draw.Src)
	for i := 0; i < bounds.Max.X; i++ {
		for j := 0; j < bounds.Max.Y; j++ {
			r, g, b, a := result.At(i, j).RGBA()
			c := Color{
				R: r,
				G: g,
				B: b,
				A: a,
			}
			sides := index[c]
			del := false
			for k := 0; k < len(sides); k++ {
				if sides[k] {
					del = true
					break
				}
			}
			if del {
				cp.SetRGBA(i, j, color.RGBA{})
			}
		}
	}

	return cp
}

func picture() {
	webcamera := NewV4LCamera()
	go webcamera.Start(*FlagDevice)
	var wc, seg []*image.Paletted
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

		if *FlagSegmentation {
			cp := Segment(img.Frame)
			opts = gif.Options{
				NumColors: 256,
				Drawer:    draw.FloydSteinberg,
			}
			bounds = cp.Bounds()
			paletted = image.NewPaletted(bounds, palette.Plan9[:opts.NumColors])
			if opts.Quantizer != nil {
				paletted.Palette = opts.Quantizer.Quantize(make(color.Palette, 0, opts.NumColors), cp)
			}
			opts.Drawer.Draw(paletted, bounds, cp, image.Point{})
			seg = append(seg, paletted)
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
	if *FlagSegmentation {
		process("segmented.gif", seg)
	}
}

var (
	// FlagDevice is the video for linux device to use
	FlagDevice = flag.String("device", "/dev/video0", "video for linux device")
	// FlagLearn a point
	FlagLearn = flag.String("learn", "", "learn a point")
	// FlagInfer
	FlagInfer = flag.Bool("infer", false, "inference mode")
	// FlagPicture take a picture
	FlagPicture = flag.Bool("picture", false, "take a picture")
	// FlagSegmentation segmentation is enabled for background removal
	FlagSegmentation = flag.Bool("segmentation", false, "segmentation is enabled for background removal")
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
		go webcamera.Start(*FlagDevice)
		image := <-webcamera.Images
		webcamera.Stream = false
		values, index := make([]complex128, EmbeddingHeight*EmbeddingWidth), 0
		for i := 0; i < EmbeddingHeight; i++ {
			for j := 0; j < EmbeddingWidth; j++ {
				values[index] = image.DCT[i][j]
				index++
			}
		}

		entry := points[*FlagLearn]
		entry.Name = *FlagLearn
		entry.Points = append(entry.Points, values)
		points[*FlagLearn] = entry

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

		webcamera := NewV4LCamera()
		go webcamera.Start(*FlagDevice)
		for {
			image := <-webcamera.Images
			vector, index := make([]complex128, EmbeddingHeight*EmbeddingWidth), 0
			for i := 0; i < EmbeddingHeight; i++ {
				for j := 0; j < EmbeddingWidth; j++ {
					value := image.DCT[i][j]
					vector[index] = value
					index++
				}
			}

			name, min := "", math.MaxFloat64
			for _, entry := range points {
				for _, point := range entry.Points {
					sum := 0.0
					for key, value := range vector {
						diff := cmplx.Abs(point[key]) - cmplx.Abs(value)
						sum += diff * diff
					}
					if sum < min {
						min, name = sum, entry.Name
					}
				}
			}
			fmt.Printf("%s %f", name, min)

			entry := points[name]
			if length := len(entry.Points); length > 2 {
				width := EmbeddingHeight * EmbeddingWidth
				length += 1
				data := make([]float64, 0, length*width)
				for _, points := range entry.Points {
					for _, point := range points {
						data = append(data, cmplx.Phase(point))
					}
				}
				for _, point := range vector {
					data = append(data, cmplx.Phase(point))
				}
				ranks := mat.NewDense(length, width, data)
				var pc stat.PC
				ok := pc.PrincipalComponents(ranks, nil)
				if !ok {
					panic("PrincipalComponents failed")
				}
				k := 2
				var proj mat.Dense
				var vec mat.Dense
				pc.VectorsTo(&vec)
				proj.Mul(ranks, vec.Slice(0, width, 0, k))

				fmt.Printf(" %f %f\n", proj.At(length-1, 0), proj.At(length-1, 1))
			}
		}
	}
}
