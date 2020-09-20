package avatar

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/rivo/uniseg"
	"golang.org/x/image/font"
)

const (
	defaultfontFace = "Roboto-Bold.ttf" //SourceSansVariable-Roman.ttf"
	fontSize        = 210.0
	imageWidth      = 500.0
	imageHeight     = 500.0
	dpi             = 72.0
	spacer          = 20
	textY           = 320
)

type Avatar struct {
	FontPath string
	FontSize float64
	Width    int
	Height   int
	Dpi      int
	Spacer   int
	TextX    int
	TextY    int

	FontColor string
	BackColor string
}

func NewAvatar() *Avatar {
	return &Avatar{
		FontPath:  defaultfontFace,
		FontSize:  fontSize,
		Width:     int(imageWidth),
		Height:    int(imageHeight),
		Dpi:       int(dpi),
		Spacer:    int(spacer),
		TextX:     0,
		TextY:     int(textY),
		FontColor: "",
		BackColor: "",
	}
}

// ConfigureFont configures font path and font size
func (a *Avatar) ConfigureFont(path string, size float64) *Avatar {
	a.FontPath, a.FontSize = path, size
	return a
}

// ConfigureSize configures size of symbols
func (a *Avatar) ConfigureSize(width, height int) *Avatar {
	a.Width, a.Height = width, height
	return a
}

// ConfigureColor configures font and background colors
func (a *Avatar) ConfigureColor(fontColor, backColor string) *Avatar {
	a.FontColor, a.BackColor = fontColor, backColor
	return a
}

// ConfigurePosition configures avatar symbols position
func (a *Avatar) ConfigurePosition(x, y int) *Avatar {
	a.TextX, a.TextY = x, y
	return a
}

// ToDisk saves the image to disk
func (a *Avatar) ToDisk(initials, path string) error {
	return a.saveToDisk(initials, path, a.BackColor, a.FontColor)
}

// ToDiskCustom saves the image to disk
func (a *Avatar) ToDiskCustom(initials, path, bgColor, fontColor string) error {
	return a.saveToDisk(initials, path, bgColor, fontColor)
}

// saveToDisk saves the image to disk
func (a *Avatar) saveToDisk(initials, path, bgColor, fontColor string) error {
	rgba, err := a.createAvatar(initials, bgColor, fontColor)
	if err != nil {
		return err
	}

	// Save image to disk
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	b := bufio.NewWriter(out)

	err = png.Encode(b, rgba)
	if err != nil {
		return err
	}

	err = b.Flush()
	if err != nil {
		return err
	}

	return nil
}

// ToHTTP sends the image to a http.ResponseWriter (as a PNG)
func (a *Avatar) ToHTTP(initials string, w http.ResponseWriter) error {
	return a.saveToHTTP(initials, "", "", w)
}

// ToHTTPCustom sends the image to a http.ResponseWriter (as a PNG)
func (a *Avatar) ToHTTPCustom(initials, bgColor, fontColor string, w http.ResponseWriter) error {
	return a.saveToHTTP(initials, bgColor, fontColor, w)
}

// saveToHTTP sends the image to a http.ResponseWriter (as a PNG)
func (a *Avatar) saveToHTTP(initials, bgColor, fontColor string, w http.ResponseWriter) error {
	rgba, err := a.createAvatar(initials, bgColor, fontColor)
	if err != nil {
		return err
	}

	b := new(bytes.Buffer)
	key := fmt.Sprintf("avatar%s", initials) // for Etag

	err = png.Encode(b, rgba)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(b.Bytes())))
	w.Header().Set("Cache-Control", "max-age=2592000") // 30 days
	w.Header().Set("Etag", `"`+key+`"`)

	if _, err := w.Write(b.Bytes()); err != nil {
		return err
	}

	return nil
}

