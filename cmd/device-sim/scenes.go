package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math/rand/v2"
	"os"
	"path/filepath"
)

// The simulator has no camera, so it draws simple scenes for what it
// "detects": a person, a car, a motorcycle, a person carrying a box. They
// are crude, but distinct enough that the CLIP embedder places a "car"
// scene nearer the words "سيارة" or "car" than a "person" scene, which lets
// search by description be tried end to end in development.

var (
	ink   = color.RGBA{28, 26, 32, 255}
	skin  = color.RGBA{214, 170, 130, 255}
	shirt = []color.RGBA{{200, 60, 50, 255}, {40, 90, 170, 255}, {240, 200, 60, 255}, {60, 140, 90, 255}}
	paint = []color.RGBA{{190, 30, 40, 255}, {230, 230, 235, 255}, {30, 60, 140, 255}, {20, 20, 24, 255}}
	box   = color.RGBA{176, 128, 72, 255}
)

func rect(img draw.Image, x0, y0, x1, y1 int, c color.Color) {
	draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{c}, image.Point{}, draw.Src)
}

func disc(img draw.Image, cx, cy, r int, c color.Color) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				img.Set(cx+x, cy+y, c)
			}
		}
	}
}

func person(img draw.Image, x, ground int, top color.RGBA) {
	disc(img, x, ground-92, 11, skin)                // head
	rect(img, x-14, ground-80, x+14, ground-38, top) // torso
	rect(img, x-20, ground-78, x-14, ground-44, top) // arms
	rect(img, x+14, ground-78, x+20, ground-44, top)
	rect(img, x-12, ground-38, x-3, ground, ink) // legs
	rect(img, x+3, ground-38, x+12, ground, ink)
}

func car(img draw.Image, x, ground int, body color.RGBA) {
	rect(img, x-70, ground-42, x+70, ground-14, body)                          // body
	rect(img, x-40, ground-66, x+34, ground-42, body)                          // cabin
	rect(img, x-34, ground-61, x-4, ground-44, color.RGBA{150, 200, 230, 255}) // windows
	rect(img, x+2, ground-61, x+28, ground-44, color.RGBA{150, 200, 230, 255})
	disc(img, x-42, ground-12, 13, ink) // wheels
	disc(img, x+42, ground-12, 13, ink)
	disc(img, x-42, ground-12, 5, color.RGBA{160, 160, 165, 255})
	disc(img, x+42, ground-12, 5, color.RGBA{160, 160, 165, 255})
}

func motorcycle(img draw.Image, x, ground int, body color.RGBA) {
	disc(img, x-34, ground-16, 16, ink)
	disc(img, x+34, ground-16, 16, ink)
	disc(img, x-34, ground-16, 7, color.RGBA{150, 150, 155, 255})
	disc(img, x+34, ground-16, 7, color.RGBA{150, 150, 155, 255})
	rect(img, x-26, ground-40, x+20, ground-24, body) // tank and seat
	rect(img, x+18, ground-58, x+24, ground-24, ink)  // fork
	person(img, x-6, ground-26, shirt[0])             // rider, sitting high
}

// dumpScenes writes sample frames as dir/<label>/NNN.jpg: a stand-in
// dataset for trying ai/train without footage from real cameras.
func dumpScenes(dir string, n int) error {
	for _, label := range []string{"person", "car", "motorcycle", "box"} {
		d := filepath.Join(dir, label)
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
		for i := 0; i < n; i++ {
			if err := os.WriteFile(filepath.Join(d, fmt.Sprintf("%03d.jpg", i)), scene(label, i), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// scene draws one frame for a label on a background tinted per camera.
func scene(label string, cam int) []byte {
	const w, h = 320, 180
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	walls := []color.RGBA{{120, 135, 150, 255}, {150, 130, 110, 255}, {120, 150, 125, 255}, {135, 125, 155, 255}}
	wall := walls[cam%len(walls)]
	rect(img, 0, 0, w, h*2/3, wall)
	rect(img, 0, h*2/3, w, h, color.RGBA{wall.R / 2, wall.G / 2, wall.B / 2, 255})
	ground := h - 12
	x := 90 + rand.IntN(140)
	switch label {
	case "car":
		car(img, x, ground, paint[rand.IntN(len(paint))])
	case "motorcycle", "bicycle":
		motorcycle(img, x, ground, paint[rand.IntN(len(paint))])
	case "box":
		person(img, x, ground, shirt[rand.IntN(len(shirt))])
		rect(img, x-24, ground-74, x+24, ground-40, box) // carried box
		rect(img, x-24, ground-58, x+24, ground-56, color.RGBA{120, 84, 44, 255})
	default:
		person(img, x, ground, shirt[rand.IntN(len(shirt))])
	}
	// Sensor noise so it reads as a camera frame, not a flat drawing.
	for i := 0; i < w*h/6; i++ {
		px, py := rand.IntN(w), rand.IntN(h)
		c := img.RGBAAt(px, py)
		n := rand.IntN(14)
		lift := func(v uint8) uint8 { return uint8(min(255, int(v)+n)) }
		img.SetRGBA(px, py, color.RGBA{lift(c.R), lift(c.G), lift(c.B), 255})
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, &jpeg.Options{Quality: 75})
	return b.Bytes()
}
