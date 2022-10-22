// October 2022
// Sokoban game  https://en.wikipedia.org/wiki/Sokoban
//
// All Sprites from:   https://kenney.nl/assets/
// Game levels from:   https://github.com/begoon/sokoban-maps

package main

import (
	_ "embed"
	"bytes"
	"log"
	"fmt"
	"image"
	"image/png"
	"time"
	
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type screenZone struct {
	nHorizontalSectors, nVerticalSectors int
	hSector, vSector int
}

type Level struct {
	w, h byte
	px, py int     // player coordinates
	psprite byte
	zfactor float64 // zoom factor (same for horizontal and vertical)
	sx, sy float64  // screen offset to center level
	grid [][]byte
}

type Game struct {
 	pressedKeys []ebiten.Key
}

const (
	screenWidth  = 1900
	screenHeight = 1000

	LEVEL_MAX = 62

	EMPTY = 89
	WALL = 98
	BOX = 6
	PLACED_BOX = 9
	GOAL = 102

	PLAYERUP = 55
	PLAYERDN = 52
	PLAYERRI = 78
	PLAYERLE = 81

	UP byte = iota
	RIGHT
	DOWN
	LEFT
)

// |        ground wall box boxgoal groundgoal
// #sprites 89     98   6   9       102
// 
// |       playerup playerdn playerri playerle
// #player 55       52       78       81

//go:embed "sokoban_tilesheet.png"
var spritePNG []byte

//go:embed "sheet_white2x.png"
var iconsPNG []byte

var (
	
	rightScreenZone = screenZone    { 20, 10, 20, 9 }
	leftScreenZone = screenZone     { 20, 10, 18, 9 }
	upScreenZone = screenZone       { 20, 10, 19, 8 }
	downScreenZone = screenZone     { 20, 10, 19, 10 }
	
	undoScreenZone = screenZone     { 20, 10, 1, 1 }
	
	nextScreenZone = screenZone     { 20, 10, 20, 1}
	previousScreenZone = screenZone { 20, 10, 19, 1}

 	tileSheet *ebiten.Image
 	iconsSheet *ebiten.Image
 
	// stack of the moves that have been played to enable undo
	moves []byte
	currentLevelNumber = 0
	curLev Level

	prevUpdateTime    = time.Now()
)

func prepareSpriteSheet(PNG []byte) *ebiten.Image {
	
	img, err := png.Decode(bytes.NewReader(PNG))
	
	if err != nil {
		log.Fatal(err)
	}

	origEbitenImage := ebiten.NewImageFromImage(img)

	w, h := origEbitenImage.Size()
	
	tileSheet := ebiten.NewImage(w, h)
	
	op := &ebiten.DrawImageOptions{}
	
	tileSheet.DrawImage(origEbitenImage, op)

	return tileSheet
}

func init() {

	// sokoban sprites
	tileSheet = prepareSpriteSheet(spritePNG)
	
	// icon sprites
	iconsSheet = prepareSpriteSheet(iconsPNG)

	// decompress current level
	curLev = decompressLevel(levels[currentLevelNumber])
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func handleMove(dx int, dy int) {

	moveOnce := int(curLev.grid[curLev.px+dx][curLev.py+dy])
	
	if moveOnce == EMPTY || moveOnce == GOAL {
		// just move the player in the grid
		curLev.px += dx
		curLev.py += dy
		
	} else if moveOnce == BOX || moveOnce == PLACED_BOX {
		var saveTile byte
		
 		moveTwice := int(curLev.grid[curLev.px+2*dx][curLev.py+2*dy])

		saveTile=EMPTY
		
		if moveOnce == PLACED_BOX {
			saveTile=GOAL
		}
		
 		if moveTwice == EMPTY {
			curLev.grid[curLev.px+dx][curLev.py+dy] = saveTile
 			curLev.grid[curLev.px+2*dx][curLev.py+2*dy] = BOX
			curLev.px += dx
			curLev.py += dy
 		} else if moveTwice == GOAL {
 			curLev.grid[curLev.px+dx][curLev.py+dy] = saveTile
 			curLev.grid[curLev.px+2*dx][curLev.py+2*dy] = PLACED_BOX
			curLev.px += dx
			curLev.py += dy
 		} 
 	}
}

func nBoxesLeft() int {

	w, h := curLev.w, curLev.h

	boxesLeft:=0
	
	for i:=0; i<int(w); i++ {
		for j:=0; j<int(h); j++ {
			if curLev.grid[i][j] == BOX {
				boxesLeft++
			}
		}
	}

	return(boxesLeft)
}

func screenZoneCoords(z screenZone) (int,int,int,int) {

	nHorizontalSectors := z.nHorizontalSectors
	nVerticalSectors := z.nVerticalSectors
	hSector := z.hSector
	vSector := z.vSector
	
	// we cut the screen horizontally in nHorizontalSectors zones of same dimension, same vertically
	// we test if the mouse is inside hSector, vSector

	sectorWidth := screenWidth / nHorizontalSectors
	sectorHeight := screenHeight / nVerticalSectors

	xMin := sectorWidth * (hSector - 1)
	xMax := sectorWidth * hSector

	yMin := sectorHeight * (vSector - 1)
	yMax := sectorHeight * vSector

	return xMin, yMin, xMax, yMax
}

func inScreenZone(z screenZone, xEvent int, yEvent int) bool {
	
	// we cut the screen horizontally in nHorizontalSectors zones of same dimension, same vertically
	// we test if the mouse is inside hSector, vSector

	xMin, yMin, xMax, yMax := screenZoneCoords(z)

	inside := xEvent>xMin && xEvent<xMax && yEvent>yMin && yEvent<yMax
	
	return inside
}

func (g *Game) Update() error {

	mouseOrTouch := false
	eventX, eventY := 0, 0

	// mouse
	xm, ym := ebiten.CursorPosition()
	pressedLeft := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	// touch events
	touches := inpututil.AppendJustPressedTouchIDs(nil)
	xt, yt := -1, -1
	touched := false
	
	if len(touches) > 0 {
		xt, yt = ebiten.TouchPosition(touches[0])
		touched = true
	}

	if(pressedLeft) {
		mouseOrTouch = true
		eventX = xm
		eventY = ym
	}

	if(touched) {
		mouseOrTouch = true
		eventX = xt
		eventY = yt
	}

	prevUpdateTime = time.Now()

	// the below style of keyboard input takes care of key repetition
        if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) || (mouseOrTouch && inScreenZone(nextScreenZone,eventX, eventY)){
		currentLevelNumber++
		if currentLevelNumber > LEVEL_MAX {
			currentLevelNumber = LEVEL_MAX
		}
		l := decompressLevel(levels[currentLevelNumber])
		moves = nil
		curLev = l
        }
	
	if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) || (mouseOrTouch && inScreenZone(previousScreenZone,eventX, eventY)) {
		currentLevelNumber--
		if currentLevelNumber<0 {
			currentLevelNumber=0
		}
		l := decompressLevel(levels[currentLevelNumber])
		moves = nil
		curLev = l
        }

	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) || ( mouseOrTouch && inScreenZone(undoScreenZone,eventX, eventY)) {

		// UNDO
		if len(moves)>0 {
			// get original level data
			l := decompressLevel(levels[currentLevelNumber])
			curLev = l

			// replay all moves but the very last one
			for i:=0;i<len(moves)-1;i++ {
				if moves[i]==RIGHT {
					curLev.psprite = PLAYERRI
					handleMove(1,0)
				} else if moves[i]==LEFT {
					curLev.psprite = PLAYERLE
					handleMove(-1,0)
				} else if moves[i]==UP {
					curLev.psprite = PLAYERUP
					handleMove(0,-1)
				} else if moves[i]==DOWN {
					curLev.psprite = PLAYERDN
					handleMove(0,1)
				}
			}
			// remove the last move
			moves = moves[:len(moves)-1]
		}
        }
	
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || (mouseOrTouch && inScreenZone(rightScreenZone,eventX, eventY) ) {
		
		curLev.psprite = PLAYERRI
		moves=append(moves, RIGHT)
		handleMove(1,0)
        }
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || (mouseOrTouch && inScreenZone(leftScreenZone,eventX, eventY) ) {
		curLev.psprite = PLAYERLE
		moves=append(moves, LEFT)
		handleMove(-1,0)
        }
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || (mouseOrTouch && inScreenZone(upScreenZone,eventX, eventY)) {
		curLev.psprite = PLAYERUP
		moves=append(moves, UP)
		handleMove(0,-1)
        }
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || (mouseOrTouch && inScreenZone(downScreenZone,eventX, eventY)) {
		curLev.psprite = PLAYERDN
		moves=append(moves, DOWN)
		handleMove(0,1)
        }

	//
	if nBoxesLeft() == 0 {
		currentLevelNumber++
		if currentLevelNumber > LEVEL_MAX {
			currentLevelNumber = LEVEL_MAX
		}
		l := decompressLevel(levels[currentLevelNumber])
		moves = nil
		curLev = l
	}

	return nil
}