func (a *Avatar) cleanString(incoming string) string {
	if len(strings.TrimSpace(incoming)) == 0 {
		return incoming
	}

	sb := strings.Builder{}
	parts := strings.Fields(incoming)

	if len(parts) == 1 && len(incoming) % 2 == 0 {
		return strings.ToUpper(incoming)
	}

	sb.WriteString(string([]rune(parts[0])[0]))

	if len(parts) > 1 {
		sb.WriteString(string([]rune(parts[1])[0]))
	}

	return sb.String()
}

func (a *Avatar) getFont() (*truetype.Font, error) {
	// Read the font data.
	fontBytes, err := ioutil.ReadFile(a.FontPath) //fmt.Sprintf("%s/%s", sourceDir, fontFaceName))
	if err != nil {
		return nil, err
	}

	return freetype.ParseFont(fontBytes)
}

var imageCache sync.Map

func (a *Avatar) getImage(initials string) *image.RGBA {
	value, ok := imageCache.Load(initials)

	if !ok {
		return nil
	}

	image, ok2 := value.(*image.RGBA)
	if !ok2 {
		return nil
	}
	return image
}

func (a *Avatar) setImage(initials string, image *image.RGBA) {
	imageCache.Store(initials, image)
}

func s2stringUtf(s string) []string {
	res := make([]string, 0)
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		runes := gr.Runes()
		for _, r := range runes {
			res = append(res, string(r))
		}
	}

	return res
}

func s2runesUtf(s string) []rune {
	res := make([]rune, 0)
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		runes := gr.Runes()
		for _, r := range runes {
			res = append(res, r)
		}
	}

	return res
}

func (a *Avatar) createAvatar(initials, bgColor, fontColor string) (*image.RGBA, error) {
	// Make sure the string is OK
	text := a.cleanString(initials)

	// Check cache
	cachedImage := a.getImage(text)
	if cachedImage != nil {
		return cachedImage, nil
	}

	// Load and get the font
	f, err := a.getFont()
	if err != nil {
		return nil, err
	}

	// Setup the colors, text white, background based on first initial
	textColor := image.White
	if fontColor != "" {
		c, err := parseHexColorFast(fontColor)
		if err == nil {
			textColor = &image.Uniform{c}
		}
	}
	background := defaultColor(text[0:1])
	if bgColor != "" {
		c, err := parseHexColorFast(bgColor)
		if err == nil {
			background = image.Uniform{c}
		}
	}

	rgba := image.NewRGBA(image.Rect(0, 0, a.Width, a.Height))
	draw.Draw(rgba, rgba.Bounds(), &background, image.ZP, draw.Src)
	c := freetype.NewContext()
	c.SetDPI(float64(a.Dpi))
	c.SetFont(f)
	c.SetFontSize(a.FontSize)
	c.SetClip(rgba.Bounds())
	c.SetDst(rgba)
	c.SetSrc(textColor)
	c.SetHinting(font.HintingFull)

	// We need to convert the font into a "font.Face" so we can read the glyph
	// info
	to := truetype.Options{}
	to.Size = a.FontSize
	face := truetype.NewFace(f, &to)

	// Calculate the widths and print to image
	xPoints := []int{0, 0}
	textWidths := []int{0, 0}

	// Get the widths of the text characters
	for i, char := range s2runesUtf(text) {
		width, ok := face.GlyphAdvance(rune(char))
		if !ok {
			return nil, err
		}

		textWidths[i] = int(width / 64)
	}

	// TODO need some tests for this
	if len(textWidths) == 1 {
		textWidths[1] = 0
	}

	// Get the combined width of the characters
	combinedWidth := textWidths[0] + a.Spacer + textWidths[1]

	// Draw first character
	xPoints[0] = int((a.Width - combinedWidth) / 2)
	xPoints[1] = int(xPoints[0] + textWidths[0] + a.Spacer)

	for i, char := range s2runesUtf(text) {
		pt := freetype.Pt(xPoints[i], a.TextY)
		_, err := c.DrawString(string(char), pt)
		if err != nil {
			return nil, err
		}
	}

	// Cache it
	a.setImage(text, rgba)

	return rgba, nil
}
