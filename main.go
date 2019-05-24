// Copyright Bjørn Borud 2019 Use of this source code is governed by
// the license found in the accompanying LICENSE file.
//
// Simple utility for turning a bitmap into colored dots whose
// diameter is proportional to the luminescence of the region the dot
// represents and the color is the average color of the area.
//
// This program is probably slow, and fairly suboptimal stemming from
// the fact that I have absolutely no experience writing graphics
// utilities.  But hopefully it is easy to read and understand.
//
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"strings"

	svg "github.com/ajstarks/svgo"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

var (
	inputFile     = flag.String("f", "", "input image in either JPEG, PNG or GIF")
	outputFile    = flag.String("o", "", "Output file")
	boxSize       = flag.Int("b", 50, "Box size for dots")
	lumaThreshold = flag.Float64("t", 1.0, "Luma threshold - don't draw dots above this luminescence value.  Value from 0.0 to 1.0")
	mono          = flag.Bool("m", false, "Set image to monochrome")
	bt701         = flag.Bool("l", false, "Use BT.701 instead of BT.601 for luma calculations")
)

// readImage reads the source image. What formats it can understand
// depends on what formats have been loaded.
func readImage(fileName string) (image.Image, error) {
	imgFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// Calculate luma based on rgb values using ITU BT.709.  The value
// returned is between 0.0 and 1.0 so it is convenient to be used for
// scaling other values.
func lumaBT709(r uint32, g uint32, b uint32) float64 {
	return ((0.2126 * float64(r)) + (0.7152 * float64(g)) + (0.0722 * float64(b))) / 255.0
}

// Calculate luma based on rgb values using ITU BT.601.  This gives
// more weight to the red and blue components. The value returned is
// between 0.0 and 1.0 so it is convenient to be used for scaling
// other values.
func lumaBT601(r uint32, g uint32, b uint32) float64 {
	return ((0.299 * float64(r)) + (0.587 * float64(g)) + (0.144 * float64(b))) / 255.0
}

// makeDots creates dots whose diameter is proportional to the
// luminance and color is the average color of the area in the image
// they represent.
func makeDots(img image.Image, boxSize int, lumaThreshold float64, mono bool, bt709 bool, outputFileName string) {
	// Create the output file
	svgFile, err := os.Create(outputFileName)
	if err != nil {
		log.Fatalf("Unable to open svg file %s: %v", outputFileName, err)
	}
	defer svgFile.Close()

	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	// Create new SVG canvas whose width and height are the same as
	// the pixels of the original picture just to make coordinates
	// match up.
	canvas := svg.New(svgFile)
	canvas.Start(width, height)
	defer canvas.End()

	// Calculate useful values
	boxHalf := boxSize / 2

	boxSizeSquared := boxSize * boxSize
	widthSteps := width / boxSize
	heightSteps := height / boxSize

	// Choose luma function
	lumaFunc := lumaBT601
	if *bt701 {
		lumaFunc = lumaBT709
	}

	// Step through the entire image dividing it up into boxes with
	// edges that are boxSize long and calculate the average color for
	// each box.
	for x := 0; x < widthSteps; x++ {
		for y := 0; y < heightSteps; y++ {

			var rSum, gSum, bSum uint32

			for i := 0; i < boxSize; i++ {
				for j := 0; j < boxSize; j++ {
					cx := (x * boxSize) + i
					cy := (y * boxSize) + j

					r, g, b, _ := img.At(cx, cy).RGBA()
					rSum += r
					gSum += g
					bSum += b
				}
			}

			// Calculate the average color for the box
			rSum /= uint32(boxSizeSquared)
			gSum /= uint32(boxSizeSquared)
			bSum /= uint32(boxSizeSquared)

			// Compensating for annoying scaling factor somewhere
			// internally in the color package
			rSum /= 0x101
			bSum /= 0x101
			gSum /= 0x101

			luma := lumaFunc(rSum, gSum, bSum)
			if luma >= lumaThreshold {
				continue
			}

			if mono {
				canvas.Circle((x*boxSize)+boxHalf, (y*boxSize)+boxHalf, int((1.0-luma)*float64(boxHalf)), "fill:black;stroke:none")
			} else {
				canvas.Circle((x*boxSize)+boxHalf, (y*boxSize)+boxHalf, int((1.0-luma)*float64(boxHalf)), fmt.Sprintf("fill:#%02x%02x%02x;stroke:none", rSum, gSum, bSum))
			}
		}
	}
}

func main() {
	flag.Parse()

	if *inputFile == "" {
		flag.Usage()
		return
	}

	if *lumaThreshold < 0.0 || *lumaThreshold > 1.0 {
		log.Fatalf("Invalid luma threshold, must be between 0.0 and 1.0")
	}

	img, err := readImage(*inputFile)
	if err != nil {
		log.Fatalf("Error reading image %s: %v", *inputFile, err)
	}

	if *outputFile == "" {
		fn := strings.TrimSuffix(*inputFile, filepath.Ext(*inputFile)) + ".svg"
		outputFile = &fn
	}

	makeDots(img, *boxSize, *lumaThreshold, *mono, *bt701, *outputFile)
}