func drawIcon(screen *ebiten.Image, iconNumber int, z screenZone, x int, y int) {

	yIcon := iconNumber % 20
	xIcon := iconNumber / 20

	op := &ebiten.DrawImageOptions{}
	op.ColorM.Scale(1, 1, 1, 0.5)

	xMin, yMin, xMax, yMax := screenZoneCoords(z)

	op.GeoM.Scale((float64(xMax-xMin))/100,(float64(yMax-yMin))/100)
        op.GeoM.Translate(float64(xMin),float64(yMin))
	
	screen.DrawImage(iconsSheet.SubImage(image.Rect(xIcon*100, yIcon*100, (1+xIcon)*100, (1+yIcon)*100)).(*ebiten.Image), op)
}

func drawSprite(screen *ebiten.Image, x int, y int, num int, startX float64, startY float64, factor float64, spriteW int, spriteH int) {

	// compute sprite number -> coordinates
	i := num % 13
	j := num / 13

	op := &ebiten.DrawImageOptions{}

	op.GeoM.Scale(factor,factor)
        op.GeoM.Translate(startX+float64(x)*float64(spriteW)*factor,startY+float64(y)*float64(spriteH)*factor)
	
	screen.DrawImage(tileSheet.SubImage(image.Rect(i*spriteW,j*spriteH,(i+1)*spriteW,(j+1)*spriteH)).(*ebiten.Image), op)
}

