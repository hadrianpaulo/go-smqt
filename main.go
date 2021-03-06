package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // Register JPEG format
	"image/png"
	// Register PNG  format
	"log"
	"os"
	"sync"
)

// func timeTrack(start time.Time, name string) {
// 	elapsed := time.Since(start)
// 	log.Printf("%s took %s", name, elapsed)
// }

type imageChannelType int

// const (
// 	RED imageChannelType = iota
// 	GREEN
// 	BLUE
// 	ALPHA
// 	HUE
// 	SATURATION
// 	INTENSITY
// )

type imageChannel struct {
	raw      []uint8 // contains the raw values
	output   []uint8 // contains SMQT intermediate and final outputs
	rawIndex []uint
	// channel imageChannelType // TODO: Add string repr
}

func (imageChan *imageChannel) init(size uint) ([]uint8, []uint8, []uint) {
	imageChan.raw = make([]uint8, size)
	imageChan.output = make([]uint8, size)
	imageChan.rawIndex = makeRange(0, size)
	return imageChan.raw, imageChan.output, imageChan.rawIndex
}

func (imageChan *imageChannel) getPixelValue(index int, newVal uint8) {
	imageChan.raw[index] = newVal
}

func calculateMean(input []uint8) float32 {
	sum := 0
	length := len(input)
	if length == 0 {
		length = length + 1
	}
	for _, value := range input {
		sum = sum + int(value)
	}
	return float32(sum / length)
}

func addBit(i uint8, shiftByOne bool) uint8 {
	if shiftByOne {
		return i<<1 | 1
	}
	return i << 1
}

func makeRange(min, max uint) []uint {
	a := make([]uint, max-min)
	for i := range a {
		a[i] = min + uint(i)
	}
	return a
}

func (imageChan *imageChannel) runSMQT(inputPositions []uint, quantizationLevel uint) {
	if quantizationLevel == 0 {
		return
	}

	var pos uint
	var inputVector []uint8

	for _, v := range inputPositions {
		inputVector = append(inputVector, imageChan.raw[v])
	}

	mean := calculateMean(inputVector)
	var lowpos, highpos []uint

	for _, v := range inputPositions {
		pos = v
		if float32(imageChan.raw[pos]) <= mean {
			lowpos = append(lowpos, pos)
			imageChan.output[pos] = addBit(imageChan.output[pos], false)
		} else {
			highpos = append(highpos, pos)
			imageChan.output[pos] = addBit(imageChan.output[pos], true)
		}
	}
	imageChan.runSMQT(highpos, quantizationLevel-1)
	imageChan.runSMQT(lowpos, quantizationLevel-1)
}

func main() {
	// defer timeTrack(time.Now(), "whole program")
	var redChannel, greenChannel, blueChannel imageChannel

	if len(os.Args) != 3 {
		log.Fatalln("Needs an input image.")
	}
	infile, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}
	defer infile.Close()

	img, _, err := image.Decode(infile)
	if err != nil {
		log.Fatalln(err)
	}

	size := img.Bounds().Size()
	w := size.X
	h := size.Y

	fmt.Println("Initializing image channels..")
	redChannel.init(uint(w * h))
	greenChannel.init(uint(w * h))
	blueChannel.init(uint(w * h))

	fmt.Println("Grabbing RGB values..")
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			r = r / 0x101
			g = g / 0x101
			b = b / 0x101
			index := (x * h) + y // need to fix this for non (0,0) bounds
			redChannel.getPixelValue(index, uint8(r))
			greenChannel.getPixelValue(index, uint8(g))
			blueChannel.getPixelValue(index, uint8(b))
		}
	}

	fmt.Println("Running SMQT..")

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		redChannel.runSMQT(redChannel.rawIndex, 8)
	}()

	go func() {
		defer wg.Done()
		blueChannel.runSMQT(blueChannel.rawIndex, 8)
	}()

	go func() {
		defer wg.Done()
		greenChannel.runSMQT(greenChannel.rawIndex, 8)

	}()
	wg.Wait()

	fmt.Println("Creating output image..")

	imgOut := image.NewRGBA(img.Bounds())
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			index := (x * h) + y
			r := redChannel.output[index]
			g := greenChannel.output[index]
			b := blueChannel.output[index]
			imgOut.Set(x, y, color.NRGBA{r, g, b, 255})
		}
	}

	out, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = png.Encode(out, imgOut)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("SMQT Successful!")
}
