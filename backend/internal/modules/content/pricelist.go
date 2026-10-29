package content

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/platform/apperr"
)

// PriceRow is one product with its plans on the price list.
type PriceRow struct {
	Product string
	Plans   []PricePlan
}

type PricePlan struct {
	Name  string
	Price string
}

var (
	fontsOnce            sync.Once
	fontErr              error
	titleFace, headFace  font.Face
	bodyFace, smallFace  font.Face
	bgColor              = color.RGBA{0xF8, 0xFA, 0xFC, 0xFF}
	headerColor          = color.RGBA{0x0F, 0x76, 0x6E, 0xFF}
	textColor            = color.RGBA{0x0F, 0x17, 0x2A, 0xFF}
	mutedColor           = color.RGBA{0x64, 0x74, 0x8B, 0xFF}
	cardColor            = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	lineColor            = color.RGBA{0xE2, 0xE8, 0xF0, 0xFF}
	errNoProducts        = apperr.Validation("no active products with active plans to put on the price list")
	priceListWidth       = 1080
	priceListPadding     = 48
	priceListRowHeight   = 44
	priceListHeaderSpace = 200
)

func loadFonts() error {
	fontsOnce.Do(func() {
		reg, err := opentype.Parse(goregular.TTF)
		if err != nil {
			fontErr = err
			return
		}
		bold, err := opentype.Parse(gobold.TTF)
		if err != nil {
			fontErr = err
			return
		}
		mk := func(f *opentype.Font, size float64) font.Face {
			face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
			if err != nil && fontErr == nil {
				fontErr = err
			}
			return face
		}
		titleFace, headFace, bodyFace, smallFace = mk(bold, 52), mk(bold, 32), mk(reg, 28), mk(reg, 22)
	})
	return fontErr
}

func text(img draw.Image, face font.Face, c color.Color, x, y int, s string) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

func textWidth(face font.Face, s string) int {
	return (&font.Drawer{Face: face}).MeasureString(s).Ceil()
}

func fill(img draw.Image, r image.Rectangle, c color.Color) {
	draw.Draw(img, r, image.NewUniform(c), image.Point{}, draw.Src)
}

// RenderPriceList draws the list as PNG bytes. The embedded Go fonts cover Latin text;
// prices are written as "Tk 350".
func RenderPriceList(business string, rows []PriceRow, footer string) ([]byte, error) {
	if len(rows) == 0 {
		return nil, errNoProducts
	}
	if err := loadFonts(); err != nil {
		return nil, err
	}
	pad, w := priceListPadding, priceListWidth
	h := priceListHeaderSpace + pad
	for _, r := range rows {
		h += 64 + len(r.Plans)*priceListRowHeight + 24
	}
	h += 80
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(img, img.Bounds(), bgColor)
	fill(img, image.Rect(0, 0, w, 150), headerColor)
	text(img, titleFace, color.White, pad, 80, business)
	text(img, smallFace, color.RGBA{0xCC, 0xFB, 0xF1, 0xFF}, pad, 120, "Price list")
	y := priceListHeaderSpace
	for _, r := range rows {
		cardH := 56 + len(r.Plans)*priceListRowHeight + 8
		fill(img, image.Rect(pad, y, w-pad, y+cardH), cardColor)
		fill(img, image.Rect(pad, y, pad+6, y+cardH), headerColor)
		text(img, headFace, textColor, pad+28, y+42, r.Product)
		ry := y + 56
		for i, p := range r.Plans {
			if i > 0 {
				fill(img, image.Rect(pad+28, ry, w-pad-28, ry+1), lineColor)
			}
			text(img, bodyFace, mutedColor, pad+28, ry+32, p.Name)
			text(img, bodyFace, textColor, w-pad-28-textWidth(bodyFace, p.Price), ry+32, p.Price)
			ry += priceListRowHeight
		}
		y += cardH + 24
	}
	text(img, smallFace, mutedColor, pad, h-36, footer)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func takaLabel(minor int64, currency string) string {
	major, frac := minor/100, minor%100
	s := groupDigits(fmt.Sprint(major))
	if frac != 0 {
		s += fmt.Sprintf(".%02d", frac)
	}
	if currency == "" || currency == "BDT" {
		return "Tk " + s
	}
	return s + " " + currency
}

func groupDigits(s string) string {
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func (s *service) GeneratePriceList(ctx context.Context, tenantID uuid.UUID, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "png"
	}
	if format != "png" {
		return "", apperr.ValidationField("format", "only png is supported")
	}
	t, err := s.d.Tenants.Get(ctx, tenantID)
	if err != nil {
		return "", err
	}
	active := true
	prods, err := s.d.Catalog.List(ctx, tenantID, catalog.Filter{IsActive: &active}, 100)
	if err != nil {
		return "", err
	}
	ids := make([]uuid.UUID, len(prods))
	for i, p := range prods {
		ids[i] = p.ID
	}
	plans, err := s.d.Catalog.PlansByProducts(ctx, tenantID, ids)
	if err != nil {
		return "", err
	}
	var rows []PriceRow
	for _, p := range prods {
		r := PriceRow{Product: p.Name}
		for _, pl := range plans[p.ID] {
			if pl.IsActive {
				r.Plans = append(r.Plans, PricePlan{Name: pl.Name, Price: takaLabel(pl.Price, pl.Currency)})
			}
		}
		if len(r.Plans) > 0 {
			rows = append(rows, r)
		}
	}
	pngBytes, err := RenderPriceList(t.Name, rows, "Updated "+s.now().Format("02 Jan 2006")+" · Order via our page inbox")
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.d.UploadDir, tenantID.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("pricelist-%d.png", s.now().Unix())
	if err := os.WriteFile(filepath.Join(dir, name), pngBytes, 0o644); err != nil {
		return "", err
	}
	return strings.TrimRight(s.d.PublicAPIURL, "/") + "/uploads/" + tenantID.String() + "/" + name, nil
}