func (g *Game) Draw(screen *ebiten.Image) {

	// draw the curLev
	w, h := curLev.w, curLev.h

	cell:=0
	for i:=0; i<int(w); i++ {
		for j:=0; j<int(h); j++ {
			drawSprite(screen, i, j, EMPTY, curLev.sx, curLev.sy, curLev.zfactor, 64.0, 64.0)
			drawSprite(screen, i, j, int(curLev.grid[i][j]), curLev.sx, curLev.sy, curLev.zfactor, 64.0, 64.0)
			cell++
		}
	}

	// Draw the player

	drawSprite(screen, int(curLev.px), int(curLev.py), int(curLev.psprite), curLev.sx, curLev.sy, curLev.zfactor, 64.0, 64.0)
	
	ebitenutil.DebugPrint(screen, fmt.Sprintf("Current level: %2d (fps: %0.2f)", currentLevelNumber, ebiten.CurrentTPS()))

	// To draw frames per second
	//	const x = 20
	//	msg := fmt.Sprintf("TPS: %0.2f", ebiten.CurrentTPS())
	//	text.Draw(screen, msg, mplusNormalFont, x, 40, color.White)

	// draw icons: left, right, up, down next level, prev level, undo

	drawIcon(screen, 45, undoScreenZone, 0, 0)
	drawIcon(screen, 9, upScreenZone, 0, 0)
	drawIcon(screen, 10, rightScreenZone, 0, 0)
	drawIcon(screen, 11, leftScreenZone, 0, 0)
	drawIcon(screen, 12, downScreenZone, 0, 0)

	drawIcon(screen, 83, nextScreenZone, 0, 0)
	drawIcon(screen, 44, previousScreenZone, 0, 0)
}

//|  -- Format of the compressed levels ( RLE style )
//|  -- Prolog
//|         char size_x
//|         char size_y
//|  -- Elements
//|         counter (bits)
//|                 0                - 1 symbol
//|                 1 D3 D2 D1       - 2+D3*4+D2*2+D1 symbols (9 max)
//|         char (bits)
//|                 0 0              - an empty space              -> 0
//|                 0 1              - the wall                    -> 1
//|                 1 0              - the box                     -> 2
//|                 1 1 1            - the box already in place    -> 3
//|                 1 1 0            - the goal for a box          -> 4
//|  -- Epilog
//|         char man_x
//|         char man_y

func decompressLevel(level []byte) Level {
	var l Level
	var length = len(level)
	var bits []bool
	var grid []byte

	l.w, l.h = level[0], level[1]
	l.px, l.py = int(level[length-2]), int(level[length-1])

	for i:=2; i<length-2; i++ {
		b:=level[i]
		for j:=7; j>=0; j-- {
			if((b & (1 << j)) > 0) {
				bits = append(bits, true)
			} else {
				bits = append(bits, false)
			}
		}
	}

	var counter int
	var object byte

	i := 0
	for len(grid) != int(l.w) * int(l.h) {

		// extract counter
		
		if(! bits[i]) {
			counter = 1
			i++
		} else {
			counter = 2
			d3 := bits[i+1]
			d2 := bits[i+2]
			d1 := bits[i+3]
			if(d3) {
				counter += 4
			}
			if(d2) {
				counter += 2
			}
			if(d1) {
				counter ++
			}
			i+=4
		}

		// extract object
		if !bits[i] {
			if!bits[i+1] {
				object = EMPTY
				
			} else {
				object = WALL
			}
			i+=2
		} else {
			if !bits[i+1] {
				object = BOX
				i+=2
			} else {
				if bits[i+2] {
					object = PLACED_BOX
				} else {
					object = GOAL
				}
				i+=3
			}
		}
		// add in grid object
		for j:=0;j<counter;j++ {
			grid = append(grid, object)
		}
	}

	// convert to a 2d array

	var grid2 = make([][]byte, l.w)
	
	for i := range grid2 {
		grid2[i] = make([]byte, l.h)
	}

	for x:=0;x<int(l.w);x++ {
		for y:=0;y<int(l.h);y++ {
			grid2[x][y] = grid[y*int(l.w)+x]
		}
	}

	l.grid = grid2

	// Compute screen specifics

	startX:=0.0
	startY:=0.0
	
	var factor float64

	width := 64.0 * float64(l.w)
	height := 64.0 * float64(l.h)
	
	factorW := float64(screenWidth)/width
	factorH := float64(screenHeight)/height

	if factorW > factorH {
		factor = factorH
		startX=(screenWidth-factorH*width)/2.0
	} else {
		factor = factorW
		startY=(screenHeight-factorW*height)/2.0
	}

	l.zfactor = factor
	l.sx, l.sy = startX, startY

	l.psprite = PLAYERUP
	
	return(l)
}

func main() {

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Sokoban")

	if err := ebiten.RunGame(&Game{}); err != nil {
		panic(err)
	}
}
